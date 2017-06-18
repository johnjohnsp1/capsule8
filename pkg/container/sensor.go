package container

import (
	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/stream"
)

type containerSensor struct {
	state map[string]event.ContainerEventType
}

func newContainerCreated(cID string, imageID string) *event.ContainerEvent {
	ev := &event.ContainerEvent{
		Id:      cID,
		Type:    event.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED,
		ImageId: imageID,
	}

	return ev
}

func newContainerRunning(cID string) *event.ContainerEvent {
	ev := &event.ContainerEvent{
		Id:   cID,
		Type: event.ContainerEventType_CONTAINER_EVENT_TYPE_RUNNING,
	}

	return ev
}

func newContainerExited(cID string) *event.ContainerEvent {
	ev := &event.ContainerEvent{
		Id:   cID,
		Type: event.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED,
	}

	return ev
}

func newContainerDestroyed(cID string) *event.ContainerEvent {
	ev := &event.ContainerEvent{
		Id:   cID,
		Type: event.ContainerEventType_CONTAINER_EVENT_TYPE_DESTROYED,
	}

	return ev
}

func (c *containerSensor) translateEvents(e interface{}) interface{} {
	ev := e.(*Event)
	var cev *event.ContainerEvent

	switch ev.State {
	case ContainerCreated:
		cev = newContainerCreated(ev.ID, ev.ImageID)

	case ContainerStarted:
		cev = newContainerRunning(ev.ID)

	case ContainerStopped:
		cev = newContainerExited(ev.ID)

	case ContainerRemoved:
		cev = newContainerDestroyed(ev.ID)

	default:
		panic("Invalid value for ContainerState")
	}

	return &event.Event{
		Event: &event.Event_Container{
			Container: cev,
		},
	}
}

// NewSensor creates a new ContainerEvent sensor
func NewSensor(sub *event.Subscription) (*stream.Stream, error) {
	c := &containerSensor{}

	s, err := NewEventStream()
	if err != nil {
		return nil, err
	}

	s = stream.Map(s, c.translateEvents)

	// TODO: filter event stream

	return s, err
}
