// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"fmt"
	"sync"
	"time"

	"os"

	api "github.com/capsule8/reactive8/pkg/api/v0"
	backend "github.com/capsule8/reactive8/pkg/pubsub"
	pbmock "github.com/capsule8/reactive8/pkg/pubsub/mock"
	pbstan "github.com/capsule8/reactive8/pkg/pubsub/stan"
	pbsensor "github.com/capsule8/reactive8/pkg/sensor"
	"github.com/golang/protobuf/proto"
	"github.com/kelseyhightower/envconfig"
	uuid "github.com/satori/go.uuid"
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
func CreateSensor(name string) (Sensor, error) {
	s := &sensor{
		uuid:          uuid.NewV4().String(),
		config:        &sensorConfig{},
		subscriptions: make(map[string]*subscriptionMetadata),
	}
	if err := s.loadConfig(name); err != nil {
		return nil, err
	}
	// Connect pubsub backend
	switch s.config.Pubsub {
	case "stan":
		s.pubsub = &pbstan.Backend{}
	case "mock":
		s.pubsub = &pbmock.Backend{}
	default:
		return nil, ErrInvalidPubsub(s.config.Pubsub)
	}
	if err := s.pubsub.Connect(); err != nil {
		return nil, err
	}
	return s, nil
}

type sensor struct {
	sync.Mutex
	uuid   string
	pubsub backend.Backend
	config *sensorConfig
	// Map of subscription ID -> Subscription metadata
	subscriptions map[string]*subscriptionMetadata
}

type sensorConfig struct {
	Pubsub              string `default:"stan"`
	SubscriptionTimeout int64  `default:"5"` // Default to a subscription timeout of 5 seconds
}

type subscriptionMetadata struct {
	lastSeen     int64 // Unix timestamp w/ second level precision of when sub was last seen
	subscription *api.Subscription
	stopChan     chan interface{}
}

func (s *sensor) loadConfig(name string) error {
	if err := envconfig.Process(name, s.config); err != nil {
		return err
	}
	return nil
}

// StartSensor starts the async subscription listener
func (s *sensor) Start() (chan interface{}, error) {
	sub, messages, err := s.pubsub.Pull("subscription.*")
	if err != nil {
		return nil, err
	}

	closeSignal := make(chan interface{})
	// Handle
	go func() {
	sendLoop:
		for {
			select {
			case <-closeSignal:
				break sendLoop
			case msg, ok := <-messages:
				if !ok {
					fmt.Fprint(os.Stderr, "Failed receiving events\n")
					break sendLoop
				}
				ss := &api.SignedSubscription{}
				if err = proto.Unmarshal(msg.Payload, ss); err != nil {
					fmt.Fprintf(os.Stderr, "No selector specified in subscription.%s\n", err.Error())
					return
				}

				// TODO: Filter subscriptions based on cluster/node information

				// Check if there is actually an EventFilter in the request. If not, ignore.
				if ss.Subscription.EventFilter == nil {
					fmt.Fprint(os.Stderr, "No EventFilter specified in subscription\n")
					return
				}

				// New subscription?
				s.Lock()
				if _, ok := s.subscriptions[ss.SubscriptionId]; !ok {
					s.subscriptions[ss.SubscriptionId] = &subscriptionMetadata{
						lastSeen:     time.Now().Add(time.Duration(s.config.SubscriptionTimeout) * time.Second).Unix(),
						subscription: ss.Subscription,
					}
					s.subscriptions[ss.SubscriptionId].stopChan = s.newSensor(ss.Subscription, ss.SubscriptionId)
				} else {
					// Existing subscription? Update unix ts
					s.subscriptions[ss.SubscriptionId].lastSeen = time.Now().Unix()
				}
				s.Unlock()

			}
		}
		sub.Close()
	}()

	if err != nil {
		return nil, ErrListeningFor("new subscriptions")
	}
	return closeSignal, nil
}

func (s *sensor) RemoveStaleSubscriptions() {
	for {
		s.Lock()
		now := time.Now().Unix()
		for subscriptionID, subscription := range s.subscriptions {
			if now-subscription.lastSeen >= s.config.SubscriptionTimeout {
				close(s.subscriptions[subscriptionID].stopChan)
				delete(s.subscriptions, subscriptionID)
			}

		}
		s.Unlock()
		time.Sleep(time.Duration(s.config.SubscriptionTimeout) * time.Second)
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
		fmt.Fprintf(os.Stderr, "Couldn't start Sensor: %v\n", err)
		os.Exit(1)
	}

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
					fmt.Fprint(os.Stderr, "Failed to get next api.\n")
					break sendLoop
				}
				//log.Println("Sending event:", ev)

				data, err := proto.Marshal(ev.(*api.Event))
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to marshal event data: %v\n", err)
				}
				// TODO: We should have some retry logic in place
				s.pubsub.Publish(
					fmt.Sprintf("api.%s", subscriptionID),
					data,
				)
			}
		}
	}()

	return stopChan
}
