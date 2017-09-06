package perf

import (
	"errors"
	"runtime"
	"sync"

	"golang.org/x/sys/unix"
)

type EventMonitor struct {
	pid            int
	flags          uintptr
	ringBufferSize int
	defaultAttr    EventAttr

	lock *sync.Mutex
	cond *sync.Cond

	isRunning   bool
	pipe        [2]int
	groupfds    []int
	otherfds    []int
	ringBuffers []*ringBuffer
	decoders    *TraceEventDecoderMap

	// Used while reading samples from ringbuffers
	samples []*decodedSample
}

func fixupEventAttr(eventAttr *EventAttr) {
	// Adjust certain fields in eventAttr that must be set a certain way
	eventAttr.Type = PERF_TYPE_TRACEPOINT
	eventAttr.Size = sizeofPerfEventAttr
	eventAttr.SamplePeriod = 1 // SampleFreq not used
	eventAttr.ReadFormat = 0

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
		var groupfd int

		if groupfds == nil {
			groupfd = -1
		} else {
			groupfd = groupfds[i]
		}
		fd, err := open(eventAttr, pid, i, groupfd, flags)
		if err != nil {
			for j := i; j >= 0; j-- {
				unix.Close(newfds[j-1])
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

func (monitor *EventMonitor) RegisterEvent(name string, fn TraceEventDecoderFn, filter string, eventAttr *EventAttr) error {
	if monitor.isRunning {
		return errors.New("monitor is already running")
	}

	id, err := monitor.decoders.AddDecoder(name, fn)
	if err != nil {
		return err
	}

	var attr EventAttr

	if eventAttr == nil {
		attr = monitor.defaultAttr
	} else {
		attr = *eventAttr
		fixupEventAttr(&attr)
	}
	attr.Config = uint64(id)

	if monitor.groupfds == nil {
		newfds, err := perfEventOpen(&attr, monitor.pid, nil, monitor.flags, filter)
		if err != nil {
			monitor.decoders.RemoveDecoder(name)
			return err
		}

		ncpu := runtime.NumCPU()
		ringBuffers := make([]*ringBuffer, ncpu)
		for i := 0; i < ncpu; i++ {
			rb, err := newRingBuffer(newfds[i], monitor.ringBufferSize, &attr)
			if err != nil {
				for j := i; j >= 0; j-- {
					ringBuffers[j-1].unmap()
				}
				for j := ncpu; j >= 0; j-- {
					unix.Close(newfds[j-1])
				}
				monitor.decoders.RemoveDecoder(name)
				return err
			}
			ringBuffers[i] = rb
		}

		monitor.groupfds = newfds
		monitor.ringBuffers = ringBuffers
	} else {
		flags := monitor.flags | PERF_FLAG_FD_OUTPUT
		newfds, err := perfEventOpen(&attr, monitor.pid, monitor.groupfds, flags, filter)
		if err != nil {
			monitor.decoders.RemoveDecoder(name)
			return err
		}

		if monitor.otherfds == nil {
			monitor.otherfds = newfds
		} else {
			monitor.otherfds = append(monitor.otherfds, newfds...)
		}
	}

	return nil
}

func (monitor *EventMonitor) Shutdown(wait bool) error {
	// if the monitor is running, stop it and wait for it to stop
	monitor.Stop(wait)

	for i := range monitor.otherfds {
		unix.Close(monitor.otherfds[i])
	}
	for i := range monitor.ringBuffers {
		monitor.ringBuffers[i].unmap()
	}
	for i := range monitor.groupfds {
		unix.Close(monitor.groupfds[i])
	}

	monitor.ringBuffers = monitor.ringBuffers[:0]
	monitor.groupfds = monitor.groupfds[:0]
	monitor.otherfds = monitor.otherfds[:0]

	return nil
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
	if err != nil {
		// Log the error, or is it logged elsewhere?
		return
	}

	// Decode the sample using monitor.decoders
	switch sampleIn.Record.(type) {
	case *SampleRecord:
		sample := &decodedSample{
			timestamp: sampleIn.Time,
			sampleIn:  sampleIn,
		}
		sample.sampleOut, sample.err = monitor.decoders.DecodeSample(sampleIn.Record.(*SampleRecord))
		monitor.samples = append(monitor.samples, sample)
	default:
		// unknown sample type; don't do anything with it for now
	}
}

// Here, `ready` is a list of indices into monitor.groupfds that are ready for
// reading. The same indices match monitor.ringBuffers.
func (monitor *EventMonitor) readRingBuffers(ready []int, fn func(interface{}, error)) {
	if monitor.samples == nil {
		monitor.samples = make([]*decodedSample, 8)
	}

	for i := range ready {
		var data [64]byte

		// Read data from monitor.groupfds[i] and discard it. There
		// shouldn't be much there, but make sure we get it all.
		_, err := unix.Read(monitor.groupfds[i], data[:])
		if err != nil {
			continue
		}

		monitor.ringBuffers[i].read(monitor.recordSample)
	}

	// TODO Sort the data read from the ringbuffers

	// Pass the sorted data to the callback function, which is as yet undefined
	for i := range monitor.samples {
		sample := monitor.samples[i]
		fn(sample.sampleOut, sample.err)
	}

	monitor.samples = monitor.samples[:0]
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

	// Enable all perf events to be monitored. Do we ignore errors here,
	// or do we revert enabling everything and return the error? Likelihood
	// of failure is extremely low. Any error here is likely a programming
	// error
	for i := range monitor.groupfds {
		enable(monitor.groupfds[i])
	}
	for i := range monitor.otherfds {
		enable(monitor.otherfds[i])
	}

	// Set up the fds for polling. We only need to monitor the groupfds,
	// since those are the only ones with ring buffers attached and so are
	// the only ones that will get notifications.
	pollfds := make([]unix.PollFd, len(monitor.groupfds)+1)
	pollfds[0].Fd = int32(monitor.pipe[0])
	pollfds[0].Events = unix.POLLIN
	for i := range monitor.groupfds {
		pollfds[i+1].Fd = int32(monitor.groupfds[i])
		pollfds[i+1].Events = unix.POLLIN
	}

runloop:
	for {
		n, err := unix.Poll(pollfds, -1)
		if err != nil && err != unix.EINTR {
			break
		}
		if n > 0 {
			ready := make([]int, 0, n)
			for i := range pollfds {
				if i == 0 {
					if (pollfds[i].Revents & ^unix.POLLIN) != 0 {
						// POLLERR, POLLHUP, or POLLNVAL set
						break runloop
					}
				} else if (pollfds[i].Revents & unix.POLLIN) != 0 {
					ready = append(ready, i-1)
				}
			}
			if len(ready) > 0 {
				monitor.readRingBuffers(ready, fn)
			}
		}
	}

	// Disable all perf events to be monitored. Unlike enabling, errors
	// should absolutely be ignored here.
	for i := range monitor.otherfds {
		disable(monitor.otherfds[i])
	}
	for i := range monitor.groupfds {
		disable(monitor.groupfds[i])
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
	if !monitor.isRunning {
		return
	}

	if monitor.pipe[1] != -1 {
		fd := monitor.pipe[1]
		monitor.pipe[1] = -1
		unix.Close(fd)
	}

	if wait {
		monitor.lock.Lock()
		defer monitor.lock.Unlock()
		for monitor.isRunning {
			monitor.cond.Wait()
		}
	}
}

func NewEventMonitor(pid int, flags uintptr, ringBufferSize int, defaultAttr *EventAttr) *EventMonitor {
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
		pid:            pid,
		flags:          flags,
		ringBufferSize: ringBufferSize,
		defaultAttr:    eventAttr,
		decoders:       NewTraceEventDecoderMap(),
	}

	return monitor
}
