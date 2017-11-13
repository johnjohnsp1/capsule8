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
	testExitCodeFilenamePatt = "./status*"
	testExitStatus           = 1
	testExitCode             = testExitStatus << 8
)

var (
	testExitFilenameRe = regexp.MustCompile("^\\./status([0-9])$")
)

type exitStatusTest struct {
	testContainer  *Container
	exitId         string
	seenExitStatus bool
}

func (est *exitStatusTest) BuildContainer(t *testing.T) {
	c := NewContainer(t, "exit_status")
	err := c.Build()
	if err != nil {
		t.Error(err)
	} else {
		glog.V(2).Infof("Built container %s\n", c.ImageID[0:12])
		est.testContainer = c
	}
}

func (est *exitStatusTest) RunContainer(t *testing.T) {
	err := est.testContainer.Run()
	if err != nil {
		t.Error(err)
	}
	glog.V(2).Infof("Running container %s\n", est.testContainer.ImageID[0:12])
}

func (est *exitStatusTest) CreateSubscription(t *testing.T) *api.Subscription {
	processEvents := []*api.ProcessEventFilter{
		&api.ProcessEventFilter{
			Type: api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
			ExecFilenamePattern: &wrappers.StringValue{
				Value: testExitCodeFilenamePatt,
			},
		},
		&api.ProcessEventFilter{
			Type: api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT,
			ExitCode: &wrappers.Int32Value{
				Value: testExitCode,
			},
		},
	}

	// Subscribing to container created events are currently necessary
	// to get imageIDs in other events.
	containerEvents := []*api.ContainerEventFilter{
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED,
		},
	}

	eventFilter := &api.EventFilter{
		ContainerEvents: containerEvents,
		ProcessEvents:   processEvents,
	}

	return &api.Subscription{
		EventFilter: eventFilter,
	}
}

func (est *exitStatusTest) HandleTelemetryEvent(t *testing.T, telemetryEvent *api.TelemetryEvent) bool {
	glog.V(2).Infof("Got event %+v\n", telemetryEvent)

	switch event := telemetryEvent.Event.Event.(type) {
	case *api.Event_Container:
		// Ignore

	case *api.Event_Process:
		glog.V(2).Infof("Process event %+v\n\n", *event.Process)

		switch event.Process.Type {

		case api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC:
			if telemetryEvent.Event.ImageId != est.testContainer.ImageID {
				return true
			}

			if m := testExitFilenameRe.FindStringSubmatch(event.Process.ExecFilename); m != nil {
				status, _ := strconv.ParseUint(m[1], 0, 32)
				if status == testExitStatus {
					est.exitId = telemetryEvent.Event.ProcessId
				}
			} else {
				return true
			}

		case api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT:
			if event.Process.ExitStatus != testExitStatus {
				t.Errorf("Expected exit code %d, got %d\n", testExitStatus, event.Process.ExitStatus)
				return false
			}
			if telemetryEvent.Event.ProcessId == est.exitId {
				est.seenExitStatus = true
			}

		default:
			t.Errorf("Unexpected process event %s", event.Process.Type)
			return false
		}

	default:
		t.Errorf("Unexpected event type %T", telemetryEvent.Event.Event)
		return false
	}

	return !est.seenExitStatus
}

// TestExitStatus tests filtering PROCESS_EVENT_TYPE_EXIT by ExitCode to look
// for a given exit status.
func TestExitStatus(t *testing.T) {
	est := &exitStatusTest{}

	tt := NewTelemetryTester(est)
	tt.RunTest(t)
}
