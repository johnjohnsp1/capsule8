package process

import (
	"bytes"
	"errors"
	"log"
	"sync"
	"sync/atomic"

	"encoding/binary"

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
}

/*
# cat /sys/kernel/debug/tracing/events/sched/sched_process_fork/format
name: sched_process_fork
ID: 284
format:
	field:unsigned short common_type;	offset:0;	size:2;	signed:0;
	field:unsigned char common_flags;	offset:2;	size:1;	signed:0;
	field:unsigned char common_preempt_count;	offset:3;	size:1;	signed:0;
	field:int common_pid;	offset:4;	size:4;	signed:1;

	field:char parent_comm[16];	offset:8;	size:16;	signed:1;
	field:pid_t parent_pid;	offset:24;	size:4;	signed:1;
	field:char child_comm[16];	offset:28;	size:16;	signed:1;
	field:pid_t child_pid;	offset:44;	size:4;	signed:1;

print fmt: "comm=%s pid=%d child_comm=%s child_pid=%d", REC->parent_comm, REC->parent_pid, REC->child_comm, REC->child_pid
*/

type schedProcessForkEvent struct {
	Type         uint16
	Flags        uint8
	PreemptCount uint8
	Pid          int32
	ParentComm   string
	ParentPid    int32
	ChildComm    string
	ChildPid     int32
}

func clen(n []byte) int {
	for i := 0; i < len(n); i++ {
		if n[i] == 0 {
			return i
		}
	}
	return len(n)
}

func readSchedProcessForkTracepointData(data []byte) (*schedProcessForkEvent, error) {

	var format struct {
		Type         uint16
		Flags        uint8
		PreemptCount uint8
		Pid          int32
		ParentComm   [16]byte
		ParentPid    int32
		ChildComm    [16]byte
		ChildPid     int32
	}

	reader := bytes.NewReader(data)
	err := binary.Read(reader, binary.LittleEndian, &format)
	if err != nil {
		return nil, err
	}

	parentComm := string(format.ParentComm[:clen(format.ParentComm[:])])
	childComm := string(format.ChildComm[:clen(format.ChildComm[:])])

	return &schedProcessForkEvent{
		Type:         format.Type,
		Flags:        format.Flags,
		PreemptCount: format.PreemptCount,
		Pid:          format.Pid,
		ParentComm:   parentComm,
		ParentPid:    format.ParentPid,
		ChildComm:    childComm,
		ChildPid:     format.ChildPid,
	}, nil
}

func decodeSchedProcessFork(sample *perf.Sample) (*Event, error) {
	tpEv, err := readSchedProcessForkTracepointData(sample.RawData)
	if err != nil {
		return nil, err
	}

	return &Event{
		State:   ProcessFork,
		Pid:     uint32(tpEv.Pid),
		Counter: sample.Time,
		ForkPid: uint32(tpEv.ChildPid),
	}, nil
}

/*
# cat /sys/kernel/debug/tracing/events/sched/sched_process_exec/format
name: sched_process_exec
ID: 283
format:
	field:unsigned short common_type;	offset:0;	size:2;	signed:0;
	field:unsigned char common_flags;	offset:2;	size:1;	signed:0;
	field:unsigned char common_preempt_count;	offset:3;	size:1;	signed:0;
	field:int common_pid;	offset:4;	size:4;	signed:1;

	field:__data_loc char[] filename;	offset:8;	size:4;	signed:1;
	field:pid_t pid;	offset:12;	size:4;	signed:1;
	field:pid_t old_pid;	offset:16;	size:4;	signed:1;

print fmt: "filename=%s pid=%d old_pid=%d", __get_str(filename), REC->pid, REC->old_pid
*/

type schedProcessExecEvent struct {
	Type         uint16
	Flags        uint8
	PreemptCount uint8
	Pid          int32
	Filename     string
}

