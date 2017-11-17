package sensor

import (
	"fmt"
	"strings"

	api "github.com/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/expression"
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

func rewriteFileEventFilter(fef *api.FileEventFilter) {
	if fef.Filename != nil {
		newExpr := expression.Equal(
			expression.Identifier("filename"),
			expression.Value(fef.Filename.Value))
		fef.FilterExpression = expression.LogicalAnd(
			newExpr, fef.FilterExpression)
		fef.Filename = nil
		fef.FilenamePattern = nil
	} else if fef.FilenamePattern != nil {
		newExpr := expression.Like(
			expression.Identifier("filename"),
			expression.Value(fef.FilenamePattern.Value))
		fef.FilterExpression = expression.LogicalAnd(
			newExpr, fef.FilterExpression)
		fef.FilenamePattern = nil
	}

	if fef.OpenFlagsMask != nil {
		newExpr := expression.BitwiseAnd(
			expression.Identifier("flags"),
			expression.Value(fef.OpenFlagsMask.Value))
		fef.FilterExpression = expression.LogicalAnd(
			newExpr, fef.FilterExpression)
		fef.OpenFlagsMask = nil
	}

	if fef.CreateModeMask != nil {
		newExpr := expression.BitwiseAnd(
			expression.Identifier("mode"),
			expression.Value(fef.OpenFlagsMask.Value))
		fef.FilterExpression = expression.LogicalAnd(
			newExpr, fef.FilterExpression)
		fef.CreateModeMask = nil
	}
}

func registerFileEvents(monitor *perf.EventMonitor, sensor *Sensor, events []*api.FileEventFilter) {
	var filterString string

	wildcard := false
	filters := make(map[string]bool, len(events))
	for _, fef := range events {
		if fef.Type != api.FileEventType_FILE_EVENT_TYPE_OPEN {
			continue
		}

		// Translate deprecated fields into an expression
		rewriteFileEventFilter(fef)

		if fef.FilterExpression == nil {
			wildcard = true
		} else {
			expr, err := expression.NewExpression(fef.FilterExpression)
			if err != nil {
				glog.V(1).Infof("Invalid file event filter: %s", err)
				continue
			}
			err = expr.ValidateKernelFilter()
			if err != nil {
				glog.V(1).Infof("Invalid file event filter as kernel filter: %s", err)
				continue
			}
			s := expr.KernelFilterString()
			filters[s] = true
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
		filterString = strings.Join(parts, " || ")
	}

	f := fileOpenFilter{
		sensor: sensor,
	}

	_, err := monitor.RegisterTracepoint("fs/do_sys_open", f.decodeDoSysOpen,
		perf.WithFilter(filterString))
	if err != nil {
		glog.V(1).Infof("Tracepoint fs/do_sys_open not found, adding a kprobe to emulate")

		_, err = monitor.RegisterKprobe(
			fsDoSysOpenKprobeAddress,
			false,
			fsDoSysOpenKprobeFetchargs,
			f.decodeDoSysOpen,
			perf.WithFilter(filterString))
		if err != nil {
			glog.Warning("Couldn't register kprobe fs/do_sys_open")
			return
		}
	}
}
