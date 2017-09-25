package perf

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/capsule8/reactive8/pkg/sys"

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

type EventMonitor struct {
	pid                int
	flags              uintptr
	ringBufferNumPages int
	defaultAttr        EventAttr

	lock *sync.Mutex
	cond *sync.Cond

	events      map[string]*registeredEvent
	isRunning   bool
	pipe        [2]int
	groupfds    []int
	eventfds    map[int]int    // fd : cpu index
	eventIDs    map[int]uint64 // fd : stream id
	eventAttrs  map[uint64]*EventAttr
	ringBuffers []*ringBuffer
	decoders    *TraceEventDecoderMap
	cgroup      *os.File

	// Used while reading samples from ringbuffers
	samples []*decodedSample
}

func fixupEventAttr(eventAttr *EventAttr) {
	// Adjust certain fields in eventAttr that must be set a certain way
	eventAttr.Type = PERF_TYPE_TRACEPOINT
	eventAttr.Size = sizeofPerfEventAttr
	eventAttr.SamplePeriod = 1 // SampleFreq not used
	eventAttr.SampleType |= PERF_SAMPLE_STREAM_ID | PERF_SAMPLE_IDENTIFIER

	eventAttr.Disabled = true
	eventAttr.Pinned = false
	eventAttr.Freq = false
	eventAttr.Watermark = true
	eventAttr.WakeupWatermark = 1 // WakeupEvents not used
}

func perfEventOpen(eventAttr *EventAttr, pid int, groupfds []int, flags uintptr, filter string) ([]int, error) {
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

		if len(filter) > 0 {
			err := setFilter(fd, filter)
			if err != nil {
				for j := i; j >= 0; j-- {
					unix.Close(newfds[j])
				}
				return nil, err
			}
		}

		newfds[i] = fd
	}

	return newfds, nil
}

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

	for _, fd := range newfds {
		id, err := unix.IoctlGetInt(fd, PERF_EVENT_IOC_ID)
		if err != nil {
			for _, fd := range newfds {
				unix.Close(fd)
				id, ok := monitor.eventIDs[fd]
				if ok {
					delete(monitor.eventAttrs, id)
					delete(monitor.eventIDs, fd)
				}
			}
			monitor.decoders.RemoveDecoder(name)
			return nil, err
		}
		monitor.eventAttrs[uint64(id)] = &attr
		monitor.eventIDs[fd] = uint64(id)
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
	if monitor.isRunning {
		return errors.New("monitor is already running")
	}

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

	if monitor.isRunning {
		return "", errors.New("monitor is already running")
	}

	key := fmt.Sprintf("%s %s", name, filter)
	_, ok := monitor.events[key]
	if ok {
		return "", fmt.Errorf("event \"%s\" is already registered", name)
	}

	name, err := AddKprobe(name, address, onReturn, output)
	if err != nil {
		return "", nil
	}

	event, err := monitor.newRegisteredEvent(name, fn, filter, eventAttr, EVENT_TYPE_KPROBE)
	if err != nil {
		RemoveKprobe(name)
		return "", err
	}

	monitor.events[key] = event
	return name, nil
}

