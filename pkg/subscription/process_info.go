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
	"sync"

	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/capsule8/capsule8/pkg/sys/proc"
	"github.com/golang/glog"
)

const cacheArraySize = 32768

var (
	fs   *proc.FileSystem
	once sync.Once
)

type processInfoCache interface {
	status(int32) *proc.ProcessStatus
	setStatus(int32, *proc.ProcessStatus)

	containerID(int32) string
	setContainerID(int32, string)
}

type arrayProcessInfoCache struct {
	entries [cacheArraySize]processCacheEntry
}

func newArrayProcessInfoCache() *arrayProcessInfoCache {
	return &arrayProcessInfoCache{}
}

func (c *arrayProcessInfoCache) status(pid int32) *proc.ProcessStatus {
	return c.entries[pid].status
}

func (c *arrayProcessInfoCache) setStatus(pid int32, status *proc.ProcessStatus) {
	c.entries[pid].status = status
}

func (c *arrayProcessInfoCache) containerID(pid int32) string {
	return c.entries[pid].containerID
}

func (c *arrayProcessInfoCache) setContainerID(pid int32, containerID string) {
	c.entries[pid].containerID = containerID
}

type mapProcessInfoCache struct {
	sync.Mutex
	entries map[int32]processCacheEntry
}

type processCacheEntry struct {
	status      *proc.ProcessStatus
	containerID string
}

func newMapProcessInfoCache() *mapProcessInfoCache {
	return &mapProcessInfoCache{
		entries: make(map[int32]processCacheEntry),
	}
}

func (c *mapProcessInfoCache) status(pid int32) *proc.ProcessStatus {
	c.Lock()
	defer c.Unlock()

	entry, ok := c.entries[pid]
	if ok {
		return entry.status
	}

	return nil
}

func (c *mapProcessInfoCache) setStatus(pid int32, status *proc.ProcessStatus) {
	c.Lock()
	defer c.Unlock()

	entry, ok := c.entries[pid]
	if ok {
		entry.status = status
	} else {
		entry := processCacheEntry{
			status: status,
		}
		c.entries[pid] = entry
	}
}

func (c *mapProcessInfoCache) containerID(pid int32) string {
	c.Lock()
	defer c.Unlock()

	entry, ok := c.entries[pid]
	if ok {
		return entry.containerID
	}

	return ""
}

func (c *mapProcessInfoCache) setContainerID(pid int32, containerID string) {
	c.Lock()
	defer c.Unlock()

	entry, ok := c.entries[pid]
	if ok {
		entry.containerID = containerID
	} else {
		entry := processCacheEntry{
			containerID: containerID,
		}
		c.entries[pid] = entry
	}
}

//
// In order to avoid needing to lock, we pre-allocate two arrays of
// strings for the last-seen UniqueID and ContainerID for a given PID,
// respectively.
//
var cache processInfoCache

func procFS() *proc.FileSystem {
	once.Do(func() {
		fs = sys.HostProcFS()
		maxPid := proc.MaxPid()

		if maxPid > cacheArraySize {
			cache = newMapProcessInfoCache()
		} else {
			cache = newArrayProcessInfoCache()
		}
	})

	return fs
}

// processID returns the unique ID for the process indicated by the
// given PID. The unique ID is identical whether it is derived inside
// or outside a container.
func processID(pid int32) string {
	var uid string

	ps := procFS().Stat(pid)
	if ps != nil {
		glog.V(10).Infof("setStatus(%d)", pid)
		cache.setStatus(pid, ps)
	} else {
		ps = cache.status(pid)
	}

	if ps != nil {
		uid = ps.UniqueID()
	}

	glog.V(10).Infof("processID(%d) -> %s", pid, uid)
	return uid
}

// processContainerID returns the container ID that the process
// indicated by the given host PID.
func processContainerID(pid int32) string {
	var cid string
	var err error

	cid, err = procFS().ContainerID(pid)
	if err == nil {
		if len(cid) > 0 {
			//
			// If the process was found *and* it is in a container,
			// cache the container ID for the given PID.
			//
			glog.V(10).Infof("setContainerID(%d) = [%s]", pid, cid)
			cache.setContainerID(pid, cid)
		}

		glog.V(10).Infof("processContainerID(%d) -> %s", pid, cid)
		return cid
	}

	//
	// If the process was not found, check the cache for a
	// last-seen container ID.
	//
	cid = cache.containerID(pid)
	if len(cid) > 0 {
		glog.V(10).Infof("processContainerID(%d) -> %s", pid, cid)
		return cid
	}

	//
	// If we never saw a ContainerID from /proc, then the process
	// is a container init that is already gone. Check the cache
	// for its status to find its parent and return the parent's
	// containerID.
	//
	status := cache.status(pid)
	if status != nil {
		ppid := status.ParentPID()
		cid = processContainerID(ppid)
		glog.V(10).Infof("processContainerID(%d) -> %s", pid, cid)
		return cid
	}

	glog.V(10).Infof("processContainerID(%d) -> %s", pid, cid)
	return cid
}
