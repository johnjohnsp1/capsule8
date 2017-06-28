package sensor

import (
	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/container"
	"github.com/capsule8/reactive8/pkg/stream"
	"github.com/gobwas/glob"
)

type containerFilter struct {
	containerIds   map[string]struct{}
	containerNames map[string]struct{}
	imageIds       map[string]struct{}
	imageGlobs     []glob.Glob
	states         map[container.State]struct{}
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
		Event: &event.Event_Container{
			Container: cev,
		},
	}
}

func (c *containerFilter) addContainerID(cid string) {
	if c.containerIds == nil {
		c.containerIds = make(map[string]struct{})
	}

	c.containerIds[cid] = struct{}{}
}

func (c *containerFilter) removeContainerID(cid string) {
	delete(c.containerIds, cid)
}

func (c *containerFilter) addContainerName(cname string) {
	if c.containerNames == nil {
		c.containerNames = make(map[string]struct{})
	}

	c.containerNames[cname] = struct{}{}
}

func (c *containerFilter) addImageID(iid string) {
	if c.imageIds == nil {
		c.imageIds = make(map[string]struct{})
	}

	c.imageIds[iid] = struct{}{}
}

func (c *containerFilter) addImageName(iname string) {
	g, err := glob.Compile(iname, '/')
	if err == nil {
		c.imageGlobs = append(c.imageGlobs, g)
	}
}

func (c *containerFilter) addState(state container.State) {
	if c.states == nil {
		c.states = make(map[container.State]struct{})
	}

	c.states[state] = struct{}{}
}

func (c *containerFilter) addEventType(ev event.ContainerEventType) {
	switch ev {
	case event.ContainerEventType_CONTAINER_EVENT_TYPE_UNKNOWN:
		// Match all states
		c.addState(container.ContainerCreated)
		c.addState(container.ContainerStarted)
		c.addState(container.ContainerStopped)
		c.addState(container.ContainerRemoved)

	case event.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED:
		c.addState(container.ContainerCreated)
	case event.ContainerEventType_CONTAINER_EVENT_TYPE_RUNNING:
		c.addState(container.ContainerStarted)
	case event.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED:
		c.addState(container.ContainerStopped)
	case event.ContainerEventType_CONTAINER_EVENT_TYPE_DESTROYED:
		c.addState(container.ContainerRemoved)
	}
}

func (c *containerFilter) filterEventState(e interface{}) bool {
	cev := e.(*container.Event)

	if c.states != nil {
		_, ok := c.states[cev.State]
		if !ok {
			return false
		}
	}

	return true
}

func (c *containerFilter) filterContainerEvent(e interface{}) bool {
	cev := e.(*container.Event)

	//
	// Fast path: Check if containerID is in containerIds map
	//
	if c.containerIds != nil {
		_, ok := c.containerIds[cev.ID]
		if ok {
			return true
		}
	}

	//
	// Slow path: Check if other identifiers are in maps. If they
	// are, add the containerID to containerIds map to take fast
	// path next time.
	//

	if c.containerNames != nil && cev.Name != "" {
		_, ok := c.containerNames[cev.Name]
		if ok {
			c.addContainerID(cev.ID)
			return true
		}
	}

	if c.imageIds != nil && cev.ImageID != "" {
		_, ok := c.imageIds[cev.ImageID]
		if ok {
			c.addContainerID(cev.ID)
			return true
		}
	}

	if c.imageGlobs != nil && cev.Image != "" {
		for _, g := range c.imageGlobs {
			if g.Match(cev.Image) {
				c.addContainerID(cev.ID)
				return true
			}
		}
	}

	return false
}

func (cf *containerFilter) pruneFilter(e interface{}) {
	cev := e.(*container.Event)

	if cev.State == container.ContainerRemoved {
		cf.removeContainerID(cev.ID)
	}
}

// NewSensor creates a new ContainerEvent sensor
func NewContainerSensor(sub *event.Subscription) (*stream.Stream, error) {
	//
	// The sensors behind container event streams are singletons that
	// monitor all container lifecycle events regardless of subscription
	// filters. All we need to do here for each unique subscription is
	// to filter a stream based on the container-related fields in the
	// ProcessFilter and the specified ContainerEventFilter.
	//

	cf := &containerFilter{}

	if sub.ContainerFilter != nil {
		scf := sub.ContainerFilter

		for _, v := range scf.Ids {
			cf.addContainerID(v)
		}

		for _, v := range scf.Names {
			cf.addContainerName(v)
		}

		for _, v := range scf.ImageIds {
			cf.addImageID(v)
		}

		for _, v := range scf.ImageNames {
			cf.addImageName(v)
		}

	}

	if sub.EventFilter != nil {
		for _, v := range sub.EventFilter.ContainerEvents {
			cf.addEventType(v.Type)
		}
	}

	s, err := container.NewEventStream()
	if err != nil {
		return nil, err
	}

	s = stream.Filter(s, cf.filterContainerEvent)
	s = stream.Do(s, cf.pruneFilter)
	s = stream.Filter(s, cf.filterEventState)
	s = stream.Map(s, translateContainerEvents)

	return s, err
}
