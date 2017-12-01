package perf

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/capsule8/capsule8/pkg/config"
	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/golang/glog"

	"golang.org/x/sys/unix"
)

type eventMonitorOptions struct {
	flags              uintptr
	defaultEventAttr   *EventAttr
	perfEventDir       string
	ringBufferNumPages int
	cgroups            []string
	pids               []int
}

type EventMonitorOption func(*eventMonitorOptions)

func WithFlags(flags uintptr) EventMonitorOption {
	return func(o *eventMonitorOptions) {
		o.flags = flags
	}
}

func WithDefaultEventAttr(defaultEventAttr *EventAttr) EventMonitorOption {
	return func(o *eventMonitorOptions) {
		o.defaultEventAttr = defaultEventAttr
	}
}

func WithPerfEventDir(dir string) EventMonitorOption {
	return func(o *eventMonitorOptions) {
		o.perfEventDir = dir
	}
}

func WithRingBufferNumPages(numPages int) EventMonitorOption {
	return func(o *eventMonitorOptions) {
		o.ringBufferNumPages = numPages
	}
}

func WithCgroup(cgroup string) EventMonitorOption {
	return func(o *eventMonitorOptions) {
		o.cgroups = append(o.cgroups, cgroup)
	}
}

func WithCgroups(cgroups []string) EventMonitorOption {
	return func(o *eventMonitorOptions) {
		o.cgroups = append(o.cgroups, cgroups...)
	}
}

func WithPid(pid int) EventMonitorOption {
	return func(o *eventMonitorOptions) {
		o.pids = append(o.pids, pid)
	}
}

func WithPids(pids []int) EventMonitorOption {
	return func(o *eventMonitorOptions) {
		o.pids = append(o.pids, pids...)
	}
}

type registerEventOptions struct {
	disabled  bool
	eventAttr *EventAttr
	filter    string
}

type RegisterEventOption func(*registerEventOptions)

func WithEventsDisabled() RegisterEventOption {
	return func(o *registerEventOptions) {
		o.disabled = true
	}
}

func WithEventsEnabled() RegisterEventOption {
	return func(o *registerEventOptions) {
		o.disabled = false
	}
}

func WithEventAttr(eventAttr *EventAttr) RegisterEventOption {
	return func(o *registerEventOptions) {
		o.eventAttr = eventAttr
	}
}

func WithFilter(filter string) RegisterEventOption {
	return func(o *registerEventOptions) {
		o.filter = filter
	}
}

const (
	eventTypeTracepoint int = iota
	eventTypeKprobe
)

type registeredEvent struct {
	name      string
	fds       []int
	eventType int
}

type perfEventGroup struct {
	rb         *ringBuffer
	timeBase   uint64
	timeOffset uint64
	pid        int // passed as 'pid' argument to perf_event_open()
	cpu        int // passed as 'cpu' argument to perf_event_open()
	fd         int // fd returned from perf_event_open()
	flags      uintptr
}

func (group *perfEventGroup) cleanup() {
	if group.rb != nil {
		group.rb.unmap()
	}
	unix.Close(group.fd)
	if group.flags&PERF_FLAG_PID_CGROUP == PERF_FLAG_PID_CGROUP {
		unix.Close(group.pid)
	}
}

type SampleDispatchFn func(uint64, interface{}, error)

