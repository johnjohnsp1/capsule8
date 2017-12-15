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
	"flag"
	"os"
	"testing"

	"github.com/golang/glog"
	"github.com/kelseyhightower/envconfig"
)

//
// Functional test driver environment configuration options
//
var config struct {
	// APIServer is the Capsule8 API gRPC server address to
	// connect to. The address is expected to be of the following
	// forms:
	//  hostname:port
	//  :port
	//  unix:/path/to/socket
	APIServer string `envconfig:"api_server" default:"unix:/var/run/capsule8/sensor.sock"`
}

func init() {
	err := envconfig.Process("CAPSULE8", &config)
	if err != nil {
		glog.Fatal(err)
	}
}

func TestMain(m *testing.M) {
	// TestMain is needed to set glog defaults
	flag.Set("logtostderr", "true")
	flag.Parse()

	code := m.Run()
	glog.Flush()
	os.Exit(code)
}
