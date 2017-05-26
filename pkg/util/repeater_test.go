package util

import (
	"math/rand"
	"testing"
	"time"
)

const sleepDelay = 10 * time.Millisecond

func TestNoSubscribers(t *testing.T) {
	done := make(chan struct{})
	defer close(done)

	publisher := make(chan interface{})

	_ = NewRepeater(done, publisher)

	publisher <- 0
	publisher <- 1
	publisher <- 2

	// Test succeeds if the channel sends above did not block
}

func TestOneSubscriber(t *testing.T) {
	done := make(chan struct{})
	defer close(done)

	publisher := make(chan interface{})
	subscriber := make(chan interface{}, 10)

	repeater := NewRepeater(done, publisher)
	repeater.AddSubscriber(subscriber)

	sent := rand.Int()
	var received int

	go func() {
		publisher <- sent
	}()

	i := <-subscriber
	received = i.(int)

	if received != sent {
		t.Errorf("Expected to receive %d, got %d", sent, received)
	}
}

func TestTwoSubscribers(t *testing.T) {
	done := make(chan struct{})
	defer close(done)

	publisher := make(chan interface{})
	subscriber1 := make(chan interface{}, 10)
	subscriber2 := make(chan interface{}, 10)

	repeater := NewRepeater(done, publisher)
	repeater.AddSubscriber(subscriber1)
	repeater.AddSubscriber(subscriber2)

	var n1, n2 int

	go func() {
		publisher <- rand.Int()
	}()

	i := <-subscriber1
	n1 = i.(int)

	i = <-subscriber2
	n2 = i.(int)

	if n1 != n2 {
		t.Errorf("Expected both subscribers to get same value, got %d/%d", n1, n2)
	}
}

func TestRemoveSubscriber(t *testing.T) {
	ticker := time.After(1 * time.Millisecond)
	done := make(chan struct{})
	defer close(done)

	publisher := make(chan interface{})
	subscriber := make(chan interface{}, 10)

	repeater := NewRepeater(done, publisher)
	subscriberID := repeater.AddSubscriber(subscriber)

	go func() {
		// Remove the subscriber before publishing to the repeater
		repeater.RemoveSubscriber(subscriberID)

		publisher <- rand.Int()
	}()

	select {
	case <-subscriber:
		t.Error("Expected removed subscriber to not receive published value")
	case <-ticker:
	}
}