type EventMonitor struct {
	// Ordering of fields is intentional to keep the most frequently used
	// fields together at the head of the struct in an effort to increase
	// cache locality

	// Immutable items. No protection required. These fields are all set
	// when the EventMonitor is created and never changed after that.
	groups       map[int]perfEventGroup // fd : group data
	dispatchChan chan decodedSampleList

	// Mutable by various goroutines, and also needed by the monitor
	// goroutine. All of these are thread-safe mutable without a lock.
	// The monitor goroutine only ever reads from them, so there's no lock
	// taken. The thread-safe mutation of .decoders is handled elsewhere.
	// The other safe maps will lock if the monitor goroutine is running;
	// otherwise, .lock protects in-place writes.
	eventAttrMap *safeEventAttrMap // stream id : event attr
	eventIDMap   *safeUInt64Map    // stream id : event id
	decoders     *TraceEventDecoderMap

	// Mutable only by the monitor goroutine while running. No protection
	// required.
	samples        decodedSampleList // Used while reading from ringbuffers
	pendingSamples decodedSampleList

	// Immutable once set. Only used by the .dispatchSamples() goroutine.
	// Load once there and cache locally to avoid cache misses on this
	// struct.
	dispatchFn SampleDispatchFn

	// This lock protects everything mutable below this point.
	lock *sync.Mutex

	// Mutable only by the monitor goroutine, but readable by others
	isRunning bool
	pipe      [2]int

	// Mutable by various goroutines, but not required by the monitor goroutine
	nextEventID uint64
	events      map[uint64]registeredEvent // event id : event
	eventfds    map[int]int                // fd : cpu index
	eventids    map[int]uint64             // fd : stream id

	// Immutable, used only when adding new tracepoints/probes
	defaultAttr EventAttr

	// Used only once during shutdown
	cond *sync.Cond
	wg   sync.WaitGroup
}

func fixupEventAttr(eventAttr *EventAttr) {
	// Adjust certain fields in eventAttr that must be set a certain way
	eventAttr.Type = PERF_TYPE_TRACEPOINT
	eventAttr.Size = sizeofPerfEventAttr
	eventAttr.SamplePeriod = 1 // SampleFreq not used
	eventAttr.SampleType |= PERF_SAMPLE_STREAM_ID | PERF_SAMPLE_IDENTIFIER | PERF_SAMPLE_TIME

	eventAttr.Disabled = true
	eventAttr.Pinned = false
	eventAttr.Freq = false
	eventAttr.Watermark = true
	eventAttr.UseClockID = false
	eventAttr.WakeupWatermark = 1 // WakeupEvents not used
}

func (monitor *EventMonitor) perfEventOpen(eventAttr *EventAttr, filter string) ([]int, error) {
	glog.V(2).Infof("Opening perf event: %d %s", eventAttr.Config, filter)

	newfds := make([]int, 0, len(monitor.groups))
	for groupfd, group := range monitor.groups {
		flags := group.flags | PERF_FLAG_FD_OUTPUT | PERF_FLAG_FD_NO_GROUP
		fd, err := open(eventAttr, group.pid, group.cpu, groupfd, flags)
		if err != nil {
			for j := len(newfds) - 1; j >= 0; j-- {
				unix.Close(newfds[j])
			}
			return nil, err
		}
		newfds = append(newfds, fd)

		if len(filter) > 0 {
			err := setFilter(fd, filter)
			if err != nil {
				for j := len(newfds) - 1; j >= 0; j-- {
					unix.Close(newfds[j])
				}
				return nil, err
			}
		}
	}

	return newfds, nil
}

// This should be called with monitor.lock held.
func (monitor *EventMonitor) newRegisteredEvent(name string, fn TraceEventDecoderFn, opts registerEventOptions, eventType int) (uint64, error) {
	id, err := monitor.decoders.AddDecoder(name, fn)
	if err != nil {
		return 0, err
	}

	var attr EventAttr

	if opts.eventAttr == nil {
		attr = monitor.defaultAttr
	} else {
		attr = *opts.eventAttr
		fixupEventAttr(&attr)
	}
	attr.Config = uint64(id)
	attr.Disabled = opts.disabled

	newfds, err := monitor.perfEventOpen(&attr, opts.filter)
	if err != nil {
		monitor.decoders.RemoveDecoder(name)
		return 0, err
	}

	// Choose the eventid for this event now, but don't commit to it until
	// later when no error has occurred in registering the event.
	eventid := monitor.nextEventID

	eventAttrMap := newEventAttrMap()
	eventIDMap := newUInt64Map()
	for _, fd := range newfds {
		streamid, err := unix.IoctlGetInt(fd, PERF_EVENT_IOC_ID)
		if err != nil {
			for _, fd := range newfds {
				unix.Close(fd)
				delete(monitor.eventids, fd)
			}
			monitor.decoders.RemoveDecoder(name)
			return 0, err
		}
		eventAttrMap[uint64(streamid)] = &attr
		eventIDMap[uint64(streamid)] = eventid
		monitor.eventids[fd] = uint64(streamid)
	}

	// Now is the time to commit to the eventid
	monitor.nextEventID++
	if monitor.isRunning {
		monitor.eventAttrMap.update(eventAttrMap)
		monitor.eventIDMap.update(eventIDMap)
	} else {
		monitor.eventAttrMap.updateInPlace(eventAttrMap)
		monitor.eventIDMap.updateInPlace(eventIDMap)
	}
	for i, fd := range newfds {
		monitor.eventfds[fd] = i
	}

	event := registeredEvent{
		name:      name,
		fds:       newfds,
		eventType: eventType,
	}

	monitor.events[eventid] = event
	return eventid, nil
}

