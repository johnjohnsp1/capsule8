// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"net/http"
	_ "net/http/pprof"

	"github.com/capsule8/reactive8/pkg/config"
	"github.com/capsule8/reactive8/pkg/sensor"
	"github.com/capsule8/reactive8/pkg/version"
	"github.com/golang/glog"
)

func main() {
	// Set "alsologtostderr" flag so that glog messages go stderr as well as /tmp.
	flag.Set("alsologtostderr", "true")
	flag.Parse()

	// Log version and build at "Starting ..." for debugging
	version.InitialBuildLog("sensor")

	//
	// Do this before creating the sensor to allow us to capture
	// data as early as possible.
	//
	if config.Sensor.ProfilingPort > 0 {
		addr := fmt.Sprintf("127.0.0.1:%d", config.Sensor.ProfilingPort)
		glog.Infof("Serving profiling HTTP endpoints on %s", addr)
		go func() {
			err := http.ListenAndServe(addr, nil)
			glog.V(1).Info(err)
		}()
	}

	s, err := sensor.GetSensor()
	if err != nil {
		glog.Fatalf("Couldn't create Sensor: %s", err)
	}

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)
	signal.Notify(sigChan, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		glog.Infof("Caught signal %v, stopping Sensor", sig)
		s.Stop()
	}()

	err = s.Serve()
	if err != nil {
		glog.Error(err)
	}

	glog.Info("Exiting...")
}
