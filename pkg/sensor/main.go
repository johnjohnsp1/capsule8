package sensor

import (
	"github.com/capsule8/capsule8/pkg/config"
	"github.com/capsule8/capsule8/pkg/services"
	"github.com/golang/glog"
)

// Main is the main entrypoint for the sensor
func Main() {
	manager := services.NewServiceManager()
	if config.Global.ProfilingPort > 0 {
		service := services.NewProfilingService(
			"127.0.0.1", config.Global.ProfilingPort)
		manager.RegisterService(service)
	}
	if len(config.Sensor.ListenAddr) > 0 {
		service, err := NewTelemetryService(config.Sensor.ListenAddr)
		if err != nil {
			glog.Fatalf("Could not create telemetry service: %s", err)
		}
		manager.RegisterService(service)
	}

	manager.Run()
}
