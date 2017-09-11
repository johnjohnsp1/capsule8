package sensor

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/capsule8/reactive8/pkg/config"
)

var mu sync.Mutex

// pidMap is a map from host PID to cgroup name (includes Docker container ID)
var pidMap map[int32]string

func getProcFs() string {
	return config.Sensor.ProcFs
}

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

func pidMapOnFork(parentPid int32, childPid int32) {
	mu.Lock()
	defer mu.Unlock()

	if pidMap == nil {
		pidMap = make(map[int32]string)
	}

	cgroup, ok := pidMap[parentPid]
	if !ok {
		cgroup, _ := readCgroup(parentPid)
		pidMap[parentPid] = cgroup
	}

	pidMap[childPid] = cgroup
}

func pidMapGetContainerID(hostPid int32) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	if pidMap == nil {
		pidMap = make(map[int32]string)
	}

	cgroup, ok := pidMap[hostPid]
	if !ok {
		cgroup, _ = readCgroup(hostPid)
		pidMap[hostPid] = cgroup
	}

	if strings.Index(cgroup, "/docker") == 0 {
		pathParts := strings.Split(cgroup, "/")
		cID := pathParts[2]
		return cID, nil
	}

	return "", nil
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
