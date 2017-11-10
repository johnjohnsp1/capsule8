package subscription

//
// This file implements a global process monitor that creates and
// caches unique process identifiers for all host processes. It also
// monitors for runc container starts to identify the containerID for
// a given PID namespace. Process information gathered by the process
// monitor may be retrieved through the processID and
// processContainerID functions.
//
// glog levels used:
//   10 = cache operation level tracing for debugging
//

import (
	"strings"
	"sync"

	"github.com/capsule8/capsule8/pkg/config"
	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/capsule8/capsule8/pkg/sys/perf"
	"github.com/capsule8/capsule8/pkg/sys/proc"
	"github.com/golang/glog"
)

const cacheArraySize = 32768

// UnknownID is the sentinal value for an unknown process ID or container ID
const UnknownID = "_"

var (
	fs   *proc.FileSystem
	once sync.Once
)

type taskCache interface {
	LookupTask(int, *task) bool
	InsertTask(int, task)
	SetTaskContainerID(int, string)
}

type arrayTaskCache struct {
	entries [cacheArraySize]task
}

func newArrayTaskCache() *arrayTaskCache {
	return &arrayTaskCache{}
}

func (c *arrayTaskCache) LookupTask(pid int, t *task) bool {
	*t = c.entries[pid]
	ok := t.tgid != 0

	if ok {
		glog.V(10).Infof("LookupTask(%d) -> %+v", pid, t)
	} else {
		glog.V(10).Infof("LookupTask(%d) -> nil", pid)
	}

	return ok
}

func (c *arrayTaskCache) InsertTask(pid int, t task) {
	glog.V(10).Infof("InsertTask(%d, %+v)", pid, t)
	c.entries[pid] = t
}

func (c *arrayTaskCache) SetTaskContainerID(pid int, cID string) {
	glog.V(10).Infof("SetTaskContainerID(%d) = %s", pid, cID)
	c.entries[pid].containerID = cID
}

type mapTaskCache struct {
	sync.Mutex
	entries map[int]task
}

func newMapTaskCache() *mapTaskCache {
	return &mapTaskCache{
		entries: make(map[int]task),
	}
}

func (c *mapTaskCache) LookupTask(pid int, t *task) (ok bool) {
	c.Lock()
	defer c.Unlock()

	*t, ok = c.entries[pid]

	if ok {
		glog.V(10).Infof("LookupTask(%d) -> %+v", pid, t)
	} else {
		glog.V(10).Infof("LookupTask(%d) -> nil", pid)
	}

	return ok
}

func (c *mapTaskCache) InsertTask(pid int, t task) {
	glog.V(10).Infof("InsertTask(%d, %+v)", pid, t)

	c.Lock()
	defer c.Unlock()

	c.entries[pid] = t
}

func (c *mapTaskCache) SetTaskContainerID(pid int, cID string) {
	glog.V(10).Infof("SetTaskContainerID(%d) = %s", pid, cID)

	c.Lock()
	defer c.Unlock()
	t, ok := c.entries[pid]
	if ok {
		t.containerID = cID
		c.entries[pid] = t
	}
}

//
// In order to avoid needing to lock, we pre-allocate two arrays of
// strings for the last-seen UniqueID and ContainerID for a given PID,
// respectively.
//
var cache taskCache
var procFS *proc.FileSystem

func init() {
	maxPid := proc.MaxPid()

	if maxPid > cacheArraySize {
		cache = newMapTaskCache()
	} else {
		cache = newArrayTaskCache()
	}
}

// lookupLeader finds the task info for the thread group leader of the given pid
func lookupLeader(pid int) (task, bool) {
	var t task

	for p := pid; cache.LookupTask(p, &t) && t.pid != t.tgid; p = t.ppid {
		// Do nothing
	}

	return t, t.pid == t.tgid
}

// processID returns the unique ID for the process indicated by the
// given PID. This process ID is identical whether it is derived
// inside or outside a container.
func processID(pid int) (string, bool) {
	leader, ok := lookupLeader(pid)
	if ok {
		return leader.processID, true
	}

	return "", false
}

// processContainerID returns the container ID that the process
// indicated by the given host PID.
func processContainerID(pid int) (string, bool) {
	var t task
	for p := pid; cache.LookupTask(p, &t); p = t.ppid {
		if len(t.containerID) > 0 {
			return t.containerID, true
		}
	}

	return "", false
}

