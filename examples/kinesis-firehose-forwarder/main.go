// Sample Telemetry API client that streams data to a kensis firehose in AWS

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/aws/aws-sdk-go/service/firehose/firehoseiface"
	api "github.com/capsule8/capsule8/api/v0"
	"github.com/golang/glog"
	"github.com/golang/protobuf/jsonpb"
)

const (
	// Buffer length of channel for queueing batches of records to send to Kinesis Firehose
	batchBufferLength = 1024

	// Buffer length of channel for queuing telemetry events to batch up and send to Kinesis
	channelBufferLength = 1024

	// Maximum number of records permitted in a batch. Amazon's hard limit is 500.
	maxBatchSize = 500

	// Send the batch after this amount of time if non-empty
	batchTimerMillis = 1000

	deliveryStreamName = "example-kinesis-stream"
)

var hostname string

//
// This struct is used to define the JSON object that is sent as the
// record data to Firehose
//
type firehoseEvent struct {
	Hostname       string
	Timestamp      time.Time
	TelemetryEvent interface{}
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

	networkEvents := []*api.NetworkEventFilter{
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_ATTEMPT,
		},

		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_ATTEMPT,
		},

		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_RESULT,
		},

		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_ATTEMPT,
		},

		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_RESULT,
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

	eventFilter := &api.EventFilter{
		ContainerEvents: containerEvents,
		NetworkEvents:   networkEvents,
		ProcessEvents:   processEvents,
	}

	sub := api.Subscription{
		EventFilter: eventFilter,
	}

	return sub
}

type mockFirehoseClient struct {
	firehoseiface.FirehoseAPI
}

func (m *mockFirehoseClient) DescribeDeliveryStream(input *firehose.DescribeDeliveryStreamInput) (*firehose.DescribeDeliveryStreamOutput, error) {
	output := &firehose.DescribeDeliveryStreamOutput{}
	return output, nil
}

func (m *mockFirehoseClient) PutRecordBatch(input *firehose.PutRecordBatchInput) (*firehose.PutRecordBatchOutput, error) {
	glog.Infof("PutRecordBatch: %d records", len(input.Records))
	output := &firehose.PutRecordBatchOutput{}
	return output, nil
}

func describeDeliveryStream(svc firehoseiface.FirehoseAPI, deliveryStreamName string) (string, error) {
	ddsOutput, err := svc.DescribeDeliveryStream(&firehose.DescribeDeliveryStreamInput{
		DeliveryStreamName: aws.String(deliveryStreamName),
	})

	if err != nil {
		return "", err
	}

	return ddsOutput.String(), nil
}

