package functional

import (
	"testing"

	api "github.com/capsule8/api/v0"

	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes/wrappers"
)

type execTest struct {
	testContainer   *container
	err             error
	containerID     string
	containerExited bool
	processID       string
}

func (ct *execTest) buildContainer(t *testing.T) {
	c := newContainer(t, "exec")
	err := c.build()
	if err != nil {
		t.Error(err)
	} else {
		ct.testContainer = c
	}
}

func (ct *execTest) runContainer(t *testing.T) {
	err := ct.testContainer.run()
	if err != nil {
		t.Error(err)
	}
}

func (ct *execTest) createSubscription(t *testing.T) *api.Subscription {
	containerEvents := []*api.ContainerEventFilter{
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED,
		},
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_RUNNING,
		},
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED,
		},
	}

	processEvents := []*api.ProcessEventFilter{
		&api.ProcessEventFilter{
			Type: api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
			ExecFilenamePattern: &wrappers.StringValue{
				// Want to only match execs of 'dmesg'
				Value: "*mes*",
			},
		},
	}

	eventFilter := &api.EventFilter{
		ContainerEvents: containerEvents,
		ProcessEvents:   processEvents,
	}

	sub := &api.Subscription{
		EventFilter: eventFilter,
	}

	return sub
}

func (ct *execTest) handleTelemetryEvent(t *testing.T, telemetryEvent *api.TelemetryEvent) bool {
	glog.V(2).Infof("%+v", telemetryEvent)

	switch event := telemetryEvent.Event.Event.(type) {
	case *api.Event_Container:
		if event.Container.Type == api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED {
			if event.Container.ImageName == ct.testContainer.imageID {
				if len(ct.containerID) > 0 {
					t.Error("Already saw container created")
					return false
				}

				ct.containerID = telemetryEvent.Event.ContainerId
				glog.V(1).Infof("containerID = %s", ct.containerID)
			}
		} else if event.Container.Type == api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED &&
			len(ct.containerID) > 0 &&
			telemetryEvent.Event.ContainerId == ct.containerID {

			ct.containerExited = true
			glog.V(1).Infof("containerExited = true")
		}

	case *api.Event_Process:
		if event.Process.Type == api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC &&
			telemetryEvent.Event.ContainerId == ct.containerID &&
			event.Process.ExecFilename == "/bin/dmesg" {

			if len(ct.processID) > 0 {
				t.Error("Already saw process exec")
				return false
			}

			ct.processID = telemetryEvent.Event.ProcessId
			glog.V(1).Infof("processID = %s", ct.processID)
		}
	}

	return !(ct.containerExited && len(ct.processID) > 0)
}

func TestExec(t *testing.T) {
	et := &execTest{}
	tt := newTelemetryTest(et)
	tt.runTest(t)
}
