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

func NewSensor(selector event.Selector) (*stream.Stream, error) {
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
		s, err := container.NewContainerSensor(selector)
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
	return stream.Join(streams...), nil
}
