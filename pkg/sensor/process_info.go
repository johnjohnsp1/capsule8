package sensor

//
// The purpose of this cache at present is to maintain process and
// container information after they have exited. It is only consulted
// when process or container information can't be retrieved from
// procfs.
//
// The process stat and container ID lookups through processId and
// processContainerId are highly impacting on sensor performance since
// every event emitted will require at least one call to them. Any
// changes to this path should be considered along with benchmarking
// in order catch any performance regressions early.
//
// glog levels used:
//   10 = cache operation level tracing for debugging
//

import (
	"sync"

	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/capsule8/capsule8/pkg/sys/perf"
	"github.com/capsule8/capsule8/pkg/sys/proc"
	"github.com/golang/glog"
)

var (
	procFS *proc.FileSystem
	once   sync.Once
)

const arrayTaskCacheSize = 32768

type taskCache interface {
	LookupTask(int, *task) bool
	InsertTask(int, task)
	SetTaskContainerId(int, string)
}

type arrayTaskCache struct {
	entries [arrayTaskCacheSize]task
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

func (c *arrayTaskCache) SetTaskContainerId(pid int, cID string) {
	glog.V(10).Infof("SetTaskContainerId(%d) = %s", pid, cID)
	c.entries[pid].containerId = cID
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

func (c *mapTaskCache) SetTaskContainerId(pid int, cID string) {
	glog.V(10).Infof("SetTaskContainerId(%d) = %s", pid, cID)

	c.Lock()
	defer c.Unlock()
	t, ok := c.entries[pid]
	if ok {
		t.containerId = cID
		c.entries[pid] = t
	}
}

type ProcessInfoCache struct {
	sensor *Sensor
	cache  taskCache
}

func NewProcessInfoCache(sensor *Sensor) (*ProcessInfoCache, error) {
	once.Do(func() {
		procFS = sys.HostProcFS()
		if procFS == nil {
			glog.Fatal("Couldn't find a host procfs")
		}
	})

	cache := &ProcessInfoCache{
		sensor: sensor,
	}

	maxPid := proc.MaxPid()
	if maxPid > arrayTaskCacheSize {
		cache.cache = newMapTaskCache()
	} else {
		cache.cache = newArrayTaskCache()
	}

	// Register with the sensor's global event monitor...
	eventName := "task/task_newtask"
	err := sensor.monitor.RegisterEvent(eventName, cache.decodeNewTask, "", nil)
	if err != nil {
		glog.V(1).Infof("Couldn't register process info cache event: %s", err)
		return nil, err
	}

	return cache, nil
}

// lookupLeader finds the task info for the thread group leader of the given pid
func (pc *ProcessInfoCache) lookupLeader(pid int) (task, bool) {
	var t task

	for p := pid; pc.cache.LookupTask(p, &t) && t.pid != t.tgid; p = t.ppid {
		// Do nothing
	}

	return t, t.pid == t.tgid
}

// processId returns the unique ID for the process indicated by the
// given PID. This process ID is identical whether it is derived
// inside or outside a container.
func (pc *ProcessInfoCache) ProcessId(pid int) string {
	leader, ok := pc.lookupLeader(pid)
	if ok {
		return leader.processId
	}

	return ""
}

// processContainerId returns the container ID that the process
// indicated by the given host PID.
func (pc *ProcessInfoCache) ProcessContainerId(pid int) string {
	var t task
	for p := pid; pc.cache.LookupTask(p, &t); p = t.ppid {
		if len(t.containerId) > 0 {
			return t.containerId
		}
	}

	return ""
}

const CLONE_THREAD = 0x10000

//
// task represents a schedulable task
//
type task struct {
	pid, ppid, tgid int
	cloneFlags      uint64
	command         string
	processId       string
	containerId     string
}

//
// Decodes each task/task_newtask tracepoint event into a processCacheEntry
//
func (pc *ProcessInfoCache) decodeNewTask(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
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
	var uniqueId, containerId string

	if (cloneFlags & CLONE_THREAD) != 0 {
		tgid = parentPid

		//
		// Inherit containerId from tgid if possible
		//
		containerId = pc.ProcessContainerId(tgid)
		if len(containerId) == 0 {
			//
			// This is a bit aggressive to check /proc for every new
			// thread, but it's what helps us tag the container init
			// process
			//
			containerId, _ = procFS.ContainerID(tgid)
			if len(containerId) > 0 {
				pc.cache.SetTaskContainerId(tgid, containerId)
			}
		}

		uniqueId = pc.ProcessId(tgid)
	} else {
		//
		// This is a new thread group leader, tgid is the new pid
		//
		tgid = childPid

		//
		// Create unique ID for thread group leaders
		//
		uniqueId = proc.DeriveUniqueID(tgid, parentPid)
		containerId = pc.ProcessContainerId(parentPid)
		if len(containerId) == 0 {
			containerId, _ = procFS.ContainerID(tgid)
		}
	}

	t := task{
		pid:         childPid,
		ppid:        parentPid,
		tgid:        tgid,
		cloneFlags:  cloneFlags,
		command:     command,
		processId:   uniqueId,
		containerId: containerId,
	}

	pc.cache.InsertTask(t.pid, t)

	return nil, nil
}
