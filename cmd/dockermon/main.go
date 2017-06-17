// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/capsule8/reactive8/pkg/container"
	"github.com/capsule8/reactive8/pkg/container/c8dockerclient"
)

func main() {
	s, err := container.NewEventStream()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Couldn't get container event stream: %v\n", err)
		os.Exit(1)
	}

	client := c8dockerclient.NewClient()
	events, err := client.EventChannel()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Could not get docker events channel: %v\n", err)
		os.Exit(1)
	}

	// Close the stop channel on Control-C to exit cleanly.
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt)

	go func() {
		<-signals
		s.Close()
	}()

selectLoop:
	for {
		select {
		case e, ok := <-s.Data:
			if ok {
				fmt.Fprintf(os.Stderr, "Event: %+v\n", e)
			} else {
				fmt.Fprintf(os.Stderr, "Stream closed.\n")
				break selectLoop
			}

		case e, ok := <-events:
			if ok {
				fmt.Fprintf(os.Stderr, "Event: %+v\n", e)
			} else {
				fmt.Fprintf(os.Stderr, "Stream closed.\n")
				break selectLoop
			}
		}
	}
}
