package sensor

import (
	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/perf"
	"github.com/golang/glog"
)

func decodeSchedProcessFork(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	parentPid := int32(data["parent_pid"].(uint64))
	childPid := int32(data["child_pid"].(uint64))

	// Notify pidmap of fork event
	pidMapOnFork(parentPid, childPid)

	ev := newEventFromSample(sample, data)
	ev.Event = &api.Event_Process{
		Process: &api.ProcessEvent{
			Type:         api.ProcessEventType_PROCESS_EVENT_TYPE_FORK,
			ForkChildPid: childPid,
		},
	}

	return ev, nil
}

func decodeSchedProcessExec(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	ev := newEventFromSample(sample, data)

	processEvent := &api.ProcessEvent{
		Type:         api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
		ExecFilename: data["filename"].(string),
	}

	var err error
	processEvent.ExecCommandLine, err = pidMapGetCommandLine(ev.ProcessPid)
	if err != nil {
		return nil, err
	}

	ev.Event = &api.Event_Process{
		Process: processEvent,
	}

	return ev, nil
}

func decodeSysEnterExitGroup(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	ev := newEventFromSample(sample, data)
	ev.Event = &api.Event_Process{
		Process: &api.ProcessEvent{
			Type:     api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT,
			ExitCode: int32(data["error_code"].(uint64)),
		},
	}

	return ev, nil
}

type processFilterSet struct {
	events map[api.ProcessEventType]struct{}
}

func (pes *processFilterSet) add(pef *api.ProcessEventFilter) {
	if pes.events == nil {
		pes.events = make(map[api.ProcessEventType]struct{})
	}

	pes.events[pef.Type] = struct{}{}
}

func (pes *processFilterSet) len() int {
	return len(pes.events)
}

func (pes *processFilterSet) registerEvents(monitor *perf.EventMonitor) {
	eventName := "sched/sched_process_fork"
	err := monitor.RegisterEvent(eventName, decodeSchedProcessFork, "", nil)
	if err != nil {
		glog.Infof("Couldn't get %s event id: %v", eventName, err)
	}

	eventName = "sched/sched_process_exec"
	err = monitor.RegisterEvent(eventName, decodeSchedProcessExec, "", nil)
	if err != nil {
		glog.Infof("Couldn't get %s event id: %v", eventName, err)
	}

	eventName = "syscalls/sys_enter_exit_group"
	err = monitor.RegisterEvent(eventName, decodeSysEnterExitGroup, "", nil)
	if err != nil {
		glog.Infof("Couldn't get %s event id: %v", eventName, err)
	}
}
