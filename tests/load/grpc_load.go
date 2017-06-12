// Load test the gRPC API

package main

import (
	"flag"
	"io"
	"log"
	"sync"
	"time"

	"golang.org/x/net/context"

	event "github.com/capsule8/reactive8/pkg/api/event"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

func main() {
	url := flag.String("u", "localhost:8080", "Hostname & port of gRPC API")
	duration := flag.Int("d", 30, "Duration of the request in seconds")
	numConnections := flag.Int("c", 1, "Number of connections to open up")
	flag.Parse()

	count := 0
	errcount := 0

	errcounts := make(chan int)
	counts := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < *numConnections; i++ {
		wg.Add(1)
		go runConnection(*url, *duration, counts, errcounts, &wg, i+1)
	}
	go func() {
		for {
			count += <-counts
			errcount += <-errcounts

		}
	}()
	wg.Wait()
	log.Printf("count: %d errors: %d count/s: %d\n", count, errcount, count / *duration)
}

func runConnection(url string, duration int, counts chan int, errcounts chan int, wg *sync.WaitGroup, num int) {
	conn, err := grpc.Dial(url, grpc.WithInsecure())
	if err != nil {
		grpclog.Fatal("Failed to connect to grpc server:", err)
		// Skip the program and go straight to done
		goto done
	}

	c := event.NewEventServiceClient(conn)
	stream, err := c.ReceiveEvents(context.Background())
	// Request full blast
	stream.Send(&event.ReceiveEventsRequest{
		Subscription: &event.Subscription{
			Selector: &event.Selector{
				Chargen: &event.ChargenEventSelector{
					Length: uint32(num),
				},
			},
		},
	})

	count := 0
	errcount := 0
	go func() {
		<-time.After(time.Duration(duration) * time.Second)
		stream.CloseSend()
	}()
	for {
		ev, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			errcount++
			continue
		}
		stream.Send(&event.ReceiveEventsRequest{
			Acks: [][]byte{
				ev.Ack,
			},
		})
		count++
	}

	counts <- count
	errcounts <- errcount
done:
	wg.Done()
}
