package subscription

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"strings"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/capsule8/pkg/sys/perf"
	"github.com/golang/glog"
)

func decodeSysEnter(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	var args = data["args"].([]interface{})

	// Some kernel versions misreport the args type information
	var parsedArgs []uint64
	if len(args) > 0 {
		switch args[0].(type) {
		case int8:
			buf := []byte{}
			for _, v := range args {
				buf = append(buf, byte(v.(int8)))
			}
			parsedArgs = make([]uint64, len(buf)/8)
			r := bytes.NewReader(buf)
			binary.Read(r, binary.LittleEndian, parsedArgs)
		case uint8:
			buf := []byte{}
			for _, v := range args {
				buf = append(buf, byte(v.(uint8)))
			}
			parsedArgs = make([]uint64, len(buf)/8)
			r := bytes.NewReader(buf)
			binary.Read(r, binary.LittleEndian, parsedArgs)
		case int64:
			parsedArgs = make([]uint64, len(args))
			for i, v := range args {
				parsedArgs[i] = uint64(v.(int64))
			}
		case uint64:
			parsedArgs = make([]uint64, len(args))
			for i, v := range args {
				parsedArgs[i] = v.(uint64)
			}
		}
	}

	ev := newEventFromSample(sample, data)
	ev.Event = &api.Event_Syscall{
		Syscall: &api.SyscallEvent{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER,
			Id:   data["id"].(int64),
			Arg0: parsedArgs[0],
			Arg1: parsedArgs[1],
			Arg2: parsedArgs[2],
			Arg3: parsedArgs[3],
			Arg4: parsedArgs[4],
			Arg5: parsedArgs[5],
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
