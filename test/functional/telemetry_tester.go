package functional

import (
	"sync"
	"testing"
	"time"

	api "github.com/capsule8/api/v0"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const telemetryTestTimeout = 10 * time.Second

var (
	conn     *grpc.ClientConn
	connOnce sync.Once
)

type TelemetryTest interface {
	// Build the container used for testing.
	BuildContainer(t *testing.T)

	// Run the container used for testing.
	RunContainer(t *testing.T)

	// create and return telemetry subscription to use for the test
	CreateSubscription(t *testing.T) *api.Subscription

	// return true to keep going, false if done
	HandleTelemetryEvent(t *testing.T, te *api.TelemetryEvent) bool
}

type TelemetryTester struct {
	test      TelemetryTest
	err       error
	waitGroup sync.WaitGroup
}

func NewTelemetryTester(tt TelemetryTest) *TelemetryTester {
	return &TelemetryTester{test: tt}
}

func (tt *TelemetryTester) buildContainer(t *testing.T) {
	tt.test.BuildContainer(t)
}

func (tt *TelemetryTester) runTelemetryTest(t *testing.T) {
	//
	// Dial the sensor and allow each test to set up their own subscriptions
	// over the same gRPC connection
	//
	connOnce.Do(func() {
		var err error
		conn, err = apiConn()
		if err != nil {
			glog.Fatal(err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(),
		telemetryTestTimeout)

	//
	// Connect to telemetry service first
	//
	subscription := tt.test.CreateSubscription(t)

	c := api.NewTelemetryServiceClient(conn)
	stream, err := c.GetEvents(ctx, &api.GetEventsRequest{
		Subscription: tt.test.CreateSubscription(t),
	})
	if err != nil {
		t.Error(err)
		return
	}

	receiverStarted := sync.WaitGroup{}
	receiverStarted.Add(1)

	tt.waitGroup.Add(1)
	go func() {
		// There is no deterministic way to sychronously know
		// when the telemetry subscription is active, so we
		// need a decent sized sleep here.
		time.Sleep(1 * time.Second)
		receiverStarted.Done()

		for {
			response, err := stream.Recv()
			if err != nil {
				tt.err = err
				tt.waitGroup.Done()
				return
			}

			for _, telemetryEvent := range response.Events {
				if !tt.test.HandleTelemetryEvent(t, telemetryEvent) {
					cancel()
					tt.waitGroup.Done()
					return
				}
			}
		}
	}()

	tt.waitGroup.Add(1)
	go func() {
		// Wait for receiver goroutine to have started before starting
		// starting container
		receiverStarted.Wait()

		tt.test.RunContainer(t)
		tt.waitGroup.Done()
	}()

	tt.waitGroup.Wait()

	if tt.err != nil {
		t.Error(tt.err)
	}
}

func (tt *TelemetryTester) RunTest(t *testing.T) {
	if !t.Run("buildContainer", tt.buildContainer) {
		t.Error("Couldn't build container")
		return
	}

	if !t.Run("runTelemetryTest", tt.runTelemetryTest) {
		t.Error("Couldn't run telemetry tests")
		return
	}
}
