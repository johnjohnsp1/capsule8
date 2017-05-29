// Copyright 2017 Capsule8 Inc. All rights reserved.

package inotify

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/capsule8/reactive8/pkg/stream"

	"golang.org/x/sys/unix"
)

const pollTimeoutMillis = -1
const inotifyBufferSize = (unix.SizeofInotifyEvent + unix.NAME_MAX + 1) * 128
const controlBufferSize = 4096 // Must not be greater than PIPE_BUF (4096)

type watch struct {
	descriptor int
	path       string
	mask       uint32
	recursive  bool
}

type inotifyAdd struct {
	reply     chan<- error
	path      string
	mask      uint32
	recursive bool
}

type inotifyRemove struct {
	reply chan<- error
	path  string
}

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

func (is *Instance) remove(path string) error {
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

// -----------------------------------------------------------------------------

//
// Handle a control message notification message sent over the pipe
//
func (is *Instance) handleControlBuffer(b []byte) error {
	// Pipe signals a wakeup to read message from control channel
	msg, ok := <-is.ctrl
	if !ok {
		return errors.New("Control channel closed")
	}

	switch msg.(type) {
	case *inotifyAdd:
		msg := msg.(*inotifyAdd)
		err := is.add(msg.path, msg.mask, msg.recursive)
		msg.reply <- err

	case *inotifyRemove:
		msg := msg.(*inotifyRemove)
		err := is.remove(msg.path)
		msg.reply <- err

	default:
		panic(fmt.Sprintf("Unknown control message type: %T", msg))
	}

	return nil
}

func (is *Instance) handleInotifyEvent(ev *Event, w *watch) error {
	//
	// Name is only present when subject of event is a file
	// within a watched directory
	//
	if ev.Len > 0 {
		// If event was related to a directory within a
		// recursively watched directory, propagate the watch.
		if ev.Mask&unix.IN_CREATE != 0 && ev.Mask&unix.IN_ISDIR != 0 {
			if w.recursive {
				err := is.add(ev.Path, w.mask, w.recursive)
				if err != nil {
					return err
				}
			}
		}
	}

	is.elem <- ev

	return nil
}

//
// Handle an inotify buffer received from the kernel over an inotify
// instance file descriptor
//
func (is *Instance) handleInotifyBuffer(b []byte) error {
	// Create a bytes.Buffer using amount of b that was read into
	buf := bytes.NewBuffer(b)

	//
	// Parse out each inotify_event
	//
	for buf.Len() > 0 {
		ev := Event{}
		binary.Read(buf, binary.LittleEndian, &ev.InotifyEvent)

		w := is.watch[int(ev.Wd)]

		// The name field from kernel is padded w/ NULLs to an
		// alignment boundary, remove them when converting to
		// a string.
		name := buf.Next(int(ev.Len))
		ev.Name = string(bytes.Trim(name, "\x00"))
		ev.Path = filepath.Join(w.path, ev.Name)

		is.handleInotifyEvent(&ev, w)
	}

	return nil
}

func (is *Instance) pollLoop() error {
	controlBuffer := make([]byte, controlBufferSize)
	inotifyBuffer := make([]byte, inotifyBufferSize)

	pollFds := make([]unix.PollFd, 2)

	// pipe to wake up loop to handle control channel messages
	pollFds[0].Fd = int32(is.pipe[0])
	pollFds[0].Events = unix.POLLIN

	// inotify file descriptor
	pollFds[1].Fd = int32(is.fd)
	pollFds[1].Events = unix.POLLIN

	for {
		n, err := unix.Poll(pollFds, pollTimeoutMillis)
		if err != nil {
			return err
		} else if n == 0 {
			// timeout, check the stop channel and restart poll()
			continue
		}

		//
		// We always give event file descriptor higher priority. We
		// only service the control message queue when there are no
		// inotify events to handle.
		//
		if pollFds[1].Revents&unix.POLLIN != 0 {
			n, err = unix.Read(int(pollFds[1].Fd), inotifyBuffer)
			if err != nil {
				return err
			}

			err = is.handleInotifyBuffer(inotifyBuffer[:n])
			if err != nil {
				return err
			}
		} else if pollFds[0].Revents&unix.POLLIN != 0 {
			n, err = unix.Read(int(pollFds[0].Fd), controlBuffer)
			if err != nil {
				return err
			}

			err = is.handleControlBuffer(controlBuffer[:n])
			if err != nil {
				return err
			}
		}
	}
}

// -----------------------------------------------------------------------------

func (is *Instance) sendAdd(path string, mask uint32, recursive bool) error {
	reply := make(chan error)
	msg := &inotifyAdd{
		reply:     reply,
		path:      path,
		mask:      mask,
		recursive: recursive,
	}

	// Wake the pollLoop
	buf := [1]byte{0}
	_, err := unix.Write(is.pipe[1], buf[:])
	if err != nil {
		return err
	}

	is.ctrl <- msg
	err = <-reply
	return err
}

func (is *Instance) removeWatch(path string) error {
	reply := make(chan error)
	msg := &inotifyRemove{
		reply: reply,
		path:  path,
	}

	// Wake the pollLoop
	buf := [1]byte{0}
	_, err := unix.Write(is.pipe[1], buf[:])
	if err != nil {
		return err
	}

	is.ctrl <- msg
	err = <-reply
	return err
}

// -----------------------------------------------------------------------------

// Event represents an inotify event
type Event struct {
	unix.InotifyEvent

	// Name within watched path if it's a directory
	Name string

	// Watched path associated with the event
	Path string
}

// Instance represents an initialized inotify instance
type Instance struct {
	mu          sync.Mutex
	fd          int
	elem        chan interface{}
	errc        chan error
	stop        chan interface{}
	watch       map[int]*watch
	path        map[string]*watch
	eventStream *stream.Stream
	pipe        [2]int
	ctrl        chan interface{}
}

// NewInstance creates a new inotify instance
func NewInstance() (*Instance, error) {
	fd, err := unix.InotifyInit()
	if err != nil {
		return nil, err
	}

	stop := make(chan interface{})
	elem := make(chan interface{})
	ctrl := make(chan interface{})

	is := &Instance{
		fd:    fd,
		elem:  elem,
		errc:  make(chan error, 1),
		stop:  stop,
		watch: make(map[int]*watch),
		path:  make(map[string]*watch),
		eventStream: &stream.Stream{
			Ctrl: stop,
			Data: elem,
		},
		ctrl: ctrl,
	}

	err = unix.Pipe2(is.pipe[:], unix.O_DIRECT|unix.O_NONBLOCK)
	if err != nil {
		return nil, err
	}

	go func() {
		err := is.pollLoop()
		if err != nil {
			log.Fatal(err)
		}

		err = unix.Close(is.fd)
		if err != nil {
			log.Fatal(err)
		}

		close(is.errc)
		close(is.elem)
	}()

	return is, nil
}

// Events returns a Event stream.Stream of the inotify instance's events
func (is *Instance) Events() *stream.Stream {
	return is.eventStream
}

// AddWatch adds the given path to the inotify instance's watch list for
// events matching the given mask. If the path is already being watched, the
// existing watch is modified.
func (is *Instance) AddWatch(path string, mask uint32) error {
	return is.sendAdd(path, mask, false)
}

// AddRecursiveWatch adds the given path to the inotify instance's
// watch list as well as any directories recursively identified within
// it. Newly created subdirectories within the subtree rooted at path are
// added to the watch list as well.
func (is *Instance) AddRecursiveWatch(path string, mask uint32) error {
	return is.sendAdd(path, mask, true)
}

// Remove the given path from the inotify instance's watch list.
func (is *Instance) RemoveWatch(path string) error {
	return is.removeWatch(path)
}

// Close the inotify instance and allow the kernel to free its associated
// resources. All associated watches are automatically freed.
func (is *Instance) Close() {
	close(is.stop)
}
