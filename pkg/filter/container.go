package filter

import (
	api "github.com/capsule8/reactive8/pkg/api/v0"
	"github.com/gobwas/glob"
)

// NewContainerFilter creates a new ContainerEvent filter
func NewContainerFilter(ecf *api.ContainerFilter) Filter {
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

	return cf
}

type containerFilter struct {
	containerIds   map[string]struct{}
	containerNames map[string]struct{}
	imageIds       map[string]struct{}
	imageGlobs     []glob.Glob
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

func (c *containerFilter) FilterFunc(i interface{}) bool {
	e := i.(*api.Event)

	//
	// Fast path: Check if containerID is in containerIds map
	//
	if c.containerIds != nil {
		_, ok := c.containerIds[e.ContainerId]
		if ok {
			return true
		}
	}

	switch e.Event.(type) {
	case *api.Event_Container:
		cev := e.GetContainer()

		//
		// Slow path: Check if other identifiers are in maps. If they
		// are, add the containerID to containerIds map to take fast
		// path next time.
		//

		if c.containerNames != nil && cev.Name != "" {
			_, ok := c.containerNames[cev.Name]
			if ok {
				c.addContainerID(e.ContainerId)
				return true
			}
		}

		if c.imageIds != nil && cev.ImageId != "" {
			_, ok := c.imageIds[cev.ImageId]
			if ok {
				c.addContainerID(e.ContainerId)
				return true
			}
		}

		if c.imageGlobs != nil && cev.ImageName != "" {
			for _, g := range c.imageGlobs {
				if g.Match(cev.ImageName) {
					c.addContainerID(e.ContainerId)
					return true
				}
			}
		}
	}

	return false
}

func (c *containerFilter) DoFunc(i interface{}) {
	e := i.(*api.Event)

	switch e.Event.(type) {
	case *api.Event_Container:
		cev := e.GetContainer()
		if cev.Type == api.ContainerEventType_CONTAINER_EVENT_TYPE_DESTROYED {
			c.removeContainerID(e.ContainerId)
		}
	}

}
