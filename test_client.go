package main

import (
	"io"
	"log"

	"golang.org/x/net/context"

	event "github.com/capsule8/reactive8/pkg/api/event"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

func main() {
	log.Println("[TEST-CLIENT] starting up")
	conn, err := grpc.Dial("localhost:8080", grpc.WithInsecure())
	if err != nil {
		grpclog.Fatal("Failed to connect to grpc server:", err)
	}

	c := event.NewEventServiceClient(conn)

	stream, err := c.ReceiveEvents(context.Background())
	if err != nil {
		grpclog.Fatal("Failed to create receive event stream:", err)
	}

	log.Println("[TEST-CLIENT] sending subscription request")
	// Send a subscription request first
	stream.Send(&event.ReceiveEventsRequest{
		Subscription: &event.Subscription{
			Selector: &event.Selector{
				Chargen: &event.ChargenEventSelector{
					Length: 1,
				},
			},
		},
	})

	log.Println("[TEST-CLIENT] entering receive loop")
	for {
		ev, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal("Error receiving events: ", err)
		}
		log.Println("Received event: ", ev)

		stream.Send(&event.ReceiveEventsRequest{
			Acks: []*event.Ack{
				ev.Ack,
			},
		})
	}
}
