package sys

import (
	"testing"

	"github.com/golang/glog"
)

func TestMounts(t *testing.T) {
	mounts := Mounts()

	if len(mounts) == 0 {
		t.Error("Empty mountInfo returned by GetMountInfo()")
	}

	glog.V(1).Infof("Discovered %v mounts", len(mounts))
}

func TestGetCgroupPerfEventFSMountPoint(t *testing.T) {
	perfEventDir := PerfEventDir()

	if len(perfEventDir) == 0 {
		t.Skip("Couldn't find a mounted perf_event cgroup filesystem")
	}

	glog.V(1).Infof("Found perf_event cgroup filesystem mounted at %s",
		perfEventDir)
}

func TestGetProcFs(t *testing.T) {
	procFS := ProcFS()

	if procFS == nil {
		t.Fatal("Couldn't find procfs")
	}
}

func TestGetTraceFSMountPoint(t *testing.T) {
	tracingDir := TracingDir()

	if len(tracingDir) == 0 {
		t.Skip("Could not find tracefs")
	}

	glog.V(1).Infof("Found tracefs at %s", tracingDir)
}
