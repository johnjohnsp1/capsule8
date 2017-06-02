// Package stream implements lazy functional streams using Go channels.
//
// Streams consist of a single generator, zero or more operators, and a
// single terminator. Every stream shares a stop channel, which the owner
// may use to cleanly shut down the entire stream.
//
// Streams exploit Go's lightweight channels and goroutines in order to
// implement concurrency without locking shared data structures.
//
// This is intentionally reminiscent of the Java 8 Streams API
//
package stream

import (
	"math"
	"reflect"
	"time"
	"unicode/utf8"
)

const dataChannelBufferSize = 32

// Stream represents a stream consisting of a generator, zero or more
// operators, and zero or one terminators. Consumers receive stream elements
// from the Data channel and may terminate the stream by closing the Ctrl
// channel.
type Stream struct {
	// Ctrl is the control channel to the stream. Consumers may close
	// it to shut down the stream.
	Ctrl chan<- interface{}

	// Data is the data channel of the stream. Consumers may receive
	// from it to consume stream elements.
	Data <-chan interface{}
}

// Close terminates the stream
func (s *Stream) Close() {
	close(s.Ctrl)
}

// Next receives the next element in the stream. It returns that element
// and a boolean indicating whether it was received successfully or whether
// the stream was closed.
func (s *Stream) Next() (interface{}, bool) {
	e, ok := <-s.Data
	return e, ok
}

// ----------------------------------------------------------------------------
// Generators return an output stream
// ----------------------------------------------------------------------------

// Generators wait on the control channel for it to be closed. Since the entire
// stream shares the same control channel, any downstream consumer can close it
// and shut the entire stream down. The generator should be the only component
// of the stream waiting on it, so that it can cleanly close the stream from
// the source.

// Null creates a generator that doesn't create any output elements.
func Null() *Stream {
	ctrl := make(chan interface{})
	data := make(chan interface{}, dataChannelBufferSize)

	go func() {
		defer close(data)

		for {
			select {
			case _, ok := <-ctrl:
				if !ok {
					return
				}
			}
		}
	}()

	return &Stream{
		Ctrl: ctrl,
		Data: data,
	}
}

// Chargen creates a generator that produces a stream of single-character
// strings from a pattern reminiscent of RFC864 chargen TCP/UDP services.
func Chargen() *Stream {
	ctrl := make(chan interface{})
	data := make(chan interface{}, dataChannelBufferSize)

	go func() {
		defer close(data)

		firstChar := byte(' ')
		lastChar := byte('~')

		c := firstChar

		for {
			select {
			case _, ok := <-ctrl:
				if !ok {
					return
				}
			default:
				r, _ := utf8.DecodeRune([]byte{c})
				data <- string(r)
				c++
				if c > lastChar {
					c = firstChar
				}
			}
		}

	}()

	return &Stream{
		Ctrl: ctrl,
		Data: data,
	}
}

// Iota creates a generator that produces the given count of int elements
// (or infinite, if not specified). Additional optional arguments specify
// the start and step value.
func Iota(args ...uint64) *Stream {
	ctrl := make(chan interface{})
	data := make(chan interface{}, dataChannelBufferSize)

	go func() {
		defer close(data)

		var (
			count uint64 = math.MaxUint64
			start uint64
			step  uint64 = 1
		)

		if len(args) > 0 {
			count = args[0]
		}
		if len(args) > 1 {
			start = args[1]
		}
		if len(args) > 2 {
			step = args[2]
		}

		var value = start
		for count > 0 {
			select {
			case _, ok := <-ctrl:
				if !ok {
					return
				}

			case data <- value:
			}

			count--
			value += step
		}
	}()

	return &Stream{
		Ctrl: ctrl,
		Data: data,
	}
}

// Ticker creates a generator that produces a time.Time element every
// 'tick' of the specified duration.
func Ticker(d time.Duration) *Stream {
	ctrl := make(chan interface{})
	data := make(chan interface{}, dataChannelBufferSize)

	go func() {
		defer close(data)

		ticker := time.NewTicker(d)

		for {
			select {
			case _, ok := <-ctrl:
				if !ok {
					ticker.Stop()
					return
				}

			case tick, ok := <-ticker.C:
				if ok {
					data <- tick
				} else {
					return
				}
			}
		}
	}()

	return &Stream{
		Ctrl: ctrl,
		Data: data,
	}
}

// ----------------------------------------------------------------------------
// Operators take an input stream and return an output stream
// ----------------------------------------------------------------------------

