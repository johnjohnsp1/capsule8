package sys

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/capsule8/reactive8/pkg/sys/proc"
	"github.com/golang/glog"
)

var (
	mountOnce sync.Once
	mountInfo []MountInfo

	// Host procfs mounted into our namespace when running as a container
	hostProcFSOnce sync.Once
	hostProcFS     *proc.FileSystem
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

func getMounts() ([]MountInfo, error) {
	data, err := proc.ReadFile("self/mountinfo")
	if err != nil {
		return nil, err
	}

	var mounts []MountInfo

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()

		glog.V(3).Infof("Parsing mountinfo line: %s", line)

		fields := strings.Split(line, " ")

		mountID, err := strconv.Atoi(fields[0])
		if err != nil {
			glog.V(2).Infof("Couldn't parse mountID %s", fields[0])
			continue
		}

		parentID, err := strconv.Atoi(fields[1])
		if err != nil {
			glog.V(2).Infof("Couldn't parse parentID %s", fields[1])
			continue
		}

		mm := strings.Split(fields[2], ":")
		major, err := strconv.Atoi(mm[0])
		if err != nil {
			glog.V(2).Infof("Couldn't parse major %s", mm[0])
			continue
		}

		minor, err := strconv.Atoi(mm[1])
		if err != nil {
			glog.V(2).Infof("Couldn't parse minor %s", mm[1])
			continue
		}

		glog.V(3).Infof("Parsing mountOptions: %s", fields[5])
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

		glog.V(3).Infof("Parsing superOptions: %s", superOptions)

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

// GetMountInfo returns the list of filesystems mounted at process
// startup time. It does not reflect runtime changes to the list of
// mounted filesystems.
func GetMountInfo() []MountInfo {
	mountOnce.Do(func() {
		var err error

		mountInfo, err = getMounts()
		if err != nil {
			panic(err)
		}
	})

	return mountInfo
}

// GetProcFS creates a proc.FileSystem representing the default procfs
// mountpoint /proc. When running inside a container, this will
// contain information from the container's pid namespace.
func GetProcFS() *proc.FileSystem {
	return proc.GetProcFS()
}

// GetHostProcFS creates a proc.FileSystem representing the underlying
// host's procfs. If we are running in the host pid namespace, it uses
// /proc. Otherwise, it identifies a mounted-in host procfs by it
// being mounted on a directory that isn't /proc and /proc/self
// linking to a differing PID than that returned by os.Getpid(). If we
// are running in a container and no mounted-in host procfs was
// identified, then it returns nil.
func GetHostProcFS() *proc.FileSystem {
	hostProcFSOnce.Do(func() {
		hostProcFS = getHostProcFS()
	})

	return hostProcFS
}

func getHostProcFS() *proc.FileSystem {
	//
	// Look at /proc's init to see if it is in one or more root
	// cgroup paths.
	//
	procFS := GetProcFS()
	initCgroups := procFS.GetCgroups(1)
	for _, cg := range initCgroups {
		if cg.Path == "/" {
			// /proc is a host procfs, return it
			return procFS
		}
	}

	//
	// /proc isn't a host procfs, so search all mounted filesystems for it
	//
	mountInfo := GetMountInfo()

	for _, mi := range mountInfo {
		if mi.FilesystemType == "proc" {
			if mi.MountPoint != "/proc" {
				pid := os.Getpid()
				procSelf := filepath.Join(mi.MountPoint, "self")
				ps, err := os.Readlink(procSelf)
				if err != nil {
					return nil
				}

				_, file := filepath.Split(ps)
				procPid, err := strconv.Atoi(file)
				if err != nil {
					return nil
				}

				if pid != procPid {
					return &proc.FileSystem{
						MountPoint: mi.MountPoint,
					}
				}
			}
		}
	}

	return nil
}

// GetTraceFSMountPoint returns the mountpoint of the linux kernel
// tracing subsystem pseudo-filesystem (a replacement for the older
// debugfs).
func GetTraceFSMountPoint() string {
	mountInfo := GetMountInfo()

	// Look for an existing tracefs
	for _, mi := range mountInfo {
		if mi.FilesystemType == "tracefs" {
			glog.V(1).Infof("Found tracefs at %s", mi.MountPoint)
			return mi.MountPoint
		}
	}

	return ""
}

// GetCgroupPerfEventFSMountPoint returns the mountpoint of the
// perf_event cgroup pseudo-filesystem or an empty string if it wasn't
// found.
func GetCgroupPerfEventFSMountPoint() string {
	mountInfo := GetMountInfo()

	for _, mi := range mountInfo {
		if mi.FilesystemType == "cgroup" {
			for option := range mi.SuperOptions {
				if option == "perf_event" {
					return mi.MountPoint
				}
			}
		}
	}

	return ""
}
