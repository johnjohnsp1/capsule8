package main

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/config"
	telemetry "github.com/capsule8/reactive8/pkg/sensor/telemetry"
	"github.com/golang/glog"
)

func TestGetEvents(t *testing.T) {
	config.Sensor.TelemetryServiceBindAddress = fmt.Sprintf("127.0.0.1:%d", 49152+rand.Intn(16384))

	s, err := CreateSensor()
	if err != nil {
		glog.Fatal("Error creating sensor:", err)
	}
	stopSignal, err := s.Start()
	if err != nil {
		glog.Fatal("Error starting sensor:", err)
	}

	conn, _ := grpc.Dial(config.Sensor.TelemetryServiceBindAddress, grpc.WithInsecure())
	c := telemetry.NewTelemetryServiceClient(conn)

	var stream telemetry.TelemetryService_GetEventsClient
	for {
		stream, err = c.GetEvents(context.Background(), &api.Subscription{
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
		})
		if err == nil {
			break
		}
	}

	events := make(chan *api.Event)
	go func() {
	getMessageLoop:
		for {
			select {
			case <-stopSignal:
				break getMessageLoop
			default:
				resp, err := stream.Recv()
				if err != nil {
					break getMessageLoop
				}
				for _, ev := range resp.Events {
					events <- ev
				}
			}
		}

	}()

	select {
	case <-time.After(3 * time.Second):
		t.Error("Receive msg timeout")
	case ev := <-events:
		t.Log("Recevied message:", ev)
	}

	close(stopSignal)
}