func readSchedProcessExecTracepointData(data []byte) (*schedProcessExecEvent, error) {
	reader := bytes.NewReader(data)

	var format struct {
		Type           uint16
		Flags          uint8
		PreemptCount   uint8
		CommonPid      int32
		FilenameOffset int16
		FilenameLength int16
		Pid            int32
		OldPid         int32
	}

	err := binary.Read(reader, binary.LittleEndian, &format)
	if err != nil {
		return nil, err
	}

	ev := &schedProcessExecEvent{
		Type:         format.Type,
		Flags:        format.Flags,
		PreemptCount: format.PreemptCount,
		Pid:          format.CommonPid,
	}

	if format.CommonPid != format.Pid ||
		format.Pid != format.OldPid ||
		format.CommonPid != format.OldPid {

		return nil, errors.New("Could not read sched_process_exec data")
	}

	filename := make([]byte, format.FilenameLength)
	n, err := reader.ReadAt(filename, int64(format.FilenameOffset))
	if err != nil {
		return nil, err
	} else if n != int(format.FilenameLength) {
		return nil, errors.New("sched_process_exec filename read failed")
	}
	if filename[len(filename)-1] == 0 {
		// Create string without NULL terminator
		ev.Filename = string(filename[:len(filename)-1])
	} else {
		return nil, errors.New("sched_process_exec filename not terminated")
	}

	return ev, nil
}

func decodeSchedProcessExec(sample *perf.Sample) (*Event, error) {
	tpEv, err := readSchedProcessExecTracepointData(sample.RawData)
	if err != nil {
		return nil, err
	}

	return &Event{
		State:        ProcessExec,
		Pid:          uint32(tpEv.Pid),
		Counter:      sample.Time,
		ExecFilename: tpEv.Filename,
	}, nil

}

/*
# cat /sys/kernel/debug/tracing/events/syscalls/sys_enter_exit_group/format
name: sys_enter_exit_group
ID: 120
format:
	field:unsigned short common_type;	offset:0;	size:2;	signed:0;
	field:unsigned char common_flags;	offset:2;	size:1;	signed:0;
	field:unsigned char common_preempt_count;	offset:3;	size:1;	signed:0;
	field:int common_pid;	offset:4;	size:4;	signed:1;

	field:int __syscall_nr;	offset:8;	size:4;	signed:1;
	field:int error_code;	offset:16;	size:8;	signed:0;

print fmt: "error_code: 0x%08lx", ((unsigned long)(REC->error_code))
*/

type sysEnterExitGroupEvent struct {
	Type         uint16
	Flags        uint8
	PreemptCount uint8
	Pid          int32
	SyscallNr    int32
	_            uint32
	ErrorCode    uint64
}

func readSysEnterExitGroupTracepointData(data []byte) (*sysEnterExitGroupEvent, error) {
	reader := bytes.NewReader(data)

	var ev sysEnterExitGroupEvent

	err := binary.Read(reader, binary.LittleEndian, &ev)
	if err != nil {
		return nil, err
	}

	return &ev, nil
}

func decodeSysEnterExitGroup(sample *perf.Sample) (*Event, error) {
	tpEv, err := readSysEnterExitGroupTracepointData(sample.RawData)
	if err != nil {
		return nil, err
	}

	return &Event{
		State:      ProcessExit,
		Pid:        uint32(tpEv.Pid),
		Counter:    sample.Time,
		ExitStatus: tpEv.ErrorCode,
	}, nil
}

// -----------------------------------------------------------------------------

type tracepointDecoderFn func(*perf.Sample) (*Event, error)

var tracepointDecoders struct {
	mu       sync.Mutex
	decoders atomic.Value // map[uint16]tracepointDecoderFn
}

func (ps *processStream) processSample(sample *perf.Sample) (*Event, error) {
	var tracepointEventType uint16

	reader := bytes.NewReader(sample.RawData)
	binary.Read(reader, binary.LittleEndian, &tracepointEventType)

	val := tracepointDecoders.decoders.Load()
	decoders := val.(map[uint16]tracepointDecoderFn)
	decoder := decoders[tracepointEventType]
	if decoder != nil {
		return decoder(sample)
	}

	return nil, nil
}

