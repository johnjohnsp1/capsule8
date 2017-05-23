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
		i := <-subscriber
		received = i.(int)
	}()

	publisher <- sent

	time.Sleep(sleepDelay)

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

	var n1 int
	go func() {
		i := <-subscriber1
		n1 = i.(int)
	}()

	var n2 int
	go func() {
		i := <-subscriber2
		n2 = i.(int)
	}()

	publisher <- rand.Int()

	time.Sleep(sleepDelay)

	if n1 != n2 {
		t.Errorf("Expected both subscribers to get same value, got %d/%d", n1, n2)
	}
}

func TestRemoveSubscriber(t *testing.T) {
	done := make(chan struct{})
	defer close(done)

	publisher := make(chan interface{})
	subscriber := make(chan interface{}, 10)

	repeater := NewRepeater(done, publisher)
	subscriberID := repeater.AddSubscriber(subscriber)

	var n int
	go func() {
		i := <-subscriber
		n = i.(int)
	}()

	// Remove the subscriber before publishing to the repeater
	repeater.RemoveSubscriber(subscriberID)

	publisher <- rand.Int()

	time.Sleep(sleepDelay)

	if n != 0 {
		t.Error("Expected removed subscriber to not receive published value")
	}
}