// ProcessMonitor returns the global instance of the process monitor
func ProcessMonitor() *processMonitor {
	globalProcessMonitor.Do(func() {
		var (
			err          error
			eventMonitor *perf.EventMonitor
			cgroupList   []string
		)

		procFS = sys.HostProcFS()
		if procFS == nil {
			glog.Fatal("Couldn't find a host procfs")
		}

		//
		// In order for the sensor to be able to identify the
		// associated container of *any* perf-related event,
		// process monitor needs to be able to see the
		// launches of the runc init processes. This usually
		// means it needs to monitor the entire system.
		//
		// Users who do not care about containers can list one
		// or more cgroups in config.Sensor.CgroupName to
		// reduce the entire scope of system activity
		// monitored by the Sensor (and Linux kernel). If runc
		// does not run within these cgroups, then no events
		// will include associated container information.
		//

		// If a list of cgroups have been specified, only monitor those
		for _, cgroup := range config.Sensor.CgroupName {
			if len(cgroup) == 0 || cgroup == "/" {
				continue
			}

			cgroupList = append(cgroupList, cgroup)
		}

		if len(cgroupList) > 0 {
			glog.V(1).Infof("Creating process monitor on cgroups %s",
				strings.Join(cgroupList, ","))

			eventMonitor, err = perf.NewEventMonitor(0, nil,
				perf.WithCgroups(cgroupList))
			if err != nil {
				// If there was an error, warn on it, but fall
				// through to trying to create the system-wide
				// monitor below.
				glog.Warningf("Couldn't create perf event monitor on cgroups %s: %s",
					strings.Join(cgroupList, ","), err)
			}
		}

		if eventMonitor == nil {
			glog.V(1).Info("Creating new system-wide process monitor")
			eventMonitor, err = perf.NewEventMonitor(0, nil)
			if err != nil {
				glog.Fatalf("Couldn't create event monitor: %s", err)
			}
		}

		eventName := "task/task_newtask"
		err = eventMonitor.RegisterEvent(eventName, decodeNewTask, "", nil)
		if err != nil {
			glog.Fatalf("Couldn't register event %s: %s",
				eventName, err)
		}

		//
		// Attach a probe for task_renames involving the runc
		// init processes to trigger containerID lookups
		//
		f := "oldcomm ~ runc:* || newcomm ~ runc:*"
		eventName = "task/task_rename"
		err = eventMonitor.RegisterEvent(eventName, decodeRuncTaskRename, f, nil)
		if err != nil {
			glog.Fatalf("Couldn't register event %s: %s",
				eventName, err)
		}

		go func() {
			err := eventMonitor.Run(func(sample interface{}, err error) {
				if err != nil {
					glog.Warningf("Couldn't decode sample: %s",
						err)
				}
			})

			if err != nil {
				glog.Fatalf("Couldn't run event monitor: %s",
					err)
			}

			glog.Fatal("Exiting EventMonitor.Run goroutine")
		}()

		globalProcessMonitor.eventMonitor = eventMonitor
		eventMonitor.Enable()
	})

	return &globalProcessMonitor
}

type processMonitor struct {
	sync.Once
	eventMonitor *perf.EventMonitor
}

var globalProcessMonitor processMonitor

//
// task represents a schedulable task
//
type task struct {
	pid, ppid, tgid int
	cloneFlags      uint64
	command         string
	processID       string
	containerID     string
}

//
// Decodes each task/task_newtask tracepoint event into a processCacheEntry
//
func decodeNewTask(sample *perf.SampleRecord,
	data perf.TraceEventSampleData,
) (interface{}, error) {
	parentPid := int(data["common_pid"].(int32))
	childPid := int(data["pid"].(int32))
	cloneFlags := data["clone_flags"].(uint64)

	// This is not ideal
	comm := data["comm"].([]interface{})
	comm2 := make([]byte, len(comm))
	for i, c := range comm {
		b := c.(int8)
		if b == 0 {
			break
		}

		comm2[i] = byte(b)
	}
	command := string(comm2)

	var tgid int
	var uniqueID, containerID string

	const CLONE_THREAD = 0x10000
	if (cloneFlags & CLONE_THREAD) != 0 {
		tgid = parentPid

		uniqueID, _ = processID(tgid)

		// Inherit containerID from parent
		containerID, _ = processContainerID(parentPid)
	} else {
		// This is a new thread group leader, tgid is the new pid
		tgid = childPid

		// Create unique ID for thread group leaders
		uniqueID = proc.DeriveUniqueID(tgid, parentPid)

		// Inherit containerID from parent
		containerID, _ = processContainerID(parentPid)
	}

	// Lookup containerID from /proc filesystem for runc inits
	if len(containerID) == 0 && strings.HasPrefix(command, "runc:") {
		var err error

		containerID, err = procFS.ContainerID(parentPid)
		if err == nil && len(containerID) > 0 {
			// Set it in the parent as well
			cache.SetTaskContainerID(parentPid, containerID)
		} else {
			containerID, err = procFS.ContainerID(childPid)
		}
	}

	t := task{
		pid:         childPid,
		ppid:        parentPid,
		tgid:        tgid,
		cloneFlags:  cloneFlags,
		command:     command,
		processID:   uniqueID,
		containerID: containerID,
	}

	cache.InsertTask(t.pid, t)

	return nil, nil
}

//
// decodeRuncTaskRename is called when runc exec's and obtains the containerID
// from /procfs and caches it.
//
func decodeRuncTaskRename(
	sample *perf.SampleRecord,
	data perf.TraceEventSampleData,
) (interface{}, error) {
	pid := int(data["pid"].(int32))

	var t task
	cache.LookupTask(pid, &t)

	if len(t.containerID) == 0 {
		containerID, err := procFS.ContainerID(pid)
		if err == nil && len(containerID) > 0 {
			cache.SetTaskContainerID(pid, containerID)
		}
	} else {
		var parent task
		cache.LookupTask(t.ppid, &parent)

		if len(parent.containerID) == 0 {
			containerID, err := procFS.ContainerID(parent.pid)
			if err == nil && len(containerID) > 0 {
				cache.SetTaskContainerID(parent.pid, containerID)
			}

		}
	}

	return nil, nil
}
