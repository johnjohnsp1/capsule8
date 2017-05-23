// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"fmt"
	"os/signal"
	"time"

	"os"

	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/sensor"
)

func main() {
	// Configure selector from command-line arguments

	// Add a selector
	selector := event.Selector{
		Container: &event.ContainerEventSelector{},

		Ticker: &event.TickerEventSelector{
			Duration: int64(1 * time.Second),
		},
	}

	// Create events channel. Events channels pass pointers b/c sensors
	// pass ownership of events to the receiver.
	events := make(chan *event.Event)

	// Create the sensors
	sensorStop, sensorErrors := sensor.NewDockerOCISensor(selector, events)
	if sensorStop == nil {
		fmt.Fprintf(os.Stderr, "Couldn't start Sensor\n")
		os.Exit(1)
	}

	// Close the stop channel on Control-C to exit cleanly.
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			close(sensorStop)
		}
	}()

selectLoop:
	for {
		select {
		case ev, ok := <-events:
			if ok {
				fmt.Fprintf(os.Stderr, "Event: %v\n", ev)

				/*
					bytes, err := json.Marshal(ev)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					} else {
						os.Stdout.Write(bytes)
						fmt.Println()
					}
				*/
			} else {
				break selectLoop
			}
		case err, ok := <-sensorErrors:
			if ok {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				close(sensorStop)
			} else {
				break selectLoop
			}
		}
	}

	// Drain errors
errorLoop:
	for {
		select {
		case err, ok := <-sensorErrors:
			if ok {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			} else {
				break errorLoop
			}
		default:
			break errorLoop
		}
	}

	os.Exit(0)
}
