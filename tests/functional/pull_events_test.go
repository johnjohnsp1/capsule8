package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/api/pubsub"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
)

func TestPullEvents(t *testing.T) {
	conn, err := grpc.Dial("localhost:8080", grpc.WithInsecure())
	if err != nil {
		t.Error("Failed to connect to grpc server:", err)
	}

	c := pubsub.NewPubsubServiceClient(conn)

	// Create a subscription first
	sub := &event.SignedSubscription{
		Subscription: &event.Subscription{
			Selector: &event.Selector{
				Chargen: &event.ChargenEventSelector{
					Length: 1,
				},
			},
			Modifier: &event.Modifier{
				Throttle: &event.ThrottleModifier{
					Interval:     1,
					IntervalType: 0,
				},
			},
		},
	}
	b, _ := proto.Marshal(sub)
	h := sha256.New()
	h.Write(b)
	subID := fmt.Sprintf("%x", h.Sum(nil))
	_, err = c.Publish(context.Background(), &pubsub.PublishRequest{
		Topic: fmt.Sprintf("subscription.%s", subID),
		Messages: [][]byte{
			b,
		},
	})

	// Stream subscription
	stream, err := c.Pull(context.Background(), &pubsub.PullRequest{
		Topic: fmt.Sprintf("event.%s", subID),
	})
	if err != nil {
		t.Error("Failed to create receive event stream:", err)
	}

	// Attempt to collect 1 message
	msgs := make(chan interface{})
	go func() {
		resp, err := stream.Recv()
		if err != nil {
			t.Error("Error receiving messages: ", err)
		}
		log.Println("Received messages: ", resp.Messages)

		for msg := range resp.Messages {
			msgs <- msg
		}

		acks := make([][]byte, len(resp.Messages))
		for i, msg := range resp.Messages {
			acks[i] = msg.Ack
		}
		c.Acknowledge(context.Background(), &pubsub.AcknowledgeRequest{
			Acks: acks,
		})
	}()

	select {
	case <-time.After(3 * time.Second):
		t.Error("Receive events timeout")
	case <-msgs:
		t.Log("Successfully received event")
	}
}
