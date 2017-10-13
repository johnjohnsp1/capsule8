package sysinfo

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

type utsname struct {
	Sysname    [65]byte /* Operating system name (e.g., "Linux") */
	Nodename   [65]byte /* Name within "some implementation-defined network" */
	Release    [65]byte /* Operating system release (e.g., "2.6.28") */
	Version    [65]byte /* Operating system version */
	Machine    [65]byte /* Hardware identifier */
	Domainname [65]byte /* NIS or YP domain name */
}

// Uname syscall to get system info
func Uname() (*utsname, error) {
	var err error
	var name utsname
	_, _, errno := unix.Syscall(unix.SYS_UNAME, uintptr(unsafe.Pointer(&name)), 0, 0)
	if errno != 0 {
		err = errno
	}
	return &name, err
}
