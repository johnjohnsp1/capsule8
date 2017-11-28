package sensor

//
// This file implements a process information cache that uses a sensor's
// system-global EventMonitor to keep it up-to-date. The cache also monitors
// for runc container starts to identify the containerID for a given PID
// namespace. Process information gathered by the cache may be retrieved via
// the ProcessId and ProcessContainerId methods.
//
// glog levels used:
//   10 = cache operation level tracing for debugging
//

import (
	"strings"
	"sync"

	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/capsule8/capsule8/pkg/sys/perf"
	"github.com/capsule8/capsule8/pkg/sys/proc"
	"github.com/golang/glog"
)

const (
	arrayTaskCacheSize = 32768
	commitCredsAddress = "commit_creds"
	commitCredsArgs    = "usage=+0(%di):u64 uid=+8(%di):u32 gid=+12(%di):u32"
)

var (
	procFS *proc.FileSystem
	once   sync.Once
)

type taskCache interface {
	LookupTask(int, *task) bool
	InsertTask(int, task)
	SetTaskContainerId(int, string)
	SetTaskCredentials(int, cred)
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

func (c *arrayTaskCache) SetTaskCredentials(pid int, creds cred) {
	glog.V(10).Infof("SetTaskCredentials(%d) = %+v", pid, creds)

	c.entries[pid].creds = creds
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

func (c *mapTaskCache) SetTaskCredentials(pid int, creds cred) {
	glog.V(10).Infof("SetTaskCredentials(%d) = %+v", pid, creds)

	c.Lock()
	defer c.Unlock()
	t, ok := c.entries[pid]
	if ok {
		t.creds = creds
		c.entries[pid] = t
	}
}

type ProcessInfoCache struct {
	sensor *Sensor
	cache  taskCache
}

func NewProcessInfoCache(sensor *Sensor) ProcessInfoCache {
	once.Do(func() {
		procFS = sys.HostProcFS()
		if procFS == nil {
			glog.Fatal("Couldn't find a host procfs")
		}
	})

	cache := ProcessInfoCache{
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
		glog.Fatalf("Couldn't register event %s: %s", eventName, err)
	}

	// Attach kprobe on commit_creds to capture task privileges
	name := perf.UniqueProbeName("capsule8", commitCredsAddress)
	_, err = sensor.monitor.RegisterKprobe(name, commitCredsAddress, false,
		commitCredsArgs, cache.decodeCommitCreds, "", nil)

	// Attach a probe for task_renamse involving the runc
	// init processes to trigger containerId lookups
	f := "oldcomm ~ runc* || newcomm ~ runc:*"
	eventName = "task/task_rename"
	err = sensor.monitor.RegisterEvent(eventName, cache.decodeRuncTaskRename, f, nil)
	if err != nil {
		glog.Fatalf("Couldn't register event %s: %s", eventName, err)
	}

	return cache
}

// lookupLeader finds the task info for the thread group leader of the given pid
func (pc *ProcessInfoCache) lookupLeader(pid int) (task, bool) {
	var t task

	for p := pid; pc.cache.LookupTask(p, &t) && t.pid != t.tgid; p = t.ppid {
		// Do nothing
	}

	return t, t.pid == t.tgid
}

// ProcessId returns the unique ID for the thread group of the process
// indicated by the given PID. This process ID is identical whether it
// is derived inside or outside a container.
func (pc *ProcessInfoCache) ProcessId(pid int) (string, bool) {
	leader, ok := pc.lookupLeader(pid)
	if ok {
		return proc.DeriveUniqueID(leader.pid, leader.ppid), true
	}

	return "", false
}

// processContainerId returns the container ID that the process
// indicated by the given host PID.
func (pc *ProcessInfoCache) ProcessContainerId(pid int) (string, bool) {
	var t task
	for p := pid; pc.cache.LookupTask(p, &t); p = t.ppid {
		if len(t.containerId) > 0 {
			return t.containerId, true
		}
	}

	return "", false
}

//
// task represents a schedulable task. All Linux tasks are uniquely
// identified at a given time by their PID, but those PIDs may be
// reused after hitting the maximum PID value.
//
type task struct {
	// All Linux schedulable tasks are identified by a PID. This includes
	// both processes and threads.
	pid int

	// Thread groups all have a leader, identified by its PID. The
	// thread group leader has tgid == pid.
	tgid int

	// This ppid is of the originating parent process vs. current
	// parent in case the parent terminates and the child is
	// reparented (usually to init).
	ppid int

	// Flags passed to clone(2) when this process was created.
	cloneFlags uint64

	// This is the kernel's comm field, which is initialized to a
	// the first 15 characters of the basename of the executable
	// being run. It is also set via pthread_setname_np(3) and
	// prctl(2) PR_SET_NAME. It is always NULL-terminated and no
	// longer than 16 bytes (including NULL byte).
	command string

	// Process credentials (uid, gid). This is kept up-to-date by
	// recording changes observed via a probe on commit_creds().
	creds cred

	// Unique ID for the container instance
	containerId string
}

type cred struct {
	// Set to true when this struct has been initialized. This
	// helps differentiate from processes running as root (all
	// cred fields are legitimately set to 0).
	initialized bool

	// Record uid and gid to have symmetry with eBPF get_current_uid_gid()
	uid, gid uint32
}

//
// Decodes each task/task_newtask tracepoint event into a processCacheEntry
//
func (pc *ProcessInfoCache) decodeNewTask(
	sample *perf.SampleRecord,
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
	var containerId string

	const CLONE_THREAD = 0x10000
	if (cloneFlags & CLONE_THREAD) != 0 {
		tgid = parentPid
	} else {
		// This is a new thread group leader, tgid is the new pid
		tgid = childPid
	}

	// Inherit containerId from parent
	containerId, _ = pc.ProcessContainerId(parentPid)

	// Lookup containerId from /proc filesystem for runc inits
	if len(containerId) == 0 && strings.HasPrefix(command, "runc:") {
		var err error

		containerId, err = procFS.ContainerID(parentPid)
		if err == nil && len(containerId) > 0 {
			// Set it in the parent as well
			pc.cache.SetTaskContainerId(parentPid, containerId)
		} else {
			containerId, err = procFS.ContainerID(childPid)
		}
	}

	t := task{
		pid:         childPid,
		ppid:        parentPid,
		tgid:        tgid,
		cloneFlags:  cloneFlags,
		command:     command,
		containerId: containerId,
	}

	pc.cache.InsertTask(t.pid, t)

	return nil, nil
}

//
// Decodes each commit_creds dynamic tracepoint event and updates cache
//
func (pc *ProcessInfoCache) decodeCommitCreds(
	sample *perf.SampleRecord,
	data perf.TraceEventSampleData,
) (interface{}, error) {
	pid := int(data["common_pid"].(int32))

	usage := data["usage"].(uint64)

	if usage == 0 {
		glog.Fatal("Received commit_creds with zero usage")
	}

	uid := data["uid"].(uint32)
	gid := data["gid"].(uint32)

	c := cred{
		initialized: true,
		uid:         uid,
		gid:         gid,
	}

	pc.cache.SetTaskCredentials(pid, c)

	return nil, nil
}

//
// decodeRuncTaskRename is called when runc exec's and obtains the containerID
// from /procfs and caches it.
//
func (pc *ProcessInfoCache) decodeRuncTaskRename(
	sample *perf.SampleRecord,
	data perf.TraceEventSampleData,
) (interface{}, error) {
	pid := int(data["pid"].(int32))

	var t task
	pc.cache.LookupTask(pid, &t)

	if len(t.containerId) == 0 {
		containerId, err := procFS.ContainerID(pid)
		if err == nil && len(containerId) > 0 {
			pc.cache.SetTaskContainerId(pid, containerId)
		}
	} else {
		var parent task
		pc.cache.LookupTask(t.ppid, &parent)

		if len(parent.containerId) == 0 {
			containerId, err := procFS.ContainerID(parent.pid)
			if err == nil && len(containerId) > 0 {
				pc.cache.SetTaskContainerId(parent.pid, containerId)
			}

		}
	}

	return nil, nil
}
