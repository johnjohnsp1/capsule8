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
