package sensor

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/config"
	"github.com/golang/glog"
)

var (
	// Lock used to protect the process cache map
	mu sync.Mutex

	// Mapping of process ID's to their cached data.
	pidMap map[int32]*procCacheEntry

	// Boot ID taken from /proc/sys/kernel/random/boot_id
	bootId string

	// "Once" control for getting the boot ID
	bootIdOnce sync.Once

	// Error returned when a file does not have the expected format.
	errInvalidFileFormat = func(filename string) error {
		return errors.New(fmt.Sprintf("File %s does not have the expected format", filename))
	}
)

// Note: go array indices start at 0, which is why these stat field indices are
// one less than the field numbers given in the `info proc` manpage.
const (
	// Index of /proc/PID/stat field for the process PID
	STAT_FIELD_PID = 0

	// Index of /proc/PID/stat field for the process command
	STAT_FIELD_COMMAND = 1

	// Index of /proc/PID/stat field for the parent process PID
	STAT_FIELD_PPID = 3

	// Index of /proc/PID/stat field for the process start time
	STAT_FIELD_STARTTIME = 21
)

// PID value indicating that the process PID is unknown;
// must be different from any valid process PID
const PID_UNKNOWN int32 = -1

// The cached data for a process
type procCacheEntry struct {
	// The process ID for the process;
	// This does NOT change after creation.
	processId string

	// The ID for the container of this process.
	// This does NOT change after creation.
	containerId string

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

func getProcFs() string {
	return config.Sensor.ProcFs
}

// Gets the Host system boot ID.
func getBootId() (string, error) {
	var err error = nil
	bootIdOnce.Do(func() {
		filename := getProcFs() + "/sys/kernel/random/boot_id"
		file, err := os.Open(filename)
		if err != nil {
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		if scanner.Scan() {
			bootId = scanner.Text()
		} else {
			err = scanner.Err()
		}
	})

	return bootId, err
}

// Computes the process ID for a current process, given its /proc/pid/stat data
func getProcessId(stat []string) string {
	bId, err := getBootId()
	if err != nil {
		glog.Warning("Failed to get boot ID", err)
	}

	// Compute the raw process ID from process info
	rawId := fmt.Sprintf("%s/%s/%s", bId, stat[STAT_FIELD_PID], stat[STAT_FIELD_STARTTIME])

	// Now hash the raw ID
	hashedId := sha256.Sum256([]byte(rawId))
	return fmt.Sprintf("%X", hashedId)
}

// Creates a new process cache entry.
func newProcCacheEntry(pid int32) (*procCacheEntry, error) {
	stat, err := readStat(pid)
	if err != nil {
		return nil, err
	}

	processId := getProcessId(stat)
	containerId, err := getContainerIdFromCgroup(pid)
	if err != nil {
		return nil, err
	}

	ppid, _ := strconv.ParseInt(stat[STAT_FIELD_PPID], 0, 32)

	proc := &procCacheEntry{
		processId:   processId,
		containerId: containerId,
		ppid:        int32(ppid),
		children:    make(map[int32]*procCacheEntry),
	}
	proc.command.Store(stat[STAT_FIELD_COMMAND])
	return proc, nil
}

// Gets the cache entry for a given process, creating that entry if necessary.
func getProcCacheEntry(pid int32) (procEntry *procCacheEntry, err error) {
	mu.Lock()
	procEntry, ok := pidMap[pid]
	mu.Unlock()

	if !ok {
		procEntry, err := newProcCacheEntry(pid)
		if err != nil {
			return nil, err
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

	return procEntry, nil
}

// Gets the container ID from the /proc/PID/cgroup file.
func getContainerIdFromCgroup(hostPid int32) (string, error) {
	perfEventPath, err := readCgroup(hostPid)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(perfEventPath, "/docker") {
		pathParts := strings.Split(perfEventPath, "/")
		return pathParts[2], nil
	} else {
		return "", nil
	}
}

// Gets the `perf_event` control group in the hierarchy to which the process
// belongs.
// Returns "" if the process is not in the `perf_event` control group.
func readCgroup(hostPid int32) (string, error) {
	filename := fmt.Sprintf("%s/%d/cgroup", getProcFs(), hostPid)
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)

	if err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		t := scanner.Text()
		parts := strings.Split(t, ":")
		controllerList := parts[1]
		cgroupPath := parts[2]

		if strings.Index(controllerList, "perf_event") > 0 {
			return cgroupPath, nil
		}
	}

	return "", err
}

// Returns the list of fields from a process's stat file.
func readStat(hostPid int32) ([]string, error) {
	filename := fmt.Sprintf("%s/%d/stat", getProcFs(), hostPid)
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return nil, scanner.Err()
	}
	statline := scanner.Text()

	lparenPos := strings.Index(statline, "(")
	rparenPos := strings.LastIndex(statline, ")")
	if lparenPos == -1 || rparenPos == -1 || rparenPos < lparenPos {
		return nil, errInvalidFileFormat(filename)
	}
	stat := []string{
		strings.Trim(statline[0:lparenPos], " "),
		statline[lparenPos+1:],
	}
	rest := strings.Split(statline[rparenPos+2:], " ")
	stat = append(stat, rest...)
	return stat, nil
}

// Adds the given process to its parent's children map, if the parent and child
// are in the same container.
func procInfoAddChildToParent(parentPid int32, childPid int32, childEntry *procCacheEntry) error {
	// Only add to the parent's children map when the child and parent are in
	// the same container.
	parentContainerId, err := getContainerIdFromCgroup(parentPid)
	if err != nil {
		return err
	}

	if parentContainerId == childEntry.containerId {
		parentEntry, err := getProcCacheEntry(parentPid)
		if err != nil {
			return err
		}

		parentEntry.lock.Lock()
		parentEntry.children[childPid] = childEntry
		parentEntry.lock.Unlock()
	}

	return nil
}

func procInfoOnFork(parentPid int32, childPid int32) {
	parentEntry, _ := getProcCacheEntry(parentPid)

	childStat, err := readStat(childPid)
	if err != nil {
		glog.Errorln("Cannot get data for child process", err)
		return
	}
	childEntry := &procCacheEntry{
		processId:   getProcessId(childStat),
		containerId: parentEntry.containerId,
		ppid:        parentPid,
		children:    make(map[int32]*procCacheEntry),
	}
	command := parentEntry.command.Load().(string)
	childEntry.command.Store(command)

	mu.Lock()
	pidMap[childPid] = childEntry
	mu.Unlock()

	err = procInfoAddChildToParent(parentPid, childPid, childEntry)
	if err != nil {
		glog.Errorln("Cannot get data for parent process", err)
	}
}

func procInfoOnExec(hostPid int32, command string) {
	procEntry, _ := getProcCacheEntry(hostPid)

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
	if ppid != PID_UNKNOWN {
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
		atomic.StoreInt32(&childEntry.ppid, PID_UNKNOWN)
	}
}

func procInfoGetPpid(pid int32) (int32, error) {
	mu.Lock()
	procEntry, err := getProcCacheEntry(pid)
	mu.Unlock()
	if err != nil {
		return PID_UNKNOWN, err
	}

	ppid := atomic.LoadInt32(&procEntry.ppid)

	if ppid == PID_UNKNOWN {
		stat, err := readStat(pid)
		if err != nil {
			return PID_UNKNOWN, err
		}
		new_ppid, _ := strconv.ParseInt(stat[STAT_FIELD_PPID], 0, 32)
		ppid = int32(new_ppid)
		atomic.StoreInt32(&procEntry.ppid, ppid)

		err = procInfoAddChildToParent(ppid, pid, procEntry)
		if err != nil {
			return ppid, err
		}
	}

	return ppid, nil
}

func procInfoGetContainerId(hostPid int32) (string, error) {
	procEntry, err := getProcCacheEntry(hostPid)
	if err != nil {
		return "", err
	}

	return procEntry.containerId, nil
}

func procInfoGetProcessId(hostPid int32) (string, error) {
	procEntry, err := getProcCacheEntry(hostPid)
	if err != nil {
		return "", err
	}

	return procEntry.processId, nil
}

func procInfoGetCommandLine(hostPid int32) ([]string, error) {
	//
	// This misses the command-line arguments for short-lived processes,
	// which is clearly not ideal.
	//
	filename := fmt.Sprintf("%s/%d/cmdline", getProcFs(), hostPid)
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	defer file.Close()

	if err != nil {
		return nil, err
	}

	var cmdline [4096]byte
	_, err = file.Read(cmdline[:])
	if err != nil {
		return nil, err
	}

	var commandLine []string

	reader := bufio.NewReader(bytes.NewReader(cmdline[:]))
	for {
		s, err := reader.ReadString(0)
		if err != nil {
			break
		}

		if len(s) > 1 {
			commandLine = append(commandLine, s[:len(s)-1])
		} else {
			break
		}
	}

	return commandLine, nil
}

func newProcLineageItem(pid int32, procEntry *procCacheEntry) *api.Process {
	command := procEntry.command.Load().(string)

	return &api.Process{
		Pid:     pid,
		Command: command,
	}
}

func GetProcLineage(pid int32) (lineage []*api.Process, err error) {
	procEntry, err := getProcCacheEntry(pid)
	if err != nil {
		return
	}

	var parentEntry *procCacheEntry
	var ppid int32
	lineage = []*api.Process{}
	cId := procEntry.containerId
	for ; procEntry != nil && procEntry.containerId == cId; pid, procEntry = ppid, parentEntry {
		lineage = append(lineage, newProcLineageItem(pid, procEntry))

		ppid = atomic.LoadInt32(&procEntry.ppid)

		if ppid == PID_UNKNOWN {
			ppid, err = procInfoGetPpid(pid)
			if err != nil {
				return
			}
		}

		mu.Lock()
		parentEntry = pidMap[ppid]
		mu.Unlock()
	}

	return
}

// stream.Do() function for marking an event as needing lineage
func markEventAsNeedingLineage(i interface{}) {
	e := i.(*api.Event)
	// We indicate the need for lineage by giving it a dummy api.Process
	e.ProcessLineage = []*api.Process{&api.Process{}}
}

// Sets the event process lineage if the subscription calls for it.
func setProcLineage(e *api.Event) (err error) {
	if len(e.ProcessLineage) == 1 {
		e.ProcessLineage, err = GetProcLineage(e.ProcessPid)
	}

	return
}
