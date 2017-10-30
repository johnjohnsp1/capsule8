package functional

import (
	"flag"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	"github.com/capsule8/capsule8/pkg/config"
	"github.com/golang/glog"
)

var (
	sensorConn *grpc.ClientConn
)

func init() {
	// define flags here
}

func TestMain(m *testing.M) {
	flag.Set("logtostderr", "true")
	flag.Parse()

	//
	// Dial the sensor and allow each test to set up their own subscriptions
	// over the same gRPC connection
	//
	c, err := grpc.Dial(config.Sensor.ListenAddr,
		grpc.WithDialer(dialer),
		grpc.WithDefaultCallOptions(grpc.FailFast(true)),
		grpc.WithInsecure())

	if err != nil {
		glog.Fatal(err)
	}

	sensorConn = c

	code := m.Run()
	glog.Flush()
	os.Exit(code)
}

type container struct {
	t       *testing.T
	path    string
	imageID string
	command *exec.Cmd
}

func (c *container) build() error {
	docker := exec.Command("docker", "build", c.path)
	err := docker.Run()
	if err != nil {
		return err
	}

	docker = exec.Command("docker", "build", "-q", c.path)
	dockerOutput, err := docker.Output()
	if err != nil {
		return err
	}

	c.imageID = strings.TrimSpace(string(dockerOutput))

	return nil
}

func (c *container) start() error {
	c.command = exec.Command("docker", "run", "--rm", c.imageID)
	return c.command.Start()
}

func (c *container) startContext(ctx context.Context) error {
	c.command = exec.CommandContext(ctx, "docker", "run", "--rm", c.imageID)
	return c.command.Start()
}

func (c *container) wait() error {
	return c.command.Wait()
}

func (c *container) run() error {
	c.command = exec.Command("docker", "run", "--rm", c.imageID)
	return c.command.Run()
}

func (c *container) runContext(ctx context.Context) error {
	c.command = exec.CommandContext(ctx, "docker", "run", "--rm", c.imageID)
	return c.command.Run()
}

func newContainer(t *testing.T, path string) *container {
	return &container{
		t:    t,
		path: path,
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
