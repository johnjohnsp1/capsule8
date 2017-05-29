package container

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"

	"sync"

	"github.com/capsule8/reactive8/pkg/inotify"
	"github.com/capsule8/reactive8/pkg/stream"
	"github.com/kelseyhightower/envconfig"
)

//
// Docker >= 1.11 (containerd) Sensor. For more background on containerd,
// runC, and OCI changes in 1.11, see:
// https://medium.com/@tiffanyfayj/docker-1-11-et-plus-engine-is-now-built-on-runc-and-containerd-a6d06d7e80ef
//

//
// Docker API defines the following states:
//   created|restarting|running|removing|paused|exited|dead
//
// https://docs.docker.com/engine/api/v1.28/#operation/ContainerList
//

type dockerContainerState uint

const (
	_ dockerContainerState = iota
	dockerContainerCreated
	dockerContainerRestarting
	dockerContainerRunning
	dockerContainerRemoving
	dockerContainerPaused
	dockerContainerExited
	dockerContainerDead
)

type dockerEvent struct {
	ID           string
	State        dockerContainerState
	configV2Json string
}

var dockerConfig struct {
	// LocalStorageDir is the path to the directory used for docker
	// container local storage areas (i.e. /var/lib/docker/containers)
	LocalStorageDir string `split_words:"true" default:"/var/lib/docker/containers"`
}

func init() {
	err := envconfig.Process("DOCKER", &dockerConfig)
	if err != nil {
		log.Fatal(err)
	}
}

// ----------------------------------------------------------------------------
// Docker container configuration file format V2
// ----------------------------------------------------------------------------

type dockerConfigState struct {
	Running           bool      `json:"Running"`
	Paused            bool      `json:"Paused"`
	Restarting        bool      `json:"Restarting"`
	OOMKilled         bool      `json:"OOMKilled"`
	RemovalInProgress bool      `json:"RemovalInProgress"`
	Dead              bool      `json:"Dead"`
	Pid               int       `json:"Pid"`
	StartedAt         time.Time `json:"StartedAt"`
	FinishedAt        time.Time `json:"FinishedAt"`
	Health            string    `json:"Health"`
}

type dockerConfigConfig struct {
	Hostname   string `json:"Hostname"`
	Domainname string `json:"Domainname"`
	User       string `json:"User"`

	// XXX: ...
	Image string `json:"Image"`
	// XXX: ...
}

type dockerConfigV2 struct {
	// XXX: Fill in as needed...
	ID     string             `json:"ID"`
	Image  string             `json:"Image"`
	State  dockerConfigState  `json:"State"`
	Config dockerConfigConfig `json:"Config"`
	// ...
	LogPath string `json:"LogPath"`
}

// ----------------------------------------------------------------------------
// Docker container configuration inotify event to dockerEvents state machine
// ----------------------------------------------------------------------------

func newDockerEventFromConfigData(configV2Json []byte) (*dockerEvent, error) {
	config := dockerConfigV2{}
	err := json.Unmarshal(configV2Json, &config)
	if err != nil {
		return nil, err
	}

	var state dockerContainerState

	if !config.State.Running && config.State.StartedAt.IsZero() {
		state = dockerContainerCreated
	} else if config.State.Restarting {
		// XXX: Don't appear to cause any config file updates
		state = dockerContainerRestarting
	} else if config.State.Running && !config.State.StartedAt.IsZero() {
		state = dockerContainerRunning
	} else if config.State.RemovalInProgress {
		// XXX: Don't appear to cause any config file updates
		state = dockerContainerRemoving
	} else if config.State.Paused {
		// XXX: Don't appear to cause any config file updates
		state = dockerContainerPaused
	} else if !config.State.Running && !config.State.FinishedAt.IsZero() {
		state = dockerContainerExited
	} else {
		state = 0
	}

	return &dockerEvent{
		ID:           config.ID,
		State:        state,
		configV2Json: string(configV2Json),
	}, nil
}

func onDockerConfigUpdate(configPath string) (*dockerEvent, error) {
	//
	// Look for file rename to config.v2.json to identify container created
	// events. This happens on a few updates to config.v2.json, so we need
	// to be sure to only send the first one that we see.
	//
	var err error

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	dEv, err := newDockerEventFromConfigData(data)
	if err != nil {
		return nil, err
	}

	return dEv, err
}

