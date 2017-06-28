// Package perf provides an interface to the Linux kernel performance
// monitoring subsystem through perf_event_open(2).
package perf

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"sync"

	"golang.org/x/sys/unix"
)

// cgroup filesystem mount point
// XXX: check mtab by default, allow env override
const cgroupfs = "/sys/fs/cgroup"

// TODO:
//
// ftrace filters using PERF_EVENT_IOC_SET_FILTER. Limits are string
// must be <= 4096 bytes, no more than 16384 predicates.
//
// http://elixir.free-electrons.com/linux/latest/source/kernel/trace/trace_events_filter.c#L38
//

type Perf struct {
	rwlock      *sync.RWMutex
	pipe        [2]int
	eventAttrs  []*EventAttr
	pid         int
	cpu         int
	fds         []int
	ringBuffers []*ringBuffer
}

func NewWithCgroup(eventAttrs []*EventAttr, filters []string, name string) (*Perf, error) {
	return newCgroupSession(eventAttrs, filters, name)
}

func newCgroupSession(eventAttrs []*EventAttr, filters []string, cgroup string) (*Perf, error) {
	// Cgroup can be either a:
	// - cgroupfs path ("/docker/abcd09876...")
	// - systemd cgroup path ("system.slice:docker:abcd09876...")

	var path string

	if cgroup[0] == '/' {
		path = filepath.Join(cgroupfs, "perf_event", cgroup)
	} else {
		parts := strings.Split(cgroup, ":")
		if parts[1] != "docker" {
			log.Printf("Couldn't parse cgroup %s", cgroup)
			return nil, errors.New("Couldn't parse cgroup")
		}

		dockerScope := []string{"docker-", parts[2], ".scope"}

		path = filepath.Join(cgroupfs, "perf_event", parts[0],
			strings.Join(dockerScope, ""))
	}

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	return newSession(eventAttrs, filters, PERF_FLAG_PID_CGROUP, int(f.Fd()))
}

// New creates a new performance monitoring session for the events specified
// in events. The two optional arguments pid and cpu affect the scope of
// processes and CPUs to monitor.
func New(eventAttrs []*EventAttr, filters []string, args ...int) (*Perf, error) {
	return newSession(eventAttrs, filters, 0, args...)
}

func newSession(eventAttrs []*EventAttr, filters []string, initFlags uintptr, args ...int) (*Perf, error) {
	var pid, cpu int
	var perfFds []int

	if len(args) > 0 {
		pid = args[0]
	} else {
		pid = -1
	}

	if len(args) > 1 {
		cpu = args[1]
	} else {
		cpu = -1
	}

	if cpu < 0 {
		// Monitor on all cpus
		perfFds = make([]int, runtime.NumCPU())
	} else {
		perfFds = make([]int, 1)
	}

	ringBuffers := make([]*ringBuffer, len(perfFds))

	// Create stop notification pipe
	var pipe [2]int
	err := unix.Pipe2(pipe[:], unix.O_DIRECT|unix.O_NONBLOCK)
	if err != nil {
		return nil, err
	}

	for i := range perfFds {
		for j := range eventAttrs {
			var groupFd int
			var flags = initFlags

			if j == 0 {
				// Event group leader
				groupFd = -1
				flags |= PERF_FLAG_FD_CLOEXEC
			} else {
				// Additional events in the group
				groupFd = perfFds[i]
				flags |= PERF_FLAG_FD_OUTPUT
			}

			fd, err := open(eventAttrs[j], pid, i, groupFd, flags)
			if err != nil {
				return nil, err
			}

			if filters != nil && len(filters[j]) > 0 {
				err := setFilter(fd, filters[j])
				if err != nil {
					return nil, err
				}
			}

			if j > 0 {
				continue
			}

			rb, err := newRingBuffer(fd, eventAttrs[j].SampleType,
				eventAttrs[j].ReadFormat)
			if err != nil {
				return nil, err
			}

			// Store event group leader fd and ring buffer
			perfFds[i] = fd
			ringBuffers[i] = rb
		}
	}

	p := new(Perf)
	p.rwlock = &sync.RWMutex{}
	p.pipe = pipe
	p.fds = perfFds
	p.ringBuffers = ringBuffers

	return p, nil

}

// Close terminates the performance monitoring session
func (p *Perf) Close() error {
	p.rwlock.Lock()

	err := unix.Close(p.pipe[1])
	p.pipe[1] = -1

	p.rwlock.Unlock()

	return err
}

func (p *Perf) Enable() error {
	p.rwlock.RLock()
	defer p.rwlock.RUnlock()

	for i := range p.fds {
		err := enable(p.fds[i])
		if err != nil {
			log.Printf("Couldn't enable event: %v", err)
			return err
		}
	}

	return nil
}

func (p *Perf) Disable() error {
	p.rwlock.RLock()
	defer p.rwlock.RUnlock()

	for i := range p.fds {
		err := disable(p.fds[i])
		if err != nil {
			log.Printf("Couldn't disable event: %v", err)
			return err
		}
	}

	return nil
}

func (p *Perf) SetFilter(filter string) error {
	p.rwlock.RLock()
	defer p.rwlock.RUnlock()

	for i := range p.fds {
		err := setFilter(p.fds[i], filter)
		if err != nil {
			log.Printf("Couldn't set filter on event: %v", err)
			return err
		}
	}

	return nil
}

// Run monitors the configured events and calls onEvent for each one.
// The given callback function must perform its own locking if necessary
// since it can be called from multiple goroutines simultaneously.
func (p *Perf) Run(onEvent func(*Sample, error)) error {
	p.rwlock.RLock()

	var err error

	pollFds := make([]unix.PollFd, len(p.fds)+1)

	pollFds[0].Fd = int32(p.pipe[0])
	pollFds[0].Events = unix.POLLIN

	cond := sync.NewCond(p.rwlock.RLocker())

	for i := range p.fds {
		pollFds[1+i].Fd = int32(p.fds[i])
		pollFds[1+i].Events = unix.POLLIN

		// Start a goroutine to read on the ringbuffer when signaled
		go p.ringBuffers[i].readOnCond(cond, onEvent)
	}

	p.rwlock.RUnlock()

pollLoop:
	for {
		p.rwlock.RLock()

		n := 0
		for i := range pollFds {
			if pollFds[i].Fd > 0 {
				n++
			}
		}

		p.rwlock.RUnlock()

		if n == 0 {
			return nil
		}

		n, err = unix.Poll(pollFds, -1)
		if err != nil && err != unix.EINTR {
			return err
		} else if n == 0 {
			// timeout, restart poll()
			continue
		} else if n > 0 {
			// Wake all ringbuffer reader goroutines to consume
			// all of their available events
			cond.Broadcast()

			// Now check for any errors
			for i := range pollFds {
				rev := pollFds[i].Revents
				if (rev & ^unix.POLLIN) != 0 {
					// POLLERR, POLLHUP, or POLLNVAL set
					break pollLoop
				}
			}
		}
	}

	// Shut everything down cleanly
	p.rwlock.Lock()

	for i := range p.fds {
		err = unix.Munmap(p.ringBuffers[i].memory)
		p.ringBuffers[i].memory = nil

		err = unix.Close(int(p.fds[i]))
		p.fds[i] = -1
	}

	p.rwlock.Unlock()

	// Broadcast to the goroutines after setting memory to nil to signal
	// them to exit
	cond.Broadcast()

	return err
}
