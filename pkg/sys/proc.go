package sys

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var (
	// Boot ID taken from /proc/sys/kernel/random/boot_id
	bootID string

	// "Once" control for getting the boot ID
	bootIDOnce sync.Once
)

// ProcessStatus represents process status available via /proc/[pid]/stat
type ProcessStatus struct {
	statFields []string
	pid        int32
	comm       string
	ppid       int32
	startTime  uint64
	uniqueID   string
}

// GetPID returns the PID of the process.
func (ps *ProcessStatus) GetPID() int32 {
	if ps.pid == 0 {
		pid := ps.statFields[0]
		i, err := strconv.ParseInt(pid, 0, 32)
		if err != nil {
			panic(fmt.Sprintf("Couldn't parse PID: %s", pid))
		}

		ps.pid = int32(i)
	}

	return ps.pid
}

// GetCommand returns the command name associated with the process (this is
// typically referred to as the comm value in Linux kernel interfaces).
func (ps *ProcessStatus) GetCommand() string {
	if len(ps.comm) == 0 {
		ps.comm = strings.Trim(ps.statFields[1], "()")
	}

	return ps.comm
}

// GetParentPID returns the PID of the parent of the process.
func (ps *ProcessStatus) GetParentPID() int32 {
	if ps.ppid == 0 {
		ppid := ps.statFields[3]
		i, err := strconv.ParseInt(ppid, 0, 32)
		if err != nil {
			panic(fmt.Sprintf("Couldn't parse PPID: %s", ppid))
		}

		ps.ppid = int32(i)
	}

	return ps.ppid
}

// GetStartTime returns the time in jiffies (< 2.6) or clock ticks (>= 2.6)
// after system boot when the process started.
func (ps *ProcessStatus) GetStartTime() uint64 {
	if ps.startTime == 0 {
		st := ps.statFields[21]
		i, err := strconv.ParseUint(st, 0, 64)
		if err != nil {
			panic(fmt.Sprintf("Couldn't parse starttime: %s", st))
		}

		ps.startTime = i
	}

	return ps.startTime
}

// GetUniqueProcessID returns a reproducible unique identifier for the
// process indicated by the given PID.
func (ps *ProcessStatus) GetUniqueProcessID() string {
	if len(ps.uniqueID) == 0 {
		// Hash the bootID, pid, and start time to create a
		// unique process identifier
		h := sha256.New()

		binary.Write(h, binary.LittleEndian, GetBootID())
		binary.Write(h, binary.LittleEndian, ps.GetPID())
		binary.Write(h, binary.LittleEndian, ps.GetStartTime())

		ps.uniqueID = fmt.Sprintf("%x", h.Sum(nil))
	}

	return ps.uniqueID
}

// GetProcessStatus reads process status from /proc/[pid]/stat returns a
// ProcessStatus struct containing its fields and accessors that parse them
// as needed.
func GetProcessStatus(pid int32) *ProcessStatus {
	filename := fmt.Sprintf("%s/%d/stat", GetProcFs(), pid)
	file, err := os.Open(filename)
	if err != nil {
		return nil
	}

	defer file.Close()

	br := bufio.NewReader(file)
	stat, err := br.ReadString('\n')
	if err != nil {
		return nil
	}

	statFields := strings.Split(stat, " ")

	return &ProcessStatus{
		statFields: statFields,
	}
}

// GetBootID gets the host system boot identifier
func GetBootID() string {
	bootIDOnce.Do(func() {
		filename := filepath.Join(GetProcFs(),
			"/sys/kernel/random/boot_id")

		var file *os.File

		// Make sure that we capture the err declaration above in the
		// closure here.
		file, err := os.Open(filename)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		br := bufio.NewReader(file)
		bootID, err = br.ReadString('\n')
	})

	return bootID
}

// GetUniqueProcessID returns a reproducible unique identifier for the
// process indicated by the given PID.
func GetUniqueProcessID(pid int32) string {
	ps := GetProcessStatus(pid)
	if ps == nil {
		return ""
	}

	// Hash the bootID, pid, and start time to create a
	// unique process identifier
	h := sha256.New()

	binary.Write(h, binary.LittleEndian, GetBootID())
	binary.Write(h, binary.LittleEndian, pid)
	binary.Write(h, binary.LittleEndian, ps.GetStartTime())

	return fmt.Sprintf("%x", h.Sum(nil))
}

// GetProcessDockerContainerID returns the container ID running the process
// indicated by the given PID. Returns the empty string if the process is not
// running within a container.
func GetProcessDockerContainerID(pid int32) string {
	cgroups := GetProcessCgroups(pid)

	for _, pci := range cgroups {
		if strings.HasPrefix(pci.CgroupPath, "/docker") {
			pathParts := strings.Split(pci.CgroupPath, "/")
			return pathParts[2]
		}
	}

	return ""
}

// ProcessCgroupInfo describes the cgroup membership of a process
type ProcessCgroupInfo struct {
	// Unique hierarchy ID
	HierarchyID int

	// Cgroup controllers (subsystems) bound to the hierarchy
	Controllers []string

	// CgroupPath is the pathname of the control group to which the process
	// belongs. It is relative to the mountpoint of the hierarchy.
	CgroupPath string
}

// GetProcessCgroups returns the cgroup membership of the process
// indicated by the given PID.
func GetProcessCgroups(pid int32) []ProcessCgroupInfo {
	filename := fmt.Sprintf("%s/%d/cgroup", GetProcFs(), pid)
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)

	if err != nil {
		return nil
	}

	var cgroupInfo []ProcessCgroupInfo

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		t := scanner.Text()
		parts := strings.Split(t, ":")
		h, err := strconv.Atoi(parts[0])
		if err != nil {
			panic(fmt.Sprintf("Couldn't parse cgroup line: %s", t))
		}

		ci := ProcessCgroupInfo{
			HierarchyID: h,
			Controllers: strings.Split(parts[1], ","),
			CgroupPath:  parts[2],
		}

		cgroupInfo = append(cgroupInfo, ci)
	}

	return cgroupInfo
}

// GetProcessCommandLine gets the full command-line arguments for the process
// indicated by the given PID.
func GetProcessCommandLine(pid int32) []string {
	//
	// This misses the command-line arguments for short-lived processes,
	// which is clearly not ideal.
	//
	filename := fmt.Sprintf("%s/%d/cmdline", GetProcFs(), pid)
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	defer file.Close()

	if err != nil {
		return nil
	}

	var cmdline [4096]byte
	_, err = file.Read(cmdline[:])
	if err != nil {
		return nil
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

	return commandLine
}
