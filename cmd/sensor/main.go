// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"fmt"
	"log"
	"time"

	"os"

	"github.com/capsule8/reactive8/pkg/api/apiserver"
	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/sensor"
	"github.com/golang/protobuf/proto"
	"github.com/kelseyhightower/envconfig"
	nats "github.com/nats-io/go-nats"
	stan "github.com/nats-io/go-nats-streaming"
	"github.com/nats-io/go-nats/encoders/protobuf"
)

type sensorConfig struct {
	StanClusterName     string `default:"c8-backplane"`
	NatsURL             string `default:"nats://localhost:4222"`
	SubscriptionTimeout int64  `default:"5"` // Default to a subscription timeout of 5 seconds
}

type subscriptionMetadata struct {
	lastSeen     int64 // Unix timestamp w/ second level precision of when sub was last seen
	subscription *event.Subscription
}

var Config sensorConfig

// Map of subscription ID -> client ID -> Subscription metadata
var subscriptions = make(map[string]map[string]*subscriptionMetadata)

// Map of subscription ID -> sensor stop channel
var sensorStopChans = make(map[string]chan interface{})

func main() {
	log.Println("[NODE-SENSOR] starting up")
	LoadConfig("sensor")
	StartSensor()
	log.Println("[NODE-SENSOR] started")
	// Blocking call to remove stale subscriptions on a 5 second interval
	RemoveStaleSubscriptions()
}

// LoadConfig loads env vars into config with prefix `name`
func LoadConfig(name string) {
	err := envconfig.Process(name, &Config)
	if err != nil {
		log.Fatal("Failed to read env vars:", err)
	}
}

// StartSensor starts the async subscription listener
func StartSensor() {
	hostname, _ := os.Hostname()
	stanConn, err := stan.Connect(Config.StanClusterName, fmt.Sprintf("node-sensor_%s", hostname), stan.NatsURL(Config.NatsURL))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't connect to STAN cluster: %v\n", err)
		os.Exit(1)
	}

	// Listen for subscriptions
	natsConn, err := nats.Connect(Config.NatsURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to NATS: %v\n", err)
		os.Exit(1)
	}
	ec, _ := nats.NewEncodedConn(natsConn, protobuf.PROTOBUF_ENCODER)
	_, err = ec.Subscribe("subscription.*", func(hb *apiserver.SubscriptionHeartbeat) {
		// TODO: Filter subscriptions based on cluster/node information
		// New subscription?
		if _, ok := subscriptions[hb.SubscriptionId]; !ok {
			subscriptions[hb.SubscriptionId] = make(map[string]*subscriptionMetadata)
			stopChan := newSensor(stanConn, *hb.Subscription.Selector, hb.SubscriptionId)
			sensorStopChans[hb.SubscriptionId] = stopChan
		}

		// New client for subscription?
		if _, ok := subscriptions[hb.SubscriptionId][hb.ClientId]; !ok {
			subscriptions[hb.SubscriptionId][hb.ClientId] = &subscriptionMetadata{
				lastSeen:     time.Now().Unix(), // Gives us second precision
				subscription: hb.Subscription,
			}
		} else {
			// Existing client? Update unix ts
			subscriptions[hb.SubscriptionId][hb.ClientId].lastSeen = time.Now().Unix()

		}
	})
	if err != nil {
		log.Fatal("Failed to listen for new subscriptions:", err)
	}
}

// RemoveStaleSubscriptions is a blocking call that removes stale subscriptions @ `SubscriptionTimeout` interval
func RemoveStaleSubscriptions() {
	for {
		// Need a double loop to iterate over subscription ID -> client ID -> subscription
		now := time.Now().Unix()
		for subscriptionID, clientMap := range subscriptions {
			for clientID, subscription := range clientMap {
				// Close and remove the subscription if the sub is stale
				if now-subscription.lastSeen >= Config.SubscriptionTimeout {
					delete(clientMap, clientID)
				}
			}

			// Delete subscription if there are no clients subscribed for it
			// and clean up sensor broadcasting to `SUBSCRIPTION ID`
			if len(clientMap) == 0 {
				delete(subscriptions, subscriptionID)
				close(sensorStopChans[subscriptionID])
				delete(sensorStopChans, subscriptionID)
			}

		}
		time.Sleep(time.Duration(Config.SubscriptionTimeout) * time.Second)
	}
}

func newSensor(conn stan.Conn, selector event.Selector, subscriptionID string) chan interface{} {
	stopChan := make(chan interface{})

	// Create the sensors
	stream, err := sensor.NewSensor(selector)
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
				break sendLoop
			case ev, ok := <-stream.Data:
				if !ok {
					fmt.Fprint(os.Stderr, "Failed to get next event.")
					continue sendLoop
				}
				//log.Println("Sending event:", ev)

				data, err := proto.Marshal(ev.(*event.Event))
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to marshal event data: %v\n", err)
				}
				conn.Publish(fmt.Sprintf("event.%s", subscriptionID), data)
			}
		}
	}()

	return stopChan
}
