package process

import (
	"sync"

	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/capsule8/capsule8/pkg/sys/proc"
)

var (
	cache     map[int32]*Info
	cacheLock sync.Mutex
	cacheOnce sync.Once
	procFS    *proc.FileSystem
)

// Info describes a running process on the Node
type Info struct {
	UniqueID    string
	Pid         int32
	Ppid        int32
	Command     string
	ContainerID string
}

// CacheUpdate updates the cache with the given information when they
// have valid values. If ppid, command, or containerID have invalid
// values, they will be looked up via procfs.
func CacheUpdate(hostPid int32, ppid int32, command string, cID string) *Info {
	cacheOnce.Do(func() {
		cache = make(map[int32]*Info)
		procFS = sys.HostProcFS()
	})

	ps := procFS.Stat(hostPid)
	if ps == nil {
		// Process doesn't exist, nothing to do
		return nil
	}

	cacheLock.Lock()
	defer cacheLock.Unlock()

	i, ok := cache[hostPid]
	if !ok {
		i = &Info{
			UniqueID: ps.UniqueID(),
			Pid:      hostPid,
		}

		cache[hostPid] = i
	}

	if i.Ppid == 0 {
		if ppid > 0 {
			i.Ppid = ppid
		} else {
			ps = procFS.Stat(hostPid)
			if ps != nil {
				i.Ppid = ps.ParentPID()
			}
		}
	}

	if len(i.Command) == 0 {
		if len(command) > 0 {
			i.Command = command
		} else {
			if ps == nil {
				ps = procFS.Stat(hostPid)
			}

			if ps != nil {
				i.Command = ps.Command()
			}
		}
	}

	if len(i.ContainerID) == 0 {
		if len(cID) > 0 {
			i.ContainerID = cID
		} else {
			parent, parentOk := cache[i.Ppid]
			if parentOk {
				i.ContainerID = parent.ContainerID
			}
		}
	}

	return i
}

// CacheDelete deletes a process from the cache
func CacheDelete(hostPid int32) {
	cacheOnce.Do(func() {
		cache = make(map[int32]*Info)
		procFS = sys.HostProcFS()
	})

	cacheLock.Lock()
	delete(cache, hostPid)
	cacheLock.Unlock()
}

// GetInfo information on the process indicated by the given pid.
func GetInfo(hostPid int32) *Info {
	cacheOnce.Do(func() {
		cache = make(map[int32]*Info)
		procFS = sys.HostProcFS()
	})

	cacheLock.Lock()
	i, ok := cache[hostPid]
	cacheLock.Unlock()

	if ok {
		return i
	}

	ps := procFS.Stat(hostPid)
	if ps != nil {
		i = &Info{
			UniqueID: ps.UniqueID(),
			Pid:      hostPid,
			Ppid:     ps.ParentPID(),
			Command:  ps.Command(),
		}

		// Inherit ContainerID from parent
		if i.Ppid > 0 {
			pi := GetInfo(i.Ppid)
			if pi != nil {
				i.ContainerID = pi.ContainerID
			}
		}

		cacheLock.Lock()
		cache[hostPid] = i
		cacheLock.Unlock()

		return i
	}

	return nil
}

// GetLineage returns the lineage of the process indicated by the given hostPid
func GetLineage(hostPid int32) []*Info {
	var is []*Info

	pid := hostPid
	for pid > 0 {
		i := GetInfo(pid)
		if i != nil {
			pid = i.Ppid
			is = append(is, i)
		} else {
			break
		}
	}

	return is
}
