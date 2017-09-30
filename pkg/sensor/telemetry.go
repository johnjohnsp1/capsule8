package sensor

import (
	api "github.com/capsule8/api/v0"
	"github.com/capsule8/capsule8/pkg/subscription"
	"github.com/golang/glog"
)

type telemetryServiceServer struct {
	s *Sensor
}

func (t *telemetryServiceServer) GetEvents(req *api.GetEventsRequest, stream api.TelemetryService_GetEventsServer) error {
	sub := req.Subscription

	glog.V(1).Infof("GetEvents(%+v)", sub)

	eventStream, err := subscription.NewSubscription(sub)
	if err != nil {
		glog.Errorf("Failed to get events for subscription %+v: %s",
			sub, err.Error())
		return err
	}

	go func() {
		<-stream.Context().Done()
		glog.V(1).Infof("Client disconnected, closing stream")
		eventStream.Close()
	}()

sendLoop:
	for {
		ev, ok := <-eventStream.Data
		if !ok {
			break sendLoop
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
		if err != nil {
			break sendLoop
		}
	}

	return nil
}
