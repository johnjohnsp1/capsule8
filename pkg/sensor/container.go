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
	ev := e.(*container.Event)
	var cev *event.ContainerEvent

	switch ev.State {
	case container.ContainerCreated:
		cev = newContainerCreated(ev.ID, ev.ImageID)

	case container.ContainerStarted:
		cev = newContainerRunning(ev.ID)

	case container.ContainerStopped:
		cev = newContainerExited(ev.ID)

	case container.ContainerRemoved:
		cev = newContainerDestroyed(ev.ID)

	default:
		panic("Invalid value for ContainerState")
	}

	return &event.Event{
		ContainerId: ev.ID,

		Event: &event.Event_Container{
			Container: cev,
		},
	}
}
