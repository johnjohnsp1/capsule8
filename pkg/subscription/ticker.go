package subscription

import (
	"time"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/capsule8/pkg/stream"
)

//
// Sensors are singletons that emit events through one or more sessions
// configured by a api.Selector.
//

type ticker struct {
	ctrl     chan interface{}
	data     chan interface{}
	filter   *api.TickerEventFilter
	duration time.Duration
	ticker   *stream.Stream
}

func newTickerEvent(tick time.Time) *api.Event {
	e := NewEvent()
	e.Event = &api.Event_Ticker{
		Ticker: &api.TickerEvent{
			Seconds:     tick.Unix(),
			Nanoseconds: tick.UnixNano(),
		},
	}

	return e
}

// NewTickerSensor creates a new ticker sensor configured by the given Selector
func NewTickerSensor(filter *api.TickerEventFilter) (*stream.Stream, error) {
	//
	// Each call to New creates a new session with the Sensor. It is the
	// Sensor's responsibility to handle all of its sessions in the most
	// high-performance way possible. For example, a Sensor may install
	// kernel probes for the union of all sessions, but then demux the
	// results through individual goroutines forwarding events over
	// their own channels.
	//

	duration := time.Duration(filter.Interval)
	t := &ticker{
		ctrl:     make(chan interface{}),
		data:     make(chan interface{}),
		filter:   filter,
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
					ev := newTickerEvent(tick)
					t.data <- ev

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
