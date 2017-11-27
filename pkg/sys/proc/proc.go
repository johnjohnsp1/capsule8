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

	"github.com/golang/glog"
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

// FS creates a FileSystem instance representing the default
// procfs mountpoint /proc. When running inside a container, this will
// contain information from the container's pid namespace.
func FS() *FileSystem {
	procFSOnce.Do(func() {
		//
		// Do some quick sanity checks to make sure /proc is our procfs
		//

		fi, err := os.Stat("/proc")
		if err != nil {
			glog.Fatal("/proc not found")
		}

		if !fi.IsDir() {
			glog.Fatal("/proc not a directory")
		}

		self, err := os.Readlink("/proc/self")
		if err != nil {
			glog.Fatal("couldn't read /proc/self")
		}

		_, file := filepath.Split(self)
		pid, err := strconv.Atoi(file)
		if err != nil {
			glog.Fatalf("Couldn't parse %s as pid", file)
		}

		if pid != os.Getpid() {
			glog.Fatalf("/proc/self points to wrong pid: %d", pid)
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
	return FS().ReadFile(relativePath)
}

// ReadFile returns the contents of the procfs file indicated by the
// given relative path.
func (fs *FileSystem) ReadFile(relativePath string) ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(fs.MountPoint, relativePath))
}

// CommandLine gets the full command-line arguments for the process
// indicated by the given PID.
func CommandLine(pid int) []string {
	return FS().CommandLine(pid)
}

// CommandLine gets the full command-line arguments for the process
// indicated by the given PID.
func (fs *FileSystem) CommandLine(pid int) []string {
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

// Cgroups returns the cgroup membership of the process
// indicated by the given PID.
func Cgroups(pid int) ([]Cgroup, error) {
	return FS().Cgroups(pid)
}

// Cgroups returns the cgroup membership of the process
// indicated by the given PID.
func (fs *FileSystem) Cgroups(pid int) ([]Cgroup, error) {
	filename := fmt.Sprintf("%d/cgroup", pid)
	cgroup, err := fs.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	cgroups := parseProcPidCgroup(cgroup)
	return cgroups, nil
}

// parseProcPidCgroup parses the contents of /proc/[pid]/cgroup
func parseProcPidCgroup(cgroup []byte) []Cgroup {
	var cgroups []Cgroup

	scanner := bufio.NewScanner(bytes.NewReader(cgroup))
	for scanner.Scan() {
		t := scanner.Text()
		parts := strings.Split(t, ":")
		ID, err := strconv.Atoi(parts[0])
		if err != nil {
			glog.Fatalf("Couldn't parse cgroup line: %s", t)
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

// ContainerID returns the container ID running the process indicated
// by the given PID. Returns the empty string if the process is not
// running within a container. Returns a non-nil error if the process
// indicated by the given PID wasn't found.
func ContainerID(pid int) (string, error) {
	return FS().ContainerID(pid)
}

// ContainerID returns the container ID running the process indicated
// by the given PID. Returns the empty string if the process is not
// running within a container. Returns a  non-nil error if the process
// indicated by the given PID wasn't found.
func (fs *FileSystem) ContainerID(pid int) (string, error) {
	cgroups, err := fs.Cgroups(pid)
	if err != nil {
		return "", err
	}

	glog.V(10).Infof("pid:%d cgroups:%+v", pid, cgroups)

	containerID := containerIDFromCgroups(cgroups)
	return containerID, nil
}

func containerIDFromCgroups(cgroups []Cgroup) string {
	for _, pci := range cgroups {
		if strings.HasPrefix(pci.Path, "/docker") {
			pathParts := strings.Split(pci.Path, "/")
			if len(pathParts) > 2 {
				return pathParts[2]
			}
		}
	}

	return ""
}

// UniqueID returns a reproducible namespace-independent
// unique identifier for the process indicated by the given PID.
func UniqueID(pid int) string {
	return FS().UniqueID(pid)
}

// UniqueID returns a reproducible namespace-independent
// unique identifier for the process indicated by the given PID.
func (fs *FileSystem) UniqueID(pid int) string {
	ps := fs.Stat(pid)
	if ps == nil {
		return ""
	}

	return ps.UniqueID()
}

// Stat reads the given process's status and returns a ProcessStatus
// with methods to parse and return information from that status as
// needed.
func Stat(pid int) *ProcessStatus {
	return FS().Stat(pid)
}

// statFields parses the contents of a /proc/PID/stat field into fields.
func statFields(stat string) []string {
	//
	// Parse out the command field.
	//
	// This requires special care because the command can contain white space
	// and / or punctuation. Fortunately, we are guaranteed that the command
	// will always be between the first '(' and the last ')'.
	//
	firstLParen := strings.IndexByte(stat, '(')
	lastRParen := strings.LastIndexByte(stat, ')')
	if firstLParen < 0 || lastRParen < 0 || lastRParen < firstLParen {
		return nil
	}
	command := stat[firstLParen+1 : lastRParen]
	statFields := []string{
		strings.TrimRight(stat[:firstLParen], " "),
		command,
	}
	return append(statFields, strings.Fields(stat[lastRParen+1:])...)
}

// Stat reads the given process's status from the ProcFS receiver and
// returns a ProcessStatus with methods to parse and return
// information from that status as needed.
func (fs *FileSystem) Stat(pid int) *ProcessStatus {
	stat, err := fs.ReadFile(fmt.Sprintf("%d/stat", pid))
	if err != nil {
		return nil
	}

	return &ProcessStatus{
		statFields: statFields(string(stat)),
	}
}

// ProcessStatus represents process status available via /proc/[pid]/stat
type ProcessStatus struct {
	statFields []string
	pid        int
	comm       string
	ppid       int
	startTime  uint64
	startStack uint64
	uniqueID   string
}

// PID returns the PID of the process.
func (ps *ProcessStatus) PID() int {
	if ps.pid == 0 {
		pid := ps.statFields[0]
		i, err := strconv.ParseInt(pid, 0, 32)
		if err != nil {
			glog.Fatalf("Couldn't parse PID: %s", pid)
		}

		ps.pid = int(i)
	}

	return ps.pid
}

// Command returns the command name associated with the process (this is
// typically referred to as the comm value in Linux kernel interfaces).
func (ps *ProcessStatus) Command() string {
	if len(ps.comm) == 0 {
		ps.comm = ps.statFields[1]
	}

	return ps.comm
}

// ParentPID returns the PID of the parent of the process.
func (ps *ProcessStatus) ParentPID() int {
	if ps.ppid == 0 {
		ppid := ps.statFields[3]
		i, err := strconv.ParseInt(ppid, 0, 32)
		if err != nil {
			glog.Fatalf("Couldn't parse PPID: %s", ppid)
		}

		ps.ppid = int(i)
	}

	return ps.ppid
}

// StartTime returns the time in jiffies (< 2.6) or clock ticks (>= 2.6)
// after system boot when the process started.
func (ps *ProcessStatus) StartTime() uint64 {
	if ps.startTime == 0 {
		st := ps.statFields[22-1]
		i, err := strconv.ParseUint(st, 0, 64)
		if err != nil {
			glog.Fatalf("Couldn't parse starttime: %s", st)
		}

		ps.startTime = i
	}

	return ps.startTime
}

// StartStack returns the address of the start (i.e., bottom) of the stack.
func (ps *ProcessStatus) StartStack() uint64 {
	if ps.startStack == 0 {
		ss := ps.statFields[28-1]
		i, err := strconv.ParseUint(ss, 0, 64)
		if err != nil {
			glog.Fatalf("Couldn't parse startstack: %s", ss)
		}

		ps.startStack = i
	}

	return ps.startStack
}

// UniqueID returns a reproducible unique identifier for the
// process indicated by the given PID.
func (ps *ProcessStatus) UniqueID() string {
	if len(ps.uniqueID) == 0 {
		ps.uniqueID = DeriveUniqueID(ps.PID(), ps.ParentPID())
	}

	return ps.uniqueID
}

// DeriveUniqueID returns a unique ID for thye process with the given
// PID and parent PID
func DeriveUniqueID(pid, ppid int) string {
	// Hash the bootID, PID, and parent PID to create a
	// unique process identifier that can also be calculated
	// from perf records and trace events

	h := sha256.New()

	err := binary.Write(h, binary.LittleEndian, []byte(BootID()))
	if err != nil {
		glog.Fatal(err)
	}

	err = binary.Write(h, binary.LittleEndian, int32(pid))
	if err != nil {
		glog.Fatal(err)
	}

	err = binary.Write(h, binary.LittleEndian, int32(ppid))
	if err != nil {
		glog.Fatal(err)
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

// BootID gets the host system boot identifier
func BootID() string {
	bootIDOnce.Do(func() {
		data, err := ReadFile("/sys/kernel/random/boot_id")
		if err != nil {
			panic(err)
		}

		bootID = strings.TrimSpace(string(data))
	})

	return bootID
}
