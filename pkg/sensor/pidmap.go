package sensor

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

const procfs = "/proc"

var mu sync.Mutex
var pidMap map[int32]string

func readCgroup(hostPid int32) (string, error) {
	filename := fmt.Sprintf("%s/%d/cgroup", procfs, hostPid)
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)

	if err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		t := scanner.Text()
		parts := strings.Split(t, ":")
		cgroupPath := parts[2]

		if strings.Index(cgroupPath, "/docker") == 0 {
			pathParts := strings.Split(cgroupPath, "/")
			containerID := pathParts[2]
			return containerID, nil
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

	cID, ok := pidMap[parentPid]
	if !ok {
		cID, _ = readCgroup(parentPid)
		pidMap[parentPid] = cID
	}

	pidMap[childPid] = cID
}

func pidMapGetContainerID(hostPid int32) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	if pidMap == nil {
		pidMap = make(map[int32]string)
	}

	cID, ok := pidMap[hostPid]
	if !ok {
		cID, _ = readCgroup(hostPid)
		pidMap[hostPid] = cID
	}

	return cID, nil
}
