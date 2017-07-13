package main

import (
	"log"
	"os"
	"testing"
	"time"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/config"
	"github.com/capsule8/reactive8/pkg/pubsub/mock"
)

func TestMain(m *testing.M) {
	config.Sensor.Pubsub = "mock"

	os.Exit(m.Run())
}

// TestCreateSubscription tests for the successful creation of a subscription over NATS.
// It verifies sub creation by ensuring the delivery of a single message over the sub STAN channel.
func TestCreateSubscription(t *testing.T) {
	mock.SetMockReturn("subscription.*", &api.SignedSubscription{
		Subscription: &api.Subscription{
			EventFilter: &api.EventFilter{
				ChargenEvents: []*api.ChargenEventFilter{
					&api.ChargenEventFilter{
						Length: 1,
					},
				},
			},

			Modifier: &api.Modifier{
				Throttle: &api.ThrottleModifier{
					Interval:     1,
					IntervalType: 0,
				},
			},
		},
		SubscriptionId: "SOMEID",
	})

	s, err := CreateSensor()
	if err != nil {
		log.Fatal("Error creating sensor:", err)
	}
	stopSignal, err := s.Start()
	if err != nil {
		log.Fatal("Error starting sensor:", err)
	}

	msgs := make(chan *mock.OutboundMessage)
	go func() {
	getMessageLoop:
		for {
			select {
			case <-stopSignal:
				break getMessageLoop
			default:
				if len(mock.GetOutboundMessages("event.SOMEID")) > 0 {
					// We only care about getting a single event here
					msgs <- &mock.GetOutboundMessages("event.SOMEID")[0]
				}
				time.Sleep(10 * time.Millisecond)
			}
		}

	}()

	select {
	case <-time.After(3 * time.Second):
		t.Error("Receive msg timeout")
	case ev := <-msgs:
		t.Log(ev.Topic)
		t.Log("Recevied message:", ev)
	}

	close(stopSignal)
	// Clear mock values after we're done
	mock.ClearMockValues()
}

// TestDiscover tests the discovery broadcast functionality in the sensor
func TestDiscover(t *testing.T) {
	s, err := CreateSensor()
	if err != nil {
		log.Fatal("Error creating sensor:", err)
	}
	stopSignal, err := s.Start()
	if err != nil {
		log.Fatal("Error starting sensor:", err)
	}

	msgs := make(chan *mock.OutboundMessage)
	go func() {
	getMessageLoop:
		for {
			select {
			case <-stopSignal:
				break getMessageLoop
			default:
				if len(mock.GetOutboundMessages("discover.sensor")) > 0 {
					// We only care about getting a single event here
					msgs <- &mock.GetOutboundMessages("discover.sensor")[0]
				}
				time.Sleep(10 * time.Millisecond)
			}
		}

	}()

	select {
	case <-time.After(3 * time.Second):
		t.Error("Receive msg timeout")
	case ev := <-msgs:
		t.Log("Recevied message:", ev)
	}

	close(stopSignal)
	// Clear mock values after we're done
	mock.ClearMockValues()
}
