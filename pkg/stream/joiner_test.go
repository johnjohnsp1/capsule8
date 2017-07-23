package stream

import (
	"fmt"
	"os"
	"testing"
)

func TestJoinNewClose(t *testing.T) {
	_, j := NewJoiner()
	j.Close()
}

func TestJoinAddRemove(t *testing.T) {
	n := Null()
	defer n.Close()

	_, j := NewJoiner()
	defer j.Close()

	if !j.Add(n) {
		t.Error("Adding Stream to Joiner failed")
	}

	if !j.Remove(n) {
		t.Error("Removing Stream from Joiner failed")
	}
}

func TestJoinAddTwo(t *testing.T) {
	s1 := Null()

	s, j := NewJoiner()
	defer j.Close()

	if !j.Add(s1) {
		t.Error("Adding Stream to Joiner failed")
	}

	s2 := Iota()

	if !j.Add(s2) {
		t.Error("Adding Stream to Joiner failed")
	}

	e, ok := <-s.Data
	i := e.(uint64)
	if !ok {
		t.Error("Expected element from Joiner")
	} else if i != 0 {
		t.Error("Expected element = 0 from Joiner")
	}

}

func TestJoinRemoveInvalid(t *testing.T) {
	n1 := Null()
	// Don't need to close n1, closing Joiner handles it

	n2 := Null()
	defer n2.Close()

	_, j := NewJoiner()
	defer j.Close()

	if !j.Add(n1) {
		t.Error("Adding Stream to Joiner failed")
	}

	if j.Remove(n2) {
		t.Error("Remove of incorrect Stream returned success")
	}
}

func TestJoinOne(t *testing.T) {
	iota := Iota()

	s, joiner := NewJoiner()
	defer joiner.Close()

	if !joiner.Add(iota) {
		t.Error("Adding Stream to Joiner failed")
	}

	e, ok := <-s.Data
	i := e.(uint64)
	if !ok {
		t.Error("Expected element from Joiner")
	} else if i != 0 {
		t.Error("Expected element = 0 from Joiner")
	}

	e, ok = <-s.Data
	i = e.(uint64)
	if !ok {
		t.Error("Expected element from Joiner")
	} else if i != 1 {
		t.Error("Expected element = 1 from Joiner")
	}
}

func TestJoinTwo(t *testing.T) {
	iota1 := Iota(1)

	s, joiner := NewJoiner()
	defer joiner.Close()
	joiner.Off()

	if !joiner.Add(iota1) {
		t.Error("Adding Stream to Joiner failed")
	}

	iota2 := Iota(1, 100)

	if !joiner.Add(iota2) {
		t.Error("Adding Stream to Joiner failed")
	}

	//
	// Make sure that we get an element from each Iota
	//
	joiner.On()

	e1, ok := <-s.Data
	i1 := e1.(uint64)
	if !ok {
		t.Error("Expected element from Joiner")
	}

	e2, ok := <-s.Data
	i2 := e2.(uint64)
	if !ok {
		t.Error("Expected element from Joiner")
	}

	sum := i1 + i2
	if sum != 100 {
		t.Errorf("Expected sum = 100, got %d", sum)
	}
}

func TestJoinTwoValved(t *testing.T) {
	iota1 := Iota(1)

	iota1v, iota1Valve := OnOffValve(iota1)

	s, joiner := NewJoiner()
	defer joiner.Close()

	if !joiner.Add(iota1v) {
		t.Error("Adding Stream to Joiner failed")
	}

	iota2 := Iota(1, 100)

	iota2v, iota2Valve := OnOffValve(iota2)

	if !joiner.Add(iota2v) {
		t.Error("Adding Stream to Joiner failed")
	}

	//
	// Make sure that we get an element from each Iota
	//
	iota1Valve <- true
	iota2Valve <- true

	e1, ok := <-s.Data
	i1 := e1.(uint64)
	if !ok {
		t.Error("Expected element from Joiner")
	}

	e2, ok := <-s.Data
	i2 := e2.(uint64)
	if !ok {
		t.Error("Expected element from Joiner")
	}

	sum := i1 + i2
	if sum != 100 {
		t.Errorf("Expected sum = 100, got %d", sum)
	}

}
