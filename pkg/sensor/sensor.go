package sensor

import (
	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/container"
	"github.com/capsule8/reactive8/pkg/stream"
)

// Receives a Subscription
// If Node filter doesn't match self, bail
// Configure Process Monitor based on the Process Filter
// Configure Sensors based on the Selectors

func NewSensor(selector event.Selector, modifier event.Modifier) (*stream.Stream, error) {
	streams := make([]*stream.Stream, 0)

	//
	// Create streams for each subselector in selector
	//
	if selector.Chargen != nil {
		s, err := NewChargenSensor(selector)
		if err != nil {
			return nil, err
		}
		streams = append(streams, s)
	}

	if selector.Container != nil {
		s, err := container.NewSensor(selector)
		if err != nil {
			return nil, err
		}
		streams = append(streams, s)
	}

	if selector.Ticker != nil {
		s, err := NewTickerSensor(selector)
		if err != nil {
			return nil, err
		}
		streams = append(streams, s)
	}

	// and so on...

	//
	// Return a single output stream consisting of all sensor streams.
	//
	return applyModifiers(stream.Join(streams...), modifier), nil
}

func applyModifiers(strm *stream.Stream, modifier event.Modifier) *stream.Stream {
	if modifier.Throttle != nil {
		strm = stream.Throttle(strm, *modifier.Throttle)
	}

	if modifier.Limit != nil {
		strm = stream.Limit(strm, *modifier.Limit)
	}

	return strm
}
