package sensor

import (
	"fmt"
	"strings"

	api "github.com/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/sys/perf"

	"github.com/golang/glog"
)

type syscallFilter struct {
	sensor *Sensor
}

func (f *syscallFilter) decodeSysEnter(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	var args = data["args"].([]interface{})

	ev := f.sensor.NewEventFromSample(sample, data)
	ev.Event = &api.Event_Syscall{
		Syscall: &api.SyscallEvent{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER,
			Id:   data["id"].(int64),
			Arg0: args[0].(uint64),
			Arg1: args[1].(uint64),
			Arg2: args[2].(uint64),
			Arg3: args[3].(uint64),
			Arg4: args[4].(uint64),
			Arg5: args[5].(uint64),
		},
	}

	return ev, nil
}

func (f *syscallFilter) decodeSysExit(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	ev := f.sensor.NewEventFromSample(sample, data)
	ev.Event = &api.Event_Syscall{
		Syscall: &api.SyscallEvent{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT,
			Id:   data["id"].(int64),
			Ret:  data["ret"].(int64),
		},
	}

	return ev, nil
}

func syscallEnterFilterString(sef *api.SyscallEventFilter) string {
	parts := make([]string, 1, 7)
	parts[0] = fmt.Sprintf("id == %d", sef.Id.Value)

	if sef.Arg0 != nil {
		parts = append(parts, fmt.Sprintf("args[0] == %d", sef.Arg0.Value))
	}
	if sef.Arg1 != nil {
		parts = append(parts, fmt.Sprintf("args[1] == %d", sef.Arg1.Value))
	}
	if sef.Arg2 != nil {
		parts = append(parts, fmt.Sprintf("args[2] == %d", sef.Arg2.Value))
	}
	if sef.Arg3 != nil {
		parts = append(parts, fmt.Sprintf("args[3] == %d", sef.Arg3.Value))
	}
	if sef.Arg4 != nil {
		parts = append(parts, fmt.Sprintf("args[4] == %d", sef.Arg4.Value))
	}
	if sef.Arg5 != nil {
		parts = append(parts, fmt.Sprintf("args[5] == %d", sef.Arg5.Value))
	}

	return strings.Join(parts, " && ")
}

func syscallExitFilterString(sef *api.SyscallEventFilter) string {
	parts := make([]string, 1, 2)
	parts[0] = fmt.Sprintf("id == %d", sef.Id.Value)

	if sef.Ret != nil {
		parts = append(parts, fmt.Sprintf("ret == %d", sef.Ret.Value))
	}

	return strings.Join(parts, " && ")
}

func registerSyscallEvents(monitor *perf.EventMonitor, sensor *Sensor, events []*api.SyscallEventFilter) {
	enterFilters := make(map[string]bool)
	exitFilters := make(map[string]bool)

	for _, sef := range events {
		if sef.Id == nil {
			// No wildcard filters for now
			continue
		}

		switch sef.Type {
		case api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER:
			s := syscallEnterFilterString(sef)
			enterFilters[s] = true
		case api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT:
			s := syscallExitFilterString(sef)
			exitFilters[s] = true
		default:
			continue
		}
	}

	f := syscallFilter{
		sensor: sensor,
	}

	if len(enterFilters) > 0 {
		filters := make([]string, 0, len(enterFilters))
		for k := range enterFilters {
			filters = append(filters, fmt.Sprintf("(%s)", k))
		}
		filter := strings.Join(filters, " || ")

		eventName := "raw_syscalls/sys_enter"
		err := monitor.RegisterEvent(eventName, f.decodeSysEnter,
			filter, nil)
		if err != nil {
			glog.V(1).Infof("Couldn't get %s event id: %v", eventName, err)
		}
	}

	if len(exitFilters) > 0 {
		filters := make([]string, 0, len(exitFilters))
		for k := range exitFilters {
			filters = append(filters, fmt.Sprintf("(%s)", k))
		}
		filter := strings.Join(filters, " || ")

		eventName := "raw_syscalls/sys_exit"
		err := monitor.RegisterEvent(eventName, f.decodeSysExit,
			filter, nil)
		if err != nil {
			glog.V(1).Infof("Couldn't get %s event id: %v", eventName, err)
		}
	}
}
