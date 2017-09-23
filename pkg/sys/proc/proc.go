package proc

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var (
	// Default procfs mounted on /proc
	procFSOnce sync.Once
	procFS     *FileSystem

	// Boot ID taken from /proc/sys/kernel/random/boot_id
	bootID string

	// "Once" control for getting the boot ID
	bootIDOnce sync.Once
)

// GetProcFS creates a FileSystem instance representing the default
// procfs mountpoint /proc. When running inside a container, this will
// contain information from the container's pid namespace.
func GetProcFS() *FileSystem {
	procFSOnce.Do(func() {
		//
		// Do some quick sanity checks to make sure /proc is our procfs
		//

		fi, err := os.Stat("/proc")
		if err != nil {
			panic("/proc not found")
		}

		if !fi.IsDir() {
			panic("/proc not a directory")
		}

		self, err := os.Readlink("/proc/self")
		if err != nil {
			panic("couldn't read /proc/self")
		}

		_, file := filepath.Split(self)
		pid, err := strconv.Atoi(file)
		if err != nil {
			panic(fmt.Sprintf("Couldn't parse %s as pid", file))
		}

		if pid != os.Getpid() {
			panic(fmt.Sprintf("/proc/self points to wrong pid: %d",
				pid))
		}

		procFS = &FileSystem{
			MountPoint: "/proc",
		}
	})

	return procFS
}

// FileSystem represents data accessible through the proc pseudo-filesystem.
type FileSystem struct {
	MountPoint string
}

// Open opens the procfs file indicated by the given relative path.
func (fs *FileSystem) Open(relativePath string) (*os.File, error) {
	return os.Open(filepath.Join(fs.MountPoint, relativePath))
}

// ReadFile returns the contents of the procfs file indicated by
// the given relative path.
func ReadFile(relativePath string) ([]byte, error) {
	return GetProcFS().ReadFile(relativePath)
}

// ReadFile returns the contents of the procfs file indicated by the
// given relative path.
func (fs *FileSystem) ReadFile(relativePath string) ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(fs.MountPoint, relativePath))
}

// GetCommandLine gets the full command-line arguments for the process
// indicated by the given PID.
func GetCommandLine(pid int32) []string {
	return GetProcFS().GetCommandLine(pid)
}

