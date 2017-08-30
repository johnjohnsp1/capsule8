package sensor

import (
	"log"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/perf"
)

func decodeSchedProcessFork(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	parentPid := int32(data["parent_pid"].(uint64))
	childPid := int32(data["child_pid"].(uint64))

	// Notify pidmap of fork event
	pidMapOnFork(parentPid, childPid)

	ev := newEventFromFieldData(data)
	ev.Event = &api.Event_Process{
		Process: &api.ProcessEvent{
			Type:         api.ProcessEventType_PROCESS_EVENT_TYPE_FORK,
			ForkChildPid: childPid,
		},
	}

	return ev, nil
}

func decodeSchedProcessExec(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	ev := newEventFromFieldData(data)

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
	ev := newEventFromFieldData(data)
	ev.Event = &api.Event_Process{
		Process: &api.ProcessEvent{
			Type:     api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT,
			ExitCode: int32(data["error_code"].(uint64)),
		},
	}

	return ev, nil
}

// -----------------------------------------------------------------------------

func init() {
	sensor := getSensor()

	eventName := "sched/sched_process_fork"
	err := sensor.registerDecoder(eventName, decodeSchedProcessFork)
	if err != nil {
		log.Printf("Couldn't get %s event id: %v", eventName, err)
	}

	eventName = "sched/sched_process_exec"
	err = sensor.registerDecoder(eventName, decodeSchedProcessExec)
	if err != nil {
		log.Printf("Couldn't get %s event id: %v", eventName, err)
	}

	eventName = "syscalls/sys_enter_exit_group"
	err = sensor.registerDecoder(eventName, decodeSysEnterExitGroup)
	if err != nil {
		log.Printf("Couldn't get %s event id: %v", eventName, err)
	}
}
