package sensor

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

	glog.V(0).Infof("GetEvents(%+v)", sub)

	eventStream, err := subscription.NewSubscription(sub)
	if err != nil {
		glog.Errorf("Failed to get events for subscription %+v: %s",
			sub, err.Error())
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
			glog.V(0).Infof("Client disconnected, closing stream")
			eventStream.Close()
			break sendLoop
		}
	}
	return nil
}
