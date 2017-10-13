package sensor

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/capsule8/pkg/config"
	"github.com/golang/glog"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	flag.Parse()

	tempDir, err := ioutil.TempDir("", "sensor_test")
	if err != nil {
		glog.Fatal("Couldn't create temporary directory:", err)
	}

	config.Global.RunDir = tempDir
	config.Sensor.DontMountTracing = true
	config.Sensor.DontMountPerfEvent = true

	config.Sensor.ListenAddr = fmt.Sprintf("unix:%s",
		filepath.Join(tempDir, "sensor.sock"))

	glog.V(1).Info("Listening on ", config.Sensor.ListenAddr)

	os.Exit(m.Run())
}

func TestGetEvents(t *testing.T) {
	go func() {
		Main()
		glog.V(1).Info("Main() exited.")
	}()

	stopSignal := make(chan interface{})

	glog.V(1).Infof("Dialing %s", config.Sensor.ListenAddr)
	conn, err := grpc.Dial(config.Sensor.ListenAddr,
		grpc.WithDialer(dialer),
		grpc.WithDefaultCallOptions(grpc.FailFast(false)),
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

	glog.V(1).Info("Calling GetEvents")
	stream, err = c.GetEvents(context.Background(),
		&api.GetEventsRequest{
			Subscription: sub,
		})

	if err != nil {
		t.Fatal("Couldn't call GetEvents RPC: ", err)
	}

	glog.Info("Receiving events")

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
					t.Fatal("stream.Recv(): ", err)
					break getMessageLoop
				}
				for _, ev := range resp.Events {
					events <- ev
				}
			}
		}

		glog.V(1).Info("Exiting getMessageLoop goroutine")
	}()

	glog.Info("Selecting on events")
	select {
	case <-time.After(5 * time.Second):
		t.Error("Receive msg timeout")
	case ev := <-events:
		t.Log("Recevied message:", ev)
	}

	glog.Info("Closing stopSignal")
	close(stopSignal)
}

func TestReport(t *testing.T) {
	go func() {
		Main()
		glog.V(1).Info("Main() exited.")
	}()
	metadata := &api.Capsulator{}
	Sensor.Report(metadata)
	if Sensor.ID != metadata.Id ||
		metadata.Type != api.Capsulator_TYPE_SENSOR {
		t.Error("Capsulator metadata not as expected")
	}
}

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
