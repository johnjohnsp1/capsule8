package container

import (
	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/stream"
)

type containerSensor struct {
	state map[string]event.ContainerEvent_ContainerState
}

func (c *containerSensor) translateEvents(e interface{}) interface{} {
	ev := e.(*Event)
	var cev *event.ContainerEvent

	switch ev.State {
	case ContainerCreated:
		cev = &event.ContainerEvent{
			ContainerID: ev.ID,
			State:       event.ContainerEvent_CONTAINER_CREATED,
			ImageID:     ev.ImageID,
		}

	case ContainerStarted:
		cev = &event.ContainerEvent{
			ContainerID: ev.ID,
			State:       event.ContainerEvent_CONTAINER_STARTED,
		}

	case ContainerStopped:
		cev = &event.ContainerEvent{
			ContainerID: ev.ID,
			State:       event.ContainerEvent_CONTAINER_STOPPED,
		}

	case ContainerRemoved:
		cev = &event.ContainerEvent{
			ContainerID: ev.ID,
			State:       event.ContainerEvent_CONTAINER_REMOVED,
		}

	default:
		panic("Invalid value for ContainerState")
	}

	return &event.Event{
		Subevent: &event.Event_Container{
			Container: cev,
		},
	}
}

// NewSensor creates a new ContainerEvent sensor
func NewSensor(selector event.Selector) (*stream.Stream, error) {
	c := &containerSensor{}

	s, err := NewEventStream()
	if err != nil {
		return nil, err
	}

	s = stream.Map(s, c.translateEvents)

	// TODO: filter by given selector

	return s, err
}
