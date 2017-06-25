package sensor

import (
	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/stream"
)

// Receives a Subscription
// If Node filter doesn't match self, bail
// Configure Process Monitor based on the Process Filter
// Configure Sensors based on the Selectors

func NewSensor(sub *event.Subscription) (*stream.Stream, error) {
	streams := make([]*stream.Stream, 0)

	if sub.Events != nil {
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
				s, err := NewContainerSensor(sub)
				if err != nil {
					return nil, err
				}
				streams = append(streams, s)

			case *event.EventFilter_Process:
				s, err := NewProcessSensor(sub)
				if err != nil {
					return nil, err
				}
				streams = append(streams, s)

				// and so on...
			}
		}
	} else {
		//
		// Enable all non-debugging sensors
		//
		s, err := NewContainerSensor(sub)
		if err != nil {
			return nil, err
		}
		streams = append(streams, s)

		s, err = NewProcessSensor(sub)
		if err != nil {
			return nil, err
		}
		streams = append(streams, s)
	}

	//
	// Return a single output stream consisting of all sensor streams.
	//
	s := stream.Join(streams...)

	if sub.Modifier != nil {
		return applyModifiers(s, *sub.Modifier), nil
	}

	return s, nil
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
