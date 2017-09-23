package sys

import (
	"testing"

	"github.com/golang/glog"
)

func TestGetMountInfo(t *testing.T) {
	mountInfo := GetMountInfo()

	if len(mountInfo) == 0 {
		t.Error("Empty mountInfo returned by GetMountInfo()")
	}

	glog.V(1).Infof("Discovered %v mounts", len(mountInfo))
}

func TestGetCgroupPerfEventFSMountPoint(t *testing.T) {
	perfEventCgroupFs := GetCgroupPerfEventFSMountPoint()

	if len(perfEventCgroupFs) == 0 {
		t.Skip("Couldn't find a mounted perf_event cgroup filesystem")
	}

	glog.V(1).Infof("Found perf_event cgroup filesystem mounted at %s",
		perfEventCgroupFs)
}

func TestGetProcFs(t *testing.T) {
	procFS := GetProcFS()

	if procFS == nil {
		t.Fatal("Couldn't find procfs")
	}
}

func TestGetTraceFSMountPoint(t *testing.T) {
	traceFs := GetTraceFSMountPoint()

	if len(traceFs) == 0 {
		t.Skip("Could not find tracefs")
	}

	glog.V(1).Infof("Found tracefs at %s", traceFs)
}
