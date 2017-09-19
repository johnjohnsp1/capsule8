// Copyright 2017 Capsule8 Inc. All rights reserved.

package sensor

import (
	"crypto/sha256"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/backend"
	pbmock "github.com/capsule8/reactive8/pkg/backend/mock"
	pbstan "github.com/capsule8/reactive8/pkg/backend/stan"
	"github.com/capsule8/reactive8/pkg/config"
	checks "github.com/capsule8/reactive8/pkg/health"
	"github.com/capsule8/reactive8/pkg/subscription"
	"github.com/capsule8/reactive8/pkg/sysinfo"
	"github.com/coreos/pkg/health"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
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
	ErrInvalidPubsub = func(err string) error { return fmt.Errorf("invalid pubsub backend %s", err) }
)

type Sensor struct {
	sync.Mutex

	id           string
	grpcListener net.Listener
	grpcServer   *grpc.Server
	pubsub       backend.Backend
	running      bool

	healthChecker health.Checker

	// Map of subscription ID -> Subscription metadata
	subscriptions map[string]*subscriptionMetadata

	// Wait Group for the sensor goroutines
	wg sync.WaitGroup

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

	//
	// Connect configured pubsub backend
	//
	switch config.Sensor.Backend {
	case "none":
		s.pubsub = nil
	case "stan":
		s.pubsub = &pbstan.Backend{}
	case "mock":
		s.pubsub = &pbmock.Backend{}
	default:
		return nil, ErrInvalidPubsub(config.Sensor.Backend)
	}

	//
	// Start healthchecks HTTP endpoint
	//
	if config.Sensor.MonitoringPort > 0 {
		s.configureHealthChecks()
		http.HandleFunc("/healthz", s.healthChecker.ServeHTTP)

		go func() {
			err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d",
				config.Sensor.MonitoringPort), nil)
			glog.Fatal(err)
		}()
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

// configure and initialize all Checkable variables required by the health checker
func (s *Sensor) configureHealthChecks() {
	if config.Sensor.Backend == "stan" {
		stanChecker := checks.ConnectionURL(fmt.Sprintf("%s/streaming/serverz",
			config.Backplane.NatsMonitoringURL))

		s.healthChecker.Checks = []health.Checkable{
			stanChecker,
		}
	}
}

func (s *Sensor) listenForSubscriptions() error {
	sub, messages, err := s.pubsub.Pull("subscription.*")
	if err != nil {
		return err
	}

sendLoop:
	for {
		select {
		case <-s.stopChan:
			glog.V(1).Info("stopChan closed, breaking out of sendLoop")
			break sendLoop
		case msg, ok := <-messages:
			if !ok {
				glog.Errorln("failed receiving events")
				break sendLoop
			}
			sub := &api.Subscription{}
			if err = proto.Unmarshal(msg.Payload, sub); err != nil {
				glog.Errorf("no selector specified in subscription.%s\n", err.Error())
				continue sendLoop
			}
			// Ignore subscriptions that have specified a time range
			if sub.SinceDuration != nil || sub.ForDuration != nil {
				continue sendLoop
			}

			// TODO: Filter subscriptions based on cluster/node information

			// Check if there is actually an EventFilter in the request. If not, ignore.
			if sub.EventFilter == nil {
				glog.Errorln("no EventFilter specified in subscription")
				continue sendLoop
			}

			// New subscription?
			b, _ := proto.Marshal(sub)
			h := sha256.New()
			h.Write(b)
			subID := fmt.Sprintf("%x", h.Sum(nil))
			s.Lock()
			if _, ok := s.subscriptions[subID]; !ok {
				s.subscriptions[subID] = &subscriptionMetadata{
					lastSeen:     time.Now().Add(time.Duration(config.Sensor.SubscriptionTimeout) * time.Second).Unix(),
					subscription: sub,
				}
				s.subscriptions[subID].stopChan = s.newSubscription(sub, subID)
			} else {
				// Existing subscription? Update unix ts
				s.subscriptions[subID].lastSeen = time.Now().Unix()
			}
			s.Unlock()
		}
	}
	sub.Close()

	return nil
}

func (s *Sensor) broadcastDiscoveryMessages() error {
	discoveryTimer := time.NewTimer(0)
	defer discoveryTimer.Stop()

discoveryLoop:
	for {
		select {
		case <-s.stopChan:
			break discoveryLoop

		case <-discoveryTimer.C:
			uname, err := sysinfo.Uname()
			if err != nil {
				glog.Errorf("failed to get uname info: %s\n", err.Error())
				continue discoveryLoop
			}

			nodename := string(uname.Nodename[:])

			// Override nodename if envconfig sets it
			if len(config.Sensor.NodeName) > 0 {
				nodename = config.Sensor.NodeName
			}

			s.pubsub.Publish("discover.sensor", &api.Discover{
				Info: &api.Discover_Sensor{
					Sensor: &api.Sensor{
						Id:       s.id,
						Sysname:  string(uname.Sysname[:]),
						Nodename: nodename,
						Release:  string(uname.Release[:]),
						Version:  string(uname.Version[:]),
					},
				},
			})

			discoveryTimer.Reset(10 * time.Second)
		}
	}

	return nil
}