func (ps *processStream) processPerfEvent(perfEv *perf.Event, err error) {
	if err != nil {
		ps.data <- err
		return
	}

	var procEv *Event

	switch perfEv.Data.(type) {
	case *perf.Sample:
		//
		// Sample events don't include the SampleID, so data like
		// Pid and Time must be obtained from the Sample struct.
		//

		sample := perfEv.Data.(*perf.Sample)
		procEv, err = ps.processSample(sample)
		if err != nil {
			ps.data <- err
			return
		}

	default:
		// Silently drop unknown message types
	}

	if procEv != nil && procEv.State != 0 {
		ps.data <- procEv
	}
}

func (ps *processStream) runControlLoop() {
	for {
		select {
		case e, ok := <-ps.ctrl:
			if ok {
				enable := e.(bool)
				if enable {
					ps.perf.Enable()
				} else {
					ps.perf.Disable()
				}
			} else {
				ps.perf.Close()
				return
			}
		}
	}
}

func (ps *processStream) runDataLoop() {
	ps.perf.Enable()

	// This consumes everything in the loop
	ps.perf.Run(ps.processPerfEvent)

	// Perf loop terminated, we won't be sending any more events
	close(ps.data)
}

func createStream(p *perf.Perf) (*stream.Stream, error) {
	controlChannel := make(chan interface{})
	dataChannel := make(chan interface{})

	proc := &processStream{
		ctrl: controlChannel,
		data: dataChannel,
		eventStream: &stream.Stream{
			Ctrl: controlChannel,
			Data: dataChannel,
		},
		perf: p,
	}

	go proc.runControlLoop()
	go proc.runDataLoop()

	return proc.eventStream, nil
}

func createEventAttrs() []*perf.EventAttr {
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
		perf.GetTraceEventID("syscalls/sys_enter_exit_group")
	schedProcessExecID, _ :=
		perf.GetTraceEventID("sched/sched_process_exec")
	schedProcessForkID, _ :=
		perf.GetTraceEventID("sched/sched_process_fork")

	if sysEnterExitGroupID == 0 ||
		schedProcessExecID == 0 ||
		schedProcessForkID == 0 {

		return nil
	}

	tracepointDecoders.mu.Lock()
	var val = tracepointDecoders.decoders.Load()
	if val == nil {
		decoders := make(map[uint16]tracepointDecoderFn)

		decoders[sysEnterExitGroupID] = decodeSysEnterExitGroup

		decoders[schedProcessExecID] = decodeSchedProcessExec

		decoders[schedProcessForkID] = decodeSchedProcessFork

		tracepointDecoders.decoders.Store(decoders)
	}
	tracepointDecoders.mu.Unlock()

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

func newPidStream(args ...int) (*stream.Stream, error) {
	var pid int

	if len(args) < 1 {
		pid = -1
	} else if len(args) > 1 {
		return nil, errors.New("Must specify zero or one pids")
	} else {
		pid = args[0]
	}

	eventAttrs := createEventAttrs()
	if eventAttrs == nil {
		err := errors.New("Couldn't create perf.EventAttrs")
		return nil, err
	}

	p, err := perf.New(eventAttrs, pid)
	if err != nil {
		log.Printf("Couldn't open perf events: %v\n", err)
		return nil, err

	}

	return createStream(p)
}

func newCgroupStream(cgroup string) (*stream.Stream, error) {
	eventAttrs := createEventAttrs()
	if eventAttrs == nil {
		err := errors.New("Couldn't create perf.EventAttrs")
		return nil, err
	}

	p, err := perf.NewWithCgroup(eventAttrs, cgroup)
	if err != nil {
		log.Printf("Couldn't open perf events: %v\n", err)
		return nil, err
	}

	return createStream(p)
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
