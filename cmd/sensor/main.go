// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/capsule8/reactive8/pkg/config"
	checks "github.com/capsule8/reactive8/pkg/health"
	"github.com/coreos/pkg/health"
	"github.com/golang/glog"
)

var healthChecker health.Checker

func main() {
	flag.Parse()
	glog.Infoln("starting up")
	s, err := CreateSensor()
	if err != nil {
		glog.Fatalf("error creating sensor: %s\n", err.Error())
	}
	stopSignal, err := s.Start()
	if err != nil {
		glog.Fatalf("error starting sensor: %s\n", err.Error())
	}
	glog.Infoln("started")
	// Blocking call to remove stale subscriptions on a 5 second interval
	s.RemoveStaleSubscriptions()
	close(stopSignal)

	configureHealthChecks()
	http.HandleFunc("/healthz", healthChecker.ServeHTTP)
}

// configure and initialize all Checkable variables required by the health checker
func configureHealthChecks() {
	stanChecker := checks.ConnectionURL(fmt.Sprintf("%s/streaming/serverz", config.Backplane.NatsMonitoringURL))

	healthChecker.Checks = []health.Checkable{
		stanChecker,
	}
}
