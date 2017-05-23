package sensor

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"time"

	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/capsule8/reactive8/pkg/inotify"
	"github.com/kelseyhightower/envconfig"
	"golang.org/x/sys/unix"
)

//
// Docker >= 1.11 (containerd) Sensor. For more background on containerd,
// runC, and OCI changes in 1.11, see:
// https://medium.com/@tiffanyfayj/docker-1-11-et-plus-engine-is-now-built-on-runc-and-containerd-a6d06d7e80ef
//

type configuration struct {
	// DockerContainerDir is the path to the directory used for docker
	// container local storage areas (i.e. /var/lib/docker/containers)
	DockerContainerDir string `split_words:"true" default:"/var/lib/docker/containers"`

	// OciContainerDir is the path to the directory used for the container runtime's
	// container state directories (i.e. /var/run/docker/libcontainerd)
	OciContainerDir string `split_words:"true" default:"/var/run/docker/libcontainerd"`
}

var config configuration

// ----------------------------------------------------------------------------
// Global container state tracking (shared among sessions)
// ----------------------------------------------------------------------------

type subscriber struct {
	selector event.Selector
	events   chan<- *event.Event
	stop     chan struct{}
	err      chan<- error
}

//
// Since subscriber is an atomic Value, we only need to use the mutex to lock
// writes.
//
var subscribers atomic.Value
var subscribersMutex sync.Mutex

const (
	containerUnknown int = 0
	containerCreated int = 1
	containerStarted int = 2
	containerStopped int = 3
)

type container struct {
	state int
}

type containerState struct {
	once      sync.Once
	mutex     sync.Mutex
	newSub    chan subscriber
	events    chan *event.Event
	stop      chan struct{}
	errors    chan error
	byID      map[string]*container
	inotifier *inotify.Instance
}

var containers = &containerState{}

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
}

// ----------------------------------------------------------------------------
// OCI configuration file format
// ----------------------------------------------------------------------------

// ----------------------------------------------------------------------------
// Docker and OCI container configuration file change to Events state machine
// ----------------------------------------------------------------------------

func onDockerConfigUpdate(configPath string) (*event.Event, error) {
	//
	// Look for file rename to config.v2.json to identify container created
	// events. This happens on a few updates to config.v2.json, so we need
	// to be sure to only send the first one that we see.
	//
	containerID := filepath.Base(filepath.Dir(configPath))

	var ev *event.Event
	var err error

	containers.mutex.Lock()

	if containers.byID[containerID] == nil {
		data, err := ioutil.ReadFile(configPath)

		if err == nil {
			config := &dockerConfigV2{}
			err = json.Unmarshal(data, &config)

			ev = &event.Event{
				Subevent: &event.Event_Container{
					Container: &event.ContainerEvent{
						ContainerID: containerID,
						State:       event.ContainerEvent_CONTAINER_CREATED,
					},
				},
			}

			c := &container{state: containerCreated}
			containers.byID[containerID] = c
		}
	}

	containers.mutex.Unlock()

	return ev, err

}

func onDockerConfigDelete(configPath string) (*event.Event, error) {
	//
	// Look for deletion of config.v2.json to identify container removed
	// events.
	//
	containerID := filepath.Base(filepath.Dir(configPath))

	ev := &event.Event{
		Subevent: &event.Event_Container{
			Container: &event.ContainerEvent{
				ContainerID: containerID,
				State:       event.ContainerEvent_CONTAINER_REMOVED,
			},
		},
	}

	containers.mutex.Lock()

	containers.byID[containerID] = nil

	containers.mutex.Unlock()

	return ev, nil
}

func onOciConfigUpdate(configPath string) (*event.Event, error) {
	//
	// Look for the close of an open for write to identify container started
	// events.
	//
	containerID := filepath.Base(filepath.Dir(configPath))

	ev := &event.Event{
		Subevent: &event.Event_Container{
			Container: &event.ContainerEvent{
				ContainerID: containerID,
				State:       event.ContainerEvent_CONTAINER_STARTED,
			},
		},
	}

	containers.mutex.Lock()

	containers.byID[containerID].state = containerStarted

	containers.mutex.Unlock()

	return ev, nil
}

func onOciConfigDelete(configPath string) (*event.Event, error) {
	//
	// Look for deletion of config.json to identify container stopped events.
	//
	containerID := filepath.Base(filepath.Dir(configPath))

	ev := &event.Event{
		Subevent: &event.Event_Container{
			Container: &event.ContainerEvent{
				ContainerID: containerID,
				State:       event.ContainerEvent_CONTAINER_STOPPED,
			},
		},
	}

	containers.mutex.Lock()

	containers.byID[containerID].state = containerStopped

	containers.mutex.Unlock()

	return ev, nil
}

// ----------------------------------------------------------------------------
// Docker OCI Sensor
// ----------------------------------------------------------------------------

func dockerWalk(path string, info os.FileInfo, err error) error {
	base := filepath.Base(path)

	if path == config.DockerContainerDir {
		inotifyErr := containers.inotifier.AddRecursive(path,
			unix.IN_ONLYDIR|unix.IN_CREATE|unix.IN_DELETE)
		if inotifyErr != nil {
			return inotifyErr
		}
	} else if base == "config.v2.json" {
		data, err := ioutil.ReadFile(path)

		if err == nil {
			config := &dockerConfigV2{}
			err = json.Unmarshal(data, &config)

			containerID := config.ID

			if containers.byID[containerID] == nil {
				if config.State.FinishedAt.Unix() > 0 {
					c := &container{state: containerStopped}
					containers.byID[containerID] = c
				} else if config.State.StartedAt.Unix() < 0 {
					c := &container{state: containerCreated}
					containers.byID[containerID] = c
				} else if config.State.StartedAt.Unix() > 0 {
					c := &container{state: containerStarted}
					containers.byID[containerID] = c
				} else {
					panic("Couldn't identify container state")
				}
			}
		}
	}

	return nil
}

