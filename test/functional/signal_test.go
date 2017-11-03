package functional

import (
	"testing"

	api "github.com/capsule8/api/v0"

	"github.com/golang/glog"
)

type signalTest struct {
	testContainer   *Container
	err             error
	containerID     string
	containerExited bool
	processID       string
	processExited   bool
}

func (ct *signalTest) BuildContainer(t *testing.T) {
	c := NewContainer(t, "signal")
	err := c.Build()
	if err != nil {
		t.Error(err)
	} else {
		ct.testContainer = c
	}
}

func (ct *signalTest) RunContainer(t *testing.T) {
	err := ct.testContainer.Start()
	if err != nil {
		t.Error(err)
		return
	}

	// We assume that the container will return an error, so ignore that one
	ct.testContainer.Wait()
}

func (ct *signalTest) CreateSubscription(t *testing.T) *api.Subscription {
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
			Type: api.ProcessEventType_PROCESS_EVENT_TYPE_FORK,
		},

		&api.ProcessEventFilter{
			Type: api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
		},

		&api.ProcessEventFilter{
			Type: api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT,
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

func (ct *signalTest) HandleTelemetryEvent(t *testing.T, telemetryEvent *api.TelemetryEvent) bool {
	glog.V(2).Infof("%+v", telemetryEvent)

	switch event := telemetryEvent.Event.Event.(type) {
	case *api.Event_Container:
		if event.Container.Type == api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED {
			if event.Container.ImageName == ct.testContainer.ImageID {
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

			if event.Container.ExitCode != 138 {
				t.Errorf("Expected ExitCode %d, got %d",
					138, event.Container.ExitCode)
				return false
			}

			ct.containerExited = true
			glog.V(1).Infof("containerExited = true")
		}

	case *api.Event_Process:
		if event.Process.Type == api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC {
			if event.Process.ExecFilename == "/main" &&
				telemetryEvent.Event.ContainerId == ct.containerID {
				if len(ct.processID) > 0 {
					t.Error("Already saw process exec")
					return false
				}

				ct.processID = telemetryEvent.Event.ProcessId
				glog.V(1).Infof("processID = %s", ct.processID)
			}
		} else if len(ct.processID) > 0 &&
			telemetryEvent.Event.ProcessId == ct.processID &&
			event.Process.Type == api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT {

			if event.Process.ExitSignal != 10 {
				t.Errorf("Expected ExitSignal == %d, got %d",
					10, event.Process.ExitSignal)
				return false
			}

			if event.Process.ExitCoreDumped != false {
				t.Errorf("Expected ExitCoreDumped %v, got %v",
					false, event.Process.ExitCoreDumped)
				return false
			}

			if event.Process.ExitStatus != 0 {
				t.Errorf("Expected ExitStatus %d, got %d",
					0, event.Process.ExitStatus)
				return false
			}

			ct.processExited = true
			glog.V(1).Infof("processExited = true")
		}
	}

	return !(ct.containerExited && ct.processExited)
}

func TestSignal(t *testing.T) {
	st := &signalTest{}
	tt := NewTelemetryTester(st)
	tt.RunTest(t)
}
