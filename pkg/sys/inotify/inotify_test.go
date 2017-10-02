package inotify

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/capsule8/capsule8/pkg/stream"
	"golang.org/x/sys/unix"
)

var timerDelay = 5 * time.Second

func TestFile(t *testing.T) {
	ticker := time.After(5 * time.Second)
	gotOne := make(chan struct{})

	instance, err := NewInstance()
	if err != nil {
		t.Error(err)
	}
	defer instance.Close()

	s := instance.Events()

	s = stream.Do(s, func(e interface{}) {
		gotOne <- struct{}{}
	})
	_ = stream.Wait(s)

	f, err := ioutil.TempFile("", "inotify_test_TestFile")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f.Name())

	//
	// TempFile opens for read/write, so closing it should result in
	// an IN_CLOSE_WRITE inotify event.
	//
	instance.AddWatch(f.Name(), unix.IN_CLOSE_WRITE)
	f.Close()

	// Success means not hanging
	select {
	case <-gotOne:
	// Success

	case <-ticker:
		t.Fatal("Timeout")
	}
}

func TestFiveFiles(t *testing.T) {
	instance, err := NewInstance()
	if err != nil {
		t.Error(err)
	}
	defer instance.Close()

	f1, err := ioutil.TempFile("", "inotify_test_TestFile")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f1.Name())

	f2, err := ioutil.TempFile("", "inotify_test_TestFile")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f2.Name())

	f3, err := ioutil.TempFile("", "inotify_test_TestFile")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f3.Name())

	f4, err := ioutil.TempFile("", "inotify_test_TestFile")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f4.Name())

	f5, err := ioutil.TempFile("", "inotify_test_TestFile")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f5.Name())

	ch := make(chan bool)
	go func() {
		time.Sleep(timerDelay)
		ch <- true
	}()

	var receivedEvents = 0
	go func() {
		for {
			_, ok := <-instance.Events().Data
			if !ok {
				return
			}

			receivedEvents++

			if receivedEvents == 5 {
				ch <- false
				return
			}
		}
	}()

	//
	// TempFile opens for read/write, so closing it should result in
	// an IN_CLOSE_WRITE inotify event.
	//
	instance.AddWatch(f1.Name(), unix.IN_CLOSE_WRITE)
	instance.AddWatch(f2.Name(), unix.IN_CLOSE_WRITE)
	instance.AddWatch(f3.Name(), unix.IN_CLOSE_WRITE)
	instance.AddWatch(f4.Name(), unix.IN_CLOSE_WRITE)
	instance.AddWatch(f5.Name(), unix.IN_CLOSE_WRITE)

	f1.Close()
	f2.Close()
	f3.Close()
	f4.Close()
	f5.Close()

	timerExpired := <-ch
	if timerExpired {
		t.Error("Expected 5 events, got", receivedEvents)
	}
}

func TestSubdirs(t *testing.T) {
	ticker := time.After(5 * time.Second)
	done := make(chan time.Time)

	instance, err := NewInstance()
	if err != nil {
		t.Error(err)
	}
	defer instance.Close()

	go func() {
		for {
			e, ok := <-instance.Events().Data
			ev := e.(*Event)

			if !ok {
				return
			} else if ev.Name == "file" {
				done <- time.Now()
				return
			}
		}
	}()

	dir, err := ioutil.TempDir("", "inotify_test_TestSubdirs")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(dir)

	err = instance.AddRecursiveWatch(dir, unix.IN_ONLYDIR|unix.IN_CREATE)
	if err != nil {
		t.Error(err)
	}

	d := filepath.Join(dir, "a", "1", "2")
	err = os.MkdirAll(d, 0777)

	d = filepath.Join(d, "b")
	err = os.MkdirAll(d, 0777)

	d = filepath.Join(d, "c")
	err = os.MkdirAll(d, 0777)

	d = filepath.Join(d, "d")
	err = os.MkdirAll(d, 0777)

	// Can't seem to win this race without a sleep here. Otherwise,
	// we only get 1 inotify event for the top-level dir ("a") and miss
	// the event for "file"
	time.Sleep(1 * time.Millisecond)

	tmpfn := filepath.Join(d, "file")
	content := []byte("temporary file's content")
	if err := ioutil.WriteFile(tmpfn, content, 0666); err != nil {
		t.Error(err)
	}

	select {
	case <-ticker:
		t.Fatal("Timeout")

	case <-done:
	}
}

func TestTrigger(t *testing.T) {
	ticker := time.After(1 * time.Second)
	done := make(chan time.Time)

	instance, err := NewInstance()
	if err != nil {
		t.Error(err)
	}
	defer instance.Close()

	go func() {
		for {
			e, ok := <-instance.Events().Data
			ev := e.(*Event)

			if !ok {
				return
			} else if ev.Name == "file" {
				done <- time.Now()
				return
			}
		}
	}()

	dir, err := ioutil.TempDir("", "inotify_test_TestSubdirs")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(dir)

	err = instance.AddWatch(dir, unix.IN_ONLYDIR|unix.IN_CREATE)
	if err != nil {
		t.Error(err)
	}

	// Set a watch trigger on regexp
	err = instance.AddTrigger("abc123$", unix.IN_ONLYDIR|unix.IN_CREATE)
	if err != nil {
		t.Error(err)
	}

	d1 := filepath.Join(dir, "123abc")
	err = os.MkdirAll(d1, 0777)

	d2 := filepath.Join(dir, "abc123")
	err = os.MkdirAll(d2, 0777)

	time.Sleep(1 * time.Millisecond)

	tmpfn1 := filepath.Join(d1, "file")
	content := []byte("temporary file's content")
	if err := ioutil.WriteFile(tmpfn1, content, 0666); err != nil {
		t.Error(err)
	}

	tmpfn2 := filepath.Join(d2, "file")
	if err := ioutil.WriteFile(tmpfn2, content, 0666); err != nil {
		t.Error(err)
	}

	select {
	case <-ticker:
		t.Fatal("Timeout")

	case <-done:
	}
}
