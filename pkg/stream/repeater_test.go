package stream

import "testing"

func TestRepeater(t *testing.T) {
	s := Iota(10)
	defer s.Close()

	s, valve := OnOffValve(s)

	rep := NewRepeater(s)

	a, ok := rep.NewStream()
	if !ok {
		t.Error("NewStream failed")
	}

	aye := Reduce(a, uint64(0), func(acc interface{}, x interface{}) interface{} {
		return acc.(uint64) + x.(uint64)
	})

	b, ok := rep.NewStream()
	if !ok {
		t.Error("NewStream failed")
	}

	bee := Reduce(b, uint64(0), func(acc interface{}, x interface{}) interface{} {
		return acc.(uint64) + x.(uint64)
	})

	c, ok := rep.NewStream()
	if !ok {
		t.Error("NewStream failed")
	}

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
