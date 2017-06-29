package main

import (
	"os"
	"sync"
	"testing"

	"github.com/capsule8/reactive8/pkg/api/config"
	"github.com/capsule8/reactive8/pkg/client"
)

var c client.Client

func TestMain(m *testing.M) {
	c, _ = client.CreateClient("localhost:8080")
	os.Exit(m.Run())
}

func TestCreateConfig(t *testing.T) {
	err := c.CreateConfig("config.TESTSERVICE", &config.Config{
		Payload: []byte("fake config"),
	})
	if err != nil {
		t.Error("Failed creating config:", err)
	}
}

func TestGetConfig(t *testing.T) {
	cfg, err := c.GetConfig("config.TESTSERVICE")
	if err != nil {
		t.Error("Failed creating config:", err)
	}
	t.Log("Successfully got config:", cfg)
}

func TestWatchConfig(t *testing.T) {
	configs, closeSignal, err := c.WatchConfig("config.TESTSERVICE")
	if err != nil {
		t.Error("Watch config failed:", err)
	}

	total := 3
	count := 0
	signal := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(total)

	go func() {
		for {
			if count == total {
				close(signal)
				break
			}
			<-configs

			count++
			wg.Done()
			// Send another create
			signal <- struct{}{}
		}
	}()

	// We push `total-1` creates here because there is already a config sitting in the channel
	for i := 0; i < total-1; i++ {
		// Wait for create signal before going
		<-signal
		c.CreateConfig("config.TESTSERVICE", &config.Config{
			Payload: []byte("fake config"),
		})
	}

	wg.Wait()
	close(closeSignal)
	t.Log("Received all creates successfully.")
}
