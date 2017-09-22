package subscription

import (
	"fmt"
	"sync"
	"sync/atomic"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/sys"
)

var (
	// Lock used to protect the process cache map
	mu sync.Mutex

	// Mapping of process ID's to their cached data.
	pidMap map[int32]*procCacheEntry

	// Error returned when a file does not have the expected format.
	errInvalidFileFormat = func(filename string) error {
		return fmt.Errorf("File %s does not have the expected format", filename)
	}
)

// PIDUnknown indicates that the process PID is unknown;
// must be different from any valid process PID
const PIDUnknown int32 = -1

// The cached data for a process
type procCacheEntry struct {
	// The process ID for the process;
	// This does NOT change after creation.
	processID string

	// The ID for the container of this process.
	// This does NOT change after creation.
	containerID string

	// The command for the process, as an atomic string value.
	// This can be changed via a execve() call, hence it is kept as an atomic
	// string value.
	command atomic.Value

	// The parent process PID; may be PID_UNKNOWN.
	// This will be changed whenever the process is orphaned.
	ppid int32

	// The children of this process, used to maintain the ppid field.
	// Only children in the same container as this are maintained here.
	// This will be changed by fork and exit events.
	children map[int32]*procCacheEntry

	// Lock used to guard the children map.
	lock sync.Mutex
}

func init() {
	pidMap = make(map[int32]*procCacheEntry)
}

// Creates a new process cache entry.
func newProcCacheEntry(pid int32) *procCacheEntry {
	ps := sys.GetProcessStatus(pid)
	if ps == nil {
		return nil
	}

	proc := &procCacheEntry{
		processID:   ps.GetUniqueProcessID(),
		containerID: sys.GetProcessDockerContainerID(pid),
		ppid:        ps.GetParentPID(),
		children:    make(map[int32]*procCacheEntry),
	}

	proc.command.Store(ps.GetCommand())

	return proc
}

// Gets the cache entry for a given process, creating that entry if necessary.
func getProcCacheEntry(pid int32) *procCacheEntry {
	mu.Lock()
	procEntry, ok := pidMap[pid]
	mu.Unlock()

	if !ok {
		procEntry = newProcCacheEntry(pid)
		if procEntry == nil {
			panic("newProcCacheEntry returned nil!")
		}

		mu.Lock()
		// Check if some other go routine has added an entry for this process.
		if currEntry, ok := pidMap[pid]; ok {
			procEntry = currEntry
		} else {
			pidMap[pid] = procEntry
		}
		mu.Unlock()
	}

	return procEntry
}

// Adds the given process to its parent's children map, if the parent and child
// are in the same container.
func procInfoAddChildToParent(parentPid int32, childPid int32, childEntry *procCacheEntry) {
	// Only add to the parent's children map when the child and parent are in
	// the same container.
	parentContainerID := sys.GetProcessDockerContainerID(parentPid)

	if parentContainerID == childEntry.containerID {
		parentEntry := getProcCacheEntry(parentPid)

		parentEntry.lock.Lock()
		parentEntry.children[childPid] = childEntry
		parentEntry.lock.Unlock()
	}
}

func procInfoOnFork(parentPid int32, childPid int32) {
	parentEntry := getProcCacheEntry(parentPid)
	ps := sys.GetProcessStatus(childPid)

	childEntry := &procCacheEntry{
		processID:   ps.GetUniqueProcessID(),
		containerID: parentEntry.containerID,
		ppid:        parentPid,
		children:    make(map[int32]*procCacheEntry),
	}
	command := parentEntry.command.Load().(string)
	childEntry.command.Store(command)

	mu.Lock()
	pidMap[childPid] = childEntry
	mu.Unlock()

	procInfoAddChildToParent(parentPid, childPid, childEntry)
}

func procInfoOnExec(hostPid int32, command string) {
	procEntry := getProcCacheEntry(hostPid)
	procEntry.command.Store(command)
}

