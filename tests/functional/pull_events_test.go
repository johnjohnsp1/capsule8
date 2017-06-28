package main

import (
	"os"
	"testing"
	"time"

	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/client"
)

var c client.Client

func TestMain(m *testing.M) {
	c, _ = client.CreateClient("localhost:8080")
	os.Exit(m.Run())
}

func TestPullEvents(t *testing.T) {
	ss, err := c.CreateTelemetrySubscription(&event.Subscription{
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
	})
	if err != nil {
		t.Error("Failed to create signed subscription:", err)
	}

	events, closeSignal, err := c.GetTelemetryEvents(ss)
	if err != nil {
		t.Error("Failed to get telemetry events:", err)
	}

	select {
	case <-time.After(3 * time.Second):
		t.Error("Receive events timeout")
	case _, ok := <-events:
		if !ok {
			t.Error("Failed to receive event")
		} else {
			t.Log("Successfully received event")
			close(closeSignal)
		}
	}
}
