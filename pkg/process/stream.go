package process

import (
	"errors"
	"log"
	"sync"

	"github.com/capsule8/reactive8/pkg/perf"
	"github.com/capsule8/reactive8/pkg/stream"
)

var once sync.Once

// State represents the state of a process
type State uint

const (
	_ State = iota
	ProcessFork
	ProcessExec
	ProcessExit
)

// Event represents a process event
type Event struct {
	Pid          uint32
	State        State
	Counter      uint64
	ForkPid      uint32
	ExecFilename string
	ExitStatus   uint64
}

type processStream struct {
	ctrl        chan interface{}
	data        chan interface{}
	eventStream *stream.Stream
	perf        *perf.Perf
	decoders    *perf.TraceEventDecoderMap
}

func decodeSchedProcessFork(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	return &Event{
		State:   ProcessFork,
		Pid:     uint32(data["common_pid"].(uint64)),
		Counter: sample.Time,
		ForkPid: uint32(data["child_pid"].(uint64)),
	}, nil
}

func decodeSchedProcessExec(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	return &Event{
		State:        ProcessExec,
		Pid:          uint32(data["common_pid"].(uint64)),
		Counter:      sample.Time,
		ExecFilename: data["filename"].(string),
	}, nil
}

func decodeSysEnterExitGroup(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	return &Event{
		State:      ProcessExit,
		Pid:        uint32(data["common_pid"].(uint64)),
		Counter:    sample.Time,
		ExitStatus: data["error_code"].(uint64),
	}, nil
}

// -----------------------------------------------------------------------------

func (ps *processStream) onSample(perfEv *perf.Sample, err error) {
	if err != nil {
		ps.data <- err
		return
	}

	var procEv *Event

	switch perfEv.Record.(type) {
	case *perf.SampleRecord:
		//
		// Sample events don't include the SampleID, so data like
		// Pid and Time must be obtained from the Sample struct.
		//

		sample := perfEv.Record.(*perf.SampleRecord)
		result, err := ps.decoders.DecodeSample(sample)
		if err != nil {
			ps.data <- err
			return
		}
		procEv = result.(*Event)

	default:
		log.Printf("Unknown perf record type %T", perfEv.Record)

	}

	if procEv != nil && procEv.State != 0 {
		ps.data <- procEv
	}
}

func createEventAttrs(decoders *perf.TraceEventDecoderMap) []*perf.EventAttr {
	//
	// TODO:
	//
	// - filter out thread creation from proc creation forks()
	//
	// - sched_signal_send/signal_generate/signal_delvier to capture
	// signaled processes (to/from)
	//
	// - cgroup_attach_task?
	//

	sysEnterExitGroupID, _ :=
		decoders.AddDecoder("syscalls/sys_enter_exit_group", decodeSysEnterExitGroup)
	schedProcessExecID, _ :=
		decoders.AddDecoder("sched/sched_process_exec", decodeSchedProcessExec)
	schedProcessForkID, _ :=
		decoders.AddDecoder("sched/sched_process_fork", decodeSchedProcessFork)

	if sysEnterExitGroupID == 0 ||
		schedProcessExecID == 0 ||
		schedProcessForkID == 0 {

		return nil
	}

	sampleType :=
		perf.PERF_SAMPLE_TID | perf.PERF_SAMPLE_TIME |
			perf.PERF_SAMPLE_CPU | perf.PERF_SAMPLE_RAW

	eventAttrs := []*perf.EventAttr{
		&perf.EventAttr{
			Type:            perf.PERF_TYPE_TRACEPOINT,
			Config:          uint64(sysEnterExitGroupID),
			SampleType:      sampleType,
			Inherit:         true,
			SampleIDAll:     true,
			SamplePeriod:    1,
			Watermark:       true,
			WakeupWatermark: 1,
		},
		&perf.EventAttr{
			Type:            perf.PERF_TYPE_TRACEPOINT,
			Config:          uint64(schedProcessExecID),
			SampleType:      sampleType,
			Inherit:         true,
			SampleIDAll:     true,
			SamplePeriod:    1,
			Watermark:       true,
			WakeupWatermark: 1,
		},

		&perf.EventAttr{
			Type:            perf.PERF_TYPE_TRACEPOINT,
			Config:          uint64(schedProcessForkID),
			SampleType:      sampleType,
			Inherit:         true,
			SampleIDAll:     true,
			SamplePeriod:    1,
			Watermark:       true,
			WakeupWatermark: 1,
		},
	}

	return eventAttrs
}

func createStream(p *perf.Perf, decoders *perf.TraceEventDecoderMap) (*stream.Stream, error) {
	controlChannel := make(chan interface{})
	dataChannel := make(chan interface{})

	ps := &processStream{
		ctrl: controlChannel,
		data: dataChannel,
		eventStream: &stream.Stream{
			Ctrl: controlChannel,
			Data: dataChannel,
		},
		perf:     p,
		decoders: decoders,
	}

	// Control loop
	go func(p *perf.Perf) {
		for {
			select {
			case e, ok := <-ps.ctrl:
				if ok {
					enable := e.(bool)
					if enable {
						p.Enable()
					} else {
						p.Disable()
					}
				} else {
					p.Close()
					return
				}
			}
		}
	}(p)

	// Data loop
	go func() {
		p.Enable()

		// This consumes everything in the loop
		p.Run(ps.onSample)

		// Perf loop terminated, we won't be sending any more events
		close(ps.data)
	}()

	return ps.eventStream, nil

}

func newPidStream(args ...int) (*stream.Stream, error) {
	var pid int

	if len(args) < 1 {
		pid = -1
	} else if len(args) > 1 {
		return nil, errors.New("Must specify zero or one pids")
	} else {
		pid = args[0]
	}

	decoders := perf.NewTraceEventDecoderMap()
	eventAttrs := createEventAttrs(decoders)
	if eventAttrs == nil {
		err := errors.New("Couldn't create perf.EventAttrs")
		return nil, err
	}

	p, err := perf.New(eventAttrs, nil, pid)
	if err != nil {
		log.Printf("Couldn't open perf events: %v\n", err)
		return nil, err

	}

	return createStream(p, decoders)
}

func newCgroupStream(cgroup string) (*stream.Stream, error) {
	decoders := perf.NewTraceEventDecoderMap()
	eventAttrs := createEventAttrs(decoders)
	if eventAttrs == nil {
		err := errors.New("Couldn't create perf.EventAttrs")
		return nil, err
	}

	p, err := perf.NewWithCgroup(eventAttrs, nil, cgroup)
	if err != nil {
		log.Printf("Couldn't open perf events: %v\n", err)
		return nil, err
	}

	return createStream(p, decoders)
}

// NewEventStream creates a new system-wide process event stream.
func NewEventStream() (*stream.Stream, error) {
	return newPidStream()
}

// NewEventStreamForPid creates a new process event stream rooted at the
// process with the given pid.
func NewEventStreamForPid(pid int) (*stream.Stream, error) {
	return newPidStream(pid)
}

// NewEventStreamForPidAndCPU creates a new process event stream for the
// specified pid and cpu numbers. When run from within a container,
// the process events for processes within a container contain the host
// process ids.
func NewEventStreamForPidAndCPU(pid int, cpu int) (*stream.Stream, error) {
	return newPidStream(pid, cpu)
}

// NewEventStreamForCgroup creates a new process event stream for the
// cgroup of the given name. The named cgroup must exist under
// /sys/fs/cgroup/perf_event.
func NewEventStreamForCgroup(name string) (*stream.Stream, error) {
	return newCgroupStream(name)
}
