// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/config"
	backend "github.com/capsule8/reactive8/pkg/pubsub"
	pbmock "github.com/capsule8/reactive8/pkg/pubsub/mock"
	pbstan "github.com/capsule8/reactive8/pkg/pubsub/stan"
	pbsensor "github.com/capsule8/reactive8/pkg/sensor"
	"github.com/capsule8/reactive8/pkg/sysinfo"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

// Sensor represents a node sensor which manages various sensors subsensors and subscriptions.
type Sensor interface {
	// Start listening for subscriptions
	Start() (chan interface{}, error)
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
	switch config.Sensor.Pubsub {
	case "stan":
		s.pubsub = &pbstan.Backend{}
	case "mock":
		s.pubsub = &pbmock.Backend{}
	default:
		return nil, ErrInvalidPubsub(config.Sensor.Pubsub)
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
}

type subscriptionMetadata struct {
	lastSeen     int64 // Unix timestamp w/ second level precision of when sub was last seen
	subscription *api.Subscription
	stopChan     chan interface{}
}

// StartSensor starts the async subscription listener
func (s *sensor) Start() (chan interface{}, error) {
	sub, messages, err := s.pubsub.Pull("subscription.*")
	if err != nil {
		return nil, err
	}

	closeSignal := make(chan interface{})

	// Start local telemetry service
	go startTelemetryService(s, closeSignal)

	// Handle subscriptions
	go func() {
	sendLoop:
		for {
			select {
			case <-closeSignal:
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
					s.subscriptions[subID].stopChan = s.newSensor(sub, subID)
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
	discoveryLoop:
		for {
			select {
			case <-closeSignal:
				break discoveryLoop
			default:
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
				time.Sleep(10 * time.Second)

			}
		}
	}()
	return closeSignal, nil
}

func (s *sensor) RemoveStaleSubscriptions() {
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
		time.Sleep(time.Duration(config.Sensor.SubscriptionTimeout) * time.Second)
	}
}

func (s *sensor) newSensor(sub *api.Subscription, subscriptionID string) chan interface{} {
	stopChan := make(chan interface{})
	// Handle optional subscription arguments
	modifier := sub.Modifier
	if modifier == nil {
		modifier = &api.Modifier{}
	}
	stream, err := pbsensor.NewSensor(sub)
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
				pbsensor.Remove(sub)
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
