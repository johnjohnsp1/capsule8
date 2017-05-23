package util

import (
	"math/rand"
	"sync"
)

type repeater struct {
	mutex       sync.Mutex
	publisher   <-chan interface{}
	subscribers map[int]chan<- interface{}
}

type Repeater interface {
	AddSubscriber(chan<- interface{}) int
	RemoveSubscriber(int)
}

// NewRepeater creates a new Repeater from the given event publisher channel
func NewRepeater(done <-chan struct{}, publisher <-chan interface{}) Repeater {
	r := &repeater{}
	r.publisher = publisher
	r.subscribers = make(map[int]chan<- interface{})

	go func() {
		defer func() {
			r.mutex.Lock()
			for i := range r.subscribers {
				close(r.subscribers[i])
			}
		}()

		// Receive from publisher events channel
		for ev := range publisher {

			r.mutex.Lock()
			// Non-blocking send to each subscriber events channel
			for i := range r.subscribers {
				select {
				case r.subscribers[i] <- ev:

				case <-done:
					return

				default:
					// If send would have blocked, drop event for that
					// subscriber and update drop count.
				}
			}
			r.mutex.Unlock()
		}
	}()

	return r
}

func (r *repeater) AddSubscriber(subscriber chan<- interface{}) int {
	r.mutex.Lock()

	subscriberID := rand.Int()
	for r.subscribers[subscriberID] != nil {
		subscriberID = rand.Int()
	}

	r.subscribers[subscriberID] = subscriber

	r.mutex.Unlock()

	return subscriberID
}

func (r *repeater) RemoveSubscriber(subscriberID int) {
	r.mutex.Lock()

	r.subscribers[subscriberID] = nil

	r.mutex.Unlock()
}
