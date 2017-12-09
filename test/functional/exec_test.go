package functional

import (
	"testing"

	api "github.com/capsule8/api/v0"

	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes/wrappers"
)

type execTest struct {
	testContainer   *Container
	err             error
	containerID     string
	containerExited bool
	processID       string
}

func (ct *execTest) BuildContainer(t *testing.T) string {
	c := NewContainer(t, "exec")
	err := c.Build()
	if err != nil {
		t.Error(err)
		return ""
	}

	ct.testContainer = c
	return ct.testContainer.ImageID
}

func (ct *execTest) RunContainer(t *testing.T) {
	err := ct.testContainer.Run()
	if err != nil {
		t.Error(err)
	}
}

func (ct *execTest) CreateSubscription(t *testing.T) *api.Subscription {
	processEvents := []*api.ProcessEventFilter{
		&api.ProcessEventFilter{
			Type: api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
			ExecFilenamePattern: &wrappers.StringValue{
				// Want to only match execs of 'uname'
				Value: "*nam*",
			},
		},
	}

	eventFilter := &api.EventFilter{
		ProcessEvents: processEvents,
	}

	sub := &api.Subscription{
		EventFilter: eventFilter,
	}

	return sub
}

func (ct *execTest) HandleTelemetryEvent(t *testing.T, telemetryEvent *api.TelemetryEvent) bool {
	glog.V(2).Infof("%+v", telemetryEvent)

	switch event := telemetryEvent.Event.Event.(type) {
	case *api.Event_Process:
		if event.Process.Type == api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC &&
			event.Process.ExecFilename == "/bin/uname" {

			if len(ct.processID) > 0 {
				t.Error("Already saw process exec")
				return false
			}

			ct.processID = telemetryEvent.Event.ProcessId
			glog.V(1).Infof("processID = %s", ct.processID)
		}
	}

	return len(ct.processID) == 0
}

// TestExec exercises filtering PROCESS_EVENT_TYPE_EXEC by ExecFilenamePattern
func TestExec(t *testing.T) {
	et := &execTest{}
	tt := NewTelemetryTester(et)
	tt.RunTest(t)
}