//
// Operators inherit their control and data channels from upstream. Unlike
// generators, they do not wait on the control channel. When their upstream
// data channel closes, the operator closes its downstream data channel also.
//

type DoFunc func(interface{})

// Do adds an operator in the stream that calls the given function for
// every element.
func Do(in *Stream, f DoFunc) *Stream {
	data := make(chan interface{}, dataChannelBufferSize)

	go func() {
		defer close(data)

		for {
			select {
			case e, ok := <-in.Data:
				if ok {
					f(e)
					data <- e
				} else {
					return
				}
			}
		}
	}()

	return &Stream{
		Ctrl: in.Ctrl,
		Data: data,
	}
}

type MapFunc func(interface{}) interface{}

// Map adds an operator in the stream that calls the given function for
// every element and forwards along the returned value.
func Map(in *Stream, f MapFunc) *Stream {
	data := make(chan interface{}, dataChannelBufferSize)

	go func() {
		defer close(data)

		for {
			select {
			case e, ok := <-in.Data:
				if ok {
					data <- f(e)
				} else {
					return
				}
			}
		}
	}()

	return &Stream{
		Ctrl: in.Ctrl,
		Data: data,
	}
}

type FilterFunc func(interface{}) bool

func Filter(in *Stream, f FilterFunc) *Stream {
	data := make(chan interface{}, dataChannelBufferSize)

	go func() {
		defer close(data)

		for {
			select {
			case e, ok := <-in.Data:
				if ok {
					if f(e) {
						data <- e
					}
				} else {
					return
				}
			}
		}
	}()

	return &Stream{
		Ctrl: in.Ctrl,
		Data: data,
	}
}

// Join combines multiple input Streams into a single output Stream. Closing
// the Join closes the input streams as well.
func Join(in ...*Stream) *Stream {
	ctrl := make(chan interface{})
	data := make(chan interface{}, dataChannelBufferSize)

	go func() {
		cases := make([]reflect.SelectCase, len(in)+1)

		cases[0] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ctrl),
		}

		for i := range in {
			// Data channels are odd
			cases[1+i] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(in[i].Data),
			}
		}

		activeInputs := len(in)
		for activeInputs > 0 {
			chosen, recv, recvOK := reflect.Select(cases)
			if chosen > 0 && recvOK {
				// Input element received
				data <- recv.Interface()
			} else if chosen == 0 && !recvOK {
				// Control channel closed, relay upstream
				for i := range in {
					close(in[i].Ctrl)
				}
				cases[chosen].Chan = reflect.ValueOf(nil)
			} else {
				// Input stream closed
				cases[chosen].Chan = reflect.ValueOf(nil)
				activeInputs--
			}
		}

		close(data)
	}()

	return &Stream{
		Ctrl: ctrl,
		Data: data,
	}
}

// DemuxFunc takes an input element and returns an output index to send it to.
// It may return a negative index in order to signal that the Demux operator
// should drop the element instead of forwarding it to an output stream.

/*
type DemuxFunc func(interface{}) int

func Demux(done <-chan struct{}, in <-chan interface{}, n uint, f DemuxFunc) {

}
*/

// Split splits the input stream by the given filter function
func Split(in *Stream, f FilterFunc) (*Stream, *Stream) {
	dataTrue := make(chan interface{}, dataChannelBufferSize)
	dataFalse := make(chan interface{}, dataChannelBufferSize)

	go func() {
		defer close(dataTrue)
		defer close(dataFalse)

		for {
			select {
			case e, ok := <-in.Data:
				if ok {
					if f(e) {
						dataTrue <- e
					} else {
						dataFalse <- e
					}
				} else {
					return
				}
			}
		}
	}()

	streamTrue := &Stream{
		Ctrl: in.Ctrl,
		Data: dataTrue,
	}
	streamFalse := &Stream{
		Ctrl: in.Ctrl,
		Data: dataFalse,
	}

	return streamTrue, streamFalse
}

// Tee copies the input stream to a pair of output streams (like a T-shaped
// junction)
func Tee(in *Stream) (*Stream, *Stream) {
	dataOne := make(chan interface{}, dataChannelBufferSize)
	dataTwo := make(chan interface{}, dataChannelBufferSize)

	go func() {
		defer close(dataOne)
		defer close(dataTwo)

		for {
			select {
			case e, ok := <-in.Data:
				if ok {
					dataOne <- e
					dataTwo <- e
				} else {
					return
				}
			}
		}
	}()

	streamOne := &Stream{
		Ctrl: in.Ctrl,
		Data: dataOne,
	}
	streamTwo := &Stream{
		Ctrl: in.Ctrl,
		Data: dataTwo,
	}

	return streamOne, streamTwo
}

