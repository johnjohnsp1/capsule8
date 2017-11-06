package functional

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/capsule8/capsule8/pkg/sensor"
	"github.com/golang/glog"
)

func init() {
	// define flags here
}

func TestMain(m *testing.M) {
	flag.Set("logtostderr", "true")
	flag.Parse()

	go sensor.Main()
	time.Sleep(1 * time.Second)

	code := m.Run()
	glog.Flush()
	os.Exit(code)
}
