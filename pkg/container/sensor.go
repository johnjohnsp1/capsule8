package container

import (
	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/stream"
)

func newEvent(cid string,
	state event.ContainerEvent_ContainerState) *event.Event {

	return &event.Event{
		Subevent: &event.Event_Container{
			Container: &event.ContainerEvent{
				ContainerID: cid,
				State:       state,
			},
		},
	}
}

type containerSensor struct {
	state map[string]event.ContainerEvent_ContainerState
}

func (c *containerSensor) processEvents(e interface{}) interface{} {
	var ev *event.Event

	switch e.(type) {
	case *dockerEvent:
		e := e.(*dockerEvent)
		if e.State == dockerContainerCreated {
			ev = newEvent(e.ID,
				event.ContainerEvent_CONTAINER_CREATED)
		} else if e.State == dockerContainerDead {
			ev = newEvent(e.ID,
				event.ContainerEvent_CONTAINER_REMOVED)
		}

	case *ociEvent:
		e := e.(*ociEvent)
		if e.State == ociRunning {
			ev = newEvent(e.ID,
				event.ContainerEvent_CONTAINER_STARTED)
		} else if e.State == ociStopped {
			ev = newEvent(e.ID,
				event.ContainerEvent_CONTAINER_STOPPED)
		}
	}

	return ev
}

func filterNils(e interface{}) bool {
	ev := e.(*event.Event)
	return ev != nil
}

// NewContainerSensor creates a new ContainerEvent sensor
func NewContainerSensor(selector event.Selector) (*stream.Stream, error) {
	//
	// Join upstream Docker and OCI container event streams
	//
	dockerEvents, err := NewDockerEventStream()
	if err != nil {
		return nil, err
	}

	ociEvents, err := NewOciEventStream()
	if err != nil {
		return nil, err
	}

	c := &containerSensor{}
	s := stream.Join(dockerEvents, ociEvents)
	s = stream.Map(s, c.processEvents)

	// XXX: filter by given selector
	s = stream.Filter(s, filterNils)

	return s, err
}
