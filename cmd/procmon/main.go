package main

import (
	"log"
	"os"
	"os/signal"
	"time"

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
	log.SetFlags(log.LUTC | log.Llongfile | log.Lmicroseconds)

	flag.StringVar(&config.cgroup, "cgroup", "",
		"cgroup to monitor")
	flag.StringVar(&config.image, "image", "",
		"container image wildcard pattern to monitor")
	flag.IntVar(&config.pid, "pid", 0,
		"process to monitor")
}

func setupEventStreams(joiner *stream.Joiner) {
	defer joiner.Close()

	if len(config.cgroup) > 0 {
		s, err := process.NewEventStreamForCgroup(config.cgroup)
		if err != nil {
			log.Fatal(err)
		}

		joiner.Add(s)
	} else if len(config.image) > 0 {
		fmt.Printf("Watching for containers running image %s\n",
			config.image)

		g := glob.MustCompile(config.image)

		// First we get a container event stream to listen for container
		// launches of interest
		cEvents, err := container.NewEventStream()

		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Couldn't get container event stream: %v\n", err)
			os.Exit(1)
		}

		containers := make(map[string]int)

		matched := stream.Filter(cEvents, func(e interface{}) bool {
			cEv := e.(*container.Event)

			if g.Match(cEv.Image) {
				if cEv.Pid > 0 {
					containers[cEv.ID] = int(cEv.Pid)
				} else {
					containers[cEv.ID] = -1
				}

				return true
			} else if containers[cEv.ID] != 0 {
				return true
			} else if cEv.State == container.ContainerStopped {
				delete(containers, cEv.ID)

				return true
			}

			return false
		})

		a, b := stream.Tee(matched)

		joiner.Add(a)

		stream.ForEach(b, func(e interface{}) {
			cEv := e.(*container.Event)

			if containers[cEv.ID] != 0 && len(cEv.Cgroup) > 0 {
				cg := cEv.Cgroup

				s, err := process.NewEventStreamForCgroup(cg)
				if err != nil {
					log.Fatal(err)
				}

				ok := joiner.Add(s)
				if !ok {
					log.Fatal("Could not add to joiner")
				}
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
}

func main() {
	flag.Parse()

	events, joiner := stream.NewJoiner()
	go setupEventStreams(joiner)

	ticker := time.Tick(1 * time.Second)
	numEvents := 0

eventLoop:
	for {
		select {
		case <-ticker:
			fmt.Printf("Events/s: %d\n", numEvents)
			numEvents = 0

		case e, ok := <-events.Data:
			if ok {
				numEvents++
				fmt.Printf("Event: %v\n", e)
			} else {
				fmt.Fprintf(os.Stderr, "Stream closed.\n")
				break eventLoop
			}

		}
	}
}
