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
