package main

import (
	"net"
	"os"
	"path"
	"strings"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/config"
	"github.com/capsule8/reactive8/pkg/subscription"
	"github.com/golang/glog"
	"google.golang.org/grpc"
)

func startTelemetryService(s *sensor) {
	g := grpc.NewServer()
	t := &telemetryServiceServer{
		s: s,
	}
	api.RegisterTelemetryServiceServer(g, t)

	var err error
	var lis net.Listener

	parts := strings.Split(config.Sensor.ListenAddr, ":")
	if len(parts) > 1 && parts[0] == "unix" {
		socketPath := parts[1]
		socketDir := path.Dir(socketPath)

		err = os.MkdirAll(socketDir, 0600)
		if err != nil {
			glog.Fatalf("Couldn't create socket directory: %s",
				socketDir)
		}

		lis, err = net.Listen("unix", parts[1])
	} else {
		lis, err = net.Listen("tcp", config.Sensor.ListenAddr)
	}

	if err != nil {
		// We should probably give up if we can't start this.
		glog.Fatalf("Failed to start local telemetry service on %s: %s",
			config.Sensor.ListenAddr, err)
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
