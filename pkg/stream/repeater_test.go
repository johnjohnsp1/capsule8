package stream

import "testing"
import "time"

func TestRepeater(t *testing.T) {
	s := Iota(10)
	defer s.Close()

	s, valve := OnOffValve(s)

	rep := NewRepeater(s)

	a := rep.NewStream()
	aye := Reduce(a, uint64(0), func(acc interface{}, x interface{}) interface{} {
		return acc.(uint64) + x.(uint64)
	})

	b := rep.NewStream()
	bee := Reduce(b, uint64(0), func(acc interface{}, x interface{}) interface{} {
		return acc.(uint64) + x.(uint64)
	})

	c := rep.NewStream()
	see := Reduce(c, uint64(0), func(acc interface{}, x interface{}) interface{} {
		return acc.(uint64) + x.(uint64)
	})

	//
	// Open the valve and force the reduced values now
	//
	valve <- true

	sumA := (<-aye).(uint64)
	sumB := (<-bee).(uint64)
	sumC := (<-see).(uint64)

	if sumA != sumB || sumB != sumC || sumA != sumC {
		t.Errorf("Repeater error: %v, %v, %v\n",
			sumA, sumB, sumC)
	}
}

func TestRepeaterNoStreams(t *testing.T) {
	//
	// A repeater should act as a terminator and consume events when
	// there are no child streams.
	//

	s := Iota(10)
	defer s.Close()

	gotTen := make(chan bool)

	elemCount := 0
	s = Do(s, func(e interface{}) {
		elemCount++
		if elemCount == 10 {
			gotTen <- true
		}
	})

	_ = NewRepeater(s)

	timeout := time.After(1 * time.Second)

	select {
	case <-gotTen:
		break
	case <-timeout:
		t.Error("Timeout")
	}
}
