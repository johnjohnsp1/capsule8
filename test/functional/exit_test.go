package functional

import (
	"regexp"
	"strconv"
	"testing"

	api "github.com/capsule8/api/v0"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes/wrappers"
)

const (
	// Test process 1
	testExitProc1Filename = "/main"
	testExitProc1Status   = 1

	// Test process 2
	testExitProc2Patt   = "/status*"
	testExitProc2Status = 2
	testExitProc2Code   = testExitProc2Status << 8
)

var (
	testExitFilenameRe    = regexp.MustCompile("^/status([0-9])$")
	testExitProc2Filename = "/status" + strconv.Itoa(testExitProc2Status)
)

type exitTest struct {
	testContainer   *Container
	containerID     string
	containerExited bool
	process1ID      string
	process1Exited  bool
	process2ID      string
	process2Exited  bool
}

func (ct *exitTest) BuildContainer(t *testing.T) string {
	c := NewContainer(t, "exit")
	err := c.Build()
	if err != nil {
		t.Error(err)
		return ""
	}

	ct.testContainer = c
	return ct.testContainer.ImageID
}

func (ct *exitTest) RunContainer(t *testing.T) {
	err := ct.testContainer.Start()
	if err != nil {
		t.Error(err)
		return
	}

	// We assume that the container will return an error, so ignore that one
	ct.testContainer.Wait()
}

func (ct *exitTest) CreateSubscription(t *testing.T) *api.Subscription {
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
			Type:         api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
			ExecFilename: &wrappers.StringValue{testExitProc1Filename},
		},

		&api.ProcessEventFilter{
			Type:                api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
			ExecFilenamePattern: &wrappers.StringValue{testExitProc2Patt},
		},

		&api.ProcessEventFilter{
			Type:         api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT,
			ExecFilename: &wrappers.StringValue{testExitProc1Filename},
		},

		&api.ProcessEventFilter{
			Type:                api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT,
			ExecFilenamePattern: &wrappers.StringValue{testExitProc2Patt},
			ExitCode:            &wrappers.Int32Value{testExitProc2Code},
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

func (ct *exitTest) HandleTelemetryEvent(t *testing.T, telemetryEvent *api.TelemetryEvent) bool {
	glog.V(2).Infof("%+v", telemetryEvent)

	switch event := telemetryEvent.Event.Event.(type) {
	case *api.Event_Container:
		switch event.Container.Type {
		case api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED:
			if event.Container.ImageId == ct.testContainer.ImageID {
				if len(ct.containerID) > 0 {
					t.Error("Already saw container created")
					return false
				}

				ct.containerID = telemetryEvent.Event.ContainerId
				glog.V(1).Infof("containerID = %s", ct.containerID)
			}

		case api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED:
			if len(ct.containerID) > 0 &&
				telemetryEvent.Event.ContainerId == ct.containerID {

				if event.Container.ExitCode != testExitProc1Status {
					t.Errorf("Expected ExitCode %d, got %d",
						testExitProc1Status, event.Container.ExitCode)
					return false
				}

				ct.containerExited = true
				glog.V(1).Infof("containerExited = true")
			}
		}

	case *api.Event_Process:
		switch event.Process.Type {
		case api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC:
			if telemetryEvent.Event.ContainerId == ct.containerID {
				switch event.Process.ExecFilename {
				case testExitProc1Filename:
					if len(ct.process1ID) > 0 {
						t.Error("Already saw process 1 exec")
						return false
					}

					ct.process1ID = telemetryEvent.Event.ProcessId
					glog.V(1).Infof("process1ID = %s", ct.process1ID)

				case testExitProc2Filename:
					if len(ct.process2ID) > 0 {
						t.Error("Already saw process 2 exec")
						return false
					}

					ct.process2ID = telemetryEvent.Event.ProcessId
					glog.V(1).Infof("process2ID = %s", ct.process2ID)
				}
			}

		case api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT:
			switch telemetryEvent.Event.ProcessId {
			case "":

			case ct.process1ID:
				if event.Process.ExitSignal != 0 {
					t.Errorf("Expected ExitSignal %d, got %d",
						0, event.Process.ExitSignal)
					return false
				}

				if event.Process.ExitCoreDumped {
					t.Errorf("Expected ExitCoreDumped %t, got %t",
						false, event.Process.ExitCoreDumped)
					return false
				}

				if event.Process.ExitStatus != testExitProc1Status {
					t.Errorf("Expected ExitStatus %d, got %d",
						testExitProc1Status, event.Process.ExitStatus)
					return false
				}

				ct.process1Exited = true
				glog.V(1).Infof("process1Exited = true")

			case ct.process2ID:
				if event.Process.ExitStatus != testExitProc2Status {
					t.Errorf("Expected ExitStatus %d, got %d",
						testExitProc2Status, event.Process.ExitStatus)
					return false
				}

				ct.process2Exited = true
				glog.V(1).Infof("process2Exited = true")

			}
		}
	}

	return !(ct.containerExited && ct.process1Exited && ct.process2Exited)
}

func TestExit(t *testing.T) {
	et := &exitTest{}
	tt := NewTelemetryTester(et)
	tt.RunTest(t)
}
