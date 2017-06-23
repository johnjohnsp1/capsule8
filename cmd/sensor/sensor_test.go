package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/gogo/protobuf/proto"
	nats "github.com/nats-io/go-nats"
	stan "github.com/nats-io/go-nats-streaming"
	"github.com/nats-io/nats-streaming-server/server"
	"github.com/nats-io/nats-streaming-server/test"
)

var NATS_PORT = 4223                        // Default NATS port is 4222, let's use 4223 to avoid conflicts
var STAN_CLUSTER_NAME = "test-c8-backplane" //  Not test cluster ID to avoid conflicts w/ "c8-backplane" default.

func TestMain(m *testing.M) {
	// Start STAN/NATS server for testing
	opts := server.GetDefaultOptions()
	opts.ID = STAN_CLUSTER_NAME
	natsOpts := server.DefaultNatsServerOptions
	natsOpts.Port = NATS_PORT
	server := test.RunServerWithOpts(opts, &natsOpts)
	defer server.Shutdown()

	// Set test env variables
	os.Setenv("STAN_NATSURL", fmt.Sprintf("nats://localhost:%d", NATS_PORT))
	os.Setenv("STAN_CLUSTERNAME", STAN_CLUSTER_NAME)

	LoadConfig("testsensor")
	// We don't need to remove stale subscriptions here so we're not calling `RemoveStaleSubscriptions`
	StartSensor()

	os.Exit(m.Run())
}

// TestCreateSubscription tests for the successful creation of a subscription over NATS.
// It verifies sub creation by ensuring the delivery of a single message over the sub STAN channel.
func TestCreateSubscription(t *testing.T) {
	nc, err := nats.Connect(Config.NatsURL)
	if err != nil {
		t.Error("Failed to connect to NATS:", err)
	}
	// Broadcast a subscription
	b, _ := proto.Marshal(&event.Subscription{
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
	nc.Publish("subscription.TESTID", b)

	sc, err := stan.Connect(Config.StanClusterName, "testsensor", stan.NatsURL(Config.NatsURL))
	if err != nil {
		t.Error("Failed to connect to STAN:", err)
	}

	msgs := make(chan interface{}, 1)
	// Fetch messages in a separate goroutine and pass msges into a chan which we can
	// use with a `select` to check for timeouts
	go func() {
		// Only one message so we don't need a for loop
		sc.Subscribe("event.TESTID", func(m *stan.Msg) {
			ev := &event.Event{}
			proto.Unmarshal(m.Data, ev)
			msgs <- ev
		})
	}()

	select {
	case <-time.After(3 * time.Second):
		t.Error("Receive msg timeout")
	case ev := <-msgs:
		t.Log("Recevied message:", ev)
	}
}
