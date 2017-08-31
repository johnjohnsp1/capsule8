package sensor

import (
	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/perf"
	"github.com/golang/glog"
)

func decodeSysEnter(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	var args = data["args"].([]uint64)

	ev := newEventFromFieldData(data)
	ev.Event = &api.Event_Syscall{
		Syscall: &api.SyscallEvent{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER,
			Id:   int64(data["id"].(uint64)),
			Arg0: args[0],
			Arg1: args[1],
			Arg2: args[2],
			Arg3: args[3],
			Arg4: args[4],
			Arg5: args[5],
		},
	}

	return ev, nil
}

func decodeSysExit(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	ev := newEventFromFieldData(data)
	ev.Event = &api.Event_Syscall{
		Syscall: &api.SyscallEvent{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT,
			Id:   int64(data["id"].(uint64)),
			Ret:  int64(data["ret"].(uint64)),
		},
	}

	return ev, nil
}

// -----------------------------------------------------------------------------

func init() {
	sensor := getSensor()

	eventName := "raw_syscalls/sys_enter"
	err := sensor.registerDecoder(eventName, decodeSysEnter)
	if err != nil {
		glog.Infof("Couldn't get %s event id: %v", eventName, err)
	}

	eventName = "raw_syscalls/sys_exit"
	err = sensor.registerDecoder(eventName, decodeSysExit)
	if err != nil {
		glog.Infof("Couldn't get %s event id: %v", eventName, err)
	}
}
