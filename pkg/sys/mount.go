// Copyright 2017 Capsule8, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sys

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/capsule8/capsule8/pkg/sys/proc"
	"github.com/golang/glog"
)

var (
	// Host procfs mounted into our namespace when running as a container
	hostProcFSOnce sync.Once
	hostProcFS     *proc.FileSystem
)

// Mount holds information about a mount in the process's mount namespace.
type Mount struct {
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

func readMounts() []Mount {
	//
	// We don't return an error, we just crash if the data format from the
	// Linux kernel has changed incompatibly.
	//

	data, err := proc.ReadFile("self/mountinfo")
	if err != nil {
		glog.Fatalf("Couldn't read self/mountinfo from proc")
	}

	var mounts []Mount

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, " ")

		mountID, err := strconv.Atoi(fields[0])
		if err != nil {
			glog.Fatalf("Couldn't parse mountID %s", fields[0])
		}

		parentID, err := strconv.Atoi(fields[1])
		if err != nil {
			glog.Fatalf("Couldn't parse parentID %s", fields[1])
		}

		mm := strings.Split(fields[2], ":")
		major, err := strconv.Atoi(mm[0])
		if err != nil {
			glog.Fatalf("Couldn't parse major %s", mm[0])
		}

		minor, err := strconv.Atoi(mm[1])
		if err != nil {
			glog.Fatalf("Couldn't parse minor %s", mm[1])
		}

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

		superOptionsMap := make(map[string]string)
		for _, option := range strings.Split(superOptions, ",") {
			nameValue := strings.Split(option, "=")
			if len(nameValue) > 1 {
				superOptionsMap[nameValue[0]] = nameValue[1]
			} else {
				superOptionsMap[nameValue[0]] = ""
			}
		}

		m := Mount{
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

		mounts = append(mounts, m)
	}

	return mounts
}

// Mounts returns the list of currently mounted filesystems.
func Mounts() []Mount {
	return readMounts()
}

// ProcFS creates a proc.FileSystem representing the default procfs
// mountpoint /proc. When running inside a container, this will
// contain information from the container's pid namespace.
func ProcFS() *proc.FileSystem {
	return proc.FS()
}

// HostProcFS creates a proc.FileSystem representing the underlying
// host's procfs. If we are running in the host pid namespace, it uses
// /proc. Otherwise, it identifies a mounted-in host procfs by it
// being mounted on a directory that isn't /proc and /proc/self
// linking to a differing PID than that returned by os.Getpid(). If we
// are running in a container and no mounted-in host procfs was
// identified, then it returns nil.
func HostProcFS() *proc.FileSystem {
	hostProcFSOnce.Do(func() {
		hostProcFS = findHostProcFS()
	})

	return hostProcFS
}

func findHostProcFS() *proc.FileSystem {
	//
	// Look at /proc's init to see if it is in one or more root
	// cgroup paths.
	//
	procFS := ProcFS()
	initCgroups, err := procFS.Cgroups(1)
	if err != nil {
		glog.Fatalf("Couldn't get cgroups for pid 1: %s", err)
	}

	for _, cg := range initCgroups {
		if cg.Path == "/" {
			// /proc is a host procfs, return it
			return procFS
		}
	}

	//
	// /proc isn't a host procfs, so search all mounted filesystems for it
	//
	for _, mi := range Mounts() {
		if mi.FilesystemType == "proc" {
			if mi.MountPoint != "/proc" {
				fs := proc.FileSystem{MountPoint: mi.MountPoint}
				initCgroups, err := fs.Cgroups(1)
				if err != nil {
					glog.Warningf("Couldn't get cgroups for pid 1 on procfs %s: %s",
						mi.MountPoint, err)
				}

				for _, cg := range initCgroups {
					if cg.Path == "/" {
						return &fs
					}
				}
			}
		}
	}

	return nil
}

// TracingDir returns the directory on either the debugfs or tracefs
// used to control the Linux kernel trace event subsystem.
func TracingDir() string {
	mounts := Mounts()

	// Look for an existing tracefs
	for _, m := range mounts {
		if m.FilesystemType == "tracefs" {
			glog.V(1).Infof("Found tracefs at %s", m.MountPoint)
			return m.MountPoint
		}
	}

	// If no mounted tracefs has been found, look for it as a
	// subdirectory of the older debugfs
	for _, m := range mounts {
		if m.FilesystemType == "debugfs" {
			d := filepath.Join(m.MountPoint, "tracing")
			s, err := os.Stat(filepath.Join(d, "events"))
			if err == nil && s.IsDir() {
				glog.V(1).Infof("Found debugfs w/ tracing at %s", d)
				return d
			}

			return m.MountPoint
		}
	}

	return ""
}

// PerfEventDir returns the mountpoint of the perf_event cgroup
// pseudo-filesystem or an empty string if it wasn't found.
func PerfEventDir() string {
	for _, mi := range Mounts() {
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

func MountTempFS(source string, target string, fstype string, flags uintptr, data string) error {
	// Make sure that `target` exists.
	err := os.MkdirAll(target, 0500)
	if err != nil {
		glog.V(2).Infof("Couldn't create temp %s mountpoint: %s", fstype, err)
		return err
	}

	err = unix.Mount(source, target, fstype, flags, data)
	if err != nil {
		glog.V(2).Infof("Couldn't mount %s on %s: %s", fstype, target, err)
		return err
	}

	return nil
}

func UnmountTempFS(dir string, fstype string) error {
	err := unix.Unmount(dir, 0)
	if err != nil {
		glog.V(2).Infof("Couldn't unmount %s at %s: %s", fstype, dir, err)
		return err
	}

	err = os.Remove(dir)
	if err != nil {
		glog.V(2).Infof("Couldn't remove %s: %s", dir, err)
		return err
	}

	return nil
}
