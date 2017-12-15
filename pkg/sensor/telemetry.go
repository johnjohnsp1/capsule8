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

package sensor

import (
	"net"
	"os"
	"strings"

	api "github.com/capsule8/capsule8/api/v0"
	"github.com/golang/glog"

	"golang.org/x/sys/unix"

	"google.golang.org/grpc"
)

type TelemetryService struct {
	server *grpc.Server
	sensor *Sensor

	address string
}

func NewTelemetryService(sensor *Sensor, address string) *TelemetryService {
	return &TelemetryService{
		address: address,
		sensor:  sensor,
	}
}

func (ts *TelemetryService) Name() string {
	return "gRPC Telemetry Server"
}

func (ts *TelemetryService) Serve() error {
	var (
		err error
		lis net.Listener
	)

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
