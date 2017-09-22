package sys

import (
	"os"
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

func TestMountPrivateTracingFs(t *testing.T) {
	// TODO: Check for CAP_SYS_ADMIN instead
	if os.Getuid() != 0 {
		t.Skip("Don't have root privileges needed to mount filesystems")
	}

	mountDir, err := mountPrivateTracingFs()
	if err != nil {
		t.Fatal(err)
	}

	mountInfo, err := discoverMounts()
	if err != nil {
		t.Fatal(err)
	}

	glog.V(1).Infof("%+v", mountInfo)

	err = unmountPrivateTracingFs(mountDir)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetPerfEventCgroupFs(t *testing.T) {
	perfEventCgroupFs, err := getPerfEventCgroupFs()
	if err != nil {
		t.Fatal(err)
	}

	if len(perfEventCgroupFs) == 0 {
		t.Fatal("Couldn't find a mounted perf_event cgroup filesystem")
	}

	glog.V(1).Infof("Found perf_event cgroup filesystem mounted at %s",
		perfEventCgroupFs)
}

func TestGetProcFs(t *testing.T) {
	oldProcFs := config.Sensor.ProcFs
	config.Sensor.ProcFs = ""
	procFs := getProcFs()
	config.Sensor.ProcFs = oldProcFs

	if len(procFs) == 0 {
		t.Fatal("Couldn't find procfs")
	}
}

func TestGetTraceFs(t *testing.T) {
	oldTraceFs := config.Sensor.TraceFs
	config.Sensor.TraceFs = ""

	traceFs := getTraceFs()
	config.Sensor.TraceFs = oldTraceFs

	if len(traceFs) == 0 {
		t.Fatal("Could not find tracefs")
	}

	glog.V(1).Infof("Found tracefs at %s", traceFs)
}
