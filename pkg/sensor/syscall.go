// Copyright 2017 Capsule8, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sensor

import (
	"fmt"
	"strings"
	"sync/atomic"

	api "github.com/capsule8/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/expression"
	"github.com/capsule8/capsule8/pkg/sys/perf"

	"github.com/golang/glog"
)

type syscallFilter struct {
	sensor *Sensor
}

func (f *syscallFilter) decodeDummySysEnter(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	return nil, nil
}

func (f *syscallFilter) decodeSyscallTraceEnter(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	ev := f.sensor.NewEventFromSample(sample, data)
	ev.Event = &api.Event_Syscall{
		Syscall: &api.SyscallEvent{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER,
			Id:   data["id"].(int64),
			Arg0: data["arg0"].(uint64),
			Arg1: data["arg1"].(uint64),
			Arg2: data["arg2"].(uint64),
			Arg3: data["arg3"].(uint64),
			Arg4: data["arg4"].(uint64),
			Arg5: data["arg5"].(uint64),
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

func containsIDFilter(expr *api.Expression) bool {
	if expr == nil {
		return false
	}

	switch expr.GetType() {
	case api.Expression_LOGICAL_AND:
		operands := expr.GetBinaryOp()
		return containsIDFilter(operands.Lhs) ||
			containsIDFilter(operands.Rhs)
	case api.Expression_LOGICAL_OR:
		operands := expr.GetBinaryOp()
		return containsIDFilter(operands.Lhs) &&
			containsIDFilter(operands.Rhs)
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

const (
	syscallNewEnterKprobeAddress string = "syscall_trace_enter_phase1"
	syscallOldEnterKprobeAddress string = "syscall_trace_enter"

	// These offsets index into the x86_64 version of struct pt_regs
	// in the kernel. This is a stable structure.
	syscallEnterKprobeFetchargs string = "id=+120(%di):s64 " + // orig_ax
		"arg0=+112(%di):u64 " + // di
		"arg1=+104(%di):u64 " + // si
		"arg2=+96(%di):u64 " + // dx
		"arg3=+56(%di):u64 " + // r10
		"arg4=+72(%di):u64 " + // r8
		"arg5=+64(%di):u64" // r9
)

func registerSyscallEvents(sensor *Sensor, eventMap subscriptionMap, events []*api.SyscallEventFilter) {
	enterFilters := make(map[string]bool)
	exitFilters := make(map[string]bool)

	for _, sef := range events {
		// Translate deprecated fields into an expression
		rewriteSyscallEventFilter(sef)

		if !containsIDFilter(sef.FilterExpression) {
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

	if len(enterFilters) > 0 {
		filters := make([]string, 0, len(enterFilters))
		for k := range enterFilters {
			filters = append(filters, fmt.Sprintf("(%s)", k))
		}
		filter := strings.Join(filters, " || ")

		if atomic.AddInt64(&sensor.dummySyscallEventCount, 1) == 1 {
			// Create the dummy syscall event. This event is needed
			// to put the kernel into a mode where it'll make the
			// function calls needed to make the kprobe we'll add
			// fire. Add the tracepoint, but make sure it never
			// adds events into the ringbuffer by using a filter
			// that will never evaluate true.
			eventName := "raw_syscalls/sys_enter"
			eventID, err := sensor.monitor.RegisterTracepoint(
				eventName, f.decodeDummySysEnter,
				perf.WithFilter("id == 0x7fffffff"))
			if err != nil {
				glog.V(1).Infof("Couldn't register dummy syscall event %s: %v", eventName, err)
				atomic.AddInt64(&sensor.dummySyscallEventCount, -1)
			} else {
				sensor.dummySyscallEventID = eventID
			}
		}

		// There are two possible kprobes. Newer kernels (>= 4.1) have
		// refactored syscall entry code, so syscall_trace_enter_phase1
		// is the right one, but for older kernels syscall_trace_enter
		// is the right one. Both have the same signature, so the
		// fetchargs doesn't have to change. Try the new probe first,
		// because the old probe will also set in the newer kernels,
		// but it won't fire.
		eventID, err := sensor.monitor.RegisterKprobe(
			syscallNewEnterKprobeAddress, false,
			syscallEnterKprobeFetchargs,
			f.decodeSyscallTraceEnter,
			perf.WithFilter(filter))
		if err != nil {
			eventID, err = sensor.monitor.RegisterKprobe(
				syscallOldEnterKprobeAddress, false,
				syscallEnterKprobeFetchargs,
				f.decodeSyscallTraceEnter,
				perf.WithFilter(filter))
		}
		if err != nil {
			glog.V(1).Infof("Couldn't register syscall enter kprobe: %v", err)
		} else {
			eventMap[eventID] = &subscription{
				unregister: func(uint64, *subscription) {
					eventID := sensor.dummySyscallEventID
					if atomic.AddInt64(&sensor.dummySyscallEventCount, -1) == 0 {
						sensor.monitor.UnregisterEvent(eventID)
					}
				},
			}
		}
	}

	if len(exitFilters) > 0 {
		filters := make([]string, 0, len(exitFilters))
		for k := range exitFilters {
			filters = append(filters, fmt.Sprintf("(%s)", k))
		}
		filter := strings.Join(filters, " || ")

		eventName := "raw_syscalls/sys_exit"
		eventID, err := sensor.monitor.RegisterTracepoint(eventName, f.decodeSysExit,
			perf.WithFilter(filter))
		if err != nil {
			glog.V(1).Infof("Couldn't get %s event id: %v", eventName, err)
		} else {
			eventMap[eventID] = &subscription{}
		}
	}
}
