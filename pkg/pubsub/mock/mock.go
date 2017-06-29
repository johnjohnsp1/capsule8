// Copyright 2017 Capsule8 Inc. All rights reserved.

package mock

import (
	"fmt"
	"reflect"
	"regexp"
	"time"

	pbconfig "github.com/capsule8/reactive8/pkg/api/config"
	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/api/pubsub"
	backend "github.com/capsule8/reactive8/pkg/pubsub"
	"github.com/golang/protobuf/proto"
)

var (
	maybeSubscription = regexp.MustCompile(`subscription\..*`)
	maybeEvent        = regexp.MustCompile(`event\..*`)
	mockTopics        = make(map[string]interface{})
	outboundMessages  = []*OutboundMessage{}
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
	case *event.SignedSubscription:
		// do nothing
	case *pbconfig.Config:
		// do nothing
	case []byte:
		// do nothing
	default:
		// Message must be one of the types above
		return fmt.Errorf("Message is of unknown type %s", reflect.TypeOf(message))
	}
	// Append the message to outbound messages
	outboundMessages = append(outboundMessages, &OutboundMessage{
		Topic: topic,
		Value: message,
	})
	return nil
}

// Pull messages off of a topic
func (sb *Backend) Pull(topic string) (backend.Subscription, <-chan *pubsub.ReceivedMessage, error) {
	// Return one channel for receiving messages
	messages := make(chan *pubsub.ReceivedMessage)

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
				break sendLoop
			default:
				msg := &pubsub.ReceivedMessage{
					Ack: []byte("mock ack"),
				}
				// Check to see if the topic is configured to emit a mock event
				if ev, ok := mockTopics[topic]; ok {
					switch {
					case maybeSubscription.MatchString(topic):
						payload := ev.(*event.SignedSubscription)
						b, _ := proto.Marshal(payload)
						msg.Payload = b
					case maybeEvent.MatchString(topic):
						payload := ev.(*event.Event)
						b, _ := proto.Marshal(payload)
						msg.Payload = b
					}
				}
				messages <- msg
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
func GetOutboundMessages() []*OutboundMessage {
	// TODO: If we want to concurrently remove and add to `outboundMessages`,
	// we will need a lock at some point
	return outboundMessages
}

// SetMockReturn is an extra method for setting mock return values
func SetMockReturn(topic string, value interface{}) {
	mockTopics[topic] = value
}

// ClearMockValues clears mock values
func ClearMockValues() {
	mockTopics = make(map[string]interface{})
	outboundMessages = []*OutboundMessage{}
}
