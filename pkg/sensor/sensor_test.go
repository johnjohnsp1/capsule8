package sensor

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/capsule8/capsule8/pkg/config"
	"github.com/golang/glog"
)

func TestMain(m *testing.M) {
	flag.Parse()

	config.Sensor.DontMountTracing = true
	config.Sensor.DontMountPerfEvent = true

	tempDir, err := ioutil.TempDir("", "sensor_test")
	if err != nil {
		glog.Fatal("Couldn't create temporary directory:", err)
	}

	config.Sensor.ListenAddr = fmt.Sprintf("unix:%s",
		filepath.Join(tempDir, "sensor.sock"))

	glog.V(1).Info("Listening on ", config.Sensor.ListenAddr)

	os.Exit(m.Run())
}
