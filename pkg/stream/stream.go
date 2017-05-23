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

type stream struct {
	stop chan struct{}
	elem chan interface{}
}

type Stream interface {
	// Next waits for the next element in the stream and returns it and
	// a boolean indicating whether the element was received OK. A false
	// value for ok indicates that the stream has terminated.
	Next() (e interface{}, ok bool)

	// Channel returns the underlying elements channel for the stream. Using
	// Next() is recommended for most clients, but giving direct access to
	// the channel permits its use in a select among multiple channels.
	Channel() chan interface{}

	// Close the stream and cleanly shut down the upstream generator and operators.
	Close()
}

// NewStream creates a new Stream from the given channel. The channel is
// stored as a receive-only channel in the Stream so that sending on the
// channel is restricted to the caller of NewStream.
func NewStream(elem chan interface{}, stop chan struct{}) Stream {
	if elem == nil {
		elem = make(chan interface{})
	}

	if stop == nil {
		stop = make(chan struct{})
	}

	return &stream{
		stop: stop,
		elem: elem,
	}
}

func (s *stream) Next() (interface{}, bool) {
	select {
	case <-s.stop:
		return nil, false

	case e, ok := <-s.elem:
		return e, ok
	}
}

func (s *stream) Channel() chan interface{} {
	return s.elem
}

func (s *stream) Close() {
	//
	// Channels can only be stopped once without causing a panic. We
	// intentionally don't guard this close to ensure correctness of client
	// code.
	//
	close(s.stop)
}

// ----------------------------------------------------------------------------
// Generators return an output stream
// ----------------------------------------------------------------------------

// Chargen creates a generator that produces a stream of single-character
// strings from a pattern reminiscent of RFC864 chargen TCP/UDP services.
func Chargen() Stream {
	elem := make(chan interface{})
	stop := make(chan struct{})
	s := NewStream(elem, stop)

	go func() {
		defer close(elem)

		firstChar := byte(' ')
		lastChar := byte('~')

		c := firstChar

		for {
			select {
			case <-stop:
				return

			default:
				r, _ := utf8.DecodeRune([]byte{c})
				elem <- string(r)
				c++
				if c > lastChar {
					c = firstChar
				}
			}
		}

	}()

	return s
}

// Iota creates a generator that produces the given count of int elements
// (or infinite, if not specified). Additional optional arguments specify
// the start and step value.
func Iota(args ...uint64) Stream {
	elem := make(chan interface{})
	stop := make(chan struct{})
	s := NewStream(elem, stop)

	go func() {
		defer close(elem)

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

		for i := start; i < count; i += step {
			select {
			case <-stop:
				return
			case elem <- i:
			}
		}
	}()

	return s
}

// Ticker creates a generator that produces a time.Time element every
// 'tick' of the specified duration.
func Ticker(d time.Duration) Stream {
	elem := make(chan interface{})
	stop := make(chan struct{})
	s := NewStream(elem, stop)

	go func() {
		defer close(elem)

		ticker := time.NewTicker(d)

		for {
			select {
			case <-stop:
				ticker.Stop()
				return

			case tick := <-ticker.C:
				elem <- tick
			}
		}
	}()

	return s
}

// ----------------------------------------------------------------------------
// Operators take an input stream and return an output stream
// ----------------------------------------------------------------------------

type DoFunc func(interface{})

// Do adds an operator in the stream that calls the given function for
// every element.
func Do(in Stream, f DoFunc) Stream {
	s := in.(*stream)
	elem := make(chan interface{})
	out := NewStream(elem, s.stop)

	go func() {
		defer close(elem)

		for {
			e, ok := s.Next()
			if ok {
				f(e)
				elem <- e
			} else {
				return
			}
		}
	}()

	return out
}

type MapFunc func(interface{}) interface{}

// Map adds an operator in the stream that calls the given function for
// every element and forwards along the returned value.
func Map(in Stream, f MapFunc) Stream {
	s := in.(*stream)
	elem := make(chan interface{})
	out := NewStream(elem, s.stop)

	go func() {
		defer close(elem)

		for {
			e, ok := in.Next()
			if ok {
				elem <- f(e)
			} else {
				return
			}
		}
	}()

	return out
}

type FilterFunc func(interface{}) bool

func Filter(in Stream, f FilterFunc) Stream {
	s := in.(*stream)
	elem := make(chan interface{})
	out := NewStream(elem, s.stop)

	go func() {
		defer close(elem)

		for {
			e, ok := in.Next()
			if ok {
				if f(e) {
					elem <- e
				}
			} else {
				return
			}
		}
	}()

	return out
}

