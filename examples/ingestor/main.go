// Sample Telemetry API client that streams data to a kensis firehose in AWS

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/firehose"
	api "github.com/capsule8/capsule8/api/v0"
	"github.com/capsule8/capsule8/pkg/expression"
	"github.com/golang/glog"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/wrappers"
)

const (
	maxBatchSize       = 400
	deliveryStreamName = "example-kinesis-stream"
)

type telemetry struct {
	PublishTimeMicros string
	PublishTime       string
	Event             interface{}
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

func createSubscription() api.Subscription {
	processEvents := []*api.ProcessEventFilter{
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

	syscallEvents := []*api.SyscallEventFilter{
		// Get all syscalls that return an error
		&api.SyscallEventFilter{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT,

			Id: &wrappers.Int64Value{
				Value: 2, // SYS_OPEN
			},
		},
	}

	fileEvents := []*api.FileEventFilter{
		&api.FileEventFilter{
			Type: api.FileEventType_FILE_EVENT_TYPE_OPEN,

			//
			// The glob accepts a wild card character
			// (*,?) and character classes ([).
			//
			FilenamePattern: &wrappers.StringValue{
				Value: "*foo*",
			},
		},
	}

	sinFamilyFilter := expression.Equal(
		expression.Identifier("sin_family"),
		expression.Value(uint16(2)))
	kernelCallEvents := []*api.KernelFunctionCallFilter{
		//
		// Install a kprobe on connect(2)
		//
		&api.KernelFunctionCallFilter{
			Type:   api.KernelFunctionCallEventType_KERNEL_FUNCTION_CALL_EVENT_TYPE_ENTER,
			Symbol: "SyS_connect",
			Arguments: map[string]string{
				"sin_family": "+0(%si):u16",
				"sin_port":   "+2(%si):u16",
				"sin_addr":   "+4(%si):u32",
			},
			FilterExpression: sinFamilyFilter,
		},
	}

	containerEvents := []*api.ContainerEventFilter{
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED,
		},
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_RUNNING,
		},
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED,
		},
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_DESTROYED,
		},
	}

	// Ticker events are used for debugging and performance testing
	tickerEvents := []*api.TickerEventFilter{
		&api.TickerEventFilter{
			Interval: int64(1 * time.Second),
		},
	}

	chargenEvents := []*api.ChargenEventFilter{
	/*
		&api.ChargenEventFilter{
			Length: 16,
		},
	*/
	}

	eventFilter := &api.EventFilter{
		ProcessEvents:   processEvents,
		SyscallEvents:   syscallEvents,
		KernelEvents:    kernelCallEvents,
		FileEvents:      fileEvents,
		ContainerEvents: containerEvents,
		TickerEvents:    tickerEvents,
		ChargenEvents:   chargenEvents,
	}

	sub := api.Subscription{
		EventFilter: eventFilter,
	}

	return sub
}

func main() {
	url := flag.String("u", "capsule8-apiserver:8484",
		"Hostname & port of Sensor gRPC API")
	configFile := flag.String("c", "",
		"Path to recorder configuration file to use for subscription")

	flag.Set("logtostderr", "true")
	flag.Parse()

	firehoseService := firehose.New(session.New())

	// Create telemetry service client
	conn, err := grpc.Dial(*url,
		grpc.WithDialer(dialer),
		grpc.WithInsecure())

	if err != nil {
		fmt.Fprintf(os.Stderr, "grpc.Dial: %s\n", err)
		os.Exit(1)
	}

	c := api.NewTelemetryServiceClient(conn)
	pc := api.NewPubsubServiceClient(conn)

	ctx, cancel := context.WithCancel(context.Background())

	var sub api.Subscription
	if len(*configFile) > 0 {
		cfgFile, err := os.Open(*configFile)
		if err != nil {
			glog.Fatal("failed to load subscription config file:", err)
		}
		if err := jsonpb.Unmarshal(cfgFile, &sub); err != nil {
			glog.Fatal("failed to marshal subscription config:", err)
		}
	} else {
		sub = createSubscription()
	}

	acksChan := make(chan [][]byte, 128)
	go func() {
		for {
			acks, ok := <-acksChan
			if ok {
				pc.Acknowledge(ctx, &api.AcknowledgeRequest{
					Acks: acks,
				})
			} else {
				return
			}
		}
	}()

	stream, err := c.GetEvents(ctx, &api.GetEventsRequest{
		Subscription: &sub,
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

	ticker := time.NewTicker(1 * time.Second)

	msgs := 0
	bytes := 0

	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Printf("%d msgs/s %d KB/s\n", msgs, bytes/1024)
				msgs = 0
				bytes = 0
			}
		}
	}()

	for {
		response, err := stream.Recv()
		if err != nil {
			glog.Fatalf("Recv: %s\n", err)
		}

		acks := [][]byte{}
		for _, event := range response.Events {
			glog.V(1).Infof("%+v", event)

			err := putTelemetryRecordKinesis(event)
			if err != nil {
				glog.Errorf("failed to push telemetry to kinesis: %s", err)
			}

			msgs++
			b, _ := proto.Marshal(event)
			bytes += len(b)

			if len(event.Ack) > 0 {
				acks = append(acks, event.Ack)
			}
		}

		if len(acks) > 0 {
			acksChan <- acks
		}
	}
}

func putTelemetryRecordKinesis(telemetryEvent *api.TelemetryEvent) error {
	var event telemetry
	var eventData bytes.Buffer

	jsonMarshaller := jsonpb.Marshaler{
		EmitDefaults: true,
	}

	err := jsonMarshaller.Marshal(&eventData, telemetryEvent)
	if err != nil {
		return err
	}

	err = json.Unmarshal(eventData.Bytes(), &event)
	if err != nil {
		return err
	}

	publishTimeInt, err := strconv.ParseInt(event.PublishTimeMicros, 10, 64)
	if err != nil {
		return err
	}

	publishTime := time.Unix(0, publishTimeInt)
	event.PublishTime = publishTime.Format(time.RFC3339)

	eb, err := json.Marshal(event)

	firehoseService := firehose.New(session.New())
	output, err := firehoseService.PutRecord(&firehose.PutRecordInput{
		DeliveryStreamName: aws.String(deliveryStreamName),
		Record: &firehose.Record{
			Data: eb,
		},
	})
	if err != nil {
		return err
	}

	glog.V(1).Infof("Record: %v\n Put Record Result: %+v\n\n", string(eb), output)
	return nil
}
