package sensor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	api "github.com/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/expression"
	"github.com/capsule8/capsule8/pkg/sys/perf"

	"github.com/golang/glog"
)

type syscallFilter struct {
	sensor *Sensor
}

func (f *syscallFilter) decodeSysEnter(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
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

	ev := f.sensor.NewEventFromSample(sample, data)
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

func (f *syscallFilter) decodeSysExit(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	ev := f.sensor.NewEventFromSample(sample, data)
	ev.Event = &api.Event_Syscall{
		Syscall: &api.SyscallEvent{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT,
			Id:   data["id"].(int64),
			Ret:  data["ret"].(int64),
		},
	}

	return ev, nil
}

func containsIdFilter(expr *api.Expression) bool {
	if expr == nil {
		return false
	}

	switch expr.GetType() {
	case api.Expression_LOGICAL_AND:
		operands := expr.GetBinaryOp()
		return containsIdFilter(operands.Lhs) ||
			containsIdFilter(operands.Rhs)
	case api.Expression_LOGICAL_OR:
		operands := expr.GetBinaryOp()
		return containsIdFilter(operands.Lhs) &&
			containsIdFilter(operands.Rhs)
	case api.Expression_EQ:
		operands := expr.GetBinaryOp()
		if operands.Lhs.GetType() != api.Expression_IDENTIFIER {
			return false
		}
		if operands.Lhs.GetIdentifier() != "id" {
			return false
		}
		return true
	}
	return false
}

func rewriteSyscallEventFilter(sef *api.SyscallEventFilter) {
	if sef.Id != nil {
		newExpr := expression.Equal(
			expression.Identifier("id"),
			expression.Value(sef.Id.Value))
		sef.FilterExpression = expression.LogicalAnd(
			newExpr, sef.FilterExpression)
		sef.Id = nil
	}
	if sef.Type == api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT {
		if sef.Ret != nil {
			newExpr := expression.Equal(
				expression.Identifier("ret"),
				expression.Value(sef.Ret.Value))
			sef.FilterExpression = expression.LogicalAnd(
				newExpr, sef.FilterExpression)
			sef.Ret = nil
		}
	}
}

func registerSyscallEvents(monitor *perf.EventMonitor, sensor *Sensor, events []*api.SyscallEventFilter) []uint64 {
	enterFilters := make(map[string]bool)
	exitFilters := make(map[string]bool)

	for _, sef := range events {
		// Translate deprecated fields into an expression
		rewriteSyscallEventFilter(sef)

		if !containsIdFilter(sef.FilterExpression) {
			// No wildcard filters for now
			continue
		}

		expr, err := expression.NewExpression(sef.FilterExpression)
		if err != nil {
			glog.V(1).Infof("Invalid syscall event filter: %s", err)
			continue
		}
		err = expr.ValidateKernelFilter()
		if err != nil {
			glog.V(1).Infof("Invalid syscall event filter as kernel filter: %s", err)
			continue
		}
		s := expr.KernelFilterString()

		switch sef.Type {
		case api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER:
			enterFilters[s] = true
		case api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT:
			exitFilters[s] = true
		default:
			continue
		}
	}

	f := syscallFilter{
		sensor: sensor,
	}

	var eventIDs []uint64

	if len(enterFilters) > 0 {
		filters := make([]string, 0, len(enterFilters))
		for k := range enterFilters {
			filters = append(filters, fmt.Sprintf("(%s)", k))
		}
		filter := strings.Join(filters, " || ")

		eventName := "raw_syscalls/sys_enter"
		eventID, err := monitor.RegisterTracepoint(eventName, f.decodeSysEnter,
			perf.WithFilter(filter))
		if err != nil {
			glog.V(1).Infof("Couldn't get %s event id: %v", eventName, err)
		} else {
			eventIDs = append(eventIDs, eventID)
		}
	}

	if len(exitFilters) > 0 {
		filters := make([]string, 0, len(exitFilters))
		for k := range exitFilters {
			filters = append(filters, fmt.Sprintf("(%s)", k))
		}
		filter := strings.Join(filters, " || ")

		eventName := "raw_syscalls/sys_exit"
		eventID, err := monitor.RegisterTracepoint(eventName, f.decodeSysExit,
			perf.WithFilter(filter))
		if err != nil {
			glog.V(1).Infof("Couldn't get %s event id: %v", eventName, err)
		} else {
			eventIDs = append(eventIDs, eventID)
		}
	}

	return eventIDs
}
