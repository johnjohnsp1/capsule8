package main

import (
	"context"
	"testing"
	"time"

	"github.com/capsule8/reactive8/pkg/api/event"
	"google.golang.org/grpc"
)

func TestReceiveEvents(t *testing.T) {
	conn, err := grpc.Dial("localhost:8080", grpc.WithInsecure())
	if err != nil {
		t.Error("Failed to connect to grpc server:", err)
	}

	c := event.NewEventServiceClient(conn)

	stream, err := c.ReceiveEvents(context.Background())
	if err != nil {
		t.Error("Failed to create receive event stream:", err)
	}

	// Send a subscription request first
	stream.Send(&event.ReceiveEventsRequest{
		Subscription: &event.Subscription{
			Selector: &event.Selector{
				Chargen: &event.ChargenEventSelector{
					Length: 1,
				},
			},
			Modifier: &event.Modifier{
				Limit: &event.LimitModifier{
					Limit: 1,
				},
			},
		},
	})

	// Attempt to collect 1 message
	msgs := make(chan interface{})
	go func() {
		ev, err := stream.Recv()
		if err != nil {
			t.Error("Error receiving events: ", err)
		}
		t.Log("Received event: ", ev)

		msgs <- ev

		stream.Send(&event.ReceiveEventsRequest{
			Acks: [][]byte{
				ev.Ack,
			},
		})
	}()

	select {
	case <-time.After(3 * time.Second):
		t.Error("Receive events timeout")
	case <-msgs:
		t.Log("Successfully received event")
	}
}
