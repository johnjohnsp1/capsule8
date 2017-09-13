package sensor

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

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

	// Index of /proc/PID/stat field for the process start time
	STAT_FIELD_STARTTIME = 21
)

// The cached data for a process
type procCacheEntry struct {
	// Lock used to guard those fields in this struct that can change after
	// the creation of this structure.
	lock sync.Mutex

	// The process ID for the process;
	// This does NOT change after creation.
	processId string

	// The ID for the container of this process.
	// This does NOT change after creation.
	containerId string

	// The command for the process.
	// This can be changed via a execve() call.
	command string
}

func init() {
	pidMap = make(map[int32]*procCacheEntry)

	var err error
	bootId, err = getBootId()
	if err != nil {
		glog.Fatal("Failed to get boot ID", err)
	}
}

func getProcFs() string {
	return config.Sensor.ProcFs
}

// Gets the Host system boot ID.
func getBootId() (string, error) {
	filename := getProcFs() + "/sys/kernel/random/boot_id"
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		return scanner.Text(), nil
	} else {
		return "", scanner.Err()
	}
}

// Computes the process ID for a current process, given its /proc/pid/stat data
func getProcessId(stat []string) string {
	return fmt.Sprintf("%s/%s/%s", bootId, stat[STAT_FIELD_PID], stat[STAT_FIELD_STARTTIME])
}

// Creates a new process cache entry.
func newProcCacheEntry(pid int32) (*procCacheEntry, error) {
	stat, err := readStat(pid)
	if err != nil {
		return nil, err
	}

	processId := getProcessId(stat)

	var containerId string = ""
	perfEventPath, err := readCgroup(pid)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(perfEventPath, "/docker") {
		pathParts := strings.Split(perfEventPath, "/")
		containerId = pathParts[2]
	}

	command := stat[STAT_FIELD_COMMAND]

	proc := &procCacheEntry{
		processId:   processId,
		containerId: containerId,
		command:     command,
	}
	return proc, nil
}

// Gets the cache entry for a given process, creating that entry if necessary
func getProcCacheEntry(pid int32) (*procCacheEntry, error) {
	mu.Lock()
	defer mu.Unlock()

	procEntry, ok := pidMap[pid]
	if !ok {
		procEntry, err := newProcCacheEntry(pid)
		if err != nil {
			return nil, err
		}
		pidMap[pid] = procEntry
	}

	return procEntry, nil
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

func pidMapOnFork(parentPid int32, childPid int32) {
	parentEntry, _ := getProcCacheEntry(parentPid)

	childStat, err := readStat(childPid)
	if err != nil {
		glog.Errorln("Cannot get data for child process", err)
		return
	}
	childEntry := &procCacheEntry{
		processId:   getProcessId(childStat),
		containerId: parentEntry.containerId,
		command:     parentEntry.command,
	}

	mu.Lock()
	defer mu.Unlock()
	pidMap[childPid] = childEntry
}

func pidMapOnExec(hostPid int32) {
	procEntry, _ := getProcCacheEntry(hostPid)

	stat, err := readStat(hostPid)
	if err != nil {
		glog.Errorln("Cannot get stat data for exec process", err)
		return
	}
	procEntry.lock.Lock()
	defer procEntry.lock.Unlock()
	procEntry.command = stat[STAT_FIELD_COMMAND]
}

func pidMapGetContainerID(hostPid int32) (string, error) {
	procEntry, err := getProcCacheEntry(hostPid)
	if err != nil {
		return "", err
	}

	return procEntry.containerId, nil
}

func pidMapGetProcessID(hostPid int32) (string, error) {
	procEntry, err := getProcCacheEntry(hostPid)
	if err != nil {
		return "", err
	}

	return procEntry.processId, nil
}

func pidMapGetCommandLine(hostPid int32) ([]string, error) {
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
