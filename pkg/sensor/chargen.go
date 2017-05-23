package sensor

import (
	"strings"

	"github.com/capsule8/reactive8/pkg/api/event"
)

//
// Sensors are singletons that emit events through one or more sessions
// configured by a event.Selector.
//

// NewChargenSensor creates a new chargen sensor configured by the given Selector
func NewChargenSensor(selector event.Selector, events chan<- *event.Event) (chan<- struct{}, <-chan error) {
	//
	// Each call to New creates a new session with the Sensor. It is the
	// Sensor's responsibility to handle all of its sessions in the most
	// high-performance way possible. For example, a Sensor may install
	// kernel probes for the union of all sessions, but then demux the
	// results through individual goroutines forwarding events over
	// their own channels.
	//

	//
	// Sensors return a signal channel in order for the caller to signal
	// that the sensor should shut down by closing it.
	//
	sensorStop := make(chan struct{})

	//
	// Since we run primarily in a goroutine, we use a channel to
	// report errors, even if they occur before starting the goroutine.
	//
	sensorErrors := make(chan error)

	go func() {
		index := uint64(0)
		length := int(selector.Chargen.Length)

		for {
			select {
			case <-sensorStop:
				close(sensorErrors)
				return

			default:
				events <- &event.Event{
					Subevent: &event.Event_Chargen{
						Chargen: &event.ChargenEvent{
							Index:      index,
							Characters: strings.Repeat("A", length),
						},
					},
				}

				index++
			}
		}
	}()

	return sensorStop, sensorErrors
}