// Copy duplicates elements from the input stream into the given number of
// output streams.
func Copy(in *Stream, n int) []*Stream {
	data := make([]chan interface{}, n)
	out := make([]*Stream, n)

	for i := 0; i < n; i++ {
		data[i] = make(chan interface{}, dataChannelBufferSize)

		out[i] = &Stream{
			Ctrl: in.Ctrl,
			Data: data[i],
		}
	}

	go func() {
		for {
			select {
			case e, ok := <-in.Data:
				if ok {
					for _, d := range data {
						d <- e
					}
				} else {
					for _, d := range data {
						close(d)
					}

					return
				}
			}
		}
	}()

	return out
}

// Buffer stores up to the given number of elements from the input Stream
// before blocking
func Buffer(in *Stream, size int) *Stream {
	data := make(chan interface{}, size)

	go func() {
		defer close(data)

		for {
			select {
			case e, ok := <-in.Data:
				if ok {
					data <- e
				} else {
					return
				}
			}
		}
	}()

	return &Stream{
		Ctrl: in.Ctrl,
		Data: data,
	}
}

// Overflow drops elements instead of blocking on the output Stream
func Overflow(in *Stream) *Stream {
	data := make(chan interface{})

	go func() {
		defer close(data)

		for {
			select {
			case e, ok := <-in.Data:
				if ok {
					// Non-blocking send or drop
					select {
					case data <- e:
					default:
					}
				} else {
					return
				}
			}
		}
	}()

	return &Stream{
		Ctrl: in.Ctrl,
		Data: data,
	}
}

// OnOffValve adds a simple on/off valve operator onto the stream. It
// defaults to off to prevent a race condition from accepting input before
// being able to be switched off.
func OnOffValve(in *Stream) (*Stream, chan<- bool) {
	ctrl := make(chan bool)
	data := make(chan interface{})

	go func() {
		defer close(data)

		//
		// We default to off to prevent a race condition among our
		// control channel and input stream in the case that closed is
		// the desired initial state for the valve.
		//
		cases := []reflect.SelectCase{
			{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(ctrl),
			},
			{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(nil),
			},
		}

		for {
			chosen, recv, recvOK := reflect.Select(cases)
			if chosen == 0 {
				if recvOK {
					enable := recv.Interface().(bool)

					var v interface{}
					if enable {
						v = in.Data
					} else {
						v = nil
					}
					cases[1].Chan = reflect.ValueOf(v)
				} else {
					// Closing the control channel leaves
					// the valve in its current state
					// permanently.
					cases[0].Chan = reflect.ValueOf(nil)
				}
			} else {
				if recvOK {
					data <- recv.Interface()
				} else {
					return
				}
			}
		}
	}()

	s := &Stream{
		Ctrl: in.Ctrl,
		Data: data,
	}

	return s, ctrl
}

// ----------------------------------------------------------------------------
// Terminators accept an input stream and return a terminal value. They are
// typically used to aggregate a value over the entire stream.
// ----------------------------------------------------------------------------

type ReduceFunc func(interface{}, interface{}) interface{}

// Reduce adds a terminator onto the stream that accumulates a value using
// the given function and then returns it once the input stream terminates.
func Reduce(in *Stream, initVal interface{}, f ReduceFunc) chan interface{} {
	data := make(chan interface{}, dataChannelBufferSize)

	go func() {
		acc := initVal
		for {
			select {
			case e, ok := <-in.Data:
				if ok {
					acc = f(acc, e)
				} else {
					data <- acc
					close(data)
					return
				}
			}
		}
	}()

	return data
}

// ForEach adds a terminator onto the stream that consumes each element and
// runs the given function on them.
func ForEach(in *Stream, f DoFunc) chan interface{} {
	done := make(chan interface{})

	go func() {
		for {
			select {
			case e, ok := <-in.Data:
				if ok {
					f(e)
				} else {
					close(done)
					return
				}
			}
		}
	}()

	return done
}

// Wait adds a terminator onto the stream that throws away all elements and
// just waits for stream termination.
func Wait(in *Stream) chan interface{} {
	done := make(chan interface{})

	go func() {
		for {
			select {
			case _, ok := <-in.Data:
				if !ok {
					close(done)
					return
				}
			}
		}
	}()

	return done
}
