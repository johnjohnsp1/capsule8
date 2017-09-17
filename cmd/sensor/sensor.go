// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/backend"
	pbmock "github.com/capsule8/reactive8/pkg/backend/mock"
	pbstan "github.com/capsule8/reactive8/pkg/backend/stan"
	"github.com/capsule8/reactive8/pkg/config"
	pbsensor "github.com/capsule8/reactive8/pkg/sensor"
	"github.com/capsule8/reactive8/pkg/sysinfo"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

// Sensor represents a node sensor which manages various sensors subsensors and subscriptions.
type Sensor interface {
	// Start listening for subscriptions
	Start() error
	// Shut down the sensor
	Shutdown()
	// Wait until shutdown completes
	Wait()
	// RemoveStaleSubscriptions is a blocking call that removes
	// stale subscriptions @ `SubscriptionTimeout` interval
	RemoveStaleSubscriptions()
}

// Errors
var (
	ErrListeningFor  = func(err string) error { return fmt.Errorf("error listening for %s", err) }
	ErrConnectingTo  = func(err string) error { return fmt.Errorf("error connecting to %s", err) }
	ErrInvalidPubsub = func(err string) error { return fmt.Errorf("invalid pubsub backend %s", err) }
)

// CreateSensor creates a new node sensor
func CreateSensor() (Sensor, error) {
	s := &sensor{
		id:            pbsensor.SensorID,
		subscriptions: make(map[string]*subscriptionMetadata),
	}

	// Connect pubsub backend
	switch config.Sensor.Backend {
	case "stan":
		s.pubsub = &pbstan.Backend{}
	case "mock":
		s.pubsub = &pbmock.Backend{}
	default:
		return nil, ErrInvalidPubsub(config.Sensor.Backend)
	}
	if err := s.pubsub.Connect(); err != nil {
		return nil, err
	}
	return s, nil
}

type sensor struct {
	sync.Mutex
	id     string
	pubsub backend.Backend

	// Map of subscription ID -> Subscription metadata
	subscriptions map[string]*subscriptionMetadata
	// Wait Group for the sensor goroutines
	wg sync.WaitGroup
	// Channel that signals stopping the sensor
	stopChan chan interface{}
}

type subscriptionMetadata struct {
	lastSeen     int64 // Unix timestamp w/ second level precision of when sub was last seen
	subscription *api.Subscription
	stopChan     chan interface{}
}

// StartSensor starts the async subscription listener
func (s *sensor) Start() error {
	sub, messages, err := s.pubsub.Pull("subscription.*")
	if err != nil {
		return err
	}

	s.stopChan = make(chan interface{})

	// Start local telemetry service
	startTelemetryService(s)

	// Handle subscriptions
	go func() {
		s.wg.Add(1)
		defer s.wg.Done()
	sendLoop:
		for {
			select {
			case <-s.stopChan:
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
	}()

	// Broadcast over discovery topic
	go func() {
		s.wg.Add(1)
		defer s.wg.Done()

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
	}()
	return nil
}

// sensor.Shutdown gracefully stops the sensor's telemetry server
func (s *sensor) Shutdown() {
	close(s.stopChan)
}

// sensor.Shutdown gracefully stops the sensor's telemetry server
func (s *sensor) Wait() {
	s.wg.Wait()
	s.pubsub.Close()
}

func (s *sensor) RemoveStaleSubscriptions() {
	timeout := time.Duration(config.Sensor.SubscriptionTimeout) * time.Second
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	s.wg.Add(1)
	defer s.wg.Done()

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

func (s *sensor) newSubscription(sub *api.Subscription, subscriptionID string) chan interface{} {
	stopChan := make(chan interface{})
	// Handle optional subscription arguments
	modifier := sub.Modifier
	if modifier == nil {
		modifier = &api.Modifier{}
	}
	stream, err := pbsensor.NewSubscription(sub)
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
