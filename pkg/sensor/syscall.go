package sensor

import (
	"fmt"
	"reflect"
	"strings"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/perf"
	"github.com/golang/glog"
)

func decodeSysEnter(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	var args = data["args"].([]uint64)

	ev := newEventFromSample(sample, data)
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
	ev := newEventFromSample(sample, data)
	ev.Event = &api.Event_Syscall{
		Syscall: &api.SyscallEvent{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT,
			Id:   int64(data["id"].(uint64)),
			Ret:  int64(data["ret"].(uint64)),
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

func (f *syscallExitFilter) String() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("id == %d", f.nr))
	if f.ret != nil {
		parts = append(parts, fmt.Sprintf("ret == %d", *f.ret))
	}

	return strings.Join(parts, " && ")
}

type syscallFilterSet struct {
	enter []*syscallEnterFilter
	exit  []*syscallExitFilter
}

func (ses *syscallFilterSet) addEnter(sef *api.SyscallEventFilter) {
	filter := newSyscallEnterFilter(sef)
	if filter == nil {
		return
	}

	for _, v := range ses.enter {
		if reflect.DeepEqual(filter, v) {
			return
		}
	}

	ses.enter = append(ses.enter, filter)
}

func (ses *syscallFilterSet) addExit(sef *api.SyscallEventFilter) {
	filter := newSyscallExitFilter(sef)
	if filter == nil {
		return
	}

	for _, v := range ses.exit {
		if reflect.DeepEqual(filter, v) {
			return
		}
	}

	ses.exit = append(ses.exit, filter)
}

func (ses *syscallFilterSet) add(sef *api.SyscallEventFilter) {
	switch sef.Type {
	case api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER:
		ses.addEnter(sef)

	case api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT:
		ses.addExit(sef)
	}
}

func (ses *syscallFilterSet) len() int {
	return len(ses.enter) + len(ses.exit)
}

func (sfs *syscallFilterSet) registerEvents(monitor *perf.EventMonitor) {
	var enter_filters []string
	for _, f := range sfs.enter {
		s := f.String()
		if len(s) > 0 {
			enter_filters = append(enter_filters, fmt.Sprintf("(%s)", s))
		}
	}
	enter_filter := strings.Join(enter_filters, " || ")

	var exit_filters []string
	for _, f := range sfs.exit {
		s := f.String()
		if len(s) > 0 {
			exit_filters = append(exit_filters, fmt.Sprintf("(%s)", s))
		}
	}
	exit_filter := strings.Join(exit_filters, " || ")

	eventName := "raw_syscalls/sys_enter"
	err := monitor.RegisterEvent(eventName, decodeSysEnter, enter_filter, nil)
	if err != nil {
		glog.Infof("Couldn't get %s event id: %v", eventName, err)
	}

	eventName = "raw_syscalls/sys_exit"
	err = monitor.RegisterEvent(eventName, decodeSysExit, exit_filter, nil)
	if err != nil {
		glog.Infof("Couldn't get %s event id: %v", eventName, err)
	}
}
