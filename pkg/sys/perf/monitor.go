package perf

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/golang/glog"

	"golang.org/x/sys/unix"
)

const (
	EVENT_TYPE_TRACEPOINT int = iota
	EVENT_TYPE_KPROBE
)

type registeredEvent struct {
	name      string
	filter    string
	decoderfn TraceEventDecoderFn
	fds       []int
	eventAttr *EventAttr
	eventType int
}

type SampleDispatchFn func(interface{}, error)

type EventMonitor struct {
	// Ordering of fields is intentional to keep the most frequently used
	// fields together at the head of the struct in an effort to increase
	// cache locality

	// Immutable items. No protection required. These fields are all set
	// when the EventMonitor is created and never changed after that.
	ringBuffers  map[int]*ringBuffer
	dispatchChan chan decodedSampleList
	baseTime     uint64
	timeOffsets  map[int]uint64 // group fd : time offset

	// Mutable by various goroutines, and also needed by the monitor
	// goroutine. Both of these are thread-safe mutable without a lock.
	// The monitor goroutine only ever reads from them, so there's no lock
	// taken. .eventAttrs Writers will lock if the monitor goroutine is
	// running; otherwise, .lock protects in-place writes. The thread-safe
	// mutation of .decoders is handled elsewhere.
	eventAttrs *safeEventAttrMap     // stream id : event attr
	decoders   *TraceEventDecoderMap

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
	events   map[string]*registeredEvent
	eventfds map[int]int    // fd : cpu index
	eventIDs map[int]uint64 // fd : stream id

	// Immutable, rarely referenced
	pid                int
	flags              uintptr
	ringBufferNumPages int
	defaultAttr        EventAttr
	groupfds           []int
	cgroup             *os.File

	// Used only once during shutdown
	cond *sync.Cond
}

type eventAttrMap map[uint64]*EventAttr

func newEventAttrMap() eventAttrMap {
	return make(map[uint64]*EventAttr)
}

type safeEventAttrMap struct {
	sync.Mutex              // used only by writers
	active     atomic.Value // map[uint64]*EventAttr
}

func newSafeEventAttrMap() *safeEventAttrMap {
	return &safeEventAttrMap{}
}

func (m *safeEventAttrMap) getMap() eventAttrMap {
	value := m.active.Load()
	if value == nil {
		return nil
	}
	return value.(eventAttrMap)
}

func (m *safeEventAttrMap) removeInPlace(ids []uint64) {
	em := m.getMap()
	if em == nil {
		return
	}

	for _, id := range ids {
		delete(em, id)
	}
}

func (m *safeEventAttrMap) remove(ids []uint64) {
	m.Lock()
	defer m.Unlock()

	oem := m.getMap()
	nem := newEventAttrMap()
	if oem != nil {
		for k, v := range oem {
			nem[k] = v
		}
	}
	for _, id := range ids {
		delete(nem, id)
	}

	m.active.Store(nem)
}

func (m *safeEventAttrMap) updateInPlace(emfrom eventAttrMap) {
	em := m.getMap()
	if em == nil {
		em = newEventAttrMap()
		m.active.Store(em)
	}

	for k, v := range emfrom {
		em[k] = v
	}
}

