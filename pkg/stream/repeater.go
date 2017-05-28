package stream

import "reflect"

type Repeater struct {
	ctrl chan<- interface{}
}

type repeaterOutput struct {
	ctrl chan interface{}
	data chan interface{}
}

type repeater struct {
	ctrl chan interface{}
	in   *Stream
	out  []repeaterOutput
}

type repeaterNewStream struct {
	reply chan *Stream
}

func (r *repeater) add() *Stream {
	data := make(chan interface{})
	ctrl := make(chan interface{})

	output := repeaterOutput{
		ctrl: ctrl,
		data: data,
	}

	r.out = append(r.out, output)

	s := &Stream{
		Ctrl: ctrl,
		Data: data,
	}

	return s
}

func (r *repeater) remove(out repeaterOutput) bool {
	var t []repeaterOutput

	for i, o := range r.out {
		if o == out {
			t = append(r.out[:i], r.out[i+1:]...)
			break
		}
	}

	if t != nil {
		r.out = t
	}

	return t != nil
}

func (r *repeater) close() {
	for _, e := range r.out {
		close(e.data)
	}

	r.out = nil
}

func (r *repeater) dataHandler(e interface{}) {
	//
	// Use a Select to send to output channels in whatever order they
	// are available to receive. We'll block in this function until all
	// output channels have received the value.
	//

	var cases []reflect.SelectCase

	for _, c := range r.out {
		sc := reflect.SelectCase{
			Dir:  reflect.SelectSend,
			Chan: reflect.ValueOf(c.data),
			Send: reflect.ValueOf(e),
		}
		cases = append(cases, sc)
	}

	outputs := len(cases)
	for outputs > 0 {
		chosen, _, _ := reflect.Select(cases)
		cases[chosen].Chan = reflect.ValueOf(nil)
		outputs--
	}
}

func (r *repeater) controlHandler(m interface{}) {
	switch m.(type) {
	case *repeaterNewStream:
		m := m.(*repeaterNewStream)
		s := r.add()
		m.reply <- s

	default:
		panic("Unknown control message")
	}

}

func (r *repeater) loop() {
	for {
		var selectCases []reflect.SelectCase

		if r.ctrl != nil {
			sc := reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(r.ctrl),
			}
			selectCases = append(selectCases, sc)
		}

		if r.in.Data != nil {
			sc := reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(r.in.Data),
			}
			selectCases = append(selectCases, sc)
		}

		for _, e := range r.out {
			sc := reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(e.ctrl),
			}
			selectCases = append(selectCases, sc)
		}

		if len(selectCases) == 0 {
			// Nothing to do, we're done
			return
		}

		chosen, recv, recvOK := reflect.Select(selectCases)
		if chosen == 0 {
			if recvOK {
				r.controlHandler(recv.Interface())
			} else {
				// Control channel closed, relay it upstream
				close(r.in.Ctrl)
				r.ctrl = nil
			}
		} else if chosen == 1 {
			if recvOK {
				r.dataHandler(recv.Interface())
			} else {
				// Input data channel closed, close output
				// streams.
				r.close()
				r.in.Data = nil
			}
		} else {
			if !recvOK {
				// Close of an output stream control channel
				ctrl := selectCases[chosen].Chan.Interface()
				for _, e := range r.out {
					if e.ctrl == ctrl {
						r.remove(e)
						close(e.data)
						break
					}
				}
			}
		}
	}
}

// NewRepeater creates a controllable repeater for the given stream. New
// streams can be created from the Repeater and later removed.
func NewRepeater(in *Stream) *Repeater {
	control := make(chan interface{})

	go func() {
		r := &repeater{ctrl: control, in: in}

		go r.loop()
	}()

	return &Repeater{
		ctrl: control,
	}
}

func (r *Repeater) NewStream() (*Stream, bool) {
	reply := make(chan *Stream)

	req := &repeaterNewStream{
		reply: reply,
	}

	r.ctrl <- req
	rep, ok := <-reply
	return rep, ok
}

func (r *Repeater) Close() {
	// Closing the control channel signals to the looper to shutdown.
	close(r.ctrl)
}
