package sensor

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/expression"
	"github.com/capsule8/capsule8/pkg/sys/perf"

	"github.com/golang/glog"
)

type kprobeFilter struct {
	symbol    string
	onReturn  bool
	arguments map[string]string
	filter    string
	sensor    *Sensor
}

var validSymbolRegex *regexp.Regexp = regexp.MustCompile("^[A-Za-z_]{1}[\\w]*$")

func newKprobeFilter(kef *api.KernelFunctionCallFilter) *kprobeFilter {
	// The symbol must begin with [A-Za-z_] and contain only [A-Za-z0-9_]
	// We do not accept addresses or offsets
	if !validSymbolRegex.MatchString(kef.Symbol) {
		return nil
	}

	var filterString string

	if kef.FilterExpression != nil {
		expr, err := expression.NewExpression(kef.FilterExpression)
		if err != nil {
			glog.V(1).Infof("Bad kprobe filter expression: %s", err)
			return nil
		}

		filterString = expr.KernelFilterString()
	}

	filter := &kprobeFilter{
		symbol:    kef.Symbol,
		arguments: kef.Arguments,
		filter:    filterString,
	}

	switch kef.Type {
	case api.KernelFunctionCallEventType_KERNEL_FUNCTION_CALL_EVENT_TYPE_ENTER:
		filter.onReturn = false
	case api.KernelFunctionCallEventType_KERNEL_FUNCTION_CALL_EVENT_TYPE_EXIT:
		filter.onReturn = true
	default:
		return nil
	}

	return filter
}

func (f *kprobeFilter) decodeKprobe(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	args := make(map[string]*api.KernelFunctionCallEvent_FieldValue)
	for k, v := range data {
		value := &api.KernelFunctionCallEvent_FieldValue{}
		switch v := v.(type) {
		case []byte:
			value.FieldType = api.KernelFunctionCallEvent_BYTES
			value.Value = &api.KernelFunctionCallEvent_FieldValue_BytesValue{BytesValue: v}
		case string:
			value.FieldType = api.KernelFunctionCallEvent_STRING
			value.Value = &api.KernelFunctionCallEvent_FieldValue_StringValue{StringValue: v}
		case int8:
			value.FieldType = api.KernelFunctionCallEvent_SINT8
			value.Value = &api.KernelFunctionCallEvent_FieldValue_SignedValue{SignedValue: int64(v)}
		case int16:
			value.FieldType = api.KernelFunctionCallEvent_SINT16
			value.Value = &api.KernelFunctionCallEvent_FieldValue_SignedValue{SignedValue: int64(v)}
		case int32:
			value.FieldType = api.KernelFunctionCallEvent_SINT32
			value.Value = &api.KernelFunctionCallEvent_FieldValue_SignedValue{SignedValue: int64(v)}
		case int64:
			value.FieldType = api.KernelFunctionCallEvent_SINT64
			value.Value = &api.KernelFunctionCallEvent_FieldValue_SignedValue{SignedValue: v}
		case uint8:
			value.FieldType = api.KernelFunctionCallEvent_UINT8
			value.Value = &api.KernelFunctionCallEvent_FieldValue_UnsignedValue{UnsignedValue: uint64(v)}
		case uint16:
			value.FieldType = api.KernelFunctionCallEvent_UINT16
			value.Value = &api.KernelFunctionCallEvent_FieldValue_UnsignedValue{UnsignedValue: uint64(v)}
		case uint32:
			value.FieldType = api.KernelFunctionCallEvent_UINT32
			value.Value = &api.KernelFunctionCallEvent_FieldValue_UnsignedValue{UnsignedValue: uint64(v)}
		case uint64:
			value.FieldType = api.KernelFunctionCallEvent_UINT64
			value.Value = &api.KernelFunctionCallEvent_FieldValue_UnsignedValue{UnsignedValue: v}
		}
		args[k] = value
	}

	ev := f.sensor.NewEventFromSample(sample, data)
	ev.Event = &api.Event_KernelCall{
		KernelCall: &api.KernelFunctionCallEvent{
			Arguments: args,
		},
	}

	return ev, nil
}

func (f *kprobeFilter) fetchargs() string {
	args := make([]string, 0, len(f.arguments))
	for k, v := range f.arguments {
		args = append(args, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(args, " ")
}

func registerKernelEvents(monitor *perf.EventMonitor, sensor *Sensor, events []*api.KernelFunctionCallFilter) {
	for _, kef := range events {
		f := newKprobeFilter(kef)
		if f == nil {
			glog.V(1).Infof("Invalid kprobe symbol: %s", kef.Symbol)
			continue
		}

		f.sensor = sensor
		_, err := monitor.RegisterKprobe(
			f.symbol, f.onReturn, f.fetchargs(),
			f.decodeKprobe,
			f.filter,
			nil)
		if err != nil {
			var loc string
			if f.onReturn {
				loc = "return"
			} else {
				loc = "entry"
			}

			glog.V(1).Infof("Couldn't register kprobe on %s %s [%s]: %v",
				f.symbol, loc, f.fetchargs(), err)
			continue
		}
	}
}
