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

func NewSensor(sub *event.Subscription) (*stream.Stream, error) {
	streams := make([]*stream.Stream, 0)

	for _, ef := range sub.Events {
		//
		// Create streams for each subselector in selector
		//
		switch ef.Filter.(type) {
		case *event.EventFilter_Chargen:
			s, err := NewChargenSensor(ef.GetChargen())
			if err != nil {
				return nil, err
			}
			streams = append(streams, s)

		case *event.EventFilter_Ticker:
			s, err := NewTickerSensor(ef.GetTicker())
			if err != nil {
				return nil, err
			}
			streams = append(streams, s)

		case *event.EventFilter_Container:
			s, err := container.NewSensor(sub)
			if err != nil {
				return nil, err
			}
			streams = append(streams, s)

			// and so on...
		}
	}

	//
	// Return a single output stream consisting of all sensor streams.
	//
	return applyModifiers(stream.Join(streams...), *sub.Modifier), nil
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
