package subscription

import (
	"fmt"
	"strings"

	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/perf"
)

func (sfs *syscallFilterSet) getPerfFilters(filters map[uint16]string) {
	sysEnterID, _ :=
		perf.GetTraceEventID("raw_syscalls/sys_enter")

	sysExitID, _ :=
		perf.GetTraceEventID("raw_syscalls/sys_exit")

	var enter_filters []string
	var exit_filters []string

	for _, f := range sfs.enter {
		var filters []string

		s := fmt.Sprintf("id == %d", f.nr)
		filters = append(filters, s)

		for a, v := range f.args {
			s := fmt.Sprintf("args[%d] == %x", a, v)
			filters = append(filters, s)
		}

		s = fmt.Sprintf("(%s)", strings.Join(filters, " && "))
		enter_filters = append(enter_filters, s)
	}

	for _, f := range sfs.exit {
		var filters []string

		s := fmt.Sprintf("id == %d", f.nr)
		filters = append(filters, s)

		if f.ret != nil {
			s := fmt.Sprintf("ret == %x", *f.ret)
			filters = append(filters, s)
		}

		s = fmt.Sprintf("(%s)", strings.Join(filters, " && "))
		exit_filters = append(exit_filters, s)
	}

	if len(enter_filters) > 0 {
		filters[sysEnterID] = strings.Join(enter_filters, " || ")
	}

	if len(exit_filters) > 0 {
		filters[sysExitID] = strings.Join(exit_filters, " || ")
	}
}

func (pfs *processFilterSet) getPerfFilters(filters map[uint16]string) {
	_, ok := pfs.events[event.ProcessEventType_PROCESS_EVENT_TYPE_FORK]
	if ok {
		eventID, _ := perf.GetTraceEventID("sched/sched_process_fork")

		filters[eventID] = ""
	}

	_, ok = pfs.events[event.ProcessEventType_PROCESS_EVENT_TYPE_EXEC]
	if ok {
		eventID, _ := perf.GetTraceEventID("sched/sched_process_exec")

		filters[eventID] = ""
	}

	_, ok = pfs.events[event.ProcessEventType_PROCESS_EVENT_TYPE_EXIT]
	if ok {
		eventID, _ :=
			perf.GetTraceEventID("syscalls/sys_enter_exit_group")

		filters[eventID] = ""
	}
}

func (ffs *fileFilterSet) getPerfFilters(filters map[uint16]string) {
	eventID, _ := perf.GetTraceEventID("fs/do_sys_open")

	var open_filters []string

	for _, f := range ffs.filters {
		var filters []string

		if len(f.filename) > 0 {
			s := fmt.Sprintf("filename == %s", f.filename)
			filters = append(filters, s)
		}

		if len(f.filenamePattern) > 0 {
			s := fmt.Sprintf("filename == %s", f.filenamePattern)
			filters = append(filters, s)
		}

		if f.openFlagsMask != 0 {
			s := fmt.Sprintf("flags & %x", f.openFlagsMask)
			filters = append(filters, s)
		}

		if f.createModeMask != 0 {
			s := fmt.Sprintf("mode & %x", f.createModeMask)
			filters = append(filters, s)
		}

		s := fmt.Sprintf("(%s)", strings.Join(filters, " && "))
		open_filters = append(open_filters, s)
	}

	filters[eventID] = strings.Join(open_filters, " || ")
}

func (fs *FilterSet) GetPerfFilters() map[uint16]string {
	filters := make(map[uint16]string)

	if fs.syscalls != nil {
		fs.syscalls.getPerfFilters(filters)
	}

	if fs.processes != nil {
		fs.processes.getPerfFilters(filters)
	}

	if fs.files != nil {
		fs.files.getPerfFilters(filters)
	}

	return filters
}
