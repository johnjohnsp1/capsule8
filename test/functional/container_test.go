package functional

import (
	"testing"

	api "github.com/capsule8/api/v0"
	"github.com/golang/glog"
)

type containerTest struct {
	testContainer *Container
	containerID   string
	seenEvts      map[api.ContainerEventType]bool
}

func newContainerTest() *containerTest {
	return &containerTest{seenEvts: make(map[api.ContainerEventType]bool)}
}

func (ct *containerTest) BuildContainer(t *testing.T) string {
	c := NewContainer(t, "container")
	err := c.Build()
	if err != nil {
		t.Error(err)
	} else {
		glog.V(2).Infof("Built container %s\n", c.ImageID[0:12])
		ct.testContainer = c
	}

	return ct.testContainer.ImageID
}

func (ct *containerTest) RunContainer(t *testing.T) {
	err := ct.testContainer.Run()
	if err != nil {
		t.Error(err)
	}
	glog.V(2).Infof("Running container %s\n", ct.testContainer.ImageID[0:12])
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
	glog.V(2).Infof("%+v\n", te.Event)
	switch event := te.Event.Event.(type) {
	case *api.Event_Container:
		switch event.Container.Type {
		case api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED:
			if event.Container.ImageId == ct.testContainer.ImageID {
				if ct.containerID != "" {
					t.Errorf("Already seen container event %s", event.Container.Type)
				}
				ct.containerID = te.Event.ContainerId
				ct.seenEvts[event.Container.Type] = true

				glog.V(1).Infof("Found container %s",
					ct.containerID)
			}

		case api.ContainerEventType_CONTAINER_EVENT_TYPE_RUNNING,
			api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED,
			api.ContainerEventType_CONTAINER_EVENT_TYPE_DESTROYED:
			if ct.containerID != "" && te.Event.ContainerId == ct.containerID {
				if ct.seenEvts[event.Container.Type] {
					t.Errorf("Already saw container event type %v",
						event.Container.Type)
				}

				ct.seenEvts[event.Container.Type] = true

				glog.V(1).Infof("Got container event %s for %s",
					event.Container.Type, ct.containerID)
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
