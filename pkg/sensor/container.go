package sensor

import (
	api "github.com/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/container"
	"github.com/capsule8/capsule8/pkg/filter"
	"github.com/capsule8/capsule8/pkg/stream"

	"golang.org/x/sys/unix"
)

type ContainerEventRepeater struct {
	repeater *stream.Repeater
	sensor   *Sensor

	// We get two ContainerCreated events. Use this map to merge them
	createdMap map[string]*api.ContainerEvent

	// We get two ContainerStarted events from the container EventStream:
	// one from Docker and one from OCI. Use this map to merge them
	startedMap map[string]*api.ContainerEvent
}

func newContainerCreated(cID string) *api.ContainerEvent {
	return &api.ContainerEvent{
		Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED,
	}
}

func newContainerRunning(cID string) *api.ContainerEvent {
	return &api.ContainerEvent{
		Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_RUNNING,
	}
}

func newContainerExited(cID string) *api.ContainerEvent {
	return &api.ContainerEvent{
		Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED,
	}
}

func newContainerDestroyed(cID string) *api.ContainerEvent {
	return &api.ContainerEvent{
		Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_DESTROYED,
	}
}

func (cer *ContainerEventRepeater) translateContainerEvents(e interface{}) interface{} {
	ce := e.(*container.Event)
	var ece *api.ContainerEvent

	switch ce.State {
	case container.ContainerCreated:
		if cer.createdMap == nil {
			cer.createdMap = make(map[string]*api.ContainerEvent)
		}

		if cer.createdMap[ce.ID] != nil {
			ece = cer.createdMap[ce.ID]
		} else {
			ece = newContainerCreated(ce.ID)
			ece.Name = ce.Name
			ece.ImageId = ce.ImageID
			ece.ImageName = ce.Image
		}

		if len(ce.DockerConfig) > len(ece.DockerConfigJson) {
			ece.DockerConfigJson = ce.DockerConfig
		}

		if cer.createdMap[ce.ID] == nil {
			cer.createdMap[ce.ID] = ece
			ece = nil
		} else {
			delete(cer.createdMap, ce.ID)
		}

	case container.ContainerStarted:
		if cer.startedMap == nil {
			cer.startedMap = make(map[string]*api.ContainerEvent)
		}

		if cer.startedMap[ce.ID] != nil {
			//
			// If we have already received one container
			// started event, merge the 2nd one into it
			//
			ece = cer.startedMap[ce.ID]
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

		if cer.startedMap[ce.ID] == nil {
			cer.startedMap[ce.ID] = ece
			ece = nil
		} else {
			delete(cer.startedMap, ce.ID)
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
		ece.ExitCode = ce.ExitCode

		ws := unix.WaitStatus(ce.ExitCode)

		if ws.Exited() {
			ece.ExitStatus = uint32(ws.ExitStatus())
		} else if ws.Signaled() {
			ece.ExitSignal = uint32(ws.Signal())
			ece.ExitCoreDumped = ws.CoreDump()
		}

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

		ev := cer.sensor.NewEventFromContainer(ce.ID)
		ev.Event = &api.Event_Container{
			Container: ece,
		}

		return ev
	}

	return nil
}

func NewContainerEventRepeater(sensor *Sensor) (*ContainerEventRepeater, error) {
	ces, err := container.NewEventStream()
	if err != nil {
		return nil, err
	}

	cer := &ContainerEventRepeater{
		sensor: sensor,
	}

	// Translate container events to protobuf versions
	ces = stream.Map(ces, cer.translateContainerEvents)
	ces = stream.Filter(ces, filterNils)
	cer.repeater = stream.NewRepeater(ces)

	return cer, nil
}

func (cer *ContainerEventRepeater) NewEventStream(sub *api.Subscription) (*stream.Stream, error) {
	s := cer.repeater.NewStream()

	// Apply a filter based on this unique subscription to the copy
	// of the container event stream that we got from the Repeater.
	s = stream.Filter(s, func(i interface{}) bool {
		e := i.(*api.Event)

		switch e.Event.(type) {
		case *api.Event_Container:
			cev := e.GetContainer()

			for _, cef := range sub.EventFilter.ContainerEvents {
				mappings := make(map[string]*filter.MappedField)
				match, _ :=
					filter.CompareFields(cef, cev, mappings)

				if !match {
					continue
				}

				if cef.View != api.ContainerEventView_FULL {
					cev.OciConfigJson = ""
					cev.DockerConfigJson = ""
				}

				return true
			}
		}

		return false
	})

	return s, nil
}
