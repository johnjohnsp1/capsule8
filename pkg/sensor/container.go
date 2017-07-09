package sensor

import (
	api "github.com/capsule8/reactive8/pkg/api/v0"
	"github.com/capsule8/reactive8/pkg/container"
)

func newContainerCreated(cID string, imageID string) *api.ContainerEvent {
	ev := &api.ContainerEvent{
		Type:    api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED,
		ImageId: imageID,
	}

	return ev
}

func newContainerRunning(cID string) *api.ContainerEvent {
	ev := &api.ContainerEvent{
		Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_RUNNING,
	}

	return ev
}

func newContainerExited(cID string) *api.ContainerEvent {
	ev := &api.ContainerEvent{
		Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED,
	}

	return ev
}

func newContainerDestroyed(cID string) *api.ContainerEvent {
	ev := &api.ContainerEvent{
		Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_DESTROYED,
	}

	return ev
}

func translateContainerEvents(e interface{}) interface{} {
	ce := e.(*container.Event)
	var ece *api.ContainerEvent

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
	ev.Event = &api.Event_Container{
		Container: ece,
	}

	return ev
}