func ociWalk(path string, info os.FileInfo, err error) error {
	base := filepath.Base(path)

	if path == config.OciContainerDir {
		inotifyErr := containers.inotifier.AddRecursive(path,
			unix.IN_ONLYDIR|unix.IN_CREATE|unix.IN_DELETE)
		if inotifyErr != nil {
			return inotifyErr
		}
	} else if base == "config.json" {
		containerID := filepath.Base(filepath.Dir(path))

		if containers.byID[containerID] == nil ||
			containers.byID[containerID].state < containerStarted {
			//
			// If container state is unknown or containerCreated, update it
			//
			containers.byID[containerID].state = containerStarted
		}
	}

	return nil
}

func eventReceiver() {
	for {
		iev, ok := containers.inotifier.Next()
		if ok {
			dirname := containers.inotifier.Path(iev.Wd)
			path := filepath.Join(dirname, iev.Name)

			if iev.Name == "config.json" {
				if iev.Mask&unix.IN_CLOSE_WRITE != 0 {
					ev, _ := onOciConfigUpdate(path)
					if ev != nil {
						containers.events <- ev
					}

				} else if iev.Mask&unix.IN_DELETE != 0 {
					ev, _ := onOciConfigDelete(path)
					if ev != nil {
						containers.events <- ev
					}
				}
			} else if iev.Name == "config.v2.json" {
				if iev.Mask&unix.IN_MOVED_TO != 0 {
					ev, _ := onDockerConfigUpdate(path)

					if ev != nil {
						containers.events <- ev
					}
				} else if iev.Mask&unix.IN_DELETE != 0 {
					ev, _ := onDockerConfigDelete(path)

					if ev != nil {
						containers.events <- ev
					}
				}

			} else if dirname == config.DockerContainerDir && len(iev.Name) == 64 {
				containers.inotifier.Add(path, unix.IN_MOVED_TO|unix.IN_DELETE)
			} else if dirname == config.OciContainerDir && len(iev.Name) == 64 {
				containers.inotifier.Add(path, unix.IN_CLOSE_WRITE|unix.IN_DELETE)
			}
		}
	}
}

func eventSender() {
	select {
	case <-containers.stop:
		return

	// Receive new subscriber channels on a channel
	case sub, ok := <-containers.newSub:
		if ok {
			subscribersMutex.Lock()

			s1 := subscribers.Load().([]subscriber)
			s2 := append(s1, sub)
			subscribers.Store(s2)

			subscribersMutex.Unlock()

		} else {
			return
		}

	case ev, ok := <-containers.events:
		if ok {
			// Send it to any relevant subscribers
			s := subscriber.Load()
			sub := s.(*subscriberState)
			if sub.events != nil {
				sub.events <- ev
			}
		} else {
			return
		}
	}
}

//
// Initialize singleton sensor and its global state. It is wasteful to have
// multiple subscribers creating redundant inotify instances in the kernel,
// so we instead manage a singleton instance of the union of subscribers'
// interests.
//
func initializeSensor() error {
	//
	// Apply configuration overrides from environment.
	//
	err := envconfig.Process("DOCKER_OCI_SENSOR", &config)
	if err != nil {
		return err
	}

	inotifier, err := inotify.NewInstance()
	if err != nil {
		return err
	}

	containers.inotifier = inotifier
	containers.events = make(chan *event.Event)
	containers.stop = make(chan struct{})
	containers.errors = make(chan error)
	containers.byID = make(map[string]*container)

	// Scan for existing containers and add inotify watches
	filepath.Walk(config.DockerContainerDir, dockerWalk)
	filepath.Walk(config.OciContainerDir, ociWalk)

	// Create a goroutine to send selected Events to subscribers
	go eventSender()

	//
	// Create a goroutine to receive inotify events and process them into
	// Events.
	//
	go eventReceiver()

	return nil
}

// NewDockerOCISensor creates a new ContainerEvent sensor for OCI-compliant
// versions of Docker (>= 1.11).
func NewDockerOCISensor(selector event.Selector, events chan<- *event.Event) (chan<- struct{}, <-chan error) {
	errorChan := make(chan error, 1)
	stopChan := make(chan struct{})

	// Initialize global sensor state only once
	containers.once.Do(func() {
		err := initializeSensor()

		if err != nil {
			errorChan <- err
		}
	})

	//
	// XXX: Only supports one subscriber for now
	//
	sub := &subscriber{
		selector: selector,
		stop:     stopChan,
		events:   events,
		err:      errorChan,
	}

	subscriberMutex.Lock()
	subscribers.Store(sub)
	subscriberMutex.Unlock()

	//
	// Create a goroutine to wait on the sensor stop channel and forward events
	//
	go func() {
		defer func() {
			subscriberMutex.Lock()
			subscribers.Store(&subscriber{})
			subscriberMutex.Unlock()

			close(errorChan)
		}()

		for {
			select {
			case <-stopChan:
				return

			case e, ok := <-containers.events:
				if ok {
					events <- e
				} else {
					return
				}
			}
		}
	}()

	return stopChan, errorChan
}