func (monitor *EventMonitor) RegisterTracepoint(name string,
	fn TraceEventDecoderFn, options ...RegisterEventOption) (uint64, error) {

	// Process options
	opts := registerEventOptions{}
	for _, option := range options {
		option(&opts)
	}

	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	eventid, err := monitor.newRegisteredEvent(name, fn, opts, eventTypeTracepoint)
	if err != nil {
		return 0, err
	}

	return eventid, nil
}

func (monitor *EventMonitor) RegisterKprobe(address string, onReturn bool, output string,
	fn TraceEventDecoderFn, options ...RegisterEventOption) (uint64, error) {

	// Process options
	opts := registerEventOptions{}
	for _, option := range options {
		option(&opts)
	}

	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	// Choose a name to refer to the kprobe by. We could allow the kernel
	// to assign a name, but getting the right name back from the kernel
	// can be somewhat unreliable. Choosing our own unique name ensures
	// that we're always dealing with the right event and that we're not
	// stomping on some other process's probe
	ts := unix.Timespec{}
	unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)
	name := fmt.Sprintf("capsule8/sensor_%d_%d", unix.Getpid(), ts.Nano())

	name, err := AddKprobe(name, address, onReturn, output)
	if err != nil {
		return 0, err
	}

	eventid, err := monitor.newRegisteredEvent(name, fn, opts, eventTypeKprobe)
	if err != nil {
		RemoveKprobe(name)
		return 0, err
	}

	return eventid, nil
}

// This should be called with monitor.lock held
func (monitor *EventMonitor) removeRegisteredEvent(event registeredEvent) {
	ids := make([]uint64, 0, len(event.fds))
	for _, fd := range event.fds {
		delete(monitor.eventfds, fd)
		id, ok := monitor.eventids[fd]
		if ok {
			ids = append(ids, id)
			delete(monitor.eventids, fd)
		}
		unix.Close(fd)
	}

	if monitor.isRunning {
		monitor.eventAttrMap.removeInPlace(ids)
		monitor.eventIDMap.removeInPlace(ids)
	} else {
		monitor.eventAttrMap.remove(ids)
		monitor.eventIDMap.remove(ids)
	}

	switch event.eventType {
	case eventTypeTracepoint:
		break
	case eventTypeKprobe:
		RemoveKprobe(event.name)
	}

	monitor.decoders.RemoveDecoder(event.name)
}

func (monitor *EventMonitor) UnregisterEvent(eventid uint64) error {
	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	event, ok := monitor.events[eventid]
	if !ok {
		return errors.New("event is not registered")
	}
	delete(monitor.events, eventid)
	monitor.removeRegisteredEvent(event)

	return nil
}

func (monitor *EventMonitor) Close(wait bool) error {
	// if the monitor is running, stop it and wait for it to stop
	monitor.Stop(wait)

	// This lock isn't strictly necessary -- by the time .Close() is
	// called, it would be a programming error for multiple go routines
	// to be trying to close the monitor or update events. It doesn't
	// hurt to lock, so do it anyway just to be on the safe side.
	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	for _, event := range monitor.events {
		monitor.removeRegisteredEvent(event)
	}
	monitor.events = nil

	if len(monitor.eventfds) != 0 {
		panic("internal error: stray event fds left after monitor Close")
	}
	monitor.eventfds = nil

	if len(monitor.eventids) != 0 {
		panic("internal error: stray event ids left after monitor Close")
	}
	monitor.eventids = nil

	if len(monitor.eventAttrMap.getMap()) != 0 {
		panic("internal error: stray event attrs left after monitor Close")
	}
	monitor.eventAttrMap = nil

	if len(monitor.eventIDMap.getMap()) != 0 {
		panic("internal error: stray event IDs left after monitor Close")
	}
	monitor.eventIDMap = nil

	for _, group := range monitor.groups {
		group.cleanup()
	}
	monitor.groups = nil

	return nil
}