func (m *safeEventAttrMap) update(emfrom eventAttrMap) {
	m.Lock()
	defer m.Unlock()

	oem := m.getMap()
	nem := newEventAttrMap()
	if oem != nil {
		for k, v := range oem {
			nem[k] = v
		}
	}
	for k, v := range emfrom {
		nem[k] = v
	}

	m.active.Store(nem)
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

func perfEventOpen(eventAttr *EventAttr, pid int, groupfds []int, flags uintptr, filter string) ([]int, error) {
	glog.V(2).Infof("Opening perf event: %d %s", eventAttr.Config, filter)
	ncpu := runtime.NumCPU()
	newfds := make([]int, ncpu)

	for i := 0; i < ncpu; i++ {
		fd, err := open(eventAttr, pid, i, groupfds[i], flags)
		if err != nil {
			for j := i - 1; j >= 0; j-- {
				unix.Close(newfds[j])
			}
			return nil, err
		}
		newfds[i] = fd

		if len(filter) > 0 {
			err := setFilter(fd, filter)
			if err != nil {
				for j := i; j >= 0; j-- {
					unix.Close(newfds[j])
				}
				return nil, err
			}
		}
	}

	return newfds, nil
}

// This should be called with monitor.lock held.
func (monitor *EventMonitor) newRegisteredEvent(name string, fn TraceEventDecoderFn, filter string, eventAttr *EventAttr, eventType int) (*registeredEvent, error) {
	id, err := monitor.decoders.AddDecoder(name, fn)
	if err != nil {
		return nil, err
	}

	var attr EventAttr

	if eventAttr == nil {
		attr = monitor.defaultAttr
	} else {
		attr = *eventAttr
		fixupEventAttr(&attr)
	}
	attr.Config = uint64(id)

	flags := monitor.flags | PERF_FLAG_FD_OUTPUT | PERF_FLAG_FD_NO_GROUP
	newfds, err := perfEventOpen(&attr, monitor.pid, monitor.groupfds, flags, filter)
	if err != nil {
		monitor.decoders.RemoveDecoder(name)
		return nil, err
	}

	eventAttrs := newEventAttrMap()
	for _, fd := range newfds {
		id, err := unix.IoctlGetInt(fd, PERF_EVENT_IOC_ID)
		if err != nil {
			for _, fd := range newfds {
				unix.Close(fd)
				delete(monitor.eventIDs, fd)
			}
			monitor.decoders.RemoveDecoder(name)
			return nil, err
		}
		eventAttrs[uint64(id)] = &attr
		monitor.eventIDs[fd] = uint64(id)
	}

	if monitor.isRunning {
		monitor.eventAttrs.update(eventAttrs)
	} else {
		monitor.eventAttrs.updateInPlace(eventAttrs)
	}
	for i, fd := range newfds {
		monitor.eventfds[fd] = i
	}

	event := &registeredEvent{
		name:      name,
		filter:    filter,
		decoderfn: fn,
		fds:       newfds,
		eventAttr: &attr,
		eventType: eventType,
	}

	return event, nil
}

func (monitor *EventMonitor) RegisterEvent(name string, fn TraceEventDecoderFn, filter string, eventAttr *EventAttr) error {
	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	key := fmt.Sprintf("%s %s", name, filter)
	_, ok := monitor.events[key]
	if ok {
		return fmt.Errorf("event \"%s\" is already registered", name)
	}

	event, err := monitor.newRegisteredEvent(name, fn, filter, eventAttr, EVENT_TYPE_TRACEPOINT)
	if err != nil {
		return err
	}

	monitor.events[key] = event
	return nil
}

func (monitor *EventMonitor) RegisterKprobe(name string, address string, onReturn bool, output string,
	fn TraceEventDecoderFn, filter string, eventAttr *EventAttr) (string, error) {

	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	key := fmt.Sprintf("%s %s", name, filter)
	_, ok := monitor.events[key]
	if ok {
		return "", fmt.Errorf("event \"%s\" is already registered", name)
	}

	name, err := AddKprobe(name, address, onReturn, output)
	if err != nil {
		return "", err
	}

	event, err := monitor.newRegisteredEvent(name, fn, filter, eventAttr, EVENT_TYPE_KPROBE)
	if err != nil {
		RemoveKprobe(name)
		return "", err
	}

	monitor.events[key] = event
	return name, nil
}

// This should be called with monitor.lock held
func (monitor *EventMonitor) removeRegisteredEvent(event *registeredEvent) {
	ids := make([]uint64, 0, len(event.fds))
	for _, fd := range event.fds {
		delete(monitor.eventfds, fd)
		id, ok := monitor.eventIDs[fd]
		if ok {
			ids = append(ids, id)
			delete(monitor.eventIDs, fd)
		}
		unix.Close(fd)
	}

	if monitor.isRunning {
		monitor.eventAttrs.removeInPlace(ids)
	} else {
		monitor.eventAttrs.remove(ids)
	}

	switch event.eventType {
	case EVENT_TYPE_TRACEPOINT:
		break
	case EVENT_TYPE_KPROBE:
		RemoveKprobe(event.name)
	}

	monitor.decoders.RemoveDecoder(event.name)
}

func (monitor *EventMonitor) UnregisterEvent(name string, filter string) error {
	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	key := fmt.Sprintf("%s %s", name, filter)
	event, ok := monitor.events[key]
	if !ok {
		return errors.New("event is not registered")
	}
	delete(monitor.events, key)
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

	if len(monitor.eventIDs) != 0 {
		panic("internal error: stray event ids left after monitor Close")
	}
	monitor.eventIDs = nil

	if len(monitor.eventAttrs.getMap()) != 0 {
		panic("internal error: stray event attrs left after monitor Close")
	}
	monitor.eventAttrs = nil

	for _, rb := range monitor.ringBuffers {
		rb.unmap()
	}
	monitor.ringBuffers = nil

	for _, fd := range monitor.groupfds {
		unix.Close(fd)
	}
	monitor.groupfds = nil

	if monitor.cgroup != nil {
		monitor.cgroup.Close()
		monitor.cgroup = nil
	}

	return nil
}

func (monitor *EventMonitor) Disable() {
	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	// Group FDs are always disabled
	for fd := range monitor.eventfds {
		disable(fd)
	}
}

func (monitor *EventMonitor) Enable() {
	monitor.lock.Lock()
	defer monitor.lock.Unlock()

	// Group FDs are always disabled
	for fd := range monitor.eventfds {
		enable(fd)
	}
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

func (monitor *EventMonitor) dispatchSamples() {
	dispatchFn := monitor.dispatchFn
	for {
		select {
		case samples, ok := <-monitor.dispatchChan:
			if !ok {
				// Channel is closed; stop dispatch
				monitor.dispatchChan = nil
				return
			}

			// First sort the data read from the ringbuffers
			sort.Sort(decodedSampleList(samples))

			for _, ds := range samples {
				switch record := ds.sample.Record.(type) {
				case *SampleRecord:
					// Adjust the sample time so that it
					// matches the normalized timestamp.
					record.Time = ds.sample.Time
					s, err := monitor.decoders.DecodeSample(record)
					dispatchFn(s, err)
				default:
					dispatchFn(&ds.sample, ds.err)
				}
			}
		}
	}
}

func (monitor *EventMonitor) readSamples(data []byte) {
	reader := bytes.NewReader(data)
	for reader.Len() > 0 {
		ds := decodedSample{}
		ds.err = ds.sample.read(reader, nil, monitor.eventAttrs.getMap())
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
		monitor.ringBuffers[fd].read(monitor.readSamples)
		for i := first; i < len(monitor.samples); i++ {
			monitor.samples[i].sample.Time = monitor.baseTime +
				(monitor.samples[i].sample.Time - monitor.timeOffsets[fd])
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

	monitor.dispatchChan = make(chan decodedSampleList, 128)
	monitor.dispatchFn = fn
	go monitor.dispatchSamples()

	// Set up the fds for polling. Monitor only the groupfds, because they
	// will encapsulate all of the eventfds and they're tied to the ring
	// buffers.
	pollfds := make([]unix.PollFd, 0, len(monitor.groupfds)+1)
	pollfds = addPollFd(pollfds, monitor.pipe[0])
	for _, fd := range monitor.groupfds {
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
			monitor.cond.Wait()
		}
	}
}

var referenceEventAttr EventAttr = EventAttr{
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
	defer unix.Close(fd)

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
			return 0, err
		}
		if n == 0 {
			continue
		}

		rb.read(monitor.readReferenceSamples)
		if len(monitor.samples) < 0 {
			continue
		}

		ds := monitor.samples[0]
		monitor.samples = monitor.samples[:0]
		if ds.err != nil {
			return 0, err
		}

		return ds.sample.Time - rb.timeRunning(), nil
	}
}

func (monitor *EventMonitor) initializeGroupLeaders() error {
	groupEventAttr := &EventAttr{
		Type:            PERF_TYPE_SOFTWARE,
		Size:            sizeofPerfEventAttr,
		Config:          PERF_COUNT_SW_DUMMY, // Added in Linux 3.12
		Disabled:        true,
		Watermark:       true,
		WakeupWatermark: 1,
	}

	ncpu := runtime.NumCPU()
	newfds := make([]int, ncpu)
	ringBuffers := make(map[int]*ringBuffer, ncpu)
	monitor.timeOffsets = make(map[int]uint64, ncpu)

	for i := 0; i < ncpu; i++ {
		fd, err := open(groupEventAttr, monitor.pid, i, -1, monitor.flags)
		if err != nil {
			for j := i - 1; j >= 0; j-- {
				unix.Close(newfds[j])
			}
			return err
		}
		newfds[i] = fd

		rb, err := newRingBuffer(newfds[i], monitor.ringBufferNumPages)
		if err == nil {
			var offset uint64

			ringBuffers[fd] = rb

			offset, err = monitor.cpuTimeOffset(i, fd, rb)
			if err == nil {
				monitor.timeOffsets[newfds[i]] = offset
				continue
			}
		}

		// This is the common error case
		unix.Close(newfds[i])
		for j := i - 1; j >= 0; j-- {
			ringBuffers[newfds[j]].unmap()
			unix.Close(newfds[j])
		}
		return err
	}

	monitor.groupfds = newfds
	monitor.ringBuffers = ringBuffers

	return nil
}

func NewEventMonitor(pid int, flags uintptr, ringBufferNumPages int, defaultAttr *EventAttr) (*EventMonitor, error) {
	var eventAttr EventAttr

	if defaultAttr == nil {
		eventAttr = EventAttr{
			SampleType:  PERF_SAMPLE_TID | PERF_SAMPLE_TIME | PERF_SAMPLE_CPU | PERF_SAMPLE_RAW,
			Inherit:     true,
			SampleIDAll: true,
		}
	} else {
		eventAttr = *defaultAttr
	}
	fixupEventAttr(&eventAttr)

	// Only allow certain flags to be passed
	flags &= PERF_FLAG_FD_CLOEXEC | PERF_FLAG_PID_CGROUP

	monitor := &EventMonitor{
		pid:                pid,
		flags:              flags,
		ringBufferNumPages: ringBufferNumPages,
		defaultAttr:        eventAttr,
		events:             make(map[string]*registeredEvent),
		eventfds:           make(map[int]int),
		eventIDs:           make(map[int]uint64),
		decoders:           NewTraceEventDecoderMap(),
		eventAttrs:         newSafeEventAttrMap(),
		baseTime:           uint64(time.Now().UnixNano()),
	}
	monitor.lock = &sync.Mutex{}
	monitor.cond = sync.NewCond(monitor.lock)

	err := monitor.initializeGroupLeaders()
	if err != nil {
		return nil, err
	}

	return monitor, nil
}

func NewEventMonitorWithCgroup(cgroup string, flags uintptr, ringBufferNumPages int, defaultAttr *EventAttr) (*EventMonitor, error) {
	path := filepath.Join(sys.PerfEventDir(), cgroup)
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	monitor, err := NewEventMonitor(int(f.Fd()), flags|PERF_FLAG_PID_CGROUP, ringBufferNumPages, defaultAttr)
	if err != nil {
		f.Close()
		return nil, err
	}
	monitor.cgroup = f

	return monitor, nil
}
