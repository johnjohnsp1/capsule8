// Copyright 2017 Capsule8, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sensor

import (
	api "github.com/capsule8/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/stream"
)

type chargen struct {
	ctrl    chan interface{}
	data    chan interface{}
	sensor  *Sensor
	filter  *api.ChargenEventFilter
	chargen *stream.Stream
	index   uint64
	length  uint64
	payload []byte
}

func (c *chargen) newChargenEvent(index uint64, characters string) *api.Event {
	e := c.sensor.NewEvent()
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
		c.data <- c.newChargenEvent(c.index, string(c.payload))
	}
}

func newChargenSource(sensor *Sensor, filter *api.ChargenEventFilter) (*stream.Stream, error) {
	// Each call to New creates a new session with the Sensor. It is the
	// Sensor's responsibility to handle all of its sessions in the most
	// high-performance way possible. For example, a Sensor may install
	// kernel probes for the union of all sessions, but then demux the
	// results through individual goroutines forwarding events over
	// their own channels.

	c := &chargen{
		ctrl:    make(chan interface{}),
		data:    make(chan interface{}),
		sensor:  sensor,
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
				if !ok {
					return
				}
				c.emitNextEvent(e)
			}
		}
	}()

	return &stream.Stream{
		Ctrl: c.ctrl,
		Data: c.data,
	}, nil
}
