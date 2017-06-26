package process

import "github.com/capsule8/reactive8/pkg/api/event"

func NewProcessFilter(filters []*event.ProcessFilter) {
	// If filtering by one or more pids, add an ftrace filter on
	// pids

	// If filtering by one or more filenames, add an ftrace filter
	// on exec.

	// If filtering by cgroup, ... ?

	// If filtering by containers, monitor containers and update list
	// of cgroups
}
