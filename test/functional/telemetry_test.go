package functional

import (
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"

	api "github.com/capsule8/api/v0"
)

const telemetryTestTimeout = 10 * time.Second

type TelemetryTest interface {
	// Build the container used for testing.
	buildContainer(t *testing.T)

	// Run the container used for testing.
	runContainer(t *testing.T)

	// create and return telemetry subscription to use for the test
	createSubscription(t *testing.T) *api.Subscription

	// return true to keep going, false if done
	handleTelemetryEvent(t *testing.T, te *api.TelemetryEvent) bool
}

type telemetryTest struct {
	test      TelemetryTest
	err       error
	waitGroup sync.WaitGroup
}

func newTelemetryTest(tt TelemetryTest) *telemetryTest {
	return &telemetryTest{test: tt}
}

func (tt *telemetryTest) buildContainer(t *testing.T) {
	tt.test.buildContainer(t)
}

func (tt *telemetryTest) runTelemetryTest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(),
		telemetryTestTimeout)

	//
	// Connect to telemetry service first
	//
	c := api.NewTelemetryServiceClient(sensorConn)
	stream, err := c.GetEvents(ctx, &api.GetEventsRequest{
		Subscription: tt.test.createSubscription(t),
	})
	if err != nil {
		t.Error(err)
		return
	}

	receiverStarted := sync.WaitGroup{}
	receiverStarted.Add(1)

	tt.waitGroup.Add(1)
	go func() {
		receiverStarted.Done()

		for {
			response, err := stream.Recv()
			if err != nil {
				tt.err = err
				tt.waitGroup.Done()
				return
			}

			for _, telemetryEvent := range response.Events {
				if !tt.test.handleTelemetryEvent(t, telemetryEvent) {
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

		tt.test.runContainer(t)
		tt.waitGroup.Done()
	}()

	tt.waitGroup.Wait()

	if tt.err != nil {
		t.Error(tt.err)
	}
}

func (tt *telemetryTest) runTest(t *testing.T) {
	if !t.Run("buildContainer", tt.buildContainer) {
		t.Error("Couldn't build container")
		return
	}

	if !t.Run("runTelemetryTest", tt.runTelemetryTest) {
		t.Error("Couldn't run telemetry tests")
		return
	}
}
