package sys

import (
	"github.com/golang/glog"
)

func InContainer() bool {
	procFS := ProcFS()
	initCgroups, err := procFS.Cgroups(1)
	if err != nil {
		glog.Fatalf("Couldn't get cgroups for pid 1: %s", err)
	}

	for _, cg := range initCgroups {
		if cg.Path == "/" {
			// /proc is a host procfs, return it
			return false
		}
	}

	return true
}
