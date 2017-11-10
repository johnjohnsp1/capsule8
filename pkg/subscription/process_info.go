package subscription

//
// The purpose of this cache at present is to maintain process and
// container information after they have exited. It is only consulted
// when process or container information can't be retrieved from
// procfs.
//
// The process stat and container ID lookups through processID and
// processContainerID are highly impacting on sensor performance since
// every event emitted will require at least one call to them. Any
// changes to this path should be considered along with benchmarking
// in order catch any performance regressions early.
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
func processID(pid int) string {
	leader, ok := lookupLeader(pid)
	if ok {
		return leader.processID
	}

	return ""
}

// processContainerID returns the container ID that the process
// indicated by the given host PID.
func processContainerID(pid int) string {
	var t task
	for p := pid; cache.LookupTask(p, &t); p = t.ppid {
		if len(t.containerID) > 0 {
			return t.containerID
		}
	}

	return ""
}

const CLONE_THREAD = 0x10000

// ProcessMonitor returns the global instance of the process monitor
func ProcessMonitor() *processMonitor {
	globalProcessMonitor.Do(func() {
		procFS = sys.HostProcFS()
		if procFS == nil {
			glog.Fatal("Couldn't find a host procfs")
		}

		var cgroupList []string

		// If a list of cgroups have been specified, only monitor those
		for _, cgroup := range config.Sensor.CgroupName {
			if len(cgroup) == 0 || cgroup == "/" {
				continue
			}

			cgroupList = append(cgroupList, cgroup)
		}

		// If no cgroups have been specified, but we're running
		// in a container, monitor the 'docker' cgroup
		if len(cgroupList) == 0 && sys.InContainer() {
			cgroupList = append(cgroupList, "docker")
		}

		var eventMonitor *perf.EventMonitor
		var err error

		if len(sys.PerfEventDir()) > 0 && len(cgroupList) > 0 {
			glog.V(1).Infof("Creating process monitor on cgroups %s",
				strings.Join(cgroupList, ","))
			eventMonitorOptions := []perf.EventMonitorOption{
				perf.WithPerfEventDir(sys.PerfEventDir()),
				perf.WithCgroups(cgroupList),
			}

			eventMonitor, err = perf.NewEventMonitor(0, nil, eventMonitorOptions...)
		}

		if err != nil || len(sys.PerfEventDir()) == 0 || len(cgroupList) == 0 {
			glog.V(1).Info("Creating new system-wide process monitor")
			eventMonitorOptions := []perf.EventMonitorOption{
				perf.WithPid(-1),
			}

			eventMonitor, err = perf.NewEventMonitor(0, nil, eventMonitorOptions...)
			if err != nil {
				glog.Fatal(err)
			}
		}

		eventName := "task/task_newtask"
		err = eventMonitor.RegisterEvent(eventName, decodeNewTask, "", nil)
		if err != nil {
			glog.Fatal(err)
		}

		go func() {
			err := eventMonitor.Run(func(sample interface{}, err error) {
				if err != nil {
					glog.Warning(err)
				}
			})

			if err != nil {
				glog.Fatal(err)
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
func decodeNewTask(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
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

	if (cloneFlags & CLONE_THREAD) != 0 {
		tgid = parentPid

		//
		// Inherit containerID from tgid if possible
		//
		containerID = processContainerID(tgid)
		if len(containerID) == 0 {
			//
			// This is a bit aggressive to check /proc for every new
			// thread, but it's what helps us tag the container init
			// process
			//
			containerID, _ = procFS.ContainerID(tgid)
			if len(containerID) > 0 {
				cache.SetTaskContainerID(tgid, containerID)
			}
		}

		uniqueID = processID(tgid)
	} else {
		//
		// This is a new thread group leader, tgid is the new pid
		//
		tgid = childPid

		//
		// Create unique ID for thread group leaders
		//
		uniqueID = proc.DeriveUniqueID(tgid, parentPid)
		containerID = processContainerID(parentPid)
		if len(containerID) == 0 {
			containerID, _ = procFS.ContainerID(tgid)
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
