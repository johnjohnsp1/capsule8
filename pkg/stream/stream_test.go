package stream

import "testing"

func TestNext(t *testing.T) {
	s := Iota(10)
	defer s.Close()

	zero, ok := s.Next()
	if !ok || zero.(uint64) != 0 {
		t.Fatalf("Expected 0, got %d", zero.(uint64))
	}

	one, ok := s.Next()
	if !ok || one.(uint64) != 1 {
		t.Fatalf("Expected 1, got %d", one.(uint64))
	}

	two, ok := s.Next()
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

	i := v.(uint64)
	if i != 45 {
		t.Errorf("Expected v = 45, got %d\n", v)
	}
}
