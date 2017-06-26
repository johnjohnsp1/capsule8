/*
Package subscription implements sensor event subscription handling for
the node sensor. The node sensor receives Subscription protobuf messages
from the backplane and calls subscription.Add() and subscription.Remove()
to notify this package of explicit subscription addition and removals.
*/
package subscription

import (
	"reflect"

	"github.com/capsule8/reactive8/pkg/api/event"
)

//
// This package is a bridge between the protocol buffer-based Subscription
// APIs in pkg/api and other packages. Types exported from it do not depend
// upon types defined in pkg/api.
//

//
// Tracepoints and kprobes for system calls can be attached on syscall enter
// and/or syscall exit, so we track the filters for enter/exit separately.
//

type syscallEnterFilter struct {
	nr   int64
	args map[uint8]uint64
}

type syscallExitFilter struct {
	nr int64

	// Pointer is nil if no return value was specified
	ret *uint64
}

type syscallFilterSet struct {
	// enter events
	enter []syscallEnterFilter

	// exit events
	exit []syscallExitFilter
}

func (ses *syscallFilterSet) addEnter(sef *event.SyscallEventFilter) {
	syscallEnter := &syscallEnterFilter{
		nr: sef.Id.Value,
	}

	m := make(map[uint8]uint64)

	if sef.Arg0 != nil {
		m[0] = sef.Arg0.Value
	}

	if sef.Arg1 != nil {
		m[1] = sef.Arg1.Value
	}

	if sef.Arg2 != nil {
		m[2] = sef.Arg2.Value
	}

	if sef.Arg3 != nil {
		m[3] = sef.Arg3.Value
	}

	if sef.Arg4 != nil {
		m[4] = sef.Arg4.Value
	}

	if sef.Arg5 != nil {
		m[5] = sef.Arg5.Value
	}

	if len(m) > 0 {
		syscallEnter.args = m
	}

	found := false
	for _, v := range ses.enter {
		if reflect.DeepEqual(syscallEnter, v) {
			found = true
			break
		}
	}

	if !found {
		ses.enter = append(ses.enter, *syscallEnter)
	}
}

func (ses *syscallFilterSet) addExit(sef *event.SyscallEventFilter) {
	syscallExit := &syscallExitFilter{
		nr: sef.Id.Value,
	}

	if sef.Ret != nil {
		syscallExit.ret = &sef.Ret.Value
	}

	found := false
	for _, v := range ses.enter {
		if reflect.DeepEqual(syscallExit, v) {
			found = true
			break
		}
	}

	if !found {
		ses.exit = append(ses.exit, *syscallExit)
	}
}

func (ses *syscallFilterSet) add(sef *event.SyscallEventFilter) {
	if sef.Id == nil {
		// System call number is required.
		return
	}

	switch sef.Type {
	case event.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER:
		ses.addEnter(sef)

	case event.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT:
		ses.addExit(sef)
	}

}

func (ses *syscallFilterSet) len() int {
	return len(ses.enter) + len(ses.exit)
}

type processFilterSet struct {
	events map[event.ProcessEventType]struct{}
}

func (pes *processFilterSet) add(pef *event.ProcessEventFilter) {
	if pes.events == nil {
		pes.events = make(map[event.ProcessEventType]struct{})
	}

	pes.events[pef.Type] = struct{}{}
}

func (pes *processFilterSet) len() int {
	return len(pes.events)
}

type fileOpenFilter struct {
	filename        string
	filenamePattern string
	openFlagsMask   int32
	createModeMask  int32
}

type fileFilterSet struct {
	filters []*fileOpenFilter
}

func (fes *fileFilterSet) add(fef *event.FileEventFilter) {
	if fef.Type != event.FileEventType_FILE_EVENT_TYPE_OPEN {
		// Do nothing if UNKNOWN
		return
	}

	filter := new(fileOpenFilter)

	if fef.Filename != nil {
		filter.filename = fef.Filename.Value
	}

	if fef.FilenamePattern != nil {
		filter.filenamePattern = fef.FilenamePattern.Value
	}

	if fef.OpenFlagsMask != nil {
		filter.openFlagsMask = fef.OpenFlagsMask.Value
	}

	if fef.CreateModeMask != nil {
		filter.createModeMask = fef.CreateModeMask.Value
	}

	found := false
	for _, v := range fes.filters {
		if reflect.DeepEqual(filter, v) {
			found = true
			break
		}
	}

	if !found {
		fes.filters = append(fes.filters, filter)
	}
}

func (fes *fileFilterSet) len() int {
	return len(fes.filters)
}

//
// FilterSet represents the union of all requested events for a
// subscription. It consists of sub-event sets for each supported type of
// event, which similarly represent the union of all requested events of
// a given type.
//
// The perf event backend translates a FilterSet into a set of tracepoint,
// kprobe, and uprobe events with ftrace filters. The eBPF backend translates
// a FilterSet into an eBPF program on tracepoints, kprobes, and uprobes.
//
type FilterSet struct {
	syscalls  *syscallFilterSet
	processes *processFilterSet
	files     *fileFilterSet
}

func (fs *FilterSet) addSyscallEventFilter(sef *event.SyscallEventFilter) {
	if fs.syscalls == nil {
		fs.syscalls = new(syscallFilterSet)
	}

	fs.syscalls.add(sef)
}

func (fs *FilterSet) addProcessEventFilter(pef *event.ProcessEventFilter) {
	if fs.processes == nil {
		fs.processes = new(processFilterSet)
	}

	fs.processes.add(pef)
}

func (fs *FilterSet) addFileEventFilter(fef *event.FileEventFilter) {
	if fs.files == nil {
		fs.files = new(fileFilterSet)
	}

	fs.files.add(fef)
}

func (fs *FilterSet) Len() int {
	length := 0

	if fs.syscalls != nil {
		length += fs.syscalls.len()
	}

	if fs.processes != nil {
		length += fs.processes.len()
	}

	if fs.files != nil {
		length += fs.files.len()
	}

	return length
}

func Add(subscription *event.Subscription) *FilterSet {
	//
	// Coalesce subscription into a FilterSet of all events filters
	// specified
	//

	filterSet := new(FilterSet)

	if subscription.Events != nil {
		for _, ef := range subscription.Events {
			switch ef.Filter.(type) {
			case *event.EventFilter_Syscall:
				sef := ef.GetSyscall()
				if sef != nil {
					filterSet.addSyscallEventFilter(sef)
				}

			case *event.EventFilter_Process:
				pef := ef.GetProcess()
				if pef != nil {
					filterSet.addProcessEventFilter(pef)
				}

			case *event.EventFilter_File:
				fef := ef.GetFile()
				if fef != nil {
					filterSet.addFileEventFilter(fef)
				}
			}
		}
	}

	return filterSet
}

func Remove(subscription *event.Subscription) {
	// TODO
}
