// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/capsule8/reactive8/pkg/config"
	checks "github.com/capsule8/reactive8/pkg/health"
	"github.com/capsule8/reactive8/pkg/version"
	"github.com/coreos/pkg/health"
	"github.com/golang/glog"
)

var healthChecker health.Checker

func main() {
	flag.Parse()

	// Log version and build at start up for debugging
	version.InitialBuildLog("Sensor")

	configureHealthChecks()
	http.HandleFunc("/healthz", healthChecker.ServeHTTP)
	go http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", config.Sensor.MonitoringPort), nil)

	glog.Infoln("starting up")
	s, err := CreateSensor()
	if err != nil {
		glog.Fatalf("error creating sensor: %s\n", err.Error())
	}
	err = s.Start()
	if err != nil {
		glog.Fatalf("error starting sensor: %s\n", err.Error())
	}
	glog.Infoln("started")

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGTERM)
	go func() {
		<-sigChan
		s.Shutdown()
	}()

	// Blocking call to remove stale subscriptions on a 5 second interval
	s.RemoveStaleSubscriptions()
	s.Wait()
}

// configure and initialize all Checkable variables required by the health checker
func configureHealthChecks() {
	stanChecker := checks.ConnectionURL(fmt.Sprintf("%s/streaming/serverz", config.Backplane.NatsMonitoringURL))

	healthChecker.Checks = []health.Checkable{
		stanChecker,
	}
}
