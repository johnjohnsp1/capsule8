// Copyright 2017 Capsule8, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package functional

import (
	"sync"
	"testing"
	"time"

	api "github.com/capsule8/capsule8/api/v0"
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
	// Build the container used for testing. Returns the imageID
	// of the container image or the empty string to not filter
	// telemetry to containers running the built image.
	BuildContainer(t *testing.T) string

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
	imageID   string
}

func NewTelemetryTester(tt TelemetryTest) *TelemetryTester {
	return &TelemetryTester{test: tt}
}

func (tt *TelemetryTester) buildContainer(t *testing.T) {
	tt.imageID = tt.test.BuildContainer(t)

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

	sub := tt.test.CreateSubscription(t)

	// Subscribing to container created events are currently
	// necessary to get imageIDs in other events. Make sure that
	// the subscription includes it until this is fixed.
	sub.EventFilter.ContainerEvents = append(sub.EventFilter.ContainerEvents,
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED,
		},
	)

	// If buildContainer returned an ImageID, restrict events to
	// containers running that image
	if len(tt.imageID) > 0 {
		containerFilter := &api.ContainerFilter{}
		containerFilter.ImageIds = append(containerFilter.ImageIds, tt.imageID)
		sub.ContainerFilter = containerFilter
	}

	//
	// Connect to telemetry service first
	//
	c := api.NewTelemetryServiceClient(conn)
	stream, err := c.GetEvents(ctx, &api.GetEventsRequest{
		Subscription: sub,
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
