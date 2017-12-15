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

func TestNext(t *testing.T) {
	s := Iota(10)
	defer s.Close()

	zero, ok := <-s.Data
	if !ok || zero.(uint64) != 0 {
		t.Fatalf("Expected 0, got %d", zero.(uint64))
	}

	one, ok := <-s.Data
	if !ok || one.(uint64) != 1 {
		t.Fatalf("Expected 1, got %d", one.(uint64))
	}

	two, ok := <-s.Data
	if !ok || two.(uint64) != 2 {
		t.Fatalf("Expected 2, got %d", two.(uint64))
	}
}

func TestReduce(t *testing.T) {
	s := Iota(10)
	defer s.Close()

	s = Do(s, func(e interface{}) {
		// Intentionally do nothing
	})

	v := Reduce(s, uint64(0), func(a interface{}, b interface{}) interface{} {
		return a.(uint64) + b.(uint64)
	})

	// Force the promise returned by Reduce as a uint64
	i := <-v
	sum := i.(uint64)
	if sum != 45 {
		t.Errorf("Expected v = 45, got %d\n", sum)
	}
}

func TestIota1(t *testing.T) {
	s := Iota(1)

	e, ok := <-s.Data

	if !ok {
		t.Errorf("Expected value from iota, got closed")
	}

	i := e.(uint64)
	if i != 0 {
		t.Errorf("Expected i == 0, got %d", i)
	}

	e, ok = <-s.Data
	if ok {
		t.Errorf("Expected iota closed, got element: %v", e)
	}

	e, ok = <-s.Data
	if ok {
		t.Errorf("Expected iota closed, got element: %v", e)
	}
}

func TestIota2(t *testing.T) {
	s := Iota(1, 100)

	e, ok := <-s.Data

	if !ok {
		t.Errorf("Expected value from iota, got closed")
	}

	i := e.(uint64)
	if i != 100 {
		t.Errorf("Expected i == 0, got %d", i)
	}

	e, ok = <-s.Data
	if ok {
		t.Errorf("Expected iota closed, got element: %v", e)
	}

	e, ok = <-s.Data
	if ok {
		t.Errorf("Expected iota closed, got element: %v", e)
	}
}

func TestIota3(t *testing.T) {
	s := Iota(5, 100, 10)
	defer s.Close()

	total := uint64(0)

	for i := range s.Data {
		i := i.(uint64)
		total += i

	}

	expected := uint64(100 + 110 + 120 + 130 + 140)
	if total != expected {
		t.Errorf("Expected total = %d, got %d\n", expected, total)
	}
}
