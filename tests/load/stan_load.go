// Load test a STAN server

package main

import (
	"flag"
	"log"
	"time"

	"github.com/tylertreat/bench"
	"github.com/tylertreat/bench/requester"
)

func main() {
	// Parse cli args
	natsURL := flag.String("nats", "nats://localhost:4222", "url of NATS server")
	subject := flag.String("sub", "bench-test", "NATS subject")
	// For reference, surfing the web = ~ 9000/s | kernel compile peaks ~ 6000/s
	requestRate := flag.Uint64("r", 5000, "rate of requests")
	connections := flag.Uint64("c", 1, "number of connections")
	burstRate := flag.Uint64("b", 0, "burst rate")
	duration := flag.Uint64("d", 30, "duration in seconds")
	// For reference, a `ChargenEvent` comes out to 24 bytes.
	payloadSize := flag.Int("p", 100, "payload size in bytes")
	flag.Parse()

	r := &requester.NATSStreamingRequesterFactory{
		PayloadSize: *payloadSize,
		Subject:     *subject,
		ClientID:    "bench",
		URL:         *natsURL,
	}
	benchmark := bench.NewBenchmark(
		r,
		*requestRate,
		*connections,
		time.Duration(*duration)*time.Second,
		*burstRate,
	)

	summary, err := benchmark.Run()
	if err != nil {
		log.Fatal(err)
	}

	log.Println(summary)
	summary.GenerateLatencyDistribution(bench.Logarithmic, "stan-bench.txt")
}
