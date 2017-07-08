package sensor

import (
	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/container"
)

func newContainerCreated(cID string, imageID string) *event.ContainerEvent {
	ev := &event.ContainerEvent{
		Type:    event.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED,
		ImageId: imageID,
	}

	return ev
}

func newContainerRunning(cID string) *event.ContainerEvent {
	ev := &event.ContainerEvent{
		Type: event.ContainerEventType_CONTAINER_EVENT_TYPE_RUNNING,
	}

	return ev
}

func newContainerExited(cID string) *event.ContainerEvent {
	ev := &event.ContainerEvent{
		Type: event.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED,
	}

	return ev
}

func newContainerDestroyed(cID string) *event.ContainerEvent {
	ev := &event.ContainerEvent{
		Type: event.ContainerEventType_CONTAINER_EVENT_TYPE_DESTROYED,
	}

	return ev
}

func translateContainerEvents(e interface{}) interface{} {
	ce := e.(*container.Event)
	var ece *event.ContainerEvent

	switch ce.State {
	case container.ContainerCreated:
		ece = newContainerCreated(ce.ID, ce.ImageID)

	case container.ContainerStarted:
		ece = newContainerRunning(ce.ID)

	case container.ContainerStopped:
		ece = newContainerExited(ce.ID)

	case container.ContainerRemoved:
		ece = newContainerDestroyed(ce.ID)

	default:
		panic("Invalid value for ContainerState")
	}

	ev := newEventFromContainer(ce.ID)
	ev.Event = &event.Event_Container{
		Container: ece,
	}

	return ev
}
