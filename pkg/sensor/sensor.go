// Copyright 2017 Capsule8 Inc. All rights reserved.

package sensor

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/net/context"
	"golang.org/x/sys/unix"

	"google.golang.org/grpc"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/capsule8/pkg/config"
	"github.com/capsule8/capsule8/pkg/subscription"
	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/capsule8/capsule8/pkg/version"
	"github.com/coreos/pkg/health"
	"github.com/golang/glog"
)

// Globals
var (
	sensorOnce     sync.Once
	sensorInstance *Sensor
)

// Errors
var (
	ErrListeningFor  = func(err string) error { return fmt.Errorf("error listening for %s", err) }
	ErrConnectingTo  = func(err string) error { return fmt.Errorf("error connecting to %s", err) }
	ErrMountTraceFS  = func(err error) error { return fmt.Errorf("error mounting tracefs: %s", err) }
)

// Sensor represents the state of the singleton Sensor instance
type Sensor struct {
	sync.Mutex

	perfEventMountPoint string
	traceFSMountPoint   string
	id                  string
	grpcListener        net.Listener
	grpcServer          *grpc.Server
	stopped             bool

	healthChecker health.Checker

	// Map of subscription ID -> Subscription metadata
	subscriptions map[string]*subscriptionMetadata

	// Channel that signals stopping the sensor
	stopChan chan struct{}
}

type subscriptionMetadata struct {
	lastSeen     int64 // Unix timestamp w/ second level precision of when sub was last seen
	subscription *api.Subscription
	stopChan     chan struct{}
}

func createSensor() (*Sensor, error) {
	s := &Sensor{
		id:            subscription.SensorID,
		subscriptions: make(map[string]*subscriptionMetadata),
	}

	return s, nil
}

// GetSensor returns the singleton instance of the Sensor, creating it if
// necessary.
func GetSensor() (*Sensor, error) {
	var err error

	sensorOnce.Do(func() {
		sensorInstance, err = createSensor()
	})

	return sensorInstance, err
}

func (s *Sensor) startLocalRPCServer() error {
	var err error
	var lis net.Listener

	parts := strings.Split(config.Sensor.ListenAddr, ":")
	if len(parts) > 1 && parts[0] == "unix" {
		socketPath := parts[1]

		//
		// Check whether socket already exists and if someone
		// is already listening on it.
		//
		_, err = os.Stat(socketPath)
		if err == nil {
			ua, err := net.ResolveUnixAddr("unix", socketPath)
			if err == nil {
				c, err := net.DialUnix("unix", nil, ua)
				if err == nil {
					// There is another running
					// Sensor, try to listen below
					// and return the error.

					c.Close()
				} else {
					// Remove the stale socket so
					// the listen below will
					// succed.
					os.Remove(socketPath)
				}
			}
		}

		oldMask := unix.Umask(0077)
		lis, err = net.Listen("unix", socketPath)
		unix.Umask(oldMask)
	} else {
		lis, err = net.Listen("tcp", config.Sensor.ListenAddr)
	}

	if err != nil {
		return err
	}

	s.grpcListener = lis

	// Start local gRPC service on listener
	s.grpcServer = grpc.NewServer()

	t := &telemetryServiceServer{
		s: s,
	}
	api.RegisterTelemetryServiceServer(s.grpcServer, t)

	return nil
}

// Serve listens for subscriptions over configured interfaces (i.e. gRPC, STAN).
// Serve always returns non-nill error.
func (s *Sensor) Serve() error {
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
			return ErrMountTraceFS(err)
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
	s.stopped = false
	s.Unlock()

	wg := sync.WaitGroup{}

	//
	// Start monitoring HTTP endpoint
	//
	if config.Sensor.MonitoringPort > 0 {
		glog.Info("Serving HTTP monitoring on ",
			config.Sensor.MonitoringPort)

		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", s.healthChecker.ServeHTTP)
		mux.HandleFunc("/version", version.HTTPHandler)
		httpServer := &http.Server{
			Addr:    fmt.Sprintf("0.0.0.0:%d", config.Sensor.MonitoringPort),
			Handler: mux,
		}

		var lis net.Listener
		lis, err = net.Listen("tcp", httpServer.Addr)
		if err != nil {
			glog.Fatalf("Couldn't start HTTP listener on %s: %s",
				httpServer.Addr, err)
		} else {
			wg.Add(1)

			go func() {
				<-s.stopChan
				glog.Info("Stopping HTTP monitoring endpoint...")
				httpServer.Shutdown(context.Background())
			}()

			go func() {
				serveErr := httpServer.Serve(lis)
				glog.V(1).Info(serveErr)

				lis.Close()

				wg.Done()
			}()
		}
	}

	//
	// If we have a ListenAddr configured, start local gRPC Server on it
	//
	if len(config.Sensor.ListenAddr) > 0 {
		glog.Info("Serving gRPC API on ", config.Sensor.ListenAddr)

		err = s.startLocalRPCServer()
		if err != nil {
			glog.Fatal("Couldn't start local gRPC server: ", err)
		} else {
			wg.Add(1)

			go func() {
				<-s.stopChan
				glog.Info("Stopping gRPC Server")
				s.grpcServer.GracefulStop()
			}()

			go func() {
				// grpc.Serve always returns with non-nil error,
				// so don't return it.
				serveErr := s.grpcServer.Serve(s.grpcListener)
				glog.V(1).Info(serveErr)

				s.grpcListener.Close()
				wg.Done()
			}()
		}
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

func (s *Sensor) mountTraceFS() error {
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

func (s *Sensor) unmountTraceFS() error {
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

func (s *Sensor) mountPerfEventCgroupFS() error {
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

func (s *Sensor) unmountPerfEventCgroupFS() error {
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
func (s *Sensor) Stop() {
	s.Lock()
	defer s.Unlock()

	//
	// It's ok to call Stop multiple times, so only close the stopChan if
	// the Sensor is running.
	//
	if !s.stopped {
		close(s.stopChan)
		s.stopped = true
	}
}
