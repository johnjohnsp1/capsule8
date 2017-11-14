package functional

import (
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
)

// apiConn() returns a connection to the Capsule8 API server at config.APIServer
func apiConn() (*grpc.ClientConn, error) {
	return grpc.Dial(config.APIServer,
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
