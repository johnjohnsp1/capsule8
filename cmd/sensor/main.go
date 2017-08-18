// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"flag"

	"github.com/golang/glog"
)

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
}
