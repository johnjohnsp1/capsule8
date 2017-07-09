// Copyright 2017 Capsule8 Inc. All rights reserved.

package pubsub

import (
	api "github.com/capsule8/reactive8/pkg/api/v0"
)

// Backend interface to the internal pubsub service.
type Backend interface {
	// Connect to the pubsub service
	// NOTE: Each pubsub backend will expect different env variable(s)
	Connect() error
	// Publish to topic
	// ARGS:
	//      topic (string)
	//      message (interface{})
	// RETURN:
	//      error
	Publish(string, interface{}) error
	// Pulls messages from topic
	// ARGS:
	//      topic (string)
	// RETURN:
	//      subscription (Subscription)
	//      messages (<-chan *ReceivedMessage)
	//      message acknowledgements (chan<- []byte)
	//      error
	Pull(string) (Subscription, <-chan *api.ReceivedMessage, error)
	// Acknowledge one or more acks
	// ARGS:
	//      acks ([][]byte)
	// RETURN:
	//      failed acks ([][]byte)
	//      error
	Acknowledge([][]byte) ([][]byte, error)
}

// Subscription interface for managing an open subscription
type Subscription interface {
	// Close the subscription
	Close() error
}
