package sensor

import (
	"net"
	"os"
	"strings"

	api "github.com/capsule8/api/v0"
	"github.com/golang/glog"

	"golang.org/x/sys/unix"

	"google.golang.org/grpc"
)

type TelemetryService struct {
	server *grpc.Server
	sensor *Sensor

	address string
}

func NewTelemetryService(address string) (*TelemetryService, error) {
	sensor, err := NewSensor()
	if err != nil {
		return nil, err
	}

	ts := &TelemetryService{
		address: address,
		sensor:  sensor,
	}

	return ts, nil
}

func (ts *TelemetryService) Name() string {
	return "gRPC Telemetry Server"
}

func (ts *TelemetryService) Serve() error {
	var (
		err error
		lis net.Listener
	)

	err = ts.sensor.Start()
	if err != nil {
		return err
	}
	defer ts.sensor.Stop()

	glog.V(1).Info("Serving gRPC API on ", ts.address)

	parts := strings.Split(ts.address, ":")
	if len(parts) > 1 && parts[0] == "unix" {
		socketPath := parts[1]

		// Check whether socket already exists and if someone
		// is already listening on it.
		_, err = os.Stat(socketPath)
		if err == nil {
			var ua *net.UnixAddr

			ua, err = net.ResolveUnixAddr("unix", socketPath)
			if err == nil {
				var c *net.UnixConn

				c, err = net.DialUnix("unix", nil, ua)
				if err == nil {
					// There is another running service.
					// Try to listen below and return the
					// error.
					c.Close()
				} else {
					// Remove the stale socket so the
					// listen below will succeed.
					os.Remove(socketPath)
				}
			}
		}

		oldMask := unix.Umask(0077)
		lis, err = net.Listen("unix", socketPath)
		unix.Umask(oldMask)
	} else {
		lis, err = net.Listen("tcp", ts.address)
	}

	if err != nil {
		return err
	}
	defer lis.Close()

	// Start local gRPC service on listener
	ts.server = grpc.NewServer()
	t := &telemetryServiceServer{
		sensor: ts.sensor,
	}
	api.RegisterTelemetryServiceServer(ts.server, t)

	return ts.server.Serve(lis)
}

func (ts *TelemetryService) Stop() {
	ts.server.GracefulStop()
}

type telemetryServiceServer struct {
	sensor *Sensor
}

func (t *telemetryServiceServer) GetEvents(req *api.GetEventsRequest, stream api.TelemetryService_GetEventsServer) error {
	sub := req.Subscription

	glog.V(1).Infof("GetEvents(%+v)", sub)

	eventStream, err := t.sensor.NewSubscription(sub)
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
