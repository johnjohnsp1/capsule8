package perf

import (
	"log"
	"runtime"

	"golang.org/x/sys/unix"
)

// TODO:
//
// Multiple events per event group
//
// ftrace filters
//
// disabling perf events for ourselves:
//    Using prctl(2)
//       A process can enable or disable all the event  groups  that  are  attached  to  it  using  the
//       prctl(2)  PR_TASK_PERF_EVENTS_ENABLE and PR_TASK_PERF_EVENTS_DISABLE operations.  This applies
//       to all counters on the calling process, whether created by this process  or  by  another,  and
//       does  not affect any counters that this process has created on other processes.  It enables or
//       disables only the group leaders, not any other members in the groups.
//

type Perf struct {
	fds         []int
	ringBuffers []*ringBuffer
}

// New creates a new performance monitoring session on all processes/CPUs
// for events specified in the given EventAttr slice. It is initially
// disabled and needs to be enabled by calling Enable().
func New(eventAttr EventAttr) (*Perf, error) {
	perfFds := make([]int, runtime.NumCPU())
	ringBuffers := make([]*ringBuffer, len(perfFds))

	for i := range perfFds {
		fd, err := open(eventAttr, -1, i, -1, PERF_FLAG_FD_CLOEXEC)
		if err != nil {
			return nil, err
		}

		rb, err := newRingBuffer(fd, eventAttr.SampleType)
		if err != nil {
			return nil, err
		}

		perfFds[i] = fd
		ringBuffers[i] = rb

		/*  // For additional events in the event group, specify PERF_FLAG_FD_OUTPUT
		for j := 1; j < len(eventAttrs); i++ {
			_, err := open(eventAttrs[j], -1, i, fd, PERF_FLAG_FD_OUTPUT)
			if err != nil {
				return nil, err
			}
		}
		*/

	}

	p := new(Perf)
	p.fds = perfFds
	p.ringBuffers = ringBuffers

	return p, nil
}

func (p *Perf) Enable() error {
	for i := range p.fds {
		err := enable(p.fds[i])
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Perf) Disable() error {
	for i := range p.fds {
		err := disable(p.fds[i])
		if err != nil {
			return err
		}
	}

	return nil
}

// Run monitors the configured events and calls onEvent for each one
func (p *Perf) Run(onEvent func(*Event, error)) {

	pollFds := make([]unix.PollFd, len(p.fds))

	for i := range pollFds {
		pollFds[i].Fd = int32(p.fds[i])
		pollFds[i].Events = unix.POLLIN
	}

	for {
		n, err := unix.Poll(pollFds, -1)
		if err != nil {
			log.Fatal(err)
		} else if n > 0 {
			for i := range pollFds {
				if pollFds[i].Revents&unix.POLLIN != 0 {
					p.ringBuffers[i].read(onEvent)
				}
			}
		}
	}
}
