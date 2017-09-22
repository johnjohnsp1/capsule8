package sys

import (
	"testing"

	"github.com/capsule8/reactive8/pkg/config"
	"github.com/golang/glog"
)

func TestDiscoverMounts(t *testing.T) {
	mountInfo, err := discoverMounts()
	if err != nil {
		t.Fatal(err)
	}

	if len(mountInfo) == 0 {
		t.Error("Empty mountInfo returned by discoverMounts()")
	}

	glog.V(1).Infof("Discovered %v mounts", len(mountInfo))
}

func TestGetPerfEventCgroupFs(t *testing.T) {
	perfEventCgroupFs := GetPerfEventCgroupFs()

	if len(perfEventCgroupFs) == 0 {
		t.Fatal("Couldn't find a mounted perf_event cgroup filesystem")
	}

	glog.V(1).Infof("Found perf_event cgroup filesystem mounted at %s",
		perfEventCgroupFs)
}

func TestGetProcFs(t *testing.T) {
	oldProcFs := config.Sensor.ProcFs
	config.Sensor.ProcFs = ""
	procFs := GetProcFs()
	config.Sensor.ProcFs = oldProcFs

	if len(procFs) == 0 {
		t.Fatal("Couldn't find procfs")
	}
}

func TestGetTraceFs(t *testing.T) {
	oldTraceFs := config.Sensor.TraceFs
	config.Sensor.TraceFs = ""

	traceFs := GetTraceFs()
	config.Sensor.TraceFs = oldTraceFs

	if len(traceFs) == 0 {
		t.Skip("Could not find tracefs")
	}

	glog.V(1).Infof("Found tracefs at %s", traceFs)
}
