// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"

	api "github.com/capsule8/api/v0"
	sensorConfig "github.com/capsule8/capsule8/pkg/config"

	"github.com/capsule8/capsule8/pkg/sensor"
	"github.com/capsule8/capsule8/pkg/version"
)

var config struct {
	endpoint string
	image    string
	verbose  bool
}

func init() {
	flag.StringVar(&config.endpoint, "endpoint",
		"unix:/var/run/capsule8/sensor.sock",
		"Capsule8 gRPC API endpoint")

	flag.StringVar(&config.image, "image", "",
		"container image wildcard pattern to monitor")

	flag.BoolVar(&config.verbose, "verbose", false,
		"verbose (print events received)")
}

// Custom gRPC Dialer that understands "unix:/path/to/sock" as well as TCP addrs
func dialer(addr string, timeout time.Duration) (net.Conn, error) {
	var network, address string

	parts := strings.Split(addr, ":")
	if len(parts) > 1 && parts[0] == "unix" {
		network = "unix"
		address = parts[1]
	} else {
		network = "tcp"
		address = addr
	}

	return net.DialTimeout(network, address, timeout)
}

func createSubscription() *api.Subscription {
	processEvents := []*api.ProcessEventFilter{
		//
		// Get all process lifecycle events
		//
		&api.ProcessEventFilter{
			Type: api.ProcessEventType_PROCESS_EVENT_TYPE_FORK,
		},
		&api.ProcessEventFilter{
			Type: api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
		},
		&api.ProcessEventFilter{
			Type: api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT,
		},
	}

	containerEvents := []*api.ContainerEventFilter{
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_RUNNING,
		},
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED,
		},
	}

	eventFilter := &api.EventFilter{
		ProcessEvents:   processEvents,
		ContainerEvents: containerEvents,
	}

	sub := &api.Subscription{
		EventFilter: eventFilter,
	}

	if config.image != "" {
		fmt.Fprintf(os.Stderr,
			"Watching for container images matching %s\n",
			config.image)

		containerFilter := &api.ContainerFilter{}

		containerFilter.ImageNames =
			append(containerFilter.ImageNames, config.image)

		sub.ContainerFilter = containerFilter
	}

	return sub
}

var subscriptionEvents int64

var startSubEvents int64
var startMetrics map[string]sensor.MetricsCounters
var startRusage map[string]unix.Rusage

func onContainerRunning(cID string) {
	var rusage unix.Rusage

	if startMetrics == nil {
		startMetrics = make(map[string]sensor.MetricsCounters)
	}

	startMetrics[cID] = sensor.Metrics

	startSubEvents = subscriptionEvents

	err := unix.Getrusage(unix.RUSAGE_SELF, &rusage)
	if err != nil {
		log.Fatal(err)
	}

	if startRusage == nil {
		startRusage = make(map[string]unix.Rusage)
	}
	startRusage[cID] = rusage
}

func max(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}

func onContainerExited(cID string) {
	var metricsStart, metricsEnd, metricsDelta sensor.MetricsCounters

	metricsEnd = sensor.Metrics
	metricsStart = startMetrics[cID]

	metricsDelta.Events = metricsEnd.Events - metricsStart.Events
	metricsDelta.Subscriptions = int32(max(int64(metricsEnd.Subscriptions),
		int64(metricsStart.Subscriptions)))

	var rusage, delta unix.Rusage
	err := unix.Getrusage(unix.RUSAGE_SELF, &rusage)
	if err != nil {
		log.Fatal(err)
	}

	start := startRusage[cID]

	startTotalUserUsec := (start.Utime.Sec * 1000000) + start.Utime.Usec
	startTotalSysUsec := (start.Stime.Sec * 1000000) + start.Stime.Usec

	stopTotalUserUsec := (rusage.Utime.Sec * 1000000) + rusage.Utime.Usec
	stopTotalSysUsec := (rusage.Stime.Sec * 1000000) + rusage.Stime.Usec

	deltaTotalUserUsec := stopTotalUserUsec - startTotalUserUsec
	deltaTotalSysUsec := stopTotalSysUsec - startTotalSysUsec

	delta.Utime.Sec = deltaTotalUserUsec / 1000000
	delta.Utime.Usec = deltaTotalUserUsec % 1000000
	delta.Stime.Sec = deltaTotalSysUsec / 1000000
	delta.Stime.Usec = deltaTotalSysUsec % 1000000

	delta.Maxrss = max(rusage.Maxrss, start.Maxrss)
	delta.Minflt = rusage.Minflt - start.Minflt
	delta.Majflt = rusage.Majflt - start.Majflt
	delta.Inblock = rusage.Inblock - start.Inblock
	delta.Oublock = rusage.Oublock - start.Oublock
	delta.Nvcsw = rusage.Nvcsw - start.Nvcsw
	delta.Nivcsw = rusage.Nivcsw - start.Nivcsw

	nEvents := subscriptionEvents - startSubEvents
	usecsPerEvent := deltaTotalUserUsec / nEvents
	ssecsPerEvent := deltaTotalSysUsec / nEvents

	fmt.Printf("%s Events:%d avg_user_us_per_event:%d avg_sys_us_per_event:%d %+v %+v\n",
		cID, nEvents, usecsPerEvent, ssecsPerEvent,
		metricsDelta, delta)
}

func main() {
	// Set "alsologtostderr" flag so that glog messages go stderr as well as /tmp.
	//flag.Set("alsologtostderr", "true")
	flag.Parse()

	// Enable profiling by default
	sensorConfig.Global.ProfilingAddr = ":6060"

	// Log version and build at "Starting ..." for debugging
	version.InitialBuildLog("benchmark")

	go func() {
		sensor.Main()
	}()

	time.Sleep(1 * time.Second)

	// Create telemetry service client
	conn, err := grpc.Dial(config.endpoint,
		grpc.WithDialer(dialer),
		grpc.WithInsecure())

	c := api.NewTelemetryServiceClient(conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grpc.Dial: %s\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stream, err := c.GetEvents(ctx, &api.GetEventsRequest{
		Subscription: createSubscription(),
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "GetEvents: %s\n", err)
		os.Exit(1)
	}

	// Exit cleanly on Control-C
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt)

	go func() {
		<-signals
		cancel()
	}()

	for {
		response, err := stream.Recv()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Recv: %s\n", err)
			os.Exit(1)
		}

		for _, event := range response.Events {
			e := event.Event

			if config.verbose {
				fmt.Printf("%+v\n", e)
			}

			switch x := e.Event.(type) {
			case *api.Event_Container:
				switch x.Container.GetType() {
				case api.ContainerEventType_CONTAINER_EVENT_TYPE_RUNNING:
					onContainerRunning(e.ContainerId)

				case api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED:
					onContainerExited(e.ContainerId)
				}
			}

			subscriptionEvents++
		}
	}
}
