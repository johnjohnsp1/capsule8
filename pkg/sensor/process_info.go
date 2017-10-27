package sensor

//
// The purpose of this cache at present is to maintain process and
// container information after they have exited. It is only consulted
// when process or container information can't be retrieved from
// procfs.
//
// The process stat and container Id lookups through processId and
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

	containerId(int32) string
	setContainerId(int32, string)
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

func (c *arrayProcessInfoCache) containerId(pid int32) string {
	return c.entries[pid].containerId
}

func (c *arrayProcessInfoCache) setContainerId(pid int32, containerId string) {
	c.entries[pid].containerId = containerId
}

type mapProcessInfoCache struct {
	sync.Mutex
	entries map[int32]processCacheEntry
}

type processCacheEntry struct {
	status      *proc.ProcessStatus
	containerId string
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

func (c *mapProcessInfoCache) containerId(pid int32) string {
	c.Lock()
	defer c.Unlock()

	entry, ok := c.entries[pid]
	if ok {
		return entry.containerId
	}

	return ""
}

func (c *mapProcessInfoCache) setContainerId(pid int32, containerId string) {
	c.Lock()
	defer c.Unlock()

	entry, ok := c.entries[pid]
	if ok {
		entry.containerId = containerId
	} else {
		entry := processCacheEntry{
			containerId: containerId,
		}
		c.entries[pid] = entry
	}
}

//
// In order to avoid needing to lock, we pre-allocate two arrays of
// strings for the last-seen UniqueId and ContainerId for a given PID,
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

// processId returns the unique Id for the process indicated by the
// given PID. The unique Id is identical whether it is derived inside
// or outside a container.
func processId(pid int32) string {
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

	glog.V(10).Infof("processId(%d) -> %s", pid, uid)
	return uid
}

// processContainerId returns the container Id that the process
// indicated by the given host PID.
func processContainerId(pid int32) string {
	var cid string
	var err error

	cid, err = procFS().ContainerID(pid)
	if err == nil {
		if len(cid) > 0 {
			//
			// If the process was found *and* it is in a container,
			// cache the container Id for the given PID.
			//
			glog.V(10).Infof("setContainerId(%d) = [%s]", pid, cid)
			cache.setContainerId(pid, cid)
		}

		glog.V(10).Infof("processContainerId(%d) -> %s", pid, cid)
		return cid
	}

	//
	// If the process was not found, check the cache for a
	// last-seen container Id.
	//
	cid = cache.containerId(pid)
	if len(cid) > 0 {
		glog.V(10).Infof("processContainerId(%d) -> %s", pid, cid)
		return cid
	}

	//
	// If we never saw a ContainerId from /proc, then the process
	// is a container init that is already gone. Check the cache
	// for its status to find its parent and return the parent's
	// containerId.
	//
	status := cache.status(pid)
	if status != nil {
		ppid := status.ParentPID()
		cid = processContainerId(ppid)
		glog.V(10).Infof("processContainerId(%d) -> %s", pid, cid)
		return cid
	}

	glog.V(10).Infof("processContainerId(%d) -> %s", pid, cid)
	return cid
}
