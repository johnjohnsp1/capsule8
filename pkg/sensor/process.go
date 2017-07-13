package sensor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/perf"
)

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
	perf.TraceEvent
	ParentComm string
	ParentPid  int32
	ChildComm  string
	ChildPid   int32
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
		TraceEvent: perf.TraceEvent{
			Type:         format.Type,
			Flags:        format.Flags,
			PreemptCount: format.PreemptCount,
			Pid:          format.Pid,
		},
		ParentComm: parentComm,
		ParentPid:  format.ParentPid,
		ChildComm:  childComm,
		ChildPid:   format.ChildPid,
	}, nil
}

func decodeSchedProcessFork(rawData []byte) (interface{}, error) {
	tpEv, err := readSchedProcessForkTracepointData(rawData)
	if err != nil {
		return nil, err
	}

	// Notify pidmap of fork event
	pidMapOnFork(tpEv.ParentPid, tpEv.ChildPid)

	ev := newEventFromTraceEvent(&tpEv.TraceEvent)
	ev.Event = &api.Event_Process{
		Process: &api.ProcessEvent{
			Type:         api.ProcessEventType_PROCESS_EVENT_TYPE_FORK,
			ForkChildPid: tpEv.ChildPid,
		},
	}

	return ev, nil
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
	perf.TraceEvent
	Filename string
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
		TraceEvent: perf.TraceEvent{
			Type:         format.Type,
			Flags:        format.Flags,
			PreemptCount: format.PreemptCount,
			Pid:          format.CommonPid,
		},
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

func decodeSchedProcessExec(rawData []byte) (interface{}, error) {
	tpEv, err := readSchedProcessExecTracepointData(rawData)
	if err != nil {
		return nil, err
	}

	processEvent := &api.ProcessEvent{
		Type:         api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
		ExecFilename: tpEv.Filename,
	}

	processEvent.ExecCommandLine, err = pidMapGetCommandLine(tpEv.Pid)

	ev := newEventFromTraceEvent(&tpEv.TraceEvent)
	ev.Event = &api.Event_Process{
		Process: processEvent,
	}

	return ev, nil
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
	perf.TraceEvent
	SyscallNr int32
	_         uint32
	ErrorCode uint64
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

func decodeSysEnterExitGroup(rawData []byte) (interface{}, error) {
	tpEv, err := readSysEnterExitGroupTracepointData(rawData)
	if err != nil {
		return nil, err
	}

	ev := newEventFromTraceEvent(&tpEv.TraceEvent)
	ev.Event = &api.Event_Process{
		Process: &api.ProcessEvent{
			Type:     api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT,
			ExitCode: int32(tpEv.ErrorCode),
		},
	}

	return ev, nil
}

// -----------------------------------------------------------------------------

func init() {
	sensor := getSensor()

	eventName := "sched/sched_process_fork"
	schedProcessForkID, err := perf.GetTraceEventID(eventName)
	if err != nil {
		log.Printf("Couldn't get %s event id: %v", eventName, err)
	}

	sensor.registerDecoder(schedProcessForkID, decodeSchedProcessFork)

	eventName = "sched/sched_process_exec"
	schedProcessExecID, err := perf.GetTraceEventID(eventName)
	if err != nil {
		log.Printf("Couldn't get %s event id: %v", eventName, err)
	}

	sensor.registerDecoder(schedProcessExecID, decodeSchedProcessExec)

	eventName = "syscalls/sys_enter_exit_group"
	sysEnterExitGroupID, err := perf.GetTraceEventID(eventName)

	if err != nil {
		log.Printf("Couldn't get %s event id: %v", eventName, err)
	}

	sensor.registerDecoder(sysEnterExitGroupID, decodeSysEnterExitGroup)
}