func (s *Sensor) removeStaleSubscriptions() {
	timeout := time.Duration(config.Sensor.SubscriptionTimeout) * time.Second
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

staleRemovalLoop:
	for {
		s.Lock()
		now := time.Now().Unix()
		for subscriptionID, subscription := range s.subscriptions {
			if now-subscription.lastSeen >= config.Sensor.SubscriptionTimeout {
				glog.Infoln("SENSOR REMOVING STALE SUB:", subscriptionID)
				close(s.subscriptions[subscriptionID].stopChan)
				delete(s.subscriptions, subscriptionID)
			}

		}
		s.Unlock()

		select {
		case <-s.stopChan:
			break staleRemovalLoop
		case <-timeoutTimer.C:
			timeoutTimer.Reset(timeout)
		}
	}
}

func (s *Sensor) startLocalRPCServer() error {
	var err error
	var lis net.Listener

	parts := strings.Split(config.Sensor.ListenAddr, ":")
	if len(parts) > 1 && parts[0] == "unix" {
		socketPath := parts[1]
		socketDir := path.Dir(socketPath)

		err = os.MkdirAll(socketDir, 0600)
		if err != nil {
			return err
		}

		lis, err = net.Listen("unix", parts[1])
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

	s.Lock()
	s.running = true
	s.stopChan = make(chan struct{})
	s.Unlock()

	//
	// If we have a ListenAddr configured, start local gRPC Server on it
	//
	if len(config.Sensor.ListenAddr) > 0 {
		err = s.startLocalRPCServer()
		if err != nil {
			s.Stop()
		} else {
			s.wg.Add(1)

			go func() {
				<-s.stopChan
				s.grpcServer.GracefulStop()
			}()

			go func() {
				// grpc.Serve always returns with non-nil error,
				// so don't return it. Instead we log it w/ V(1).
				glog.V(1).Info("Starting gRPC Server on %s",
					s.grpcListener)

				serveErr := s.grpcServer.Serve(s.grpcListener)
				glog.V(1).Info(serveErr)

				s.grpcListener.Close()
				s.wg.Done()
			}()
		}
	}

	//
	// If we have a pubsub backend configured, startup async services on it
	//
	if s.pubsub != nil {
		if err := s.pubsub.Connect(); err != nil {
			s.Stop()
		} else {
			// Broadcast sensor identity over discovery topic
			s.wg.Add(1)
			go func() {
				err = s.broadcastDiscoveryMessages()
				if err != nil {
					s.Stop()
				}

				s.wg.Done()
			}()

			// Listen for telemetry subscriptions
			s.wg.Add(1)
			go func() {
				err = s.listenForSubscriptions()
				if err != nil {
					s.Stop()
				}

				s.wg.Done()
			}()

			// Reap stale subscriptions
			s.wg.Add(1)
			go func() {
				s.removeStaleSubscriptions()
				s.wg.Done()
			}()
		}
	}

	// Block until goroutines have exited and then clean up after ourselves
	s.wg.Wait()

	if s.pubsub != nil {
		s.pubsub.Close()
	}

	return err
}

// Stop signals for the Sensor to stop listening for subscriptions and shut down
func (s *Sensor) Stop() {
	s.Lock()
	defer s.Unlock()

	//
	// It's ok to call Stop multiple times, so only close the stopChan if
	// the Sensor is running.
	//
	if s.running {
		close(s.stopChan)
		s.running = false
	}
}

func (s *Sensor) newSubscription(sub *api.Subscription, subscriptionID string) chan struct{} {
	stopChan := make(chan struct{})
	// Handle optional subscription arguments
	modifier := sub.Modifier
	if modifier == nil {
		modifier = &api.Modifier{}
	}
	stream, err := subscription.NewSubscription(sub)
	if err != nil {
		glog.Errorf("couldn't start Sensor: %s\n", err.Error())
	}

	glog.Infoln("STARTING NEW LIVE SUBSCRIPTION:", subscriptionID)
	go func() {
	sendLoop:
		for {
			select {
			// Stop the send loop
			case <-stopChan:
				stream.Close()
				break sendLoop
			case ev, ok := <-stream.Data:
				if !ok {
					glog.Errorln("Failed to get next event.")
					break sendLoop
				}
				//glog.Infoln("Sending event:", ev, "sub id", subscriptionID)

				data, err := proto.Marshal(ev.(*api.Event))
				if err != nil {
					glog.Errorf("Failed to marshal event data: %s\n", err.Error())
					continue sendLoop
				}
				// TODO: We should have some retry logic in place
				s.pubsub.Publish(
					fmt.Sprintf("event.%s", subscriptionID),
					data,
				)
			}
		}
	}()

	return stopChan
}
