// Copyright 2017 Capsule8 Inc. All rights reserved.

package sensor

import (
	"net"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"

	"google.golang.org/grpc"

	"github.com/capsule8/capsule8/pkg/config"
	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/golang/glog"
)

// Globals
var (
	// Sensor is the global Sensor instance
	Sensor sensor
)

// Server represents a server that can be registered to be run by the Sensor.
type Server interface {
	Name() string
	Serve() error
	Stop()
}

// Sensor represents the state of the singleton Sensor instance
type sensor struct {
	sync.Mutex

	id                  string
	perfEventMountPoint string
	traceFSMountPoint   string
	servers             []Server
	grpcListener        net.Listener
	grpcServer          *grpc.Server
	stopped             bool

	// Channel that signals stopping the sensor
	stopChan chan struct{}
}

func (s *sensor) RegisterServer(server Server) {
	s.Lock()
	Sensor.servers = append(Sensor.servers, server)
	s.Unlock()
}

// Serve listens for subscriptions over configured interfaces (i.e. gRPC, STAN).
// Serve always returns non-nill error.
func (s *sensor) Serve() error {
	var err error

	//
	// We require that our run dir (usually /var/run/capsule8) exists.
	// Ensure that now before proceeding any further.
	//
	err = os.MkdirAll(config.Global.RunDir, 0700)
	if err != nil {
		glog.Warningf("Couldn't mkdir %s: %s",
			config.Global.RunDir, err)
		return err
	}

	//
	// If there is no mounted tracefs, the Sensor really can't do anything.
	// Try mounting our own private mount of it.
	//
	if !config.Sensor.DontMountTracing && len(sys.TracingDir()) == 0 {
		// If we couldn't find one, try mounting our own private one
		glog.V(2).Info("Can't find mounted tracefs, mounting one")
		err = s.mountTraceFS()
		if err != nil {
			glog.V(1).Info(err)
			return err
		}
	}

	//
	// If there is no mounted cgroupfs for the perf_event cgroup, we can't
	// efficiently separate processes in monitored containers from host
	// processes. We can run without it, but it's better performance when
	// available.
	//
	if !config.Sensor.DontMountPerfEvent && len(sys.PerfEventDir()) == 0 {
		glog.V(2).Info("Can't find mounted perf_event cgroupfs, mounting one")
		err = s.mountPerfEventCgroupFS()
		if err != nil {
			glog.V(1).Info(err)
			// This is not a fatal error condition, proceed on
		}
	}

	s.Lock()
	s.stopChan = make(chan struct{})
	servers := s.servers
	s.Unlock()

	wg := sync.WaitGroup{}

	go func() {
		<-s.stopChan

		for _, server := range servers {
			glog.V(1).Infof("Stopping %s", server.Name())
			server.Stop()
		}

		s.stopped = true
	}()

	glog.Info("Starting servers...")
	for _, server := range servers {
		wg.Add(1)

		s := server
		go func() {
			glog.V(1).Infof("Starting %s", s.Name())
			serveErr := s.Serve()
			glog.V(1).Infof("%s Serve(): %v", s.Name(), serveErr)
			wg.Done()
		}()
	}

	// Block until goroutines have exited and then clean up after ourselves
	glog.Info("Sensor is ready")
	wg.Wait()

	if len(s.traceFSMountPoint) > 0 {
		err = s.unmountTraceFS()
	}

	if len(s.perfEventMountPoint) > 0 {
		err = s.unmountPerfEventCgroupFS()
	}

	return err
}

func (s *sensor) mountTraceFS() error {
	dir := filepath.Join(config.Global.RunDir, "tracing")
	err := os.MkdirAll(dir, 0500)
	if err != nil {
		glog.V(2).Infof("Couldn't create temp tracefs mountpoint: %s", err)
		return err
	}

	err = unix.Mount("tracefs", dir, "tracefs", 0, "")
	if err != nil {
		glog.V(2).Infof("Couldn't mount tracefs on %s: %s", dir, err)
		return err
	}

	s.traceFSMountPoint = dir
	return nil
}

func (s *sensor) unmountTraceFS() error {
	err := unix.Unmount(s.traceFSMountPoint, 0)
	if err != nil {
		glog.V(2).Infof("Couldn't unmount tracefs at %s: %s",
			s.traceFSMountPoint, err)
		return err
	}

	err = os.Remove(s.traceFSMountPoint)
	if err != nil {
		glog.V(2).Infof("Couldn't remove %s: %s",
			s.traceFSMountPoint, err)

		return err
	}

	s.traceFSMountPoint = ""
	return nil
}

func (s *sensor) mountPerfEventCgroupFS() error {
	dir := filepath.Join(config.Global.RunDir, "perf_event")
	err := os.MkdirAll(dir, 0500)
	if err != nil {
		glog.V(2).Infof("Couldn't create perf_event cgroup mountpoint: %s",
			err)
		return err
	}

	err = unix.Mount("cgroup", dir, "cgroup", 0, "perf_event")
	if err != nil {
		glog.V(2).Infof("Couldn't mount perf_event cgroup on %s: %s", dir, err)
		return err
	}

	s.perfEventMountPoint = dir
	return nil
}

func (s *sensor) unmountPerfEventCgroupFS() error {
	err := unix.Unmount(s.perfEventMountPoint, 0)
	if err != nil {
		glog.V(2).Infof("Couldn't unmount perf_event cgroup at %s: %s",
			s.perfEventMountPoint, err)
		return err
	}

	err = os.Remove(s.perfEventMountPoint)
	if err != nil {
		glog.V(2).Infof("Couldn't remove %s: %s",
			s.perfEventMountPoint, err)

		return err
	}

	s.perfEventMountPoint = ""
	return nil
}

// Stop signals for the Sensor to stop listening for subscriptions and shut down
func (s *sensor) Stop() {
	s.Lock()
	defer s.Unlock()

	//
	// It's ok to call Stop multiple times, so only close the stopChan if
	// the Sensor is running.
	//
	if !s.stopped {
		close(s.stopChan)
	}
}
