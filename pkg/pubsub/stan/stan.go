// Copyright 2017 Capsule8 Inc. All rights reserved.

package stan

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"time"

	api "github.com/capsule8/api/v0"
	backend "github.com/capsule8/reactive8/pkg/pubsub"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	nats "github.com/nats-io/go-nats"
	stan "github.com/nats-io/go-nats-streaming"
	npb "github.com/nats-io/go-nats-streaming/pb"
	uuid "github.com/satori/go.uuid"

	"github.com/capsule8/reactive8/pkg/config"
)

// Errors
var (
	ErrInvalidMessageType  = func(err string) error { return fmt.Errorf("invalid message type %s", err) }
	ErrNoSubscriptionFound = errors.New("no subscription found")
)

// Backend is actually both STAN/NATS backends
type Backend struct {
	stanConn stan.Conn
	natsConn *nats.Conn
}

// Connect backend to STAN/NATS cluster(s)
func (sb *Backend) Connect() error {
	var err error

	if sb.stanConn, err = stan.Connect(config.Backplane.ClusterName, uuid.NewV4().String(), stan.NatsURL(config.Backplane.NatsURL)); err != nil {
		glog.Errorf("Failed to connect to STAN server: %v\n", err.Error())
		return err
	}

	if sb.natsConn, err = nats.Connect(config.Backplane.NatsURL); err != nil {
		glog.Errorf("Failed to connect to NATS server: %v\n", err.Error())
		return err
	}

	return nil
}

// Publish a known message type to a topic
func (sb *Backend) Publish(topic string, message interface{}) error {
	switch message.(type) {
	case *api.Discover:
		payload := message.(*api.Discover)
		bytes, err := proto.Marshal(payload)
		if err != nil {
			return err
		}
		if err = sb.natsConn.Publish(topic, bytes); err != nil {
			return err
		}
	case *api.Subscription:
		payload := message.(*api.Subscription)
		bytes, err := proto.Marshal(payload)
		if err != nil {
			return err
		}
		if err = sb.natsConn.Publish(topic, bytes); err != nil {
			return err
		}
	case *api.Config:
		payload := message.(*api.Config)
		bytes, err := proto.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err = sb.stanConn.PublishAsync(topic, bytes, func(_ string, _ error) {}); err != nil {
			return err
		}
	case []byte:
		// Publish arbitrary bytes to the specified topic
		bytes := message.([]byte)
		if _, err := sb.stanConn.PublishAsync(topic, bytes, func(_ string, _ error) {}); err != nil {
			return err
		}
	default:
		// Message must be one of the types above
		return ErrInvalidMessageType(fmt.Sprintf("%v", reflect.TypeOf(message)))
	}

	return nil
}

// Pull messages off of a topic
func (sb *Backend) Pull(topic string) (backend.Subscription, <-chan *api.ReceivedMessage, error) {
	// Return one channel for receiving messages
	messages := make(chan *api.ReceivedMessage)
	// Return a subscription object for managing subscriptions
	sub := &subscription{}

	// Check for topics that need special treatment
	maybeDiscover := regexp.MustCompile(`discover\..*`)
	maybeConfig := regexp.MustCompile(`config\..*`)
	maybeSubscription := regexp.MustCompile(`subscription\..*`)
	//maybeEvents := regexp.MustCompile(`events\..*`)

	switch {
	case maybeConfig.MatchString(topic):
		// We send EVERY message sitting in the channel for topic `config.*`
		stanSub, err := sb.stanSubscribe(topic, messages, stan.DeliverAllAvailable())
		if err != nil {
			return sub, messages, err
		}
		sub.stanSub = stanSub
	case (maybeSubscription.MatchString(topic) ||
		maybeDiscover.MatchString(topic)):
		natsSub, err := sb.natsSubscribe(topic, messages)
		if err != nil {
			return sub, messages, err
		}
		sub.natsSub = natsSub
	//case maybeEvents.MatchString(topic):
	// TODO: We will probably use an (in memory) STAN cluster for handling telemetry events
	default:
		stanSub, err := sb.stanSubscribe(topic, messages)
		if err != nil {
			return sub, messages, err
		}
		sub.stanSub = stanSub
	}

	return sub, messages, nil
}

// Acknowledge all raw acks
func (sb *Backend) Acknowledge(acks [][]byte) ([][]byte, error) {
	var failedAcks [][]byte
ackLoop:
	for _, ackBytes := range acks {
		ack := &api.Ack{}
		if err := proto.Unmarshal(ackBytes, ack); err != nil {
			glog.Errorf("Unable to marshal ack: %s\n", err.Error())
			failedAcks = append(failedAcks, ackBytes)
			// We don't want to give up here if we get an error
			continue ackLoop
		}
		pback := &npb.Ack{Subject: ack.Subject, Sequence: ack.Sequence}
		b, err := pback.Marshal()
		if err != nil {
			glog.Errorf("Unable to marshal ack: %s\n", err.Error())
			continue ackLoop
		}
		if err = sb.natsConn.Publish(ack.Inbox, b); err != nil {
			glog.Errorf("Unable to publish ack: %s\n", err.Error())
			failedAcks = append(failedAcks, ackBytes)
		}
	}

	return failedAcks, nil
}

func (sb *Backend) natsSubscribe(topic string, messages chan *api.ReceivedMessage) (*nats.Subscription, error) {
	sub, err := sb.natsConn.Subscribe(topic, func(m *nats.Msg) {
		messages <- &api.ReceivedMessage{
			Payload: m.Data,
		}
	})
	if err != nil {
		return sub, err
	}
	return sub, nil
}

func (sb *Backend) stanSubscribe(topic string, messages chan *api.ReceivedMessage, options ...stan.SubscriptionOption) (stan.Subscription, error) {
	var ackInbox string

	// By default, we deliver messages off of a stan channel
	// from when the subscriber subscribes
	options = append(options, stan.SetManualAckMode(), stan.AckWait(time.Duration(config.Backplane.AckWait)*time.Second))
	stanSub, err := sb.stanConn.Subscribe(topic, func(m *stan.Msg) {
		if ackInbox == "" {
			ackInbox = reflect.ValueOf(m.Sub).Elem().FieldByName("ackInbox").String()
		}
		ack := &api.Ack{
			Inbox:    ackInbox,
			Subject:  m.Subject,
			Sequence: m.Sequence,
		}
		ackBytes, err := proto.Marshal(ack)
		if err != nil {
			glog.Errorf("Failed to convert ack bytes: %s\n", err.Error())
		}

		// Pass the messages along
		messages <- &api.ReceivedMessage{
			Payload:           m.Data,
			Ack:               ackBytes,
			PublishTimeMicros: m.Timestamp,
		}

	}, options...)
	if err != nil {
		return nil, err
	}
	return stanSub, nil
}

// Subscription object wrapping a nats or stan subscription
type subscription struct {
	stanSub stan.Subscription
	natsSub *nats.Subscription
}

// Close cleans up a subscription
func (s *subscription) Close() error {
	if s.stanSub != nil {
		// Close retains durable subscriptions.
		// This way, a client can d/c -> r/c to resume their durable sub.
		return s.stanSub.Close()
	}
	if s.natsSub != nil {
		return s.natsSub.Unsubscribe()
	}
	// We always set the inner subscription but just in case.
	return ErrNoSubscriptionFound
}
