package functional

import (
	"net"
	"strings"
	"time"

	"github.com/capsule8/capsule8/pkg/config"
	"google.golang.org/grpc"
)

// SensorConn() returns a client connection to the sensor.
func SensorConn() (*grpc.ClientConn, error) {
	// Most tests are done in Docker containers
	has_docker := false
	for _, cgroup := range config.Sensor.CgroupName {
		if cgroup == "docker" {
			has_docker = true
			break
		}
	}
	if !has_docker {
		config.Sensor.CgroupName = append(config.Sensor.CgroupName, "docker")
	}

	return grpc.Dial(config.Sensor.ListenAddr,
		grpc.WithDialer(dialer),
		grpc.WithDefaultCallOptions(grpc.FailFast(true)),
		grpc.WithInsecure())
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
