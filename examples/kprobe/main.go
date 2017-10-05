package main

/*
EXAMPLES

$ sudo ./kprobe -output 'sockfd=%di sin_family=+0(%si):u16 sin_port=+2(%si):u16 sin_addr=+4(%si):u32' -filter 'sin_family == 2' SyS_connect
{"__probe_ip":18446744072118372864,"common_flags":1,"common_pid":11267,"common_preempt_count":0,"common_type":1627,"sin_addr":16777343,"sin_family":2,"sin_port":53764,"sockfd":3}
[...]

$sudo bin/kprobe -output 'dfd=%di:s32 filename=+0(%si):string flags=%dx:s32 mode=%cx:s32' do_sys_open
{"__probe_ip":18446744072308927968,"common_flags":1,"common_pid":1108,"common_preempt_count":0,"common_type":1593,"dfd":4294967196,"filename":"/proc/stat","flags":32768,"mode":438}
*/

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	api "github.com/capsule8/api/v0"
)

var config struct {
	endpoint  string
	symbol    string
	fetchargs string
	filter    string
	onReturn  bool
}

func init() {
	flag.StringVar(&config.endpoint, "endpoint",
		"unix:/var/run/capsule8/sensor.sock",
		"Capsule8 gRPC API endpoint")

	flag.StringVar(&config.symbol, "symbol", "",
		"symbol to attach kprobe on")

	flag.StringVar(&config.fetchargs, "fetchargs", "",
		"'fetchargs' string specifying what data to output in events")

	flag.StringVar(&config.filter, "filter", "",
		"trace event filter string")

	flag.BoolVar(&config.onReturn, "return", false,
		"set probe on return of given function instead of entry")
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
	arguments := make(map[string]string)

	if len(config.fetchargs) > 0 {
		for _, fa := range strings.Split(config.fetchargs, " ") {
			parts := strings.Split(fa, "=")
			arguments[parts[0]] = parts[1]
		}
	}

	kernelCallEvents := []*api.KernelFunctionCallFilter{
		//
		// Install a kprobe on connect(2)
		//
		&api.KernelFunctionCallFilter{
			Type:      api.KernelFunctionCallEventType_KERNEL_FUNCTION_CALL_EVENT_TYPE_ENTER,
			Symbol:    config.symbol,
			Arguments: arguments,
			Filter:    config.filter,
		},
	}

	eventFilter := &api.EventFilter{
		KernelEvents: kernelCallEvents,
	}

	sub := &api.Subscription{
		EventFilter: eventFilter,
	}

	return sub
}

func main() {
	flag.Parse()

	if len(config.symbol) == 0 {
		flag.Usage()
		os.Exit(1)
	}

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
		ev, err := stream.Recv()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Recv: %s\n", err)
			os.Exit(1)
		}

		for _, e := range ev.Events {
			fmt.Println(e)
		}
	}
}
