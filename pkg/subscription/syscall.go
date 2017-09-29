package subscription

import (
	"fmt"
	"reflect"
	"strings"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/sys/perf"
	"github.com/golang/glog"
)

func decodeSysEnter(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	var args = data["args"].([]interface{})

	ev := newEventFromSample(sample, data)
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

func decodeSysExit(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	ev := newEventFromSample(sample, data)
	ev.Event = &api.Event_Syscall{
		Syscall: &api.SyscallEvent{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT,
			Id:   data["id"].(int64),
			Ret:  data["ret"].(int64),
		},
	}

	return ev, nil
}

type syscallEnterFilter struct {
	nr   int64
	args map[uint8]uint64
}

func newSyscallEnterFilter(sef *api.SyscallEventFilter) *syscallEnterFilter {
	if sef.Id == nil {
		// No system call wildcards for now
		return nil
	}

	filter := &syscallEnterFilter{
		nr: sef.Id.Value,
	}

	args := make(map[uint8]uint64)
	if sef.Arg0 != nil {
		args[0] = sef.Arg0.Value
	}
	if sef.Arg1 != nil {
		args[1] = sef.Arg1.Value
	}
	if sef.Arg2 != nil {
		args[2] = sef.Arg2.Value
	}
	if sef.Arg3 != nil {
		args[3] = sef.Arg3.Value
	}
	if sef.Arg4 != nil {
		args[4] = sef.Arg4.Value
	}
	if sef.Arg5 != nil {
		args[5] = sef.Arg5.Value
	}
	if len(args) > 0 {
		filter.args = args
	}

	return filter
}

func (f *syscallEnterFilter) String() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("id == %d", f.nr))
	for a, v := range f.args {
		parts = append(parts, fmt.Sprintf("args[%d] == %d", a, v))
	}

	return strings.Join(parts, " && ")
}

type syscallExitFilter struct {
	nr  int64
	ret *int64 // Pointer is nil if no return value was specified
}

func newSyscallExitFilter(sef *api.SyscallEventFilter) *syscallExitFilter {
	if sef.Id == nil {
		// No system call wildcards for now
		return nil
	}

	filter := &syscallExitFilter{
		nr: sef.Id.Value,
	}

	if sef.Ret != nil {
		filter.ret = &sef.Ret.Value
	}

	return filter
}

func (sef *syscallExitFilter) String() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("id == %d", sef.nr))
	if sef.ret != nil {
		parts = append(parts, fmt.Sprintf("ret == %d", *sef.ret))
	}

	return strings.Join(parts, " && ")
}

type syscallFilterSet struct {
	enter []*syscallEnterFilter
	exit  []*syscallExitFilter
}

func (sfs *syscallFilterSet) addEnter(sef *api.SyscallEventFilter) {
	filter := newSyscallEnterFilter(sef)
	if filter == nil {
		return
	}

	for _, v := range sfs.enter {
		if reflect.DeepEqual(filter, v) {
			return
		}
	}

	sfs.enter = append(sfs.enter, filter)
}

func (sfs *syscallFilterSet) addExit(sef *api.SyscallEventFilter) {
	filter := newSyscallExitFilter(sef)
	if filter == nil {
		return
	}

	for _, v := range sfs.exit {
		if reflect.DeepEqual(filter, v) {
			return
		}
	}

	sfs.exit = append(sfs.exit, filter)
}

func (sfs *syscallFilterSet) add(sef *api.SyscallEventFilter) {
	switch sef.Type {
	case api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER:
		sfs.addEnter(sef)

	case api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT:
		sfs.addExit(sef)
	}
}

func (sfs *syscallFilterSet) len() int {
	return len(sfs.enter) + len(sfs.exit)
}

func (sfs *syscallFilterSet) registerEvents(monitor *perf.EventMonitor) {
	if sfs.enter != nil {
		var enterFilters []string
		for _, f := range sfs.enter {
			s := f.String()
			if len(s) > 0 {
				enterFilters = append(enterFilters, fmt.Sprintf("(%s)", s))
			}
		}
		enterFilter := strings.Join(enterFilters, " || ")

		eventName := "raw_syscalls/sys_enter"
		err := monitor.RegisterEvent(eventName, decodeSysEnter, enterFilter, nil)
		if err != nil {
			glog.Infof("Couldn't get %s event id: %v", eventName, err)
		}

	}

	if sfs.exit != nil {
		var exitFilters []string
		for _, f := range sfs.exit {
			s := f.String()
			if len(s) > 0 {
				exitFilters = append(exitFilters, fmt.Sprintf("(%s)", s))
			}
		}
		exitFilter := strings.Join(exitFilters, " || ")

		eventName := "raw_syscalls/sys_exit"
		err := monitor.RegisterEvent(eventName, decodeSysExit, exitFilter, nil)
		if err != nil {
			glog.Infof("Couldn't get %s event id: %v", eventName, err)
		}
	}
}
