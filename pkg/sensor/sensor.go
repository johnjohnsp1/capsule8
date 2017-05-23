package sensor

import (
	"reflect"

	"github.com/capsule8/reactive8/pkg/api/event"
)

// Receives a Subscription
// If Node filter doesn't match self, bail
// Configure Process Monitor based on the Process Filter
// Configure Sensors based on the Selectors

func NewSensor(selector event.Selector, events chan<- *event.Event) (chan<- struct{}, <-chan error) {
	sensorStop := make(chan struct{})
	sensorErrors := make(chan error)

	//
	// Create sensors for each subselector in selector
	//
	stopChannels := []chan<- struct{}{}
	errorChannels := []<-chan error{}

	if selector.Chargen != nil {
		stopChannel, errorChannel := NewChargenSensor(selector, events)

		stopChannels = append(stopChannels, stopChannel)
		errorChannels = append(errorChannels, errorChannel)
	}

	if selector.Container != nil {
		stopChannel, errorChannel := NewContainerSensor(selector, events)

		stopChannels = append(stopChannels, stopChannel)
		errorChannels = append(errorChannels, errorChannel)
	}

	if selector.Ticker != nil {
		stopChannel, errorChannel := NewTickerSensor(selector, events)

		stopChannels = append(stopChannels, stopChannel)
		errorChannels = append(errorChannels, errorChannel)
	}

	// and so on...

	//
	// If we have successfully created all of our channels, then create
	// a goroutine to consume events from all of them and relay them over
	// our output channel.
	//
	go func() {
		cases := make([]reflect.SelectCase, len(errorChannels)+1)

		// The first select case is our stop channel
		cases[0] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(sensorStop),
		}

		// The other cases are subsensor error channels
		for i := 0; i < len(errorChannels); i++ {
			cases[1+i] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(errorChannels[i]),
			}
		}

		activeChannels := len(errorChannels)
		for activeChannels > 0 {
			chosen, recv, recvOK := reflect.Select(cases)
			if recvOK {
				// We received a subsensor error, relay it on our error channel
				sensorErrors <- recv.Interface().(error)
			} else {
				if chosen == 0 {
					// Our stop channel was closed
					for i := 0; i < len(stopChannels); i++ {
						close(stopChannels[i])
					}
					activeChannels = 0
				} else {
					// An error channel was closed, ignore it going forward
					cases[chosen].Chan = reflect.ValueOf(nil)
					activeChannels--
				}
			}
		}

		close(sensorErrors)
	}()

	return sensorStop, sensorErrors
}
