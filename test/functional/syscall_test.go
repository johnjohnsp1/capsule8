package functional

import (
	"syscall"
	"testing"

	api "github.com/capsule8/capsule8/api/v0"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes/wrappers"
)

const ALARM_SECS = 37

type syscallTest struct {
	testContainer *Container
	pid           string
	seenEnter     bool
	seenExit      bool
}

func (st *syscallTest) BuildContainer(t *testing.T) string {
	c := NewContainer(t, "syscall")
	err := c.Build()
	if err != nil {
		t.Error(err)
		return ""
	}

	glog.V(2).Infof("Built container %s\n", c.ImageID[0:12])
	st.testContainer = c
	return st.testContainer.ImageID
}

func (st *syscallTest) RunContainer(t *testing.T) {
	err := st.testContainer.Run()
	if err != nil {
		t.Error(err)
	}
	glog.V(2).Infof("Running container %s\n", st.testContainer.ImageID[0:12])
}

func (st *syscallTest) CreateSubscription(t *testing.T) *api.Subscription {
	// TODO Add a filter expression for "Arg0 == ALARM_SECS"
	syscallEvents := []*api.SyscallEventFilter{
		&api.SyscallEventFilter{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER,
			Id:   &wrappers.Int64Value{Value: syscall.SYS_ALARM},
			// Arg0: &wrappers.UInt64Value{Value: ALARM_SECS},
		},
		&api.SyscallEventFilter{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT,
			Id:   &wrappers.Int64Value{Value: syscall.SYS_ALARM},
			Ret:  &wrappers.Int64Value{Value: ALARM_SECS},
		},
	}

	eventFilter := &api.EventFilter{
		SyscallEvents: syscallEvents,
	}

	return &api.Subscription{
		EventFilter: eventFilter,
	}
}

func (st *syscallTest) HandleTelemetryEvent(t *testing.T, te *api.TelemetryEvent) bool {
	glog.V(2).Infof("Got Event %+v\n", te.Event)
	switch event := te.Event.Event.(type) {
	case *api.Event_Container:
		switch event.Container.Type {
		case api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED:
			return true

		default:
			t.Errorf("Unexpected Container event %+v\n", event)
			return false
		}

	case *api.Event_Syscall:
		if event.Syscall.Id != syscall.SYS_ALARM {
			t.Errorf("Expected syscall number %d, got %d\n", syscall.SYS_ALARM, event.Syscall.Id)
		}
		if te.Event.ImageId == st.testContainer.ImageID {
			switch event.Syscall.Type {
			case api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER:
				if te.Event.ImageId == st.testContainer.ImageID {
					if event.Syscall.Arg0 == ALARM_SECS {
						if st.pid != "" {
							t.Error("Already saw container created")
							return false
						}
						st.pid = te.Event.ProcessId

						st.seenEnter = true
					}
				}
			case api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT:
				if te.Event.ImageId == st.testContainer.ImageID && te.Event.ProcessId == st.pid {
					if event.Syscall.Ret != ALARM_SECS {
						t.Errorf("Expected syscall return %d, got %d\n", ALARM_SECS, event.Syscall.Ret)
						return false
					}
					st.seenExit = true
				}
			}
		}

		return !st.seenEnter || !st.seenExit

	default:
		t.Errorf("Unexpected event type %T\n", event)
		return false
	}
}

//
// TestSyscall is a functional test for SyscallEventFilter subscriptions.
// It exercises filtering on Arg0 for SYSCALL_EVENT_TYPE_ENTER events, and
// filtering on Ret for SYSCALL_EVENT_TYPE_EXIT events.
//
func TestSyscall(t *testing.T) {
	// t.Skip("Skipping syscall test until expression engine is complete.")
	st := &syscallTest{}

	tt := NewTelemetryTester(st)
	tt.RunTest(t)
}
