package subscription

import (
	api "github.com/capsule8/api/v0"
	"github.com/capsule8/capsule8/pkg/stream"
)

//
// Sensors are singletons that emit events through one or more sessions
// configured by a api.Subscription.
//

type chargen struct {
	ctrl    chan interface{}
	data    chan interface{}
	filter  *api.ChargenEventFilter
	chargen *stream.Stream
	index   uint64
	length  uint64
	payload []byte
}

func newChargenEvent(index uint64, characters string) *api.Event {
	e := NewEvent()
	e.Event = &api.Event_Chargen{
		Chargen: &api.ChargenEvent{
			Index:      index,
			Characters: characters,
		},
	}

	return e
}

func (c *chargen) emitNextEvent(e interface{}) {
	i := c.index % uint64(c.length)
	str := e.(string)
	c.payload[i] = str[0]

	c.index++

	if (c.index % uint64(c.length)) == 0 {
		c.data <- newChargenEvent(c.index, string(c.payload))
	}
}

// NewChargenSensor creates a new chargen sensor configured by the given
// Filter
func NewChargenSensor(filter *api.ChargenEventFilter) (*stream.Stream, error) {
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
		length:  filter.Length,
		payload: make([]byte, filter.Length),
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
