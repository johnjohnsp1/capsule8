package subscription

import (
	"path/filepath"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/perf"
	"github.com/capsule8/reactive8/pkg/process"
	"github.com/capsule8/reactive8/pkg/sys"
	"github.com/golang/glog"
)

func decodeSchedProcessFork(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	//
	// Notify proc info cache of fork event ASAP
	//
	parentPid := data["parent_pid"].(int32)
	childPid := data["child_pid"].(int32)

	process.CacheUpdate(childPid, parentPid, "", "")

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
	//
	// Grab command line out of procfs ASAP
	//
	hostPid := data["common_pid"].(int32)
	commandLine := sys.HostProcFS().CommandLine(hostPid)

	// Update process cache with new command value
	filename := data["filename"].(string)
	_, command := filepath.Split(filename)
	process.CacheUpdate(hostPid, 0, command, "")

	ev := newEventFromSample(sample, data)
	processEvent := &api.ProcessEvent{
		Type:            api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
		ExecFilename:    filename,
		ExecCommandLine: commandLine,
	}

	ev.Event = &api.Event_Process{
		Process: processEvent,
	}

	return ev, nil
}

func decodeSysEnterExitGroup(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	// Notify process cache of process exit ASAP
	hostPid := data["common_pid"].(int32)
	process.CacheDelete(hostPid)

	// For "error_code", the value coming from the kernel is uint64, but
	// as far as I can tell, the kernel internally only ever really deals
	// in int for the process exit code. So, I'm going to just convert it
	// to sint32 here just like the kernel does.

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
