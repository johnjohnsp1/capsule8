package sensor

import (
	"fmt"

	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/process"
	"github.com/capsule8/reactive8/pkg/stream"
)

//
// The Process
//

func translateProcessEvents(e interface{}) interface{} {
	ev := e.(*process.Event)
	pev := &event.ProcessEvent{}

	switch ev.State {
	case process.ProcessFork:
		pev.Type = event.ProcessEventType_PROCESS_EVENT_TYPE_FORK
		pev.Pid = int32(ev.Pid)
		pev.ChildPid = int32(ev.ForkPid)

	case process.ProcessExec:
		pev.Type = event.ProcessEventType_PROCESS_EVENT_TYPE_EXEC
		pev.Pid = int32(ev.Pid)
		pev.ExecFilename = ev.ExecFilename

	case process.ProcessExit:
		pev.Type = event.ProcessEventType_PROCESS_EVENT_TYPE_EXIT
		pev.Pid = int32(ev.Pid)
		pev.ExitCode = int32(ev.ExitStatus)

	default:
		panic(fmt.Sprintf("Unknown process event state %v", ev.State))
	}

	return &event.Event{
		Event: &event.Event_Process{
			Process: pev,
		},
	}
}

// NewSensor creates a new ContainerEvent sensor
func NewProcessSensor(sub *event.Subscription) (*stream.Stream, error) {

	/*
		for _, processFilter := range sub.Processes {
			switch processFilter.Filter.(type) {
			case *event.ProcessFilter_Pid:
				pid := processFilter.GetPid()
				s, err := process.NewEventStreamForPid(int(pid))
				if err != nil {
					return nil, err
				}

			case *event.ProcessFilter_Cgroup:
				name := processFilter.GetCgroup()
				s, err := process.NewEventStreamForCgroup(name)
				if err != nil {
					return nil, err
				}

			case nil:
				s, err := process.NewEventStream()
				if err != nil {
					return nil, err
				}
			}
		}
	*/
	s, err := process.NewEventStream()
	if err != nil {
		return nil, err
	}

	s = stream.Map(s, translateProcessEvents)

	// TODO: filter event stream

	return s, err
}
