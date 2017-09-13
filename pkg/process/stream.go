package process

import (
	"errors"
	"sync"

	"github.com/capsule8/reactive8/pkg/perf"
	"github.com/capsule8/reactive8/pkg/stream"
	"github.com/golang/glog"
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
	Pid          int32
	State        State
	Counter      uint64
	ForkPid      int32
	ExecFilename string
	ExitStatus   uint64
}

type processStream struct {
	ctrl        chan interface{}
	data        chan interface{}
	eventStream *stream.Stream
	monitor     *perf.EventMonitor
}

func decodeSchedProcessFork(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	return &Event{
		State:   ProcessFork,
		Pid:     data["common_pid"].(int32),
		Counter: sample.Time,
		ForkPid: data["child_pid"].(int32),
	}, nil
}

func decodeSchedProcessExec(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	return &Event{
		State:        ProcessExec,
		Pid:          data["common_pid"].(int32),
		Counter:      sample.Time,
		ExecFilename: data["filename"].(string),
	}, nil
}

func decodeSysEnterExitGroup(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	return &Event{
		State:      ProcessExit,
		Pid:        data["common_pid"].(int32),
		Counter:    sample.Time,
		ExitStatus: data["error_code"].(uint64),
	}, nil
}

// -----------------------------------------------------------------------------

func (ps *processStream) onSample(event interface{}, err error) {
	if err != nil {
		ps.data <- err
		return
	}

	procEv := event.(*Event)
	if procEv != nil && procEv.State != 0 {
		ps.data <- procEv
	}
}

func setupMonitor(monitor *perf.EventMonitor) error {
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
	err := monitor.RegisterEvent("sched/sched_process_exec", decodeSchedProcessExec, "", nil)
	if err != nil {
		return err
	}
	err = monitor.RegisterEvent("sched/sched_process_fork", decodeSchedProcessFork, "", nil)
	if err != nil {
		return err
	}
	err = monitor.RegisterEvent("syscalls/sys_enter_exit_group", decodeSysEnterExitGroup, "", nil)
	if err != nil {
		return err
	}

	return nil
}

func createStream(monitor *perf.EventMonitor) (*stream.Stream, error) {
	controlChannel := make(chan interface{})
	dataChannel := make(chan interface{})

	ps := &processStream{
		ctrl: controlChannel,
		data: dataChannel,
		eventStream: &stream.Stream{
			Ctrl: controlChannel,
			Data: dataChannel,
		},
		monitor: monitor,
	}

	// Control loop
	go func() {
		for {
			select {
			case e, ok := <-ps.ctrl:
				if ok {
					enable := e.(bool)
					if enable {
						monitor.Enable()
					} else {
						monitor.Disable()
					}
				} else {
					monitor.Stop(true)
					return
				}
			}
		}
	}()

	// Data loop
	go func() {
		monitor.Enable()

		// This consumes everything in the loop
		monitor.Run(ps.onSample)

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

	monitor, err := perf.NewEventMonitor(pid, 0, 0, nil)
	if err != nil {
		glog.Infof("Couldn't create event monitor: %v", err)
		return nil, err
	}

	err = setupMonitor(monitor)
	if err != nil {
		glog.Infof("Couldn't setup event monitor: %v", err)
		return nil, err
	}

	return createStream(monitor)
}

func newCgroupStream(cgroup string) (*stream.Stream, error) {
	monitor, err := perf.NewEventMonitorWithCgroup(cgroup, 0, 0, nil)
	if err != nil {
		glog.Infof("Couldn't create event monitor: %v", err)
		return nil, err
	}

	err = setupMonitor(monitor)
	if err != nil {
		glog.Infof("Couldn't setup event monitor: %v", err)
		return nil, err
	}

	return createStream(monitor)
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
