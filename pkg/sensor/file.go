package sensor

import (
	"fmt"
	"reflect"
	"strings"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/perf"
	"github.com/golang/glog"
)

const (
	FS_DO_SYS_OPEN_KPROBE_NAME      = "fs/do_sys_open"
	FS_DO_SYS_OPEN_KPROBE_ADDRESS   = "do_sys_open"
	FS_DO_SYS_OPEN_KPROBE_FETCHARGS = "filename=+0(%si):string flags=%dx:s32 mode=%cx:s32"
)

func decodeDoSysOpen(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	ev := newEventFromSample(sample, data)
	ev.Event = &api.Event_File{
		File: &api.FileEvent{
			Type:      api.FileEventType_FILE_EVENT_TYPE_OPEN,
			Filename:  data["filename"].(string),
			OpenFlags: data["flags"].(int32),
			OpenMode:  data["mode"].(int32),
		},
	}

	return ev, nil
}

type fileOpenFilter struct {
	filename        string
	filenamePattern string
	openFlagsMask   int32
	createModeMask  int32
}

func newFileOpenFilter(fef *api.FileEventFilter) *fileOpenFilter {
	if fef.Type != api.FileEventType_FILE_EVENT_TYPE_OPEN {
		return nil
	}

	filter := &fileOpenFilter{}

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

	return filter
}

func (f *fileOpenFilter) String() string {
	var parts []string

	if len(f.filename) > 0 {
		parts = append(parts, fmt.Sprintf("filename == %s", f.filename))
	}
	if len(f.filenamePattern) > 0 {
		// Shouldn't we be taking any filename and do the filtering
		// ourselves using the pattern?
		parts = append(parts, fmt.Sprintf("filename == %s", f.filenamePattern))
	}
	if f.openFlagsMask != 0 {
		parts = append(parts, fmt.Sprintf("flags & %d", f.openFlagsMask))
	}
	if f.createModeMask != 0 {
		parts = append(parts, fmt.Sprintf("mode & %d", f.createModeMask))
	}

	return strings.Join(parts, " && ")
}

type fileFilterSet struct {
	filters []*fileOpenFilter
}

func (fes *fileFilterSet) add(fef *api.FileEventFilter) {
	filter := newFileOpenFilter(fef)
	if filter == nil {
		return
	}

	for _, v := range fes.filters {
		if reflect.DeepEqual(filter, v) {
			return
		}
	}

	fes.filters = append(fes.filters, filter)
}

func (fes *fileFilterSet) len() int {
	return len(fes.filters)
}

func (ffs *fileFilterSet) registerEvents(monitor *perf.EventMonitor) {
	var filters []string
	for _, f := range ffs.filters {
		s := f.String()
		if len(s) > 0 {
			filters = append(filters, fmt.Sprintf("(%s)", s))
		}
	}
	filter := strings.Join(filters, " || ")

	err := monitor.RegisterEvent("fs/do_sys_open", decodeDoSysOpen, filter, nil)
	if err != nil {
		glog.Infof("Tracepoint fs/do_sys_open not found, adding a kprobe to emulate")

		_, err := perf.AddKprobe(FS_DO_SYS_OPEN_KPROBE_NAME, FS_DO_SYS_OPEN_KPROBE_ADDRESS, false, FS_DO_SYS_OPEN_KPROBE_FETCHARGS)
		if err != nil {
			glog.Infof("Couldn't add do_sys_open kprobe: %s", err)
			return
		}
		err = monitor.RegisterEvent("fs/do_sys_open", decodeDoSysOpen, filter, nil)
		if err != nil {
			glog.Infof("Couldn't get trace event ID for kprobe fs/do_sys_open")
			return
		}
	}
}
