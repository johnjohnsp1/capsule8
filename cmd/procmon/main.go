package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"fmt"

	"flag"

	"github.com/capsule8/reactive8/pkg/container"
	"github.com/capsule8/reactive8/pkg/process"
	"github.com/capsule8/reactive8/pkg/stream"
	"github.com/gobwas/glob"
)

var config struct {
	cgroup string
	image  string
	pid    int
}

func init() {
	flag.StringVar(&config.cgroup, "cgroup", "",
		"cgroup to monitor")
	flag.StringVar(&config.image, "image", "",
		"container image wildcard pattern to monitor")
	flag.IntVar(&config.pid, "pid", 0,
		"process to monitor")
}

func setupEventStreams(joiner *stream.Joiner) {
	if len(config.cgroup) > 0 {
		s, err := process.NewEventStreamForCgroup(config.cgroup)
		if err != nil {
			log.Fatal(err)
		}

		joiner.Add(s)
	} else if len(config.image) > 0 {
		fmt.Printf("Watching processes for container images matching %s\n", config.image)

		g := glob.MustCompile(config.image)

		// First we get a container event stream to listen for container
		// launches of interest
		containerEvents, err := container.NewEventStream()
		defer containerEvents.Close()

		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Couldn't get container event stream: %v\n", err)
			os.Exit(1)
		}

		matchedContainers :=
			stream.Filter(containerEvents, func(e interface{}) bool {
				cEv := e.(*container.Event)

				return (cEv.State == container.ContainerStarted) && g.Match(cEv.Image)
			})

		stream.ForEach(matchedContainers, func(e interface{}) {
			cEv := e.(*container.Event)
			cgroup := filepath.Join("docker", cEv.ID)

			s, err := process.NewEventStreamForCgroup(cgroup)
			if err != nil {
				log.Fatal(err)
			}

			ok := joiner.Add(s)
			if !ok {
				log.Fatal("Could not add to joiner")
			}
		})
	} else if config.pid > 0 {
		s, err := process.NewEventStreamForPid(config.pid)
		if err != nil {
			log.Fatal(err)
		}

		joiner.Add(s)
	} else {
		s, err := process.NewEventStream()
		if err != nil {
			log.Fatal(err)
		}

		joiner.Add(s)
	}

	// Close the stop channel on Control-C to exit cleanly.
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt)
	<-signals

	joiner.Close()
}

func main() {
	flag.Parse()

	events, joiner := stream.NewJoiner()
	go setupEventStreams(joiner)

eventLoop:
	for {
		select {
		case e, ok := <-events.Data:
			if ok {
				fmt.Printf("Event: %v\n", e)
			} else {
				fmt.Fprintf(os.Stderr, "Stream closed.\n")
				break eventLoop
			}

		}
	}
}
