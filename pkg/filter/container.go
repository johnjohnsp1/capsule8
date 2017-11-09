package filter

import (
	api "github.com/capsule8/api/v0"

	"github.com/gobwas/glob"
)

func NewContainerFilter(ecf *api.ContainerFilter) Filter {
	cf := &containerFilter{}

	for _, v := range ecf.Ids {
		cf.addContainerId(v)
	}

	for _, v := range ecf.Names {
		cf.addContainerName(v)
	}

	for _, v := range ecf.ImageIds {
		cf.addImageId(v)
	}

	for _, v := range ecf.ImageNames {
		cf.addImageName(v)
	}

	return cf
}

type containerFilter struct {
	containerIds   map[string]bool
	containerNames map[string]bool
	imageIds       map[string]bool
	imageGlobs     map[string]glob.Glob
}

func (c *containerFilter) addContainerId(cid string) {
	if len(cid) > 0 {
		if c.containerIds == nil {
			c.containerIds = make(map[string]bool)
		}
		c.containerIds[cid] = true
	}
}

func (c *containerFilter) removeContainerId(cid string) {
	delete(c.containerIds, cid)
}

func (c *containerFilter) addContainerName(cname string) {
	if len(cname) > 0 {
		if c.containerNames == nil {
			c.containerNames = make(map[string]bool)
		}
		c.containerNames[cname] = true
	}
}

func (c *containerFilter) addImageId(iid string) {
	if len(iid) > 0 {
		if c.imageIds == nil {
			c.imageIds = make(map[string]bool)
		}
		c.imageIds[iid] = true
	}
}

func (c *containerFilter) addImageName(iname string) {
	if len(iname) > 0 {
		if c.imageGlobs == nil {
			c.imageGlobs = make(map[string]glob.Glob)
		} else {
			_, ok := c.imageGlobs[iname]
			if ok {
				return
			}
		}

		g, err := glob.Compile(iname, '/')
		if err == nil {
			c.imageGlobs[iname] = g
		}
	}
}

func (c *containerFilter) FilterFunc(i interface{}) bool {
	e := i.(*api.Event)

	//
	// Fast path: Check if containerId is in containerIds map
	//
	if c.containerIds != nil && c.containerIds[e.ContainerId] {
		return true
	}

	switch e.Event.(type) {
	case *api.Event_Container:
		cev := e.GetContainer()

		//
		// Slow path: Check if other identifiers are in maps. If they
		// are, add the containerId to containerIds map to take fast
		// path next time.
		//

		if c.containerNames[cev.Name] {
			c.addContainerId(e.ContainerId)
			return true
		}

		if c.imageIds[cev.ImageId] {
			c.addContainerId(e.ContainerId)
			return true
		}

		if c.imageGlobs != nil && cev.ImageName != "" {
			for _, g := range c.imageGlobs {
				if g.Match(cev.ImageName) {
					c.addContainerId(e.ContainerId)
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
			c.removeContainerId(e.ContainerId)
		}
	}
}
