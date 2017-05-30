package sensor

import (
	"time"

	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/stream"
)

//
// Sensors are singletons that emit events through one or more sessions
// configured by a event.Selector.
//

type ticker struct {
	ctrl     chan interface{}
	data     chan interface{}
	selector event.Selector
	duration time.Duration
	ticker   *stream.Stream
}

func newTickerEvent(tick time.Time) *event.Event {
	return &event.Event{
		Subevent: &event.Event_Ticker{
			Ticker: &event.TickerEvent{
				Seconds:     tick.Unix(),
				Nanoseconds: tick.UnixNano(),
			},
		},
	}
}

// NewTickerSensor creates a new ticker sensor configured by the given Selector
func NewTickerSensor(selector event.Selector) (*stream.Stream, error) {
	//
	// Each call to New creates a new session with the Sensor. It is the
	// Sensor's responsibility to handle all of its sessions in the most
	// high-performance way possible. For example, a Sensor may install
	// kernel probes for the union of all sessions, but then demux the
	// results through individual goroutines forwarding events over
	// their own channels.
	//

	duration := time.Duration(selector.Ticker.Duration)
	t := &ticker{
		ctrl:     make(chan interface{}),
		data:     make(chan interface{}),
		selector: selector,
		duration: duration,
		ticker:   stream.Ticker(duration),
	}

	go func() {
		for {
			select {
			case _, ok := <-t.ctrl:
				if !ok {
					close(t.data)
					return
				}

			case e, ok := <-t.ticker.Data:
				if ok {
					tick := e.(time.Time)
					t.data <- newTickerEvent(tick)
				} else {
					return
				}
			}
		}
	}()

	return &stream.Stream{
		Ctrl: t.ctrl,
		Data: t.data,
	}, nil
}