func (monitor *EventMonitor) Disable(eventid uint64) {
	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	event, ok := monitor.events[eventid]
	if ok {
		for _, fd := range event.fds {
			disable(fd)
		}
	}
}

func (monitor *EventMonitor) DisableAll() {
	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	// Group FDs are always disabled
	for fd := range monitor.eventfds {
		disable(fd)
	}
}

func (monitor *EventMonitor) Enable(eventid uint64) {
	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	event, ok := monitor.events[eventid]
	if ok {
		for _, fd := range event.fds {
			enable(fd)
		}
	}
}

func (monitor *EventMonitor) EnableAll() {
	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	// Group FDs are always disabled
	for fd := range monitor.eventfds {
		enable(fd)
	}
}

func (monitor *EventMonitor) SetFilter(eventid uint64, filter string) error {
	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	event, ok := monitor.events[eventid]
	if ok {
		for _, fd := range event.fds {
			err := setFilter(fd, filter)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (monitor *EventMonitor) stopWithSignal() {
	monitor.lock.Lock()
	monitor.isRunning = false
	monitor.cond.Broadcast()
	monitor.lock.Unlock()
}

type decodedSample struct {
	sample Sample
	err    error
}

type decodedSampleList []decodedSample

func (ds decodedSampleList) Len() int {
	return len(ds)
}

func (ds decodedSampleList) Swap(i, j int) {
	ds[i], ds[j] = ds[j], ds[i]
}

func (ds decodedSampleList) Less(i, j int) bool {
	return ds[i].sample.Time < ds[j].sample.Time
}

func (monitor *EventMonitor) dispatchSortedSamples(samples decodedSampleList) {
	dispatchFn := monitor.dispatchFn
	eventIDMap := monitor.eventIDMap.getMap()
	for _, ds := range samples {
		streamID := ds.sample.SampleID.StreamID
		eventID, ok := eventIDMap[streamID]
		if !ok {
			// Refresh the map in case we're dealing with a newly
			// added event, but more likely we're here because the
			// event has been removed while we're still processing
			// samples from the ringbuffer.
			eventIDMap = monitor.eventIDMap.getMap()
			eventID, ok = eventIDMap[streamID]
			if !ok {
				continue
			}
		}

		switch record := ds.sample.Record.(type) {
		case *SampleRecord:
			// Adjust the sample time so that it
			// matches the normalized timestamp.
			record.Time = ds.sample.Time
			s, err := monitor.decoders.DecodeSample(record)
			dispatchFn(eventID, s, err)
		default:
			dispatchFn(eventID, &ds.sample, ds.err)
		}
	}
}

func (monitor *EventMonitor) dispatchSamples() {
	defer monitor.wg.Done()

	for monitor.isRunning {
		select {
		case samples, ok := <-monitor.dispatchChan:
			if !ok {
				// Channel is closed; stop dispatch
				monitor.dispatchChan = nil

				// Signal completion of dispatch via WaitGroup
				return
			}

			// Sort the samples read from the ringbuffers
			sort.Sort(samples)

			// Dispatch the sorted samples
			monitor.dispatchSortedSamples(samples)
		}
	}
}

func (monitor *EventMonitor) readSamples(data []byte) {
	reader := bytes.NewReader(data)
	for reader.Len() > 0 {
		ds := decodedSample{}
		ds.err = ds.sample.read(reader, nil, monitor.eventAttrMap.getMap())
		monitor.samples = append(monitor.samples, ds)
	}
}

func (monitor *EventMonitor) readRingBuffers(readyfds []int) {
	var (
		lastTimestamp uint64
		lastIndex     int
	)

	// Group fds are created with a read_format of 0, which means that the
	// data read from each fd will always be 8 bytes. We don't care about
	// the data, so we can safely ignore it. Due to the way that the
	// interface works, we don't have to read it as long as we empty the
	// associated ring buffer, which we will.

	for _, fd := range readyfds {
		// Read the samples from the ring buffer, and then normalize
		// the timestamps to be consistent across CPUs.
		first := len(monitor.samples)
		group := monitor.groups[fd]
		group.rb.read(monitor.readSamples)
		for i := first; i < len(monitor.samples); i++ {
			monitor.samples[i].sample.Time = group.timeBase +
				(monitor.samples[i].sample.Time - group.timeOffset)
		}

		if first == 0 {
			// Sometimes readyfds get no samples from the ringbuffer
			if len(monitor.samples) > 0 {
				lastTimestamp = monitor.samples[len(monitor.samples)-1].sample.Time
			}
		} else {
			x := len(monitor.samples)
			for i := x - 1; i > lastIndex; i-- {
				if monitor.samples[i].sample.Time > lastTimestamp {
					x--
				}
			}
			if x != len(monitor.samples) {
				monitor.pendingSamples = append(monitor.pendingSamples, monitor.samples[x-1:]...)
				monitor.samples = monitor.samples[:x]
			}
		}
		lastIndex = len(monitor.samples)
	}

	if len(monitor.samples) > 0 {
		if len(monitor.pendingSamples) > 0 {
			monitor.samples = append(monitor.samples, monitor.pendingSamples...)
			monitor.pendingSamples = monitor.pendingSamples[:0]
		}
		monitor.dispatchChan <- monitor.samples
		monitor.samples = nil
	}
}

func (monitor *EventMonitor) flushPendingSamples() {
	if len(monitor.pendingSamples) > 0 {
		monitor.dispatchChan <- monitor.pendingSamples
		monitor.pendingSamples = nil
	}
}

func addPollFd(pollfds []unix.PollFd, fd int) []unix.PollFd {
	pollfd := unix.PollFd{
		Fd:     int32(fd),
		Events: unix.POLLIN,
	}
	return append(pollfds, pollfd)
}

func (monitor *EventMonitor) Run(fn SampleDispatchFn) error {
	monitor.lock.Lock()
	if monitor.isRunning {
		monitor.lock.Unlock()
		return errors.New("monitor is already running")
	}
	monitor.isRunning = true

	err := unix.Pipe2(monitor.pipe[:], unix.O_DIRECT|unix.O_NONBLOCK)
	monitor.lock.Unlock()
	if err != nil {
		monitor.stopWithSignal()
		return err
	}

	monitor.dispatchChan = make(chan decodedSampleList,
		config.Sensor.ChannelBufferLength)
	monitor.dispatchFn = fn

	monitor.wg.Add(1)
	go monitor.dispatchSamples()

	// Set up the fds for polling. Monitor only the groupfds, because they
	// will encapsulate all of the eventfds and they're tied to the ring
	// buffers.
	pollfds := make([]unix.PollFd, 0, len(monitor.groups)+1)
	pollfds = addPollFd(pollfds, monitor.pipe[0])
	for fd := range monitor.groups {
		pollfds = addPollFd(pollfds, fd)
	}

runloop:
	for {
		var n int

		// If there are pending samples, check for waiting events and
		// return immediately. If there aren't any, flush everything
		// pending and go back to waiting for events normally. If there
		// are events, the pending events will get handled during
		// normal processing
		if len(monitor.pendingSamples) > 0 {
			n, err = unix.Poll(pollfds, 0)
			if err != nil && err != unix.EINTR {
				break
			}
			if n == 0 {
				monitor.flushPendingSamples()
				continue
			}
		} else {
			n, err = unix.Poll(pollfds, -1)
			if err != nil && err != unix.EINTR {
				break
			}
		}

		if n > 0 {
			readyfds := make([]int, 0, n)
			for i, fd := range pollfds {
				if i == 0 {
					if (fd.Revents & ^unix.POLLIN) != 0 {
						// POLLERR, POLLHUP, or POLLNVAL set
						break runloop
					}
				} else if (fd.Revents & unix.POLLIN) != 0 {
					readyfds = append(readyfds, int(fd.Fd))
				}
			}
			if len(readyfds) > 0 {
				monitor.readRingBuffers(readyfds)
			} else if len(monitor.pendingSamples) > 0 {
				monitor.flushPendingSamples()
			}
		}
	}

	if monitor.dispatchChan != nil {
		close(monitor.dispatchChan)
	}

	monitor.lock.Lock()
	if monitor.pipe[1] != -1 {
		unix.Close(monitor.pipe[1])
		monitor.pipe[1] = -1
	}
	if monitor.pipe[0] != -1 {
		unix.Close(monitor.pipe[0])
		monitor.pipe[0] = -1
	}
	monitor.lock.Unlock()

	if monitor.dispatchChan != nil {
		// Wait for dispatchSamples goroutine to exit
		monitor.wg.Wait()
	}

	monitor.stopWithSignal()
	return err
}

func (monitor *EventMonitor) Stop(wait bool) {
	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	if !monitor.isRunning {
		return
	}

	if monitor.pipe[1] != -1 {
		fd := monitor.pipe[1]
		monitor.pipe[1] = -1
		unix.Close(fd)
	}

	if wait {
		for monitor.isRunning {
			// Wait for condition to signal that Run() is done
			monitor.cond.Wait()
		}
	}
}

var referenceEventAttr = EventAttr{
	Type:         PERF_TYPE_SOFTWARE,
	Size:         sizeofPerfEventAttr,
	Config:       PERF_COUNT_SW_CONTEXT_SWITCHES,
	SampleFreq:   1,
	SampleType:   PERF_SAMPLE_TIME,
	Freq:         true,
	WakeupEvents: 1,
}

func (monitor *EventMonitor) readReferenceSamples(data []byte) {
	reader := bytes.NewReader(data)
	for reader.Len() > 0 {
		ds := decodedSample{}
		ds.err = ds.sample.read(reader, &referenceEventAttr, nil)
		monitor.samples = append(monitor.samples, ds)
	}
}

func (monitor *EventMonitor) cpuTimeOffset(cpu int, groupfd int, rb *ringBuffer) (uint64, error) {
	// Create a temporary event to get a reference timestamp. What
	// type of event we use is unimportant. We just want something
	// that will be reported immediately. After the timestamp is
	// retrieved, we can get rid of it.
	fd, err := open(&referenceEventAttr, -1, cpu, groupfd,
		(PERF_FLAG_FD_OUTPUT | PERF_FLAG_FD_NO_GROUP))
	if err != nil {
		glog.V(3).Infof("Couldn't open reference event: %s", err)
		return 0, err
	}

	// Wait for the event to be ready, then pull it immediately and
	// remember the timestamp. That's our reference for this CPU.
	pollfds := []unix.PollFd{
		unix.PollFd{
			Fd:     int32(groupfd),
			Events: unix.POLLIN,
		},
	}
	for {
		n, err := unix.Poll(pollfds, -1)
		if err != nil && err != unix.EINTR {
			unix.Close(fd)
			return 0, err
		}
		if n == 0 {
			continue
		}

		rb.read(monitor.readReferenceSamples)
		if len(monitor.samples) == 0 {
			continue
		}
		ds := monitor.samples[0]

		// Close the event to prevent any more samples from being
		// added to the ring buffer. Remove anything from the ring
		// buffer that remains and discard it.
		unix.Close(fd)
		rb.read(monitor.readReferenceSamples)
		monitor.samples = nil

		if ds.err != nil {
			return 0, err
		}

		return ds.sample.Time - rb.timeRunning(), nil
	}
}

func (monitor *EventMonitor) initializeGroupLeaders(pid int, flags uintptr, ringBufferNumPages int) error {
	groupEventAttr := &EventAttr{
		Type:            PERF_TYPE_SOFTWARE,
		Size:            sizeofPerfEventAttr,
		Config:          PERF_COUNT_SW_DUMMY, // Added in Linux 3.12
		Disabled:        true,
		Watermark:       true,
		WakeupWatermark: 1,
	}

	ncpu := runtime.NumCPU()
	newfds := make(map[int]perfEventGroup, ncpu)

	for cpu := 0; cpu < ncpu; cpu++ {
		groupfd, err := open(groupEventAttr, pid, cpu, -1, flags)
		if err != nil {
			for fd := range newfds {
				unix.Close(fd)
			}
			return err
		}

		newGroup := perfEventGroup{
			pid:   pid,
			cpu:   cpu,
			fd:    groupfd,
			flags: flags,
		}

		rb, err := newRingBuffer(groupfd, ringBufferNumPages)
		if err == nil {
			var offset uint64

			newGroup.rb = rb

			offset, err = monitor.cpuTimeOffset(cpu, groupfd, rb)
			if err == nil {
				newGroup.timeBase = uint64(time.Now().UnixNano())
				newGroup.timeOffset = offset
				newfds[groupfd] = newGroup
				continue
			}
		}

		// This is the common error case
		newGroup.cleanup()
		for _, group := range newfds {
			group.cleanup()
		}
		return err
	}

	for k, v := range newfds {
		monitor.groups[k] = v
	}

	return nil
}

func NewEventMonitor(options ...EventMonitorOption) (*EventMonitor, error) {
	// Process options
	opts := eventMonitorOptions{}
	for _, option := range options {
		option(&opts)
	}

	var eventAttr EventAttr

	if opts.defaultEventAttr == nil {
		eventAttr = EventAttr{
			SampleType:  PERF_SAMPLE_TID | PERF_SAMPLE_TIME | PERF_SAMPLE_CPU | PERF_SAMPLE_RAW,
			Inherit:     true,
			SampleIDAll: true,
		}
	} else {
		eventAttr = *opts.defaultEventAttr
	}
	fixupEventAttr(&eventAttr)

	// Only allow certain flags to be passed
	opts.flags &= PERF_FLAG_FD_CLOEXEC

	// If no perf_event cgroup mountpoint was specified, scan mounts for one
	if len(opts.perfEventDir) == 0 && len(opts.cgroups) > 0 {
		opts.perfEventDir = sys.PerfEventDir()

		// If we didn't find one, we can't monitor specific cgroups
		if len(opts.perfEventDir) == 0 {
			return nil, errors.New("Can't monitor specific cgroups without perf_event cgroupfs")
		}
	}

	// If no pids or cgroups were specified, default to monitoring the
	// whole system (pid -1)
	if len(opts.pids) == 0 && len(opts.cgroups) == 0 {
		opts.pids = append(opts.pids, -1)
	}

	monitor := &EventMonitor{
		groups:       make(map[int]perfEventGroup),
		eventAttrMap: newSafeEventAttrMap(),
		eventIDMap:   newSafeUInt64Map(),
		decoders:     NewTraceEventDecoderMap(),
		nextEventID:  1,
		events:       make(map[uint64]registeredEvent),
		eventfds:     make(map[int]int),
		eventids:     make(map[int]uint64),
		defaultAttr:  eventAttr,
	}
	monitor.lock = &sync.Mutex{}
	monitor.cond = sync.NewCond(monitor.lock)

	cgroups := make(map[string]bool, len(opts.cgroups))
	for _, cgroup := range opts.cgroups {
		if cgroups[cgroup] {
			glog.V(1).Infof("Ignoring duplicate cgroup %s", cgroup)
			continue
		}
		cgroups[cgroup] = true

		path := filepath.Join(opts.perfEventDir, cgroup)
		fd, err := unix.Open(path, unix.O_RDONLY, 0)
		if err == nil {
			err = monitor.initializeGroupLeaders(fd,
				opts.flags|PERF_FLAG_PID_CGROUP,
				opts.ringBufferNumPages)
			if err == nil {
				continue
			}
		}

		for _, group := range monitor.groups {
			group.cleanup()
		}
		return nil, err
	}

	pids := make(map[int]bool, len(opts.pids))
	for _, pid := range opts.pids {
		if pids[pid] {
			glog.V(1).Infof("Ignoring duplicate pid %d", pid)
			continue
		}
		pids[pid] = true

		err := monitor.initializeGroupLeaders(pid, opts.flags,
			opts.ringBufferNumPages)
		if err == nil {
			continue
		}

		for _, group := range monitor.groups {
			group.cleanup()
		}
		return nil, err
	}

	return monitor, nil
}
