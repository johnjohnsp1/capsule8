package inotify

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/capsule8/reactive8/pkg/stream"

	"os"

	"fmt"

	"golang.org/x/sys/unix"
)

var timerDelay = 5 * time.Second

func TestFile(t *testing.T) {
	gotOne := make(chan struct{})

	instance, err := NewInstance()
	if err != nil {
		t.Error(err)
	}
	defer instance.Close()

	s := instance.Stream()

	s = stream.Do(s, func(e interface{}) {
		gotOne <- struct{}{}
	})
	go stream.Discard(s)

	f, err := ioutil.TempFile("", "inotify_test_TestFile")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f.Name())

	//
	// TempFile opens for read/write, so closing it should result in
	// an IN_CLOSE_WRITE inotify event.
	//
	instance.Add(f.Name(), unix.IN_CLOSE_WRITE)
	f.Close()

	// Success means not hanging
	<-gotOne
}

//
// This is a torture test for kernel inotify events, we'll only get the first.
//
func DISABLED_TestFiveFiles(t *testing.T) {
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

	var handlerRuns = 0
	go func() {

		for {
			_, ok := instance.Next()
			if !ok {
				return
			}

			if handlerRuns == 5 {
				ch <- false
				return
			} else {
				handlerRuns += 1
			}
		}
	}()

	//
	// TempFile opens for read/write, so closing it should result in
	// an IN_CLOSE_WRITE inotify event.
	//
	instance.Add(f1.Name(), unix.IN_CLOSE_WRITE)
	instance.Add(f2.Name(), unix.IN_CLOSE_WRITE)
	instance.Add(f3.Name(), unix.IN_CLOSE_WRITE)
	instance.Add(f4.Name(), unix.IN_CLOSE_WRITE)
	instance.Add(f5.Name(), unix.IN_CLOSE_WRITE)

	f1.Close()
	f2.Close()
	f3.Close()
	f4.Close()
	f5.Close()

	timerExpired := <-ch
	if timerExpired {
		t.Error("Timed out waiting for timer to run 5 times, got", handlerRuns)
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
			ev, ok := instance.Next()
			if !ok {
				fmt.Fprintf(os.Stderr, "TestSubdirs: received %v\n", ev)
				return
			} else if ev.Name == "file" {
				done <- time.Now()
				return
			}
		}
	}()

	content := []byte("temporary file's content")
	dir, err := ioutil.TempDir("", "inotify_test_TestSubdirs")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(dir)

	err = instance.AddRecursive(dir, unix.IN_ONLYDIR|unix.IN_CREATE)
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

	// Still can't win this race without the sleep, only get 1 inotify
	// event for the top-level dir ("a") and miss "file"
	time.Sleep(100 * time.Millisecond)

	tmpfn := filepath.Join(d, "file")

	if err := ioutil.WriteFile(tmpfn, content, 0666); err != nil {
		t.Error(err)
	}

	select {
	case <-ticker:
		t.Fatal("Timeout")

	case <-done:
	}
}