func main() {
	url := flag.String("u", "unix:/var/run/capsule8/sensor.sock",
		"Hostname & port of Sensor gRPC API")

	configFile := flag.String("c", "",
		"Path to recorder configuration file to use for subscription")

	useMock := flag.Bool("mock", false, "Use mock Firehose API")

	flag.Set("logtostderr", "true")
	flag.Parse()

	var err error

	//
	// Set our hostname (global)
	//
	hostname, err = os.Hostname()
	if err != nil {
		glog.Fatalf("Couldn't get hostname: %v", err)
	}

	var firehoseService firehoseiface.FirehoseAPI
	if *useMock {
		firehoseService = &mockFirehoseClient{}
	} else {
		firehoseService = firehose.New(session.New())
	}

	// Make sure that we can connect the Kinesis Firehose stream otherwise there isn't anything to do
	ddsOutput, err := describeDeliveryStream(firehoseService, deliveryStreamName)
	if err != nil {
		glog.Fatalf("Couldn't describe delivery stream %s: %v", deliveryStreamName, err)
	}

	glog.Infof("Delivering to Kinesis Firehose Stream: %v", ddsOutput)

	// Create telemetry service client
	conn, err := grpc.Dial(*url,
		grpc.WithDialer(dialer),
		grpc.WithInsecure())

	if err != nil {
		fmt.Fprintf(os.Stderr, "grpc.Dial: %s\n", err)
		os.Exit(1)
	}

	c := api.NewTelemetryServiceClient(conn)
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

	recordsChan := make(chan []*firehose.Record, channelBufferLength)
	kinesisBatchChan := make(chan []*firehose.Record, batchBufferLength)

	//
	// Create a goroutine to pull from queue of batches of events to send to Kinesis Firehose
	//
	go func() {
		for {
			select {
			case records, ok := <-kinesisBatchChan:
				if ok {
					output, err := firehoseService.PutRecordBatch(&firehose.PutRecordBatchInput{
						DeliveryStreamName: aws.String(deliveryStreamName),
						Records:            records,
					})

					if err != nil {
						glog.Fatal(err)
					}

					glog.V(1).Infof("PutRecordBatch: %s", output)
				} else {
					// kinesisBatchChan has been closed, so we should exit the goroutine
					glog.Info("Channel closed, exiting Kinesis Firehose sender goroutine")
					return
				}
			}
		}
	}()

	//
	// Create a goroutine to receive events from the gRPC stream,
	// convert them to Firehose Records, and push them onto a
	// channel for batching up to send to a delivery stream.
	//
	go func() {
		for {
			response, err := stream.Recv()
			if err != nil {
				glog.Errorf("Error received from gRPC stream: %v", err)

				// Close the eventChan and return
				// since there won't be any more
				// events sent to it
				close(recordsChan)

				return
			}

			kinesisRecords, err := convertTelemetryToFirehoseRecords(response.Events)
			if err != nil {
				glog.Fatal(err)
			}

			recordsChan <- kinesisRecords
		}
	}()

	//
	// Batching for Kinesis: PutRecord accepts up to 500 records
	//
	var kinesisBatch []*firehose.Record

	timerDuration := time.Duration(batchTimerMillis * time.Millisecond)
	timer := time.NewTimer(timerDuration)

	for {
		select {
		case records, ok := <-recordsChan:
			if ok {
				kinesisBatch = append(kinesisBatch, records...)

				if len(kinesisBatch) >= maxBatchSize {
					kinesisBatchChan <- kinesisBatch[:maxBatchSize]
					kinesisBatch = kinesisBatch[maxBatchSize:]

					// Reset timer
					timer.Reset(timerDuration)
				}
			} else {
				// eventsChan has been closed: send
				// the last batch, close the firehose
				// batch channel, and exit goroutine
				kinesisBatchChan <- kinesisBatch
				close(kinesisBatchChan)
				return
			}

		case <-timer.C:
			// Timer fired, queue up the batch of the
			// events that we have for sending
			if len(kinesisBatch) > 0 {
				kinesisBatchChan <- kinesisBatch
				kinesisBatch = []*firehose.Record{}
			}

			timer.Reset(timerDuration)
		}
	}
}

//
// Convert received telemetry event from proto to JSON and enrich with a
// few fields.
//
func convertTelemetryToFirehoseRecords(telemetryEvents []*api.TelemetryEvent) ([]*firehose.Record, error) {
	var records []*firehose.Record

	jsonMarshaller := jsonpb.Marshaler{
		EmitDefaults: true,
	}

	for _, telemetryEvent := range telemetryEvents {
		jsonString, err := jsonMarshaller.MarshalToString(telemetryEvent)
		if err != nil {
			glog.Warning(err)
			return records, nil
		}

		// Unmarshal to a map to allow us to modify it
		var d map[string]interface{}
		json.Unmarshal([]byte(jsonString), &d)

		delete(d, "publishTimeMicros")
		delete(d, "ack")

		d["hostname"] = hostname
		d["timestamp"] = time.Now()

		jsonBytes, err := json.Marshal(d)
		if err != nil {
			glog.Warning(err)
			return records, nil
		}

		glog.V(2).Infof("%s", string(jsonBytes))
		r := &firehose.Record{Data: jsonBytes}
		records = append(records, r)
	}

	return records, nil
}
