package main

import (
	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/subscription"
	"github.com/golang/glog"
)

type telemetryServiceServer struct {
	s *Sensor
}

func (t *telemetryServiceServer) GetEvents(req *api.GetEventsRequest, stream api.TelemetryService_GetEventsServer) error {
	sub := req.Subscription
	eventStream, err := subscription.NewSubscription(sub)
	if err != nil {
		glog.Errorf("failed to get events: %s\n", err.Error())
		return err
	}

sendLoop:
	for {
		ev, ok := <-eventStream.Data
		if !ok {
			return err
		}

		// Send back events right away
		te := &api.TelemetryEvent{
			Event: ev.(*api.Event),
		}

		err = stream.Send(&api.GetEventsResponse{
			Events: []*api.TelemetryEvent{
				te,
			},
		})

		// Client d/c'ed
		if err != nil {
			eventStream.Close()
			break sendLoop
		}
	}
	return nil
}
