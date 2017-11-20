package functional

import (
	"fmt"
	"testing"

	api "github.com/capsule8/api/v0"
	"github.com/golang/glog"
)

type kernelCallTest struct {
	testContainer *Container
	seenEnter     bool
	seenExit      bool
}

const kernelCallDataFilename = "hello.txt"

func (kt *kernelCallTest) BuildContainer(t *testing.T) {
	c := NewContainer(t, "kernelcall")
	err := c.Build()
	if err != nil {
		t.Error(err)
	} else {
		glog.V(2).Infof("Built container %s\n", c.ImageID[0:12])
		kt.testContainer = c
	}
}

func (kt *kernelCallTest) RunContainer(t *testing.T) {
	err := kt.testContainer.Run()
	if err != nil {
		t.Error(err)
	}
	glog.V(2).Infof("Running container %s\n", kt.testContainer.ImageID[0:12])
}

func (kt *kernelCallTest) CreateSubscription(t *testing.T) *api.Subscription {
	kernelEvents := []*api.KernelFunctionCallFilter{
		&api.KernelFunctionCallFilter{
			Type:   api.KernelFunctionCallEventType_KERNEL_FUNCTION_CALL_EVENT_TYPE_ENTER,
			Symbol: "do_sys_open",
			Arguments: map[string]string{
				"filename": "+0(%si):string",
				"mode":     "+0(%cx):u16",
			},
			Filter: fmt.Sprintf("filename ~ %q", kernelCallDataFilename),
		},
		&api.KernelFunctionCallFilter{
			Type:   api.KernelFunctionCallEventType_KERNEL_FUNCTION_CALL_EVENT_TYPE_EXIT,
			Symbol: "do_sys_open",
			Arguments: map[string]string{
				"ret": "$retval",
			},
		},
	}

	// Subscribing to container created events are currently necessary
	// to get imageIDs in other events.
	containerEvents := []*api.ContainerEventFilter{
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED,
		},
	}

	eventFilter := &api.EventFilter{
		KernelEvents:    kernelEvents,
		ContainerEvents: containerEvents,
	}

	return &api.Subscription{
		EventFilter: eventFilter,
	}
}

func (kt *kernelCallTest) HandleTelemetryEvent(t *testing.T, te *api.TelemetryEvent) bool {
	switch event := te.Event.Event.(type) {
	case *api.Event_Container:
		return true

	case *api.Event_KernelCall:
		glog.V(2).Infof("Got Event %+v\n", te.Event)
		if te.Event.ImageId == kt.testContainer.ImageID {

			if filename, ok := event.KernelCall.Arguments["filename"]; ok {
				if filename.FieldType != api.KernelFunctionCallEvent_STRING {
					t.Errorf("Expected argument type %s, got %s\n",
						api.KernelFunctionCallEvent_STRING, filename.FieldType)
				} else if filename.GetStringValue() != kernelCallDataFilename {
					t.Errorf("Expected argument value %q, got %q\n",
						kernelCallDataFilename, filename.GetStringValue())
				}

				kt.seenEnter = true

			} else if ret, ok := event.KernelCall.Arguments["ret"]; ok {
				if ret.FieldType != api.KernelFunctionCallEvent_UINT64 {
					t.Errorf("Expected return type %s, got %s\n",
						api.KernelFunctionCallEvent_UINT64, ret.FieldType)
				}

				kt.seenExit = true

			} else {
				t.Errorf("Unexpected Kernel event %+v", *event.KernelCall)

			}

		} // if te.Event.ImageId == kt.testContainer.ImageID

		return !kt.seenEnter || !kt.seenExit

	default:
		t.Errorf("Unexpected event type %T\n", event)
		return false
	}
}

// TestKernelCall exercises the kernel call events, including filtering.
func TestKernelCall(t *testing.T) {
	kt := &kernelCallTest{}

	tt := NewTelemetryTester(kt)
	tt.RunTest(t)
}
