package container

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/capsule8/reactive8/pkg/config"
	"github.com/capsule8/reactive8/pkg/inotify"
	"github.com/capsule8/reactive8/pkg/stream"
)

//
// OCI container lifecycle states:
// https://github.com/opencontainers/runtime-spec/blob/master/runtime.md
//

type ociState uint

const (
	_ ociState = iota
	ociCreating
	ociCreated
	ociRunning
	ociStopped
	ociDeleted
)

type ociEvent struct {
	ID          string
	Image       string
	State       ociState
	CgroupsPath string
	ConfigJSON  string
}

// ----------------------------------------------------------------------------
// OCI configuration file format
// ----------------------------------------------------------------------------

func getOciContainerDir() string {
	return config.Sensor.OciContainerDir
}

type ociConfigJson struct {
	OciVersion string `json:"ociVersion"`
	Root       struct {
		Path string `json:"path"`
	} `json:"root"`
	Linux struct {
		CgroupsPath string `json:"cgroupsPath"`
	} `json:"linux"`
}

// ----------------------------------------------------------------------------
// OCI container configuration inotify event to ociEvent state machine
// ----------------------------------------------------------------------------

func onOciConfigUpdate(configPath string) (*ociEvent, error) {
	//
	// Look for the close of an open for write to identify container started
	// events.
	//

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	configJson := ociConfigJson{}
	json.Unmarshal(data, &configJson)

	containerID := filepath.Base(filepath.Dir(configPath))

	ev := &ociEvent{
		ID:          containerID,
		State:       ociRunning,
		CgroupsPath: configJson.Linux.CgroupsPath,
		ConfigJSON:  string(data),
	}

	return ev, nil
}

func onOciConfigDelete(configPath string) (*ociEvent, error) {
	//
	// Look for deletion of config.json to identify container stopped events.
	//
	containerID := filepath.Base(filepath.Dir(configPath))

	ev := &ociEvent{
		ID:    containerID,
		State: ociStopped,
	}

	return ev, nil
}

func (o *oci) onInotifyEvent(iev *inotify.Event) *ociEvent {
	if iev.Name == "config.json" {
		if iev.Mask&unix.IN_CLOSE_WRITE != 0 {
			ev, _ := onOciConfigUpdate(iev.Path)
			return ev

		} else if iev.Mask&unix.IN_DELETE != 0 {
			ev, _ := onOciConfigDelete(iev.Path)
			return ev
		}
	}

	return nil
}

// -----------------------------------------------------------------------------
// inotify-based OCI sensor
// -----------------------------------------------------------------------------

//
// Singleton sensor state
//
type oci struct {
	ctrl          chan interface{}
	data          chan interface{}
	eventStream   *stream.Stream
	inotify       *inotify.Instance
	inotifyEvents *stream.Stream
	inotifyDone   chan interface{}
	repeater      *stream.Repeater
}

var ociOnce sync.Once
var ociControl chan interface{}

//
// Control channel messages
//
type ociEventStreamRequest struct {
	reply chan *stream.Stream
}

func (o *oci) newStream(m *ociEventStreamRequest) *stream.Stream {
	// Create a new stream from our Repeater
	return o.repeater.NewStream()
}

func (o *oci) loop() (bool, error) {
	select {
	case e, ok := <-o.ctrl:
		if ok {
			switch e.(type) {
			case *ociEventStreamRequest:
				m := e.(*ociEventStreamRequest)
				m.reply <- o.newStream(m)

			default:
				panic(fmt.Sprintf("Unknown type: %T", e))
			}
		} else {
			// control channel was closed, shut down
		}
	}

	return true, nil
}

func (o *oci) handleInotifyEvent(e interface{}) {
	iev := e.(*inotify.Event)

	ev := o.onInotifyEvent(iev)
	if ev != nil {
		o.data <- ev
	}
}

func initializeOciSensor() error {
	in, err := inotify.NewInstance()
	if err != nil {
		return err
	}

	//
	// Create the global control channel outside of the goroutine to avoid
	// a race condition in NewOciEventStream()
	//
	ociControl = make(chan interface{})

	go func() {
		var err error

		// If this goroutine exits, just crash
		defer panic(err)

		//
		// Create instance inside goroutine so that references don't
		// escape it. This keeps their allocation on the stack and free
		// from the GC.
		//

		data := make(chan interface{})
		o := &oci{
			ctrl: ociControl,
			data: data,

			eventStream: &stream.Stream{
				Ctrl: ociControl,
				Data: data,
			},

			inotify: in,
		}

		o.inotifyEvents = in.Events()
		o.inotifyDone =
			stream.ForEach(o.inotifyEvents, o.handleInotifyEvent)

		o.repeater = stream.NewRepeater(o.eventStream)

		addWatches(getOciContainerDir(), o.inotify)

		for {
			var ok bool
			ok, err = o.loop()
			if !ok {
				break
			}
		}
	}()

	return nil
}

// ----------------------------------------------------------------------------
// Exported interface
// ----------------------------------------------------------------------------

// NewOciEventStream creates a new event stream of OCI container lifecycle
// events.
func NewOciEventStream() (*stream.Stream, error) {
	var err error

	// Initialize singleton sensor if necessary
	ociOnce.Do(func() {
		err = initializeOciSensor()
	})

	if err != nil {
		return nil, err
	}

	if ociControl != nil {
		reply := make(chan *stream.Stream)
		request := &ociEventStreamRequest{
			reply: reply,
		}

		ociControl <- request
		response := <-reply

		return response, nil
	}

	return nil, errors.New("Sensor not available")
}
