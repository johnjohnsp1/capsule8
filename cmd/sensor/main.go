// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"log"
)

func main() {
	log.Println("starting up")
	s, err := CreateSensor("sensor")
	if err != nil {
		log.Fatal("Error creating sensor:", err)
	}
	stopSignal, err := s.Start()
	if err != nil {
		log.Fatal("Error starting sensor:", err)
	}
	log.Println("started")
	// Blocking call to remove stale subscriptions on a 5 second interval
	s.RemoveStaleSubscriptions()
	close(stopSignal)
}
