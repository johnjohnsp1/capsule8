package main

import (
	"net"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/config"
	pbsensor "github.com/capsule8/reactive8/pkg/sensor"
	telemetry "github.com/capsule8/reactive8/pkg/sensor/telemetry"
	"github.com/golang/glog"
	"google.golang.org/grpc"
)

func startTelemetryService(s *sensor) {
	g := grpc.NewServer()
	t := &telemetryServiceServer{
		s: s,
	}
	telemetry.RegisterTelemetryServiceServer(g, t)
	var err error
	lis, err := net.Listen("tcp", config.Sensor.TelemetryServiceBindAddress)
	if err != nil {
		// We should probably give up if we can't start this.
		glog.Fatal("Failed to start local telemetry service:", err)
	}

	go func() {
		s.wg.Add(1)
		defer s.wg.Done()

		<-s.stopChan
		g.GracefulStop()
	}()
	// Serve requests until the server is stopped.
	go func() {
		g.Serve(lis)
	}()
}

type telemetryServiceServer struct {
	s *sensor
}

func (t *telemetryServiceServer) GetEvents(sub *api.Subscription, stream telemetry.TelemetryService_GetEventsServer) error {
	eventStream, err := pbsensor.NewSensor(sub)
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
		err = stream.Send(&telemetry.GetEventsResponse{
			Events: []*api.Event{
				ev.(*api.Event),
			},
		})
		// Client d/c'ed
		if err != nil {
			pbsensor.Remove(sub)
			eventStream.Close()
			break sendLoop
		}
	}
	return nil
}
