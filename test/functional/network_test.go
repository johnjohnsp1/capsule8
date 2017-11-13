package functional

import (
	"testing"

	api "github.com/capsule8/api/v0"
	"github.com/golang/glog"
)

type networkTest struct {
	testContainer *Container
	seenEvents    map[api.NetworkEventType]bool
}

func (nt *networkTest) BuildContainer(t *testing.T) {
	c := NewContainer(t, "network")
	err := c.Build()
	if err != nil {
		t.Error(err)
	} else {
		glog.V(2).Infof("Built container %s\n", c.ImageID[0:12])
		nt.testContainer = c
	}
}

func (nt *networkTest) RunContainer(t *testing.T) {
	err := nt.testContainer.Run("-P")
	if err != nil {
		t.Error(err)
	}
	glog.V(2).Infof("Running container %s\n", nt.testContainer.ImageID[0:12])
}

func (nt *networkTest) CreateSubscription(t *testing.T) *api.Subscription {
	// TODO: Add Filter expression to at least one network event filter.
	networkEvents := []*api.NetworkEventFilter{
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_ATTEMPT,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_RESULT,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_ATTEMPT,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_RESULT,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_ATTEMPT,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_RESULT,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_ATTEMPT,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_RESULT,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_SENDTO_ATTEMPT,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_SENDTO_RESULT,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_RECVFROM_ATTEMPT,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_RECVFROM_RESULT,
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
		NetworkEvents:   networkEvents,
		ContainerEvents: containerEvents,
	}

	return &api.Subscription{
		EventFilter: eventFilter,
	}
}

func (nt *networkTest) HandleTelemetryEvent(t *testing.T, te *api.TelemetryEvent) bool {
	glog.V(2).Infof("Got Event %+v\n", te.Event)
	switch event := te.Event.Event.(type) {
	case *api.Event_Container:
		return true

	case *api.Event_Network:
		// TODO after adding network event filters, check that they work.
		if te.Event.ImageId == nt.testContainer.ImageID {
			nt.seenEvents[event.Network.Type] = true
		}

		return len(nt.seenEvents) < 12

	default:
		t.Errorf("Unexpected event type %T\n", event)
		return false
	}
}

// TestNetwork exercises the network events, including filtering.
func TestNetwork(t *testing.T) {
	nt := &networkTest{seenEvents: make(map[api.NetworkEventType]bool)}

	tt := NewTelemetryTester(nt)
	tt.RunTest(t)
}