func (monitor *EventMonitor) removeRegisteredEvent(event *registeredEvent) {
	for _, fd := range event.fds {
		delete(monitor.eventfds, fd)
		id, ok := monitor.eventIDs[fd]
		if ok {
			delete(monitor.eventAttrs, id)
			delete(monitor.eventIDs, fd)
		}
		unix.Close(fd)
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
	if monitor.isRunning {
		return errors.New("monitor is already running")
	}

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

	if len(monitor.eventAttrs) != 0 {
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
	// Group FDs are always disabled
	for fd := range monitor.eventfds {
		disable(fd)
	}
}

func (monitor *EventMonitor) Enable() {
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
	timestamp uint64
	sampleIn  interface{}
	sampleOut interface{}
	err       error
}

// TODO Add Len(), Less(), and Swap() functions to make decodedSamples sortable

func (monitor *EventMonitor) recordSample(sampleIn *Sample, err error) {
	sample := &decodedSample{
		sampleIn: sampleIn,
		err:      err,
	}

	if err == nil && sampleIn != nil {
		sample.timestamp = sampleIn.Time

		switch record := sampleIn.Record.(type) {
		case *SampleRecord:
			sample.sampleOut, sample.err = monitor.decoders.DecodeSample(record)
		default:
			// unknown sample type; pass the raw record up
			break
		}
	}

	monitor.samples = append(monitor.samples, sample)
}

func (monitor *EventMonitor) readSamples(data []byte) {
	reader := bytes.NewReader(data)
	for reader.Len() > 0 {
		sample := Sample{}
		err := sample.read(reader, nil, monitor.eventAttrs)
		monitor.recordSample(&sample, err)
	}
}

func (monitor *EventMonitor) readRingBuffers(readyfds []int, fn func(interface{}, error)) {
	// Read all of the data from each ready fd so that we can match up
	// format ids with the right EventAttr
	for _, fd := range readyfds {
		var data [1024]byte

		for more := true; more; {
			n, err := unix.Read(fd, data[:])
			if err != nil {
				continue
			}

			if n == cap(data) {
				pollfds := make([]unix.PollFd, 1)
				pollfds[0] = unix.PollFd{
					Fd:     int32(fd),
					Events: unix.POLLIN,
				}
				_, err := unix.Poll(pollfds, 0)
				if err != nil {
					more = false
				} else {
					more = (pollfds[0].Revents & unix.POLLIN) != 0
				}
			} else {
				more = false
			}
		}
	}

	for _, fd := range readyfds {
		ringBuffer := monitor.ringBuffers[monitor.eventfds[fd]]
		ringBuffer.read(monitor.readSamples)
	}

	go func(samples []*decodedSample) {
		// TODO Sort the data read from the ringbuffers

		// Pass the sorted data to the callback function, which is as yet undefined
		for _, sample := range samples {
			fn(sample.sampleOut, sample.err)
		}

	}(monitor.samples)

	monitor.samples = monitor.samples[:0]
}

func addPollFd(pollfds []unix.PollFd, fd int) []unix.PollFd {
	pollfd := unix.PollFd{
		Fd:     int32(fd),
		Events: unix.POLLIN,
	}
	return append(pollfds, pollfd)
}

func (monitor *EventMonitor) Run(fn func(interface{}, error)) error {
	var err error = nil

	monitor.lock.Lock()
	if monitor.isRunning {
		monitor.lock.Unlock()
		return errors.New("monitor is already running")
	}
	monitor.isRunning = true
	monitor.lock.Unlock()

	err = unix.Pipe2(monitor.pipe[:], unix.O_DIRECT|unix.O_NONBLOCK)
	if err != nil {
		monitor.stopWithSignal()
		return err
	}

	// Set up the fds for polling. Do not monitor the groupfds, because
	// notifications there will prevent notifications on the eventfds,
	// which are the ones we really want, because they give us the format
	// information we need to decode the raw samples.
	pollfds := make([]unix.PollFd, 0, len(monitor.eventfds)+1)
	pollfds = addPollFd(pollfds, monitor.pipe[0])
	for fd := range monitor.eventfds {
		pollfds = addPollFd(pollfds, fd)
	}

runloop:
	for {
		n, err := unix.Poll(pollfds, -1)
		if err != nil && err != unix.EINTR {
			break
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
				monitor.readRingBuffers(readyfds, fn)
			}
		}
	}

	if monitor.pipe[1] != -1 {
		unix.Close(monitor.pipe[1])
		monitor.pipe[1] = -1
	}
	if monitor.pipe[0] != -1 {
		unix.Close(monitor.pipe[0])
		monitor.pipe[0] = -1
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
			monitor.cond.Wait()
		}
	}
}

func (monitor *EventMonitor) initializeGroupLeaders() error {
	eventAttr := &EventAttr{
		Type:            PERF_TYPE_SOFTWARE,
		Size:            sizeofPerfEventAttr,
		Config:          PERF_COUNT_SW_DUMMY, // Added in Linux 3.12
		Disabled:        true,
		Watermark:       true,
		WakeupWatermark: 1,
	}

	ncpu := runtime.NumCPU()
	newfds := make([]int, ncpu)
	ringBuffers := make([]*ringBuffer, ncpu)

	for i := 0; i < ncpu; i++ {
		fd, err := open(eventAttr, monitor.pid, i, -1, monitor.flags)
		if err != nil {
			for j := i - 1; j >= 0; j-- {
				unix.Close(newfds[j])
			}
			return err
		}
		newfds[i] = fd

		rb, err := newRingBuffer(newfds[i], monitor.ringBufferNumPages)
		if err != nil {
			unix.Close(newfds[i])
			for j := i - 1; j >= 0; j-- {
				ringBuffers[j].unmap()
				unix.Close(newfds[j])
			}
			return err
		}
		ringBuffers[i] = rb
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
		eventAttrs:         make(map[uint64]*EventAttr),
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
