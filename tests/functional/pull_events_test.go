package main

import (
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
	sub := &event.Subscription{
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
	}
	b, _ := proto.Marshal(sub)
	_, err = c.Publish(context.Background(), &pubsub.PublishRequest{
		Topic: "subscription.SOMEID",
		Messages: [][]byte{
			b,
		},
	})

	// Stream subscription
	stream, err := c.Pull(context.Background())
	if err != nil {
		t.Error("Failed to create receive event stream:", err)
	}

	stream.Send(&pubsub.PullRequest{
		Topic: "event.SOMEID",
	})

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
		stream.Send(&pubsub.PullRequest{
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
