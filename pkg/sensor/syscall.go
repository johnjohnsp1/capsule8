package sensor

import (
	"bytes"
	"encoding/binary"
	"log"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/perf"
)

/*
# cat /sys/kernel/debug/tracing/events/raw_syscalls/sys_enter/format
name: sys_enter
ID: 19
format:
	field:unsigned short common_type;	offset:0;	size:2;	signed:0;
	field:unsigned char common_flags;	offset:2;	size:1;	signed:0;
	field:unsigned char common_preempt_count;	offset:3;	size:1;	signed:0;
	field:int common_pid;	offset:4;	size:4;	signed:1;

	field:long id;	offset:8;	size:8;	signed:1;
	field:unsigned long args[6];	offset:16;	size:48;	signed:0;

print fmt: "NR %ld (%lx, %lx, %lx, %lx, %lx, %lx)", REC->id, REC->args[0], REC->args[1], REC->args[2], REC->args[3], REC->args[4], REC->args[5]
*/

type sysEnterFormat struct {
	perf.TraceEvent
	ID   int64
	Args [6]uint64
}

func decodeSysEnter(rawData []byte) (interface{}, error) {
	reader := bytes.NewReader(rawData)

	format := sysEnterFormat{}
	err := binary.Read(reader, binary.LittleEndian, &format)
	if err != nil {
		return nil, err
	}

	ev := newEventFromTraceEvent(&format.TraceEvent)
	ev.Event = &api.Event_Syscall{
		Syscall: &api.SyscallEvent{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER,
			Id:   format.ID,
			Arg0: format.Args[0],
			Arg1: format.Args[1],
			Arg2: format.Args[2],
			Arg3: format.Args[3],
			Arg4: format.Args[4],
			Arg5: format.Args[5],
		},
	}

	return ev, nil
}

/*
# cat /sys/kernel/debug/tracing/events/raw_syscalls/sys_exit/format
name: sys_exit
ID: 18
format:
	field:unsigned short common_type;	offset:0;	size:2;	signed:0;
	field:unsigned char common_flags;	offset:2;	size:1;	signed:0;
	field:unsigned char common_preempt_count;	offset:3;	size:1;	signed:0;
	field:int common_pid;	offset:4;	size:4;	signed:1;

	field:long id;	offset:8;	size:8;	signed:1;
	field:long ret;	offset:16;	size:8;	signed:1;

print fmt: "NR %ld = %ld", REC->id, REC->ret
*/

type sysExitFormat struct {
	perf.TraceEvent
	ID  int64
	Ret int64
}

func decodeSysExit(rawData []byte) (interface{}, error) {
	reader := bytes.NewReader(rawData)

	format := sysExitFormat{}
	err := binary.Read(reader, binary.LittleEndian, &format)
	if err != nil {
		return nil, err
	}

	ev := newEventFromTraceEvent(&format.TraceEvent)
	ev.Event = &api.Event_Syscall{
		Syscall: &api.SyscallEvent{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT,
			Id:   format.ID,
			Ret:  format.Ret,
		},
	}

	return ev, nil
}

// -----------------------------------------------------------------------------

func init() {
	sensor := getSensor()

	eventName := "raw_syscalls/sys_enter"
	sysEnterID, err := perf.GetTraceEventID(eventName)
	if err != nil {
		log.Printf("Couldn't get %s event id: %v", eventName, err)
	}

	sensor.registerDecoder(sysEnterID, decodeSysEnter)

	eventName = "raw_syscalls/sys_exit"
	sysExitID, err := perf.GetTraceEventID(eventName)
	if err != nil {
		log.Printf("Couldn't get %s event id: %v", eventName, err)
	}

	sensor.registerDecoder(sysExitID, decodeSysExit)
}
