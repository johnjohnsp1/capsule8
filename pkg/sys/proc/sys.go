package proc

import (
	"strconv"
	"strings"

	"github.com/golang/glog"
)

// MaxPid returns the maximum PID
func MaxPid() uint {
	contents, err := ReadFile("sys/kernel/pid_max")
	if err != nil {
		glog.Fatalf("Couldn't read /proc/sys/kernel/pid_max: %s", err)
	}

	pidMax, err := strconv.Atoi(strings.TrimRight(string(contents), "\n"))
	if err != nil {
		glog.Fatalf("Couldn't parse /proc/sys/kernel/pid_max: %s", err)
	}

	return uint(pidMax)
}
