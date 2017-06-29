package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Set test env variables
	os.Setenv("TESTSENSOR_PUBSUB", "mock") // Specify mock pubsub backend
	os.Exit(m.Run())
}

// TestCreateSubscription tests for the successful creation of a subscription over NATS.
// It verifies sub creation by ensuring the delivery of a single message over the sub STAN channel.
//func TestCreateSubscription(t *testing.T) {
//	mock.SetMockReturn("subscription.*", &event.SignedSubscription{
//		Subscription: &event.Subscription{
//			EventFilter: &event.EventFilter{
//				ChargenEvents: []*event.ChargenEventFilter{
//					&event.ChargenEventFilter{
//						Length: 1,
//					},
//				},
//			},
//
//			Modifier: &event.Modifier{
//				Throttle: &event.ThrottleModifier{
//					Interval:     1,
//					IntervalType: 0,
//				},
//			},
//		},
//	})
//
//	s, err := CreateSensor("testsensor")
//	if err != nil {
//		log.Fatal("Error creating sensor:", err)
//	}
//	stopSignal, err := s.Start()
//	if err != nil {
//		log.Fatal("Error starting sensor:", err)
//	}
//
//	msgs := make(chan interface{})
//	go func() {
//	getMessageLoop:
//		for {
//			select {
//			case <-stopSignal:
//				break getMessageLoop
//			default:
//				if len(mock.GetOutboundMessages()) > 0 {
//					// We only care about getting a single event here
//					msgs <- mock.GetOutboundMessages()[0]
//				}
//				time.Sleep(10 * time.Millisecond)
//			}
//		}
//
//	}()
//
//	select {
//	case <-time.After(3 * time.Second):
//		t.Error("Receive msg timeout")
//	case ev := <-msgs:
//		t.Log("Recevied message:", ev)
//	}
//
//	close(stopSignal)
//	// Clear mock values after we're done
//	mock.ClearMockValues()
//}
