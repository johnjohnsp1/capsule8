package sensor

import (
	"fmt"
	"strings"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/capsule8/pkg/sys/perf"
	"github.com/golang/glog"
)

const (
	fsDoSysOpenKprobeAddress   = "do_sys_open"
	fsDoSysOpenKprobeFetchargs = "filename=+0(%si):string flags=%dx:s32 mode=%cx:s32"
)

type fileOpenFilter struct {
	sensor *Sensor
}

func (f *fileOpenFilter) decodeDoSysOpen(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	ev := f.sensor.NewEventFromSample(sample, data)
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

func fileFilterString(fef *api.FileEventFilter) string {
	var parts []string

	if fef.Filename != nil {
		parts = append(parts, fmt.Sprintf("filename == %s", fef.Filename.Value))
	}
	if fef.FilenamePattern != nil {
		parts = append(parts, fmt.Sprintf("filename ~ %s", fef.FilenamePattern.Value))
	}
	if fef.OpenFlagsMask != nil {
		parts = append(parts, fmt.Sprintf("flags & %d", fef.OpenFlagsMask.Value))
	}
	if fef.CreateModeMask != nil {
		parts = append(parts, fmt.Sprintf("mode & %d", fef.CreateModeMask.Value))
	}

	return strings.Join(parts, " && ")
}

func registerFileEvents(monitor *perf.EventMonitor, sensor *Sensor, events []*api.FileEventFilter) {
	var filter string

	wildcard := false
	filters := make(map[string]bool, len(events))
	for _, fef := range events {
		if fef.Type != api.FileEventType_FILE_EVENT_TYPE_OPEN {
			continue
		}

		filter = fileFilterString(fef)
		if len(filter) > 0 {
			filters[filter] = true
		} else {
			wildcard = true
			break
		}
	}

	if !wildcard {
		if len(filters) == 0 {
			return
		}

		parts := make([]string, 0, len(filters))
		for k := range filters {
			parts = append(parts, fmt.Sprintf("(%s)", k))
		}
		filter = strings.Join(parts, " || ")
	}

	f := fileOpenFilter{
		sensor: sensor,
	}

	err := monitor.RegisterEvent("fs/do_sys_open", f.decodeDoSysOpen, filter, nil)
	if err != nil {
		glog.V(1).Infof("Tracepoint fs/do_sys_open not found, adding a kprobe to emulate")

		name := perf.UniqueProbeName("capsule8", "do_sys_open")
		_, err = monitor.RegisterKprobe(
			name,
			fsDoSysOpenKprobeAddress,
			false,
			fsDoSysOpenKprobeFetchargs,
			f.decodeDoSysOpen,
			filter,
			nil)
		if err != nil {
			glog.Warning("Couldn't register kprobe fs/do_sys_open")
			return
		}
	}
}
