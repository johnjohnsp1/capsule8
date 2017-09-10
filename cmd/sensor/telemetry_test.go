package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/config"
	"github.com/golang/glog"
)

// Custom gRPC Dialer that understands "unix:/path/to/sock" as well as TCP addrs
func dialer(addr string, timeout time.Duration) (net.Conn, error) {
	var network, address string

	parts := strings.Split(addr, ":")
	if len(parts) > 1 && parts[0] == "unix" {
		network = "unix"
		address = parts[1]
	} else {
		network = "tcp"
		address = addr
	}

	return net.DialTimeout(network, address, timeout)
}

func TestGetEvents(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "TestGetEvents")
	if err != nil {
		t.Fatal("Couldn't create temporary directory:", err)
	}

	config.Sensor.ListenAddr = fmt.Sprintf("unix:%s",
		filepath.Join(tempDir, "sensor.sock"))

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

	conn, err := grpc.Dial(config.Sensor.ListenAddr,
		grpc.WithDialer(dialer),
		grpc.WithInsecure())

	if err != nil {
		t.Fatalf("Couldn't dial %s: %s", config.Sensor.ListenAddr, err)
	}

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
