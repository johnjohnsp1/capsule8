package main

import (
	"log"
	"os"
	"testing"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	"github.com/capsule8/reactive8/pkg/api/pubsub"
)

var c pubsub.PubsubServiceClient

func TestMain(m *testing.M) {
	conn, _ := grpc.Dial("localhost:8080", grpc.WithInsecure())
	c = pubsub.NewPubsubServiceClient(conn)
	os.Exit(m.Run())
}

func TestPublish(t *testing.T) {
	pubResp, err := c.Publish(context.Background(), &pubsub.PublishRequest{
		Topic: "pubsub.test-publish",
		Messages: [][]byte{
			[]byte("HELLO1"),
			[]byte("HELLO2"),
			[]byte("HELLO3"),
		},
	})
	if err != nil {
		t.Error("Unable to publish message:", err)
	}

	if len(pubResp.FailedMessages) > 0 {
		t.Error(len(pubResp.FailedMessages), "messages failed to send")
	} else {
		t.Log("All messages published successfully.")
	}
}

func TestPull(t *testing.T) {
	stream, err := c.Pull(context.Background(), &pubsub.PullRequest{
		Topic: "pubsub.test-pull",
	})
	if err != nil {
		t.Error("Pull request failed:", err)
	}

	done := make(chan interface{})
	total := 3
	go func() {
		for {
			select {
			case <-done:
				break
			default:
				time.Sleep(1 * time.Millisecond)
				for i := 0; i < total; i++ {
					log.Println("Sending message")
					c.Publish(context.Background(), &pubsub.PublishRequest{
						Topic: "pubsub.test-pull",
						Messages: [][]byte{
							[]byte("Jello"),
						},
					})
				}
			}
		}
	}()

	count := 0
	var acks [][]byte
	for {
		if count == total {
			break
		}
		msgs, err := stream.Recv()
		if err != nil {
			t.Error("Error receiving msgs:", err)
		} else {
			for _, msg := range msgs.Messages {
				log.Println("Message received!")
				acks = append(acks, msg.Ack)
				count++
			}
		}
	}

	// Send back acks for all of the messages received
	c.Acknowledge(context.Background(), &pubsub.AcknowledgeRequest{
		Acks: acks,
	})
	close(done)
	log.Println("Successfully recevied msges")
}
