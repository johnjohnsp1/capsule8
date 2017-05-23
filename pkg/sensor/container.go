package sensor

import "github.com/capsule8/reactive8/pkg/api/event"

// NewContainerSensor creates a new container sensor configured by the given Selector
func NewContainerSensor(selector event.Selector, events chan<- *event.Event) (chan<- struct{}, <-chan error) {
	// Determine whether the node are running on supports Docker or Rkt containers

	return NewDockerOCISensor(selector, events)
}
