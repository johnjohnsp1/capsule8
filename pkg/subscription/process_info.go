package subscription

import (
	"sync"

	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/capsule8/capsule8/pkg/sys/proc"
)

const cacheArraySize = 32768

var (
	fs   *proc.FileSystem
	once sync.Once
)

type processInfoCache interface {
	uniqueID(int32) string
	setUniqueID(int32, string)

	containerID(int32) string
	setContainerID(int32, string)
}

type arrayProcessInfoCache struct {
	uniqueIDs    [cacheArraySize]string
	containerIDs [cacheArraySize]string
}

func newArrayProcessInfoCache() *arrayProcessInfoCache {
	return &arrayProcessInfoCache{}
}

func (c *arrayProcessInfoCache) uniqueID(pid int32) string {
	return c.uniqueIDs[pid]
}

func (c *arrayProcessInfoCache) setUniqueID(pid int32, ID string) {
	c.uniqueIDs[pid] = ID
}

func (c *arrayProcessInfoCache) containerID(pid int32) string {
	return c.containerIDs[pid]
}

func (c *arrayProcessInfoCache) setContainerID(pid int32, containerID string) {
	c.containerIDs[pid] = containerID
}

type mapProcessInfoCache struct {
	sync.Mutex
	uniqueIDs    map[int32]string
	containerIDs map[int32]string
}

func newMapProcessInfoCache() *mapProcessInfoCache {
	return &mapProcessInfoCache{
		uniqueIDs:    make(map[int32]string),
		containerIDs: make(map[int32]string),
	}
}

func (c *mapProcessInfoCache) uniqueID(pid int32) string {
	c.Lock()
	uid := c.uniqueIDs[pid]
	c.Unlock()
	return uid
}

func (c *mapProcessInfoCache) setUniqueID(pid int32, ID string) {
	c.Lock()
	c.uniqueIDs[pid] = ID
	c.Unlock()
}

func (c *mapProcessInfoCache) containerID(pid int32) string {
	c.Lock()
	cid := c.containerIDs[pid]
	c.Unlock()
	return cid
}

func (c *mapProcessInfoCache) setContainerID(pid int32, containerID string) {
	c.Lock()
	c.containerIDs[pid] = containerID
	c.Unlock()
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
	ps := procFS().Stat(pid)
	if ps != nil {
		uid := ps.UniqueID()
		cache.setUniqueID(pid, uid)
		return uid
	}

	return cache.uniqueID(pid)
}

// processContainerID returns the container ID that the process
// indicated by the given host PID.
func processContainerID(pid int32) string {
	cID, err := procFS().ContainerID(pid)
	if err == nil {
		cache.setContainerID(pid, cID)
		return cID
	}

	return cache.containerID(pid)
}
