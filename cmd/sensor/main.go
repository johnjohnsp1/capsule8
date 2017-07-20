// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"log"
)

func main() {
	log.Println("starting up")
	s, err := CreateSensor()
	if err != nil {
		log.Fatalf("error creating sensor: %s\n", err.Error())
	}
	stopSignal, err := s.Start()
	if err != nil {
		log.Fatalf("error starting sensor: %s\n", err.Error())
	}
	log.Println("started")
	// Blocking call to remove stale subscriptions on a 5 second interval
	s.RemoveStaleSubscriptions()
	close(stopSignal)
}
