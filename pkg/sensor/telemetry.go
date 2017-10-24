package sensor

import (
	"net"
	"os"
	"strings"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/capsule8/pkg/config"
	"github.com/capsule8/capsule8/pkg/subscription"
	"github.com/golang/glog"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

type gRPCServer struct {
	listener net.Listener
	server   *grpc.Server
}

func (g *gRPCServer) Name() string {
	return "gRPC Server"
}

func (g *gRPCServer) Serve() error {
	var err error
	var lis net.Listener

	glog.Info("Serving gRPC API on ", config.Sensor.ListenAddr)

	parts := strings.Split(config.Sensor.ListenAddr, ":")
	if len(parts) > 1 && parts[0] == "unix" {
		socketPath := parts[1]

		//
		// Check whether socket already exists and if someone
		// is already listening on it.
		//
		_, err = os.Stat(socketPath)
		if err == nil {
			var ua *net.UnixAddr

			ua, err = net.ResolveUnixAddr("unix", socketPath)
			if err == nil {
				var c *net.UnixConn

				c, err = net.DialUnix("unix", nil, ua)
				if err == nil {
					// There is another running
					// Sensor, try to listen below
					// and return the error.

					c.Close()
				} else {
					// Remove the stale socket so
					// the listen below will
					// succed.
					os.Remove(socketPath)
				}
			}
		}

		oldMask := unix.Umask(0077)
		lis, err = net.Listen("unix", socketPath)
		unix.Umask(oldMask)
	} else {
		lis, err = net.Listen("tcp", config.Sensor.ListenAddr)
	}

	if err != nil {
		return err
	}

	// Start local gRPC service on listener
	server := grpc.NewServer()
	t := &telemetryServiceServer{}
	api.RegisterTelemetryServiceServer(server, t)
	g.server = server

	serveErr := server.Serve(lis)
	lis.Close()
	return serveErr
}

func (g *gRPCServer) Stop() {
	g.server.GracefulStop()
}

type telemetryServiceServer struct {
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
