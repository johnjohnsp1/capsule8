package functional

import (
	"testing"

	api "github.com/capsule8/api/v0"
	"github.com/golang/glog"
)

const testContainerTag = "c8-sensor-container-functest"

type containerTest struct {
	testContainer *Container
	innerContID   string
	seenEvts      map[api.ContainerEventType]bool
}

func newContainerTest() *containerTest {
	return &containerTest{seenEvts: make(map[api.ContainerEventType]bool)}
}

func (ct *containerTest) BuildContainer(t *testing.T) {
	c := NewContainer(t, "container")
	err := c.Build()
	if err != nil {
		t.Error(err)
	} else {
		glog.V(2).Infof("Build container %s\n", c.ImageID[1:12])
		ct.testContainer = c
	}
}

func (ct *containerTest) RunContainer(t *testing.T) {
	err := ct.testContainer.Run(
		"-v", "/var/run/docker.sock:/var/run/docker.sock")
	if err != nil {
		t.Error(err)
	}
	glog.V(2).Infof("Running container %s\n", ct.testContainer.ImageID[1:12])
}

func (ct *containerTest) CreateSubscription(t *testing.T) *api.Subscription {
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
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_DESTROYED,
		},
	}

	eventFilter := &api.EventFilter{
		ContainerEvents: containerEvents,
	}

	return &api.Subscription{
		EventFilter: eventFilter,
	}
}

func (ct *containerTest) HandleTelemetryEvent(t *testing.T, te *api.TelemetryEvent) bool {
	glog.V(2).Infof("Got Event %#v\n", te.Event)
	switch event := te.Event.Event.(type) {
	case *api.Event_Container:
		glog.V(2).Infof("Container Event %#v\n", *event.Container)
		switch event.Container.Type {
		case api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED:
			if event.Container.ImageName == testContainerTag {
				if ct.innerContID != "" {
					t.Errorf("Already seen container event %s", event.Container.Type)
				}
				ct.innerContID = te.Event.ContainerId
				ct.seenEvts[event.Container.Type] = true
			}

		case api.ContainerEventType_CONTAINER_EVENT_TYPE_RUNNING,
			api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED,
			api.ContainerEventType_CONTAINER_EVENT_TYPE_DESTROYED:
			if ct.innerContID != "" && te.Event.ContainerId == ct.innerContID {
				if ct.seenEvts[event.Container.Type] {
					t.Errorf("Already seen container event %s", event.Container.Type)
				}
				ct.seenEvts[event.Container.Type] = true
			}

		}

		return len(ct.seenEvts) < 4

	default:
		t.Errorf("Unexpected event type %T\n", event)
		return false
	}
}

//
// TestContainer checks that the sensor generates container events requested by
// the subscription.
//
func TestContainer(t *testing.T) {
	ct := newContainerTest()

	tt := NewTelemetryTester(ct)
	tt.RunTest(t)
}