// GetCommandLine gets the full command-line arguments for the process
// indicated by the given PID.
func (fs *FileSystem) GetCommandLine(pid int32) []string {
	//
	// This misses the command-line arguments for short-lived processes,
	// which is clearly not ideal.
	//
	filename := fmt.Sprintf("%d/cmdline", pid)
	cmdline, err := fs.ReadFile(filename)
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

// GetCgroups returns the cgroup membership of the process
// indicated by the given PID.
func GetCgroups(pid int32) []Cgroup {
	return GetProcFS().GetCgroups(pid)
}

// GetCgroups returns the cgroup membership of the process
// indicated by the given PID.
func (fs *FileSystem) GetCgroups(pid int32) []Cgroup {
	filename := fmt.Sprintf("%d/cgroup", pid)
	cgroup, err := fs.ReadFile(filename)
	if err != nil {
		return nil
	}

	var cgroups []Cgroup

	scanner := bufio.NewScanner(bytes.NewReader(cgroup))
	for scanner.Scan() {
		t := scanner.Text()
		parts := strings.Split(t, ":")
		ID, err := strconv.Atoi(parts[0])
		if err != nil {
			panic(fmt.Sprintf("Couldn't parse cgroup line: %s", t))
		}

		c := Cgroup{
			ID:          ID,
			Controllers: strings.Split(parts[1], ","),
			Path:        parts[2],
		}

		cgroups = append(cgroups, c)
	}

	return cgroups
}

// Cgroup describes the cgroup membership of a process
type Cgroup struct {
	// Unique hierarchy ID
	ID int

	// Cgroup controllers (subsystems) bound to the hierarchy
	Controllers []string

	// Path is the pathname of the control group to which the process
	// belongs. It is relative to the mountpoint of the hierarchy.
	Path string
}

// GetContainerID returns the container ID running the process
// indicated by the given PID. Returns the empty string if the process
// is not running within a container.
func GetContainerID(pid int32) string {
	return GetProcFS().GetContainerID(pid)
}

// GetContainerID returns the container ID running the process
// indicated by the given PID. Returns the empty string if the process
// is not running within a container.
func (fs *FileSystem) GetContainerID(pid int32) string {
	cgroups := fs.GetCgroups(pid)

	for _, pci := range cgroups {
		if strings.HasPrefix(pci.Path, "/docker") {
			pathParts := strings.Split(pci.Path, "/")
			return pathParts[2]
		}
	}

	return ""
}

// GetUniqueProcessID returns a reproducible namespace-independent
// unique identifier for the process indicated by the given PID.
func GetUniqueProcessID(pid int32) string {
	return GetProcFS().GetUniqueProcessID(pid)
}

// GetUniqueProcessID returns a reproducible namespace-independent
// unique identifier for the process indicated by the given PID.
func (fs *FileSystem) GetUniqueProcessID(pid int32) string {
	ps := fs.GetProcessStatus(pid)
	if ps == nil {
		return ""
	}

	return ps.GetUniqueProcessID()
}

// GetProcessStatus reads the given process's status and returns a
// ProcessStatus with methods to parse and return information from
// that status as needed.
func GetProcessStatus(pid int32) *ProcessStatus {
	return GetProcFS().GetProcessStatus(pid)
}

// GetProcessStatus reads the given process's status from the ProcFS
// receiver and returns a ProcessStatus with methods to parse and
// return information from that status as needed.
func (fs *FileSystem) GetProcessStatus(pid int32) *ProcessStatus {
	stat, err := fs.ReadFile(fmt.Sprintf("%d/stat", pid))
	if err != nil {
		return nil
	}

	return &ProcessStatus{
		statFields: strings.Fields(string(stat)),
	}
}

// ProcessStatus represents process status available via /proc/[pid]/stat
type ProcessStatus struct {
	statFields []string
	pid        int32
	comm       string
	ppid       int32
	startTime  uint64
	startStack uint64
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
		st := ps.statFields[22-1]
		i, err := strconv.ParseUint(st, 0, 64)
		if err != nil {
			panic(fmt.Sprintf("Couldn't parse starttime: %s", st))
		}

		ps.startTime = i
	}

	return ps.startTime
}

// GetStartStack returns the address of the start (i.e., bottom) of the stack.
func (ps *ProcessStatus) GetStartStack() uint64 {
	if ps.startStack == 0 {
		ss := ps.statFields[28-1]
		i, err := strconv.ParseUint(ss, 0, 64)
		if err != nil {
			panic(fmt.Sprintf("Couldn't parse startstack: %s", ss))
		}

		ps.startStack = i
	}

	return ps.startStack
}

// GetUniqueProcessID returns a reproducible unique identifier for the
// process indicated by the given PID.
func (ps *ProcessStatus) GetUniqueProcessID() string {
	if len(ps.uniqueID) == 0 {
		// Hash the bootID, starting stack address, and start time to
		// create a unique process identifier that has the same value
		// regardless of the pid namespace (i.e. same value from
		// within the container and from the underlying host).
		h := sha256.New()

		binary.Write(h, binary.LittleEndian, GetBootID())
		binary.Write(h, binary.LittleEndian, ps.GetStartStack())
		binary.Write(h, binary.LittleEndian, ps.GetStartTime())

		ps.uniqueID = fmt.Sprintf("%x", h.Sum(nil))
	}

	return ps.uniqueID
}

// GetBootID gets the host system boot identifier
func GetBootID() string {
	bootIDOnce.Do(func() {
		data, err := ReadFile("/sys/kernel/random/boot_id")
		if err != nil {
			panic(err)
		}

		bootID = strings.TrimSpace(string(data))
	})

	return bootID
}
