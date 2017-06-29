package sensor

import (
	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/stream"
)

//
// Sensors are singletons that emit events through one or more sessions
// configured by a event.Selector.
//

type chargen struct {
	ctrl    chan interface{}
	data    chan interface{}
	filter  *event.ChargenEventFilter
	chargen *stream.Stream
	index   uint64
}

func newChargenEvent(index uint64, characters string) *event.Event {
	return &event.Event{
		Event: &event.Event_Chargen{
			Chargen: &event.ChargenEvent{
				Index:      index,
				Characters: characters,
			},
		},
	}
}

func (c *chargen) emitNextEvent(e interface{}) {
	length := c.filter.Length
	payload := make([]byte, length)

	i := c.index % uint64(length)
	str := e.(string)
	payload[i] = str[0]

	c.index++

	if (c.index % uint64(length)) == 0 {
		c.data <- newChargenEvent(c.index, string(payload))
	}
}

// NewChargenSensor creates a new chargen sensor configured by the given
// Filter
func NewChargenSensor(filter *event.ChargenEventFilter) (*stream.Stream, error) {
	//
	// Each call to New creates a new session with the Sensor. It is the
	// Sensor's responsibility to handle all of its sessions in the most
	// high-performance way possible. For example, a Sensor may install
	// kernel probes for the union of all sessions, but then demux the
	// results through individual goroutines forwarding events over
	// their own channels.
	//

	c := &chargen{
		ctrl:    make(chan interface{}),
		data:    make(chan interface{}),
		filter:  filter,
		chargen: stream.Chargen(),
		index:   0,
	}

	go func() {
		for {
			select {
			case _, ok := <-c.ctrl:
				if !ok {
					close(c.data)
					return
				}

			case e, ok := <-c.chargen.Data:
				if ok {
					c.emitNextEvent(e)
				} else {
					return
				}
			}
		}
	}()

	return &stream.Stream{
		Ctrl: c.ctrl,
		Data: c.data,
	}, nil
}
