package sensor

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/perf"
	"github.com/golang/glog"
)

func decodeKprobe(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
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
			value.Value = &api.KernelFunctionCallEvent_FieldValue_SignedIntValue{SignedIntValue: int64(v)}
		case int16:
			value.FieldType = api.KernelFunctionCallEvent_SINT16
			value.Value = &api.KernelFunctionCallEvent_FieldValue_SignedIntValue{SignedIntValue: int64(v)}
		case int32:
			value.FieldType = api.KernelFunctionCallEvent_SINT32
			value.Value = &api.KernelFunctionCallEvent_FieldValue_SignedIntValue{SignedIntValue: int64(v)}
		case int64:
			value.FieldType = api.KernelFunctionCallEvent_SINT64
			value.Value = &api.KernelFunctionCallEvent_FieldValue_SignedIntValue{SignedIntValue: v}
		case uint8:
			value.FieldType = api.KernelFunctionCallEvent_UINT8
			value.Value = &api.KernelFunctionCallEvent_FieldValue_UnsignedIntValue{UnsignedIntValue: uint64(v)}
		case uint16:
			value.FieldType = api.KernelFunctionCallEvent_UINT16
			value.Value = &api.KernelFunctionCallEvent_FieldValue_UnsignedIntValue{UnsignedIntValue: uint64(v)}
		case uint32:
			value.FieldType = api.KernelFunctionCallEvent_UINT32
			value.Value = &api.KernelFunctionCallEvent_FieldValue_UnsignedIntValue{UnsignedIntValue: uint64(v)}
		case uint64:
			value.FieldType = api.KernelFunctionCallEvent_UINT64
			value.Value = &api.KernelFunctionCallEvent_FieldValue_UnsignedIntValue{UnsignedIntValue: v}
		}
		args[k] = value
	}

	ev := newEventFromSample(sample, data)
	ev.Event = &api.Event_KernelCall{
		KernelCall: &api.KernelFunctionCallEvent{
			Arguments: args,
		},
	}

	return ev, nil
}

type kprobeFilter struct {
	name      string
	symbol    string
	onReturn  bool
	arguments map[string]string
	filter    string
}

var validSymbolRegex *regexp.Regexp = regexp.MustCompile("^[A-Za-z_]{1}[\\w]*$")

func newKprobeFilter(kef *api.KernelFunctionCallFilter) *kprobeFilter {
	// The symbol must begin with [A-Za-z_] and contain only [A-Za-z0-9_]
	// We do not accept addresses or offsets
	if !validSymbolRegex.MatchString(kef.Symbol) {
		return nil
	}

	// Choose a name to refer to the kprobe by. We could allow the kernel
	// to assign a name, but getting the right name back from the kernel
	// can be somewhat unreliable. Choosing our own random name ensures
	// that we're always dealing with the right event and that we're not
	// stomping on some other process's probe
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	name := fmt.Sprintf("capsule8/sensor_%s", hex.EncodeToString(randomBytes))

	filter := &kprobeFilter{
		name:      name,
		symbol:    kef.Symbol,
		arguments: kef.Arguments,
		filter:    kef.Filter,
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

func (f *kprobeFilter) fetchargs() string {
	args := make([]string, 0, len(f.arguments))
	for k, v := range f.arguments {
		args = append(args, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(args, " ")
}

func (f *kprobeFilter) String() string {
	return f.filter
}

type kprobeFilterSet struct {
	filters []*kprobeFilter
}

func (kes *kprobeFilterSet) add(kef *api.KernelFunctionCallFilter) {
	filter := newKprobeFilter(kef)
	if filter == nil {
		return
	}

	for _, v := range kes.filters {
		if reflect.DeepEqual(filter, v) {
			return
		}
	}

	kes.filters = append(kes.filters, filter)
}

func (kes *kprobeFilterSet) len() int {
	return len(kes.filters)
}

func (kes *kprobeFilterSet) registerEvents(monitor *perf.EventMonitor) {
	for _, f := range kes.filters {
		name, err := monitor.RegisterKprobe(
			f.name, f.symbol, f.onReturn, f.fetchargs(),
			decodeKprobe,
			f.filter,
			nil)
		if err != nil {
			glog.Infof("Couldn't register kprobe: %v", err)
			continue
		}
		f.name = name
	}
}
