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
