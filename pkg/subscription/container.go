package subscription

import (
	"sync"
	"syscall"

	"golang.org/x/sys/unix"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/capsule8/pkg/container"
	"github.com/capsule8/capsule8/pkg/stream"
	"github.com/golang/glog"
)

func newContainerCreated(cID string) *api.ContainerEvent {
	ev := &api.ContainerEvent{
		Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED,
	}

	return ev
}

func newContainerRunning(cID string) *api.ContainerEvent {
	ev := &api.ContainerEvent{
		Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_RUNNING,
	}

	return ev
}

func newContainerExited(cID string) *api.ContainerEvent {
	ev := &api.ContainerEvent{
		Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED,
	}

	return ev
}

func newContainerDestroyed(cID string) *api.ContainerEvent {
	ev := &api.ContainerEvent{
		Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_DESTROYED,
	}

	return ev
}

//
// We get two ContainerCreated events, use this to uniq them
//
var containerCreated map[string]*api.ContainerEvent

//
// We get two ContainerStarted events from the container EventStream:
// one from Docker and one from OCI. We use this map to merge them.
//
var containerStarted map[string]*api.ContainerEvent

func translateContainerEvents(e interface{}) interface{} {
	ce := e.(*container.Event)
	var ece *api.ContainerEvent

	switch ce.State {
	case container.ContainerCreated:
		if containerCreated == nil {
			containerCreated = make(map[string]*api.ContainerEvent)
		}

		if containerCreated[ce.ID] != nil {
			ece = containerCreated[ce.ID]
		} else {
			ece = newContainerCreated(ce.ID)
			ece.Name = ce.Name
			ece.ImageId = ce.ImageID
			ece.ImageName = ce.Image
		}

		if len(ce.DockerConfig) > len(ece.DockerConfigJson) {
			ece.DockerConfigJson = ce.DockerConfig
		}

		if containerCreated[ce.ID] == nil {
			containerCreated[ce.ID] = ece
			ece = nil
		} else {
			delete(containerCreated, ce.ID)
		}

	case container.ContainerStarted:
		if containerStarted == nil {
			containerStarted = make(map[string]*api.ContainerEvent)
		}

		if containerStarted[ce.ID] != nil {
			//
			// If we have already received one container
			// started event, merge the 2nd one into it
			//
			ece = containerStarted[ce.ID]
		} else {
			ece = newContainerRunning(ce.ID)
		}

		if ce.Pid != 0 {
			ece.HostPid = int32(ce.Pid)
		}

		if len(ce.DockerConfig) > 0 {
			ece.DockerConfigJson = ce.DockerConfig
		}

		if len(ce.OciConfig) > 0 {
			ece.OciConfigJson = ce.OciConfig
		}

		if containerStarted[ce.ID] == nil {
			containerStarted[ce.ID] = ece
			ece = nil
		} else {
			delete(containerStarted, ce.ID)
		}

	case container.ContainerStopped:
		ece = newContainerExited(ce.ID)

		if ce.Pid != 0 {
			ece.HostPid = int32(ce.Pid)
		}

		if len(ce.DockerConfig) > 0 {
			ece.DockerConfigJson = ce.DockerConfig
		}

		ece.Name = ce.Name
		ece.ImageId = ce.ImageID
		ece.ImageName = ce.Image

		var exitStatus int
		var exitSignal syscall.Signal
		var coreDumped bool

		ws := unix.WaitStatus(ce.ExitCode)

		if ws.Exited() {
			exitStatus = ws.ExitStatus()
		} else if ws.Signaled() {
			exitSignal = ws.Signal()
			coreDumped = ws.CoreDump()
		}

		ece.ExitCode = ce.ExitCode
		ece.ExitStatus = uint32(exitStatus)
		ece.ExitSignal = uint32(exitSignal)
		ece.ExitCoreDumped = coreDumped

	case container.ContainerRemoved:
		ece = newContainerDestroyed(ce.ID)

	default:
		panic("Invalid value for ContainerState")
	}

	if ece != nil {
		if len(ce.DockerConfig) > 0 {
			ece.DockerConfigJson = ce.DockerConfig
		}

		if len(ce.OciConfig) > 0 {
			ece.OciConfigJson = ce.OciConfig
		}

		ev := newEventFromContainer(ce.ID)
		ev.Event = &api.Event_Container{
			Container: ece,
		}

		return ev
	}

	return nil
}

var containerEventStream struct {
	sync.Once
	err      error
	repeater *stream.Repeater
}

func createContainerEventStream(sub *api.Subscription) (*stream.Stream, error) {
	containerEventStream.Do(func() {
		glog.V(1).Info("Creating container event stream")
		ces, err := container.NewEventStream()
		if err != nil {
			if glog.V(1) {
				glog.Warning("Couldn't create container event stream: %s",
					err)
			}

			containerEventStream.err = err
			return
		}

		// Translate container events to protobuf versions
		ces = stream.Map(ces, translateContainerEvents)
		ces = stream.Filter(ces, filterNils)

		containerEventStream.repeater = stream.NewRepeater(ces)
	})

	if containerEventStream.err != nil {
		return nil, containerEventStream.err
	}

	return containerEventStream.repeater.NewStream(), nil
}
