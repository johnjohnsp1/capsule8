package functional

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/capsule8/capsule8/pkg/sensor"
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

	go sensor.Main()
	time.Sleep(1 * time.Second)

	code := m.Run()
	glog.Flush()
	os.Exit(code)
}