// Join combines multiple input Streams into a single output Stream
func Join(in ...Stream) Stream {
	elem := make(chan interface{})
	stop := make(chan struct{})

	out := NewStream(elem, stop)

	go func() {
		defer close(elem)

		cases := make([]reflect.SelectCase, len(in)*3)

		for i := range in {
			s := in[i].(*stream)

			cases[i] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(s.elem),
			}

			cases[i+len(in)] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(s.stop),
			}
		}

		activeElems := len(in)
		activeStops := len(in)

		for activeElems > 0 && activeStops > 0 {
			chosen, recv, recvOK := reflect.Select(cases)

			if chosen < len(in) {
				// Input element
				if recvOK {
					elem <- recv.Interface()
				} else {
					cases[chosen].Chan = reflect.ValueOf(nil)
					activeElems--
				}
			} else {
				// Stop channel was closed
				cases[chosen].Chan = reflect.ValueOf(nil)
				activeStops--
			}
		}
	}()

	return out
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
func Split(in Stream, f FilterFunc) (Stream, Stream) {
	s := in.(*stream)

	elemTrue := make(chan interface{})
	outTrue := NewStream(elemTrue, s.stop)

	elemFalse := make(chan interface{})
	outFalse := NewStream(elemFalse, s.stop)

	go func() {
		defer close(elemTrue)
		defer close(elemFalse)

		for {
			e, ok := in.Next()
			if ok {
				if f(e) {
					elemTrue <- e
				} else {
					elemFalse <- e
				}
			} else {
				return
			}
		}
	}()

	return outTrue, outFalse
}

// Tee copies the input stream to a pair of output streams (like a T-shaped junction)
func Tee(in Stream) (Stream, Stream) {
	s := in.(*stream)

	elemOne := make(chan interface{})
	outOne := NewStream(elemOne, s.stop)

	elemTwo := make(chan interface{})
	outTwo := NewStream(elemTwo, s.stop)

	go func() {
		defer close(elemOne)
		defer close(elemTwo)

		for {
			e, ok := in.Next()
			if ok {
				elemOne <- e
				elemTwo <- e
			} else {
				return
			}
		}
	}()

	return outOne, outTwo
}

/*
// Copy duplicates elements from the input channel into the output channels
func Copy(done <-chan struct{}, in <-chan interface{}, n int) []<-chan interface{} {
	out := make([]chan interface{}, n)
	for i := range out {
		out[i] = make(chan interface{})
	}

	//
	// XXX: Automatic conversion from []chan to []<-chan seems to not be
	// supported. This is gross, but maybe the only way to do it?
	//
	recvOnlyOut := make([]<-chan interface{}, n)
	for i := range out {
		recvOnlyOut[i] = out[i]
	}

	go func() {
		defer func() {
			for i := range out {
				close(out[i])
			}
		}()

		cases := make([]reflect.SelectCase, len(out)+1)
		cases[0] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(done),
		}

		for i, c := range out {
			cases[1+i] = reflect.SelectCase{
				Dir:  reflect.SelectSend,
				Chan: reflect.ValueOf(c),
			}
		}

		for e := range in {
			for i := 0; i < len(out); i++ {
				cases[1+i].Send = reflect.ValueOf(e)
			}

			chosen, _, _ := reflect.Select(cases)
			if chosen == 0 {
				return
			}
		}
	}()

	return recvOnlyOut
}
*/

// Buffer stores up to the given number of elements from the input Stream
// before blocking
func Buffer(in Stream, size uint) Stream {
	s := in.(*stream)

	elem := make(chan interface{}, size)
	out := NewStream(elem, s.stop)

	go func() {
		defer close(elem)

		for {
			e, ok := in.Next()
			if ok {
				elem <- e
			} else {
				return
			}
		}
	}()

	return out
}

// Overflow drops elements instead of blocking on the output Stream
func Overflow(in Stream) Stream {
	s := in.(*stream)

	elem := make(chan interface{})
	out := NewStream(elem, s.stop)

	go func() {
		defer close(elem)

		for {
			select {
			case e, ok := <-s.elem:
				if ok {
					elem <- e
				} else {
					return
				}
			default:
				// Drop

			}
		}
	}()

	return out
}

// ----------------------------------------------------------------------------
// Terminators accept an input stream and return an output stream of a terminal
// value. They are typically used to wait for a stream's clean termination.
// ----------------------------------------------------------------------------

type ReduceFunc func(interface{}, interface{}) interface{}

// Reduce adds a terminator onto the stream that accumulates a value using
// the given function and then returns it once the input stream terminates.
func Reduce(in Stream, initVal interface{}, f ReduceFunc) interface{} {
	acc := initVal

	for {
		e, ok := in.Next()
		if ok {
			acc = f(acc, e)
		} else {
			return acc
		}
	}
}

// ForEach adds a terminator onto the stream that consumes each element and
// runs the given function on them.
func ForEach(in Stream, f DoFunc) {
	for {
		e, ok := in.Next()
		if ok {
			f(e)
		} else {
			return
		}
	}
}

// Discard adds a terminator onto the stream that throws away all elements.
func Discard(in Stream) {
	for {
		_, ok := in.Next()
		if !ok {
			return
		}
	}
}
