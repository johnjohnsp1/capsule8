// Copyright 2017 Capsule8 Inc. All rights reserved.

package mock

import (
	"fmt"
	"log"
	"reflect"
	"regexp"
	"sync"
	"time"

	api "github.com/capsule8/reactive8/pkg/api/v0"
	backend "github.com/capsule8/reactive8/pkg/pubsub"
	"github.com/golang/protobuf/proto"
)

var (
	maybeConfig       = regexp.MustCompile(`config\..*`)
	maybeSubscription = regexp.MustCompile(`subscription\..*`)
	maybeEvent        = regexp.MustCompile(`event\..*`)
	mockTopics        = make(map[string]interface{})
	outboundMessages  = map[string][]*OutboundMessage{}
	mu                = &sync.RWMutex{}
)

// Backend is a mock backend
type Backend struct{}

// OutboundMessage records outgoing messages
type OutboundMessage struct {
	Topic string
	Value interface{}
}

// Connect backend to nothing
func (sb *Backend) Connect() error {
	return nil
}

// Publish a known message type to a topic
func (sb *Backend) Publish(topic string, message interface{}) error {
	switch message.(type) {
	case *api.SignedSubscription:
		// do nothing
	case *api.Config:
		// do nothing
	case *api.Discover:
		// do nothing
	case []byte:
		// do nothing
	default:
		// Message must be one of the types above
		return fmt.Errorf("Message is of unknown type %s", reflect.TypeOf(message))
	}
	// Append the message to outbound messages
	outboundMessages[topic] = append(outboundMessages[topic], &OutboundMessage{
		Topic: topic,
		Value: message,
	})
	return nil
}

// Pull messages off of a topic
func (sb *Backend) Pull(topic string) (backend.Subscription, <-chan *api.ReceivedMessage, error) {
	// Return one channel for receiving messages
	messages := make(chan *api.ReceivedMessage)

	// Return a subscription object for managing subscriptions
	closeSignal := make(chan interface{})
	sub := &subscription{
		closeSignal: closeSignal,
	}

	// Send empty messages on a 1 millisecond interval
	go func() {
	sendLoop:
		for {
			select {
			case <-closeSignal:
				log.Println("Close signal received")
				break sendLoop
			default:
				msg := &api.ReceivedMessage{
					Ack: []byte("mock ack"),
				}
				// Check to see if the topic is configured to emit a mock event
				mu.RLock()
				if ev, ok := mockTopics[topic]; ok {
					switch {
					case maybeSubscription.MatchString(topic):
						payload := ev.(*api.SignedSubscription)
						b, _ := proto.Marshal(payload)
						msg.Payload = b
					case maybeEvent.MatchString(topic):
						payload := ev.(*api.Event)
						b, _ := proto.Marshal(payload)
						msg.Payload = b
					case maybeConfig.MatchString(topic):
						payload := ev.(*api.Config)
						b, _ := proto.Marshal(payload)
						msg.Payload = b
					}
				} else {
					// By default, goto next iteration if no mock return values specified
					goto nextIter
				}
				messages <- msg
			nextIter:
				mu.RUnlock()
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	return sub, messages, nil
}

// Acknowledge all raw acks
func (sb *Backend) Acknowledge(acks [][]byte) ([][]byte, error) {
	var failedAcks [][]byte
	// Do nothing
	return failedAcks, nil
}

// Subscription object wrapping a nats or stan subscription
type subscription struct {
	closeSignal chan interface{}
}

// Close cleans up a subscription
func (s *subscription) Close() error {
	close(s.closeSignal)
	return nil
}

// GetOutboundMessages grabs all outgoing messages
func GetOutboundMessages(topic string) []OutboundMessage {
	var msgs []OutboundMessage
	mu.RLock()
	defer mu.RUnlock()
	for _, msg := range outboundMessages[topic] {
		msgs = append(msgs, *msg)
	}
	// TODO: If we want to concurrently remove and add to `outboundMessages`,
	// we will need a lock at some point
	return msgs
}

// SetMockReturn is an extra method for setting mock return values
func SetMockReturn(topic string, value interface{}) {
	mu.Lock()
	defer mu.Unlock()
	mockTopics[topic] = value
}

// ClearMockValues clears mock values
func ClearMockValues() {
	mu.Lock()
	defer mu.Unlock()
	mockTopics = make(map[string]interface{})
	outboundMessages = map[string][]*OutboundMessage{}
}
