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
	"github.com/golang/glog"
)

func TestGetEvents(t *testing.T) {
	config.Sensor.TelemetryServiceBindAddress =
		fmt.Sprintf("127.0.0.1:%d", 49152+rand.Intn(16384))

	s, err := CreateSensor()
	if err != nil {
		glog.Fatal("Error creating sensor:", err)
	}
	err = s.Start()
	if err != nil {
		glog.Fatal("Error starting sensor:", err)
	}
	defer func() {
		s.Shutdown()
		s.Wait()
	}()
	stopSignal := make(chan interface{})

	conn, _ := grpc.Dial(config.Sensor.TelemetryServiceBindAddress,
		grpc.WithInsecure())

	go func() {
		<-stopSignal
		conn.Close()
	}()
	c := api.NewTelemetryServiceClient(conn)

	var stream api.TelemetryService_GetEventsClient
	for {
		sub := &api.Subscription{
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
		}

		stream, err = c.GetEvents(context.Background(),
			&api.GetEventsRequest{
				Subscription: sub,
			})

		if err == nil {
			break
		}
	}

	events := make(chan *api.TelemetryEvent)
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