func procInfoOnExit(hostPid int32) {
	mu.Lock()
	procEntry, ok := pidMap[hostPid]
	if ok {
		delete(pidMap, hostPid)
	}
	mu.Unlock()
	if !ok {
		// This process is not in the cache, nothing to do.
		return
	}

	ppid := atomic.LoadInt32(&procEntry.ppid)
	if ppid != PIDUnknown {
		mu.Lock()
		parentEntry, ok := pidMap[ppid]
		mu.Unlock()

		if ok {
			parentEntry.lock.Lock()
			delete(parentEntry.children, hostPid)
			parentEntry.lock.Unlock()
		}
	}

	procEntry.lock.Lock()
	defer procEntry.lock.Unlock()

	// Now mark all the children as not having a known parent.
	for _, childEntry := range procEntry.children {
		atomic.StoreInt32(&childEntry.ppid, PIDUnknown)
	}
}

func procInfoGetPpid(pid int32) int32 {
	mu.Lock()
	procEntry := getProcCacheEntry(pid)
	mu.Unlock()
	if procEntry == nil {
		return PIDUnknown
	}

	ppid := atomic.LoadInt32(&procEntry.ppid)

	if ppid == PIDUnknown {
		ps := sys.GetProcessStatus(pid)
		if ps == nil {
			return PIDUnknown
		}

		atomic.StoreInt32(&procEntry.ppid, ps.GetParentPID())

		procInfoAddChildToParent(ppid, pid, procEntry)
	}

	return ppid
}

func procInfoGetContainerID(hostPid int32) string {
	procEntry := getProcCacheEntry(hostPid)
	return procEntry.containerID
}

func procInfoGetProcessID(hostPid int32) string {
	procEntry := getProcCacheEntry(hostPid)
	return procEntry.processID
}

func newProcLineageItem(pid int32, procEntry *procCacheEntry) *api.Process {
	command := procEntry.command.Load().(string)

	return &api.Process{
		Pid:     pid,
		Command: command,
	}
}

// GetProcLineage returns a slice of Process instances representing the
// "lineage" of the process indicated by the given PID. A process's lineage
// is the line of parent relationships up to the root of the process namespace.
// It represents the current state of process parent relationships as reported
// by the kernel vs. the historical state based on when each new process was
// created.
func GetProcLineage(pid int32) []*api.Process {
	procEntry := getProcCacheEntry(pid)

	var parentEntry *procCacheEntry
	var ppid int32
	lineage := []*api.Process{}
	cID := procEntry.containerID
	for ; procEntry != nil && procEntry.containerID == cID; pid, procEntry = ppid, parentEntry {
		lineage = append(lineage, newProcLineageItem(pid, procEntry))

		ppid = atomic.LoadInt32(&procEntry.ppid)

		if ppid == PIDUnknown {
			ppid = procInfoGetPpid(pid)
		}

		mu.Lock()
		parentEntry = pidMap[ppid]
		mu.Unlock()
	}

	return lineage
}

// stream.Do() function for marking an event as needing lineage
func markEventAsNeedingLineage(i interface{}) {
	e := i.(*api.Event)
	// We indicate the need for lineage by giving it a dummy api.Process
	e.ProcessLineage = []*api.Process{&api.Process{}}
}

// Sets the event process lineage if the subscription calls for it.
func procInfoUpdEvent(e *api.Event) error {
	procEntry := getProcCacheEntry(e.ProcessPid)

	e.ProcessId = procEntry.processID

	if e.ContainerId == "" {
		e.ContainerId = procEntry.containerID

		ce, err := getContainerEvent(e.ContainerId)
		if err != nil {
			return err
		}
		e.ContainerName = ce.Name
		e.ImageId = ce.ImageId
		e.ImageName = ce.Name
	}

	if len(e.ProcessLineage) == 1 {
		e.ProcessLineage = GetProcLineage(e.ProcessPid)
	}

	return nil
}
