package functional

import (
	"testing"

	api "github.com/capsule8/api/v0"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes/wrappers"
)

const testExecFilename = "./test-proc"

type procStressTest struct {
	testContainer *Container
	processCount  int
}

func (st *procStressTest) BuildContainer(t *testing.T) {
	c := NewContainer(t, "stress")
	err := c.Build()
	if err != nil {
		t.Error(err)
	} else {
		glog.V(2).Infof("Build container %s\n", c.ImageID[1:12])
		st.testContainer = c
	}
}

func (st *procStressTest) RunContainer(t *testing.T) {
	err := st.testContainer.Run()
	if err != nil {
		t.Error(err)
	}
	glog.V(2).Infof("Running container %s\n", st.testContainer.ImageID[1:12])
}

func (st *procStressTest) CreateSubscription(t *testing.T) *api.Subscription {
	processEvents := []*api.ProcessEventFilter{
		&api.ProcessEventFilter{
			Type: api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
			ExecFilename: &wrappers.StringValue{
				Value: testExecFilename,
			},
		},
	}

	eventFilter := &api.EventFilter{
		ProcessEvents: processEvents,
	}

	return &api.Subscription{
		EventFilter: eventFilter,
	}
}

func (st *procStressTest) HandleTelemetryEvent(t *testing.T, telemetryEvent *api.TelemetryEvent) bool {
	glog.V(2).Infof("%+v", telemetryEvent)

	switch event := telemetryEvent.Event.Event.(type) {
	case *api.Event_Process:
		glog.V(2).Infof("%+v", *event.Process)
		switch event.Process.Type {
		case api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC:
			if event.Process.ExecFilename != testExecFilename {
				t.Errorf("Unexpected exec file name %s", event.Process.ExecFilename)
				return false
			}
			st.processCount++
			glog.V(2).Infof("HEY image is %q, process count is %d", telemetryEvent.Event.ImageId, st.processCount)
		default:
			t.Errorf("Unexpected process event %s", event.Process.Type)
			return false
		}
	default:
		t.Errorf("Unexpected event type %T", telemetryEvent.Event.Event)
		return false
	}

	return st.processCount < 256
}

func TestProcStress(t *testing.T) {
	st := &procStressTest{}

	tt := NewTelemetryTester(st)
	tt.RunTest(t)
}
