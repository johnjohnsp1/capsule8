// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/capsule8/reactive8/pkg/container"
)

func main() {
	/*
		f, err := os.Create("trace.out")
		if err != nil {
			panic(err)
		}
		defer f.Close()

		err = trace.Start(f)
		if err != nil {
			panic(err)
		}
		defer trace.Stop()
	*/

	s, err := container.NewEventStream()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Couldn't get container event stream: %v\n", err)
		os.Exit(1)
	}

	// Close the stop channel on Control-C to exit cleanly.
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt)

	go func() {
		<-signals
		s.Close()
	}()

	for {
		e, ok := <-s.Data
		if ok {
			fmt.Fprintf(os.Stderr, "Event: %v\n", e)

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
			fmt.Fprintf(os.Stderr, "Stream closed.\n")
			break
		}
	}
}
