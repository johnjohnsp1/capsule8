package sensor

import (
	api "github.com/capsule8/reactive8/pkg/api/v0"
	"github.com/capsule8/reactive8/pkg/container"
)

func newContainerCreated(cID string) *api.ContainerEvent {
	ev := &api.ContainerEvent{
		Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED,
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
		ece = newContainerCreated(ce.ID)
		ece.Name = ce.Name
		ece.ImageId = ce.ImageID
		ece.ImageName = ce.Image

	case container.ContainerStarted:
		if ce.Pid == 0 {
			// We currently get this event in the stream twice,
			// one with the Cgroup and without Pid, and vice-versa.
			// Only translate the event with the Pid for now.
			return nil
		}

		ece = newContainerRunning(ce.ID)
		ece.HostPid = int32(ce.Pid)

	case container.ContainerStopped:
		ece = newContainerExited(ce.ID)
		// TODO: ece.ExitCode = ???

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
