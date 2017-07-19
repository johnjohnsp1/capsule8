package sensor

import (
	api "github.com/capsule8/reactive8/pkg/api/v0"
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

func (c *containerFilter) addEventType(ev api.ContainerEventType) {
	switch ev {
	case api.ContainerEventType_CONTAINER_EVENT_TYPE_UNKNOWN:
		// Match all states
		c.addState(container.ContainerCreated)
		c.addState(container.ContainerStarted)
		c.addState(container.ContainerStopped)
		c.addState(container.ContainerRemoved)

	case api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED:
		c.addState(container.ContainerCreated)
	case api.ContainerEventType_CONTAINER_EVENT_TYPE_RUNNING:
		c.addState(container.ContainerStarted)
	case api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED:
		c.addState(container.ContainerStopped)
	case api.ContainerEventType_CONTAINER_EVENT_TYPE_DESTROYED:
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

func (cf *containerFilter) pruneFilter(i interface{}) {
	cev := i.(*container.Event)

	if cev.State == container.ContainerRemoved {
		cf.removeContainerID(cev.ID)
	}
}

func (cf *containerFilter) filterEvent(i interface{}) bool {
	e := i.(*api.Event)

	if len(e.ContainerId) > 0 {
		if cf.containerIds != nil {
			_, ok := cf.containerIds[e.ContainerId]
			if ok {
				return true
			}
		}

	}

	return false
}

func NewContainerFilter(ecf *api.ContainerFilter) (*containerFilter, error) {
	cf := &containerFilter{}

	for _, v := range ecf.Ids {
		cf.addContainerID(v)
	}

	for _, v := range ecf.Names {
		cf.addContainerName(v)
	}

	for _, v := range ecf.ImageIds {
		cf.addImageID(v)
	}

	for _, v := range ecf.ImageNames {
		cf.addImageName(v)
	}

	s, err := container.NewEventStream()
	if err != nil {
		return nil, err
	}

	go func() {
		s = stream.Filter(s, cf.filterContainerEvent)
		s = stream.Do(s, cf.pruneFilter)
		s = stream.Filter(s, cf.filterEventState)

		stream.Wait(s)
	}()

	return cf, err
}
