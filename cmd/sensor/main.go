// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"fmt"
	"log"
	"time"

	"os"

	event "github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/sensor"
	"github.com/gogo/protobuf/proto"
	nats "github.com/nats-io/go-nats"
	stan "github.com/nats-io/go-nats-streaming"
	"github.com/nats-io/go-nats/encoders/protobuf"
)

type subscriptionMetadata struct {
	lastSeen     int64 // Unix timestamp w/ second level precision of when sub was last seen
	subscription *event.Subscription
}

func main() {
	log.Println("[NODE-SENSOR] starting up")
	// Map of subscription ID -> client ID -> Subscription metadata
	subscriptions := make(map[string]map[string]*subscriptionMetadata)
	// Map of subscription ID -> sensor stop channel
	sensorStopChans := make(map[string]chan interface{})

	hostname, _ := os.Hostname()
	stanConn, err := stan.Connect("test-cluster", fmt.Sprintf("node-sensor_%s", hostname))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't connect to STAN cluster: %v\n", err)
		os.Exit(1)
	}

	// Listen for subscriptions
	natsConn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to NATS: %v\n", err)
		os.Exit(1)
	}
	ec, _ := nats.NewEncodedConn(natsConn, protobuf.PROTOBUF_ENCODER)
	_, err = ec.Subscribe("subscription.*", func(hb *event.SubscriptionHeartbeat) {
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

	log.Println("[NODE-SENSOR] started")
	// Remove stale subscriptions on a 5 second interval
	for {
		// Need a double loop to iterate over subscription ID -> client ID -> subscription
		now := time.Now().Unix()
		for subscriptionId, clientMap := range subscriptions {
			for clientId, subscription := range clientMap {
				// Close and remove the subscription if the sub is stale
				if now-subscription.lastSeen >= 5 {
					delete(clientMap, clientId)
				}
			}

			// Delete subscription if there are no clients subscribed for it
			// and clean up sensor broadcasting to `SUBSCRIPTION ID`
			if len(clientMap) == 0 {
				delete(subscriptions, subscriptionId)
				close(sensorStopChans[subscriptionId])
				delete(sensorStopChans, subscriptionId)
			}

		}
		time.Sleep(5 * time.Second)
	}
}

func newSensor(conn stan.Conn, selector event.Selector, subscriptionId string) chan interface{} {
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
			default:
				ev, ok := stream.Next()
				if !ok {
					fmt.Fprint(os.Stderr, "Failed to get next event.")
					continue sendLoop
				}
				log.Println("Received event:", ev)

				data, err := proto.Marshal(ev.(*event.Event))
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to marshal event data: %v\n", err)
				}
				conn.Publish(fmt.Sprintf("event.%s", subscriptionId), data)
			}
		}
	}()

	return stopChan
}
