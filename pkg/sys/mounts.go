package sys

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/capsule8/reactive8/pkg/config"
	"github.com/golang/glog"
)

// MountInfo holds information about a mount in the process's mount namespace.
type MountInfo struct {
	MountID        uint
	ParentID       uint
	Major          uint
	Minor          uint
	Root           string
	MountPoint     string
	MountOptions   []string
	OptionalFields map[string]string
	FilesystemType string
	MountSource    string
	SuperOptions   map[string]string
}

func discoverMounts() ([]MountInfo, error) {
	//
	// Open /proc/self/mountinfo to get mounts in our namespace
	//
	mountInfo, err := os.OpenFile("/proc/self/mountinfo", os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	var mounts []MountInfo

	scanner := bufio.NewScanner(mountInfo)
	for scanner.Scan() {
		line := scanner.Text()

		glog.V(2).Infof("Parsing mountinfo line: %s", line)

		fields := strings.Split(line, " ")

		mountID, err := strconv.Atoi(fields[0])
		if err != nil {
			glog.V(1).Infof("Couldn't parse mountID %s", fields[0])
			continue
		}

		parentID, err := strconv.Atoi(fields[1])
		if err != nil {
			glog.V(1).Infof("Couldn't parse parentID %s", fields[1])
			continue
		}

		mm := strings.Split(fields[2], ":")
		major, err := strconv.Atoi(mm[0])
		if err != nil {
			glog.V(1).Infof("Couldn't parse major %s", mm[0])
			continue
		}

		minor, err := strconv.Atoi(mm[1])
		if err != nil {
			glog.V(1).Infof("Couldn't parse minor %s", mm[1])
			continue
		}

		glog.V(2).Infof("Parsing mountOptions: %s", fields[5])
		mountOptions := strings.Split(fields[5], ",")

		optionalFieldsMap := make(map[string]string)
		var i int
		for i = 6; fields[i] != "-"; i++ {
			tagValue := strings.Split(fields[i], ":")
			optionalFieldsMap[tagValue[0]] = tagValue[1]
		}

		filesystemType := fields[i+1]
		mountSource := fields[i+2]
		superOptions := fields[i+3]

		glog.V(2).Infof("Parsing superOptions: %s", superOptions)

		superOptionsMap := make(map[string]string)
		for _, option := range strings.Split(superOptions, ",") {
			nameValue := strings.Split(option, "=")
			if len(nameValue) > 1 {
				superOptionsMap[nameValue[0]] = nameValue[1]
			} else {
				superOptionsMap[nameValue[0]] = ""
			}
		}

		mi := MountInfo{
			MountID:        uint(mountID),
			ParentID:       uint(parentID),
			Major:          uint(major),
			Minor:          uint(minor),
			Root:           fields[3],
			MountPoint:     fields[4],
			MountOptions:   mountOptions,
			OptionalFields: optionalFieldsMap,
			FilesystemType: filesystemType,
			MountSource:    mountSource,
			SuperOptions:   superOptionsMap,
		}

		mounts = append(mounts, mi)
	}

	return mounts, nil
}

func mountPrivateTracingFs() (string, error) {
	dir, err := ioutil.TempDir("", "tracefs")
	if err != nil {
		return "", err
	}

	err = unix.Mount("tracefs", dir, "tracefs", 0, "rw")
	if err != nil {
		return "", err
	}

	return dir, nil
}

func unmountPrivateTracingFs(dir string) error {
	return unix.Unmount(dir, 0)
}

func getTraceFs() string {
	// Allow configuration to override
	if len(config.Sensor.TraceFs) > 0 {
		return config.Sensor.TraceFs
	}

	mountInfo, err := discoverMounts()
	if err != nil {
		return ""
	}

	// Look for an existing tracefs
	for _, mi := range mountInfo {
		if mi.FilesystemType == "tracefs" {
			return mi.MountPoint
		}
	}

	// If we couldn't find one, try mounting our own private one
	mountPoint, err := mountPrivateTracingFs()
	if err != nil {
		return ""
	}

	return mountPoint
}

func getProcFs() string {
	// Allow configuration to override
	if len(config.Sensor.ProcFs) > 0 {
		return config.Sensor.ProcFs
	}

	mountInfo, err := discoverMounts()
	if err != nil {
		return ""
	}

	for _, mi := range mountInfo {
		if mi.FilesystemType == "proc" {
			if mi.MountPoint == "/proc" {
				return mi.MountPoint
			}
		}
	}

	return ""
}

// getHostProcFs returns the underlying host's procfs when it has been mounted
// into a running container. It identifies a mounted-in host procfs by it being
// mounted on a directory that isn't /proc and /proc/self linking to a host
// namespace PID.
func getHostProcFs() (string, error) {
	mountInfo, err := discoverMounts()
	if err != nil {
		return "", err
	}

	for _, mi := range mountInfo {
		if mi.FilesystemType == "proc" {
			if mi.MountPoint != "/proc" {
				pid := os.Getpid()
				ps, err := os.Readlink("/proc/self")
				if err != nil {
					return "", err
				}

				_, file := filepath.Split(ps)
				psPid, err := strconv.Atoi(file)
				if err != nil {
					return "", err
				}

				if pid != psPid {
					return mi.MountPoint, nil
				}
			}
		}
	}

	return "", err
}

func getPerfEventCgroupFs() (string, error) {
	mountInfo, err := discoverMounts()
	if err != nil {
		return "", err
	}

	for _, mi := range mountInfo {
		if mi.FilesystemType == "cgroup" {
			for option := range mi.SuperOptions {
				if option == "perf_event" {
					return mi.MountPoint, nil
				}
			}
		}
	}

	return "", err
}

func init() {

}
