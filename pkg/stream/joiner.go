package stream

import "reflect"

type Joiner struct {
	ctrl chan<- interface{}
}

type joiner struct {
	ctrl chan interface{}
	data chan interface{}
	in   []*Stream
}

type joinerAddStream struct {
	stream *Stream
	reply  chan<- bool
}

type joinerRemoveStream struct {
	stream *Stream
	reply  chan<- bool
}

func (j *joiner) add(s *Stream) bool {
	j.in = append(j.in, s)
	return true
}

func (j *joiner) remove(s *Stream) bool {
	var t []*Stream

	for i, in := range j.in {
		if in == s {
			t = append(j.in[:i], j.in[i+1:]...)
			break
		}
	}

	if t != nil {
		j.in = t
	}

	return t != nil
}

func (j *joiner) close() {
	// Close upstream control channels to relay close signal
	for _, in := range j.in {
		close(in.Ctrl)
	}
}

func (j *joiner) controlHandler(m interface{}) {
	switch m.(type) {
	case *joinerAddStream:
		m := m.(*joinerAddStream)
		s := j.add(m.stream)
		m.reply <- s

	case *joinerRemoveStream:
		m := m.(*joinerRemoveStream)
		s := j.remove(m.stream)
		m.reply <- s
	}

}

func (j *joiner) loop() {
	for {
		//
		// XXX: It is redundant to recreate the SelectCases on each
		// loop iteration when the number of children streams hasn't
		// changed. We should only modify it on control messages
		// and child stream channel closures instead.
		//

		var selectCases []reflect.SelectCase

		if j.ctrl != nil {
			sc := reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(j.ctrl),
			}
			selectCases = append(selectCases, sc)
		}

		for _, e := range j.in {
			sc := reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(e.Data),
			}
			selectCases = append(selectCases, sc)
		}

		if len(selectCases) == 0 {
			// Nothing to do, we're done
			break
		}

		chosen, recv, recvOK := reflect.Select(selectCases)
		sc := &selectCases[chosen]

		if sc.Chan.Interface() == j.ctrl {
			if recvOK {
				j.controlHandler(recv.Interface())
			} else {
				// Control channel closed, relay upstream
				j.close()
				j.ctrl = nil
			}
		} else {
			if recvOK {
				j.data <- recv.Interface()
			} else {
				// Close of an input stream data channel
				data := sc.Chan.Interface()
				for _, e := range j.in {
					if e.Data == data {
						j.remove(e)
						break
					}
				}
			}
		}
	}

	// Close our downstream data channel since there won't be any
	// more elements sent over it.
	close(j.data)
}

// NewJoiner creates a controllable joiner for the given stream. New
// streams can be created from the Joiner and later removed.
func NewJoiner() (*Stream, *Joiner) {
	ctrl := make(chan interface{})
	data := make(chan interface{})

	go func() {
		r := &joiner{ctrl: ctrl, data: data}

		go r.loop()
	}()

	s := &Stream{
		Ctrl: ctrl,
		Data: data,
	}

	return s, &Joiner{
		ctrl: ctrl,
	}
}

func (J *Joiner) Add(s *Stream) bool {
	reply := make(chan bool)

	req := &joinerAddStream{
		stream: s,
		reply:  reply,
	}

	J.ctrl <- req
	ok := <-reply
	return ok
}

func (J *Joiner) Remove(s *Stream) bool {
	reply := make(chan bool)

	req := &joinerRemoveStream{
		stream: s,
		reply:  reply,
	}

	J.ctrl <- req
	ok := <-reply
	return ok
}

func (J *Joiner) On() {
	J.ctrl <- true
}

func (J *Joiner) Off() {
	J.ctrl <- false
}

func (J *Joiner) Close() {
	// Closing the control channel signals the loop to shutdown.
	close(J.ctrl)
}
