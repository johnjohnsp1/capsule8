// Copyright 2017 Capsule8 Inc. All rights reserved.

package inotify

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"

	"github.com/capsule8/reactive8/pkg/stream"

	"golang.org/x/sys/unix"
)

const inotifyPollTimeoutMillis = 100

type watch struct {
	descriptor int
	path       string
	mask       uint32
	recursive  bool
}

type Event struct {
	unix.InotifyEvent
	Name string
}

type Instance struct {
	mu    sync.Mutex
	fd    int
	elem  chan interface{}
	errc  chan error
	stop  chan struct{}
	watch map[int]*watch
	path  map[string]*watch
}

// Must be called with mutex held
func (is *Instance) add(path string, mask uint32, recursive bool) error {
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		//
		// For a recursive watch, we only add a watch on directories
		//
		if recursive {
			mask |= unix.IN_ONLYDIR
		}

		wd, err := unix.InotifyAddWatch(is.fd, path, mask)
		if err == nil {
			watch := &watch{
				descriptor: wd,
				path:       path,
				mask:       mask,
				recursive:  recursive,
			}

			is.watch[wd] = watch
			is.path[path] = watch
		}

		if recursive == false {
			// Added a non-recursive watch on a single file, we're done.
			return filepath.SkipDir
		} else if err != unix.ENOTDIR {
			// We ignore ENOENT on recursive watches since we added IN_ONLYDIR
			return err
		}

		return nil
	}

	return filepath.Walk(path, walkFn)
}

func (is *Instance) Add(path string, mask uint32) error {
	_, err := os.Stat(path)
	if err != nil {
		return err
	}

	is.mu.Lock()
	err = is.add(path, mask, false)
	is.mu.Unlock()

	return err
}

func (is *Instance) AddRecursive(path string, mask uint32) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	} else if !fileInfo.IsDir() {
		// Return ENOTDIR ("not a directory") if path isn't a directory
		return unix.ENOTDIR
	}

	is.mu.Lock()
	err = is.add(path, mask, true)
	is.mu.Unlock()

	return err
}

// Must be called with mutex held
func (is *Instance) remove(path string, recursive bool) error {
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			watch := is.path[path]
			if watch != nil {
				_, err := unix.InotifyRmWatch(is.fd, uint32(watch.descriptor))
				return err
			}
		}

		return nil
	}

	return filepath.Walk(path, walkFn)
}

func (is *Instance) Remove(path string, recursive bool) error {
	is.mu.Lock()

	err := is.remove(path, recursive)

	is.mu.Unlock()

	return err
}

func (is *Instance) Next() (*Event, bool) {
	select {
	case <-is.stop:
		return nil, false
	case e, ok := <-is.elem:
		if ok {
			ev := e.(*Event)
			return ev, ok
		}

		return nil, ok
	}
}

func (is *Instance) Path(wd int32) string {
	is.mu.Lock()
	w := is.watch[int(wd)]
	is.mu.Unlock()

	return w.path
}

func (is *Instance) Close() {
	close(is.stop)
}

func (is *Instance) Stream() stream.Stream {
	return stream.NewStream(is.elem, is.stop)
}

func (is *Instance) receive() {
	defer func() {
		err := unix.Close(is.fd)
		if err != nil {
			is.errc <- err
		}

		close(is.errc)
		close(is.elem)
	}()

	b := make([]byte, (unix.SizeofInotifyEvent+unix.NAME_MAX+1)*128)

	pollFds := make([]unix.PollFd, 1)
	pollFds[0].Fd = int32(is.fd)
	pollFds[0].Events = unix.POLLIN

	for {
		select {
		case <-is.stop:
			// channels are closed in the defer above
			return

		default:
			n, err := unix.Poll(pollFds, inotifyPollTimeoutMillis)
			if err != nil {
				is.errc <- err
				return
			} else if n == 0 {
				// timeout, check the stop channel and restart poll()
				continue
			}

			n, err = unix.Read(is.fd, b)
			if err != nil {
				is.errc <- err
				return
			}

			// Create a bytes.Buffer using amount of b that was read into
			buf := bytes.NewBuffer(b[:n])

			//
			// Parse out each inotify_event
			//
			for buf.Len() > 0 {
				ev := Event{}
				binary.Read(buf, binary.LittleEndian, &ev.InotifyEvent)

				//
				// Name is only present when subject of event is a file
				// within a watched directory
				//
				if ev.Len > 0 {
					// The name field from kernel is typically padded w/ NULLs
					// to an alignment boundary, remove them when converting
					// to a string.
					name := buf.Next(int(ev.Len))
					ev.Name = string(bytes.Trim(name, "\x00"))
				}

				// If event was related to a directory within a
				// recursively watched directory, propagate the watch.
				if ev.Len > 0 && ev.Mask&unix.IN_CREATE != 0 && ev.Mask&unix.IN_ISDIR != 0 {
					is.mu.Lock()
					w := is.watch[int(ev.Wd)]

					if w != nil && w.recursive {
						path := filepath.Join(w.path, ev.Name)
						err := is.add(path, w.mask, w.recursive)

						if err != nil {
							is.errc <- err
							is.mu.Unlock()
							return
						}
					}

					is.mu.Unlock()
				}

				// Blocking send on the channel can be blocked by a slow receiver
				is.elem <- &ev
			}
		}
	}
}

// NewInstance reads events from the kernel on the given inotify fd
// and forwards them as *Event elements over the output stream.
func NewInstance() (*Instance, error) {
	fd, err := unix.InotifyInit()
	if err != nil {
		return nil, err
	}

	is := &Instance{
		fd:    fd,
		elem:  make(chan interface{}),
		errc:  make(chan error, 1),
		stop:  make(chan struct{}),
		watch: make(map[int]*watch),
		path:  make(map[string]*watch),
	}

	go is.receive()

	return is, nil
}