func onDockerConfigDelete(configPath string) (*dockerEvent, error) {
	//
	// Look for deletion of config.v2.json to identify container removed
	// events.
	//
	containerID := filepath.Base(filepath.Dir(configPath))

	ev := &dockerEvent{
		ID:    containerID,
		State: dockerContainerDead,
	}

	return ev, nil
}

func (d *docker) onInotifyEvent(iev *inotify.Event) *dockerEvent {
	dir := filepath.Dir(iev.Path)

	if iev.Name == "config.v2.json" {
		if iev.Mask&unix.IN_DELETE != 0 {
			ev, _ := onDockerConfigDelete(iev.Path)
			return ev
		}

		ev, _ := onDockerConfigUpdate(iev.Path)
		return ev

	} else if dir == dockerConfig.LocalStorageDir && len(iev.Name) == 64 {
		//
		// Docker seems to normally do atomic updates by moving the
		// new file into place, but monitor IN_CLOSE_WRITE also just
		// in case there are other writes.
		//
		m := (unix.IN_DELETE | unix.IN_MOVED_TO | unix.IN_CLOSE_WRITE)
		d.inotify.AddWatch(iev.Path, uint32(m))
	}

	return nil
}

// -----------------------------------------------------------------------------
// inotify-based Docker sensor
// -----------------------------------------------------------------------------

//
// Singleton sensor state
//
type docker struct {
	ctrl          chan interface{}
	data          chan interface{}
	eventStream   *stream.Stream
	inotify       *inotify.Instance
	inotifyEvents *stream.Stream
	inotifyDone   chan interface{}
	repeater      *stream.Repeater
}

var dockerOnce sync.Once
var dockerControl chan interface{}

//
// Control channel messages
//
type dockerEventStreamRequest struct {
	reply chan *stream.Stream
}

func (d *docker) newStream(m *dockerEventStreamRequest) *stream.Stream {
	// Create a new stream from our Repeater
	return d.repeater.NewStream()
}

func (d *docker) loop() (bool, error) {
	select {
	case e, ok := <-d.ctrl:
		if ok {
			switch e.(type) {
			case *dockerEventStreamRequest:
				m := e.(*dockerEventStreamRequest)
				m.reply <- d.newStream(m)

			default:
				panic(fmt.Sprintf("Unknown type: %T", e))
			}
		} else {
			// control channel was closed, shut down
		}
	}

	return true, nil
}

func (d *docker) handleInotifyEvent(e interface{}) {
	iev := e.(*inotify.Event)

	ev := d.onInotifyEvent(iev)
	if ev != nil {
		d.data <- ev
	}
}

func initializeDockerSensor() error {
	in, err := inotify.NewInstance()
	if err != nil {
		return err
	}

	//
	// Create the global control channel outside of the goroutine to avoid
	// a race condition in NewDockerEventStream()
	//
	dockerControl = make(chan interface{})

	go func() {
		//
		// Create instance inside goroutine so that references don't
		// escape it. This keeps their allocation on the stack and free
		// from the GC.
		//

		data := make(chan interface{})
		d := &docker{
			ctrl: dockerControl,
			data: data,

			eventStream: &stream.Stream{
				Ctrl: dockerControl,
				Data: data,
			},

			inotify: in,
		}

		d.inotifyEvents = in.Events()
		d.inotifyDone =
			stream.ForEach(d.inotifyEvents, d.handleInotifyEvent)

		d.repeater = stream.NewRepeater(d.eventStream)

		// Add a recursive watch for all directories within the Docker
		// local storage directory. These events trigger us to add more
		// specific file watches in onInotifyEvent() above.
		err := d.inotify.AddRecursiveWatch(dockerConfig.LocalStorageDir,
			unix.IN_ONLYDIR|unix.IN_CREATE|unix.IN_DELETE)

		for {
			var ok bool
			ok, err = d.loop()
			if !ok {
				break
			}
		}

		// If the loop exits, just crash
		panic(err)
	}()

	return nil
}

// ----------------------------------------------------------------------------
// Exported interface
// ----------------------------------------------------------------------------

func NewDockerEventStream() (*stream.Stream, error) {
	var err error

	// Initialize singleton sensor if necessary
	dockerOnce.Do(func() {
		err = initializeDockerSensor()
	})

	if err != nil {
		return nil, err
	}

	reply := make(chan *stream.Stream)
	request := &dockerEventStreamRequest{
		reply: reply,
	}

	if dockerControl != nil {
		dockerControl <- request
		response := <-reply

		return response, nil
	}

	return nil, errors.New("Sensor not available")
}
