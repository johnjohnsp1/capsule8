package perf

import (
	"bytes"
	"unsafe"

	"golang.org/x/sys/unix"
)

func enable(fd int) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), PERF_EVENT_IOC_ENABLE, 0)

	err := error(nil)
	if errno != 0 {
		err = errno
	}

	return err
}

func disable(fd int) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), PERF_EVENT_IOC_DISABLE, 0)

	err := error(nil)
	if errno != 0 {
		err = errno
	}

	return err
}

func open(attr *EventAttr, pid int, cpu int, groupFd int, flags uintptr) (int, error) {
	buf := new(bytes.Buffer)

	attr.write(buf)

	b := buf.Bytes()

	r1, _, errno := unix.Syscall6(unix.SYS_PERF_EVENT_OPEN, uintptr(unsafe.Pointer(&b[0])),
		uintptr(pid), uintptr(cpu), uintptr(groupFd), uintptr(flags), uintptr(0))

	err := error(nil)
	if errno != 0 {
		err = errno
	}

	return int(r1), err
}
