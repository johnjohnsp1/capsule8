package filter

import (
	"os"
	"testing"

	api "github.com/capsule8/api/v0"
	"github.com/golang/protobuf/ptypes/wrappers"
)

func TestFilterSyscallEventType(t *testing.T) {
	cf := NewEventFilter(&api.EventFilter{
		SyscallEvents: []*api.SyscallEventFilter{
			&api.SyscallEventFilter{
				Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER,
			},
		},
	})

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_Syscall{
			Syscall: &api.SyscallEvent{
				Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER,
			},
		},
	}); !match {
		t.Error("No matching syscall event found for type ENTER")
	}

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_Syscall{
			Syscall: &api.SyscallEvent{
				Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT,
			},
		},
	}); match {
		t.Error("Unexpected matching syscall event found for type EXIT")
	}
}

func TestFilterSyscallEventId(t *testing.T) {
	cf := NewEventFilter(&api.EventFilter{
		SyscallEvents: []*api.SyscallEventFilter{
			&api.SyscallEventFilter{
				Id: &wrappers.Int64Value{
					Value: 1,
				},
			},
		},
	})

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_Syscall{
			Syscall: &api.SyscallEvent{
				Id: 1,
			},
		},
	}); !match {
		t.Error("No matching syscall event found for id 1")
	}

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_Syscall{
			Syscall: &api.SyscallEvent{
				Id: 2,
			},
		},
	}); match {
		t.Error("Unexpected matching syscall event found for id 2")
	}
}

func TestFilterSyscallEventArgs(t *testing.T) {
	cf := NewEventFilter(&api.EventFilter{
		SyscallEvents: []*api.SyscallEventFilter{
			&api.SyscallEventFilter{
				// Arbitrarily AND together arguments
				Arg0: &wrappers.UInt64Value{
					Value: 0,
				},
				Arg3: &wrappers.UInt64Value{
					Value: 3,
				},
				Arg5: &wrappers.UInt64Value{
					Value: 5,
				},
			},
		},
	})

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_Syscall{
			Syscall: &api.SyscallEvent{
				Arg0: 0,
				Arg3: 3,
				Arg5: 5,
			},
		},
	}); !match {
		t.Error("No matching syscall event found for args")
	}

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_Syscall{
			Syscall: &api.SyscallEvent{
				Arg0: 0,
				Arg3: 2,
				Arg5: 5,
			},
		},
	}); match {
		t.Error("Unexpected matching syscall event found for args")
	}
}

func TestFilterSyscallEventRet(t *testing.T) {
	cf := NewEventFilter(&api.EventFilter{
		SyscallEvents: []*api.SyscallEventFilter{
			&api.SyscallEventFilter{
				Ret: &wrappers.Int64Value{
					Value: 0,
				},
			},
		},
	})

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_Syscall{
			Syscall: &api.SyscallEvent{
				Ret: 0,
			},
		},
	}); !match {
		t.Error("No matching syscall event found for ret 0")
	}

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_Syscall{
			Syscall: &api.SyscallEvent{
				Ret: 1,
			},
		},
	}); match {
		t.Error("Unexpected matching syscall event found for ret 1")
	}
}

func TestFilterProcessEventType(t *testing.T) {
	cf := NewEventFilter(&api.EventFilter{
		ProcessEvents: []*api.ProcessEventFilter{
			&api.ProcessEventFilter{
				Type: api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
			},
		},
	})

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_Process{
			Process: &api.ProcessEvent{
				Type: api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
			},
		},
	}); !match {
		t.Error("No matching process event found for type EXEC")
	}

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_Process{
			Process: &api.ProcessEvent{
				Type: api.ProcessEventType_PROCESS_EVENT_TYPE_FORK,
			},
		},
	}); match {
		t.Error("Unexpected matching process event found for type FORK")
	}
}

func TestFilterFileEventType(t *testing.T) {
	cf := NewEventFilter(&api.EventFilter{
		FileEvents: []*api.FileEventFilter{
			&api.FileEventFilter{
				Type: api.FileEventType_FILE_EVENT_TYPE_OPEN,
			},
		},
	})

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_File{
			File: &api.FileEvent{
				Type: api.FileEventType_FILE_EVENT_TYPE_OPEN,
			},
		},
	}); !match {
		t.Error("No matching file event found for type OPEN")
	}

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_File{
			File: &api.FileEvent{
				Type: api.FileEventType_FILE_EVENT_TYPE_UNKNOWN,
			},
		},
	}); match {
		t.Error("Unexpected matching file event found for type UNKNOWN")
	}
}

func TestFilterFileEventFilename(t *testing.T) {
	cf := NewEventFilter(&api.EventFilter{
		FileEvents: []*api.FileEventFilter{
			&api.FileEventFilter{
				Filename: &wrappers.StringValue{
					Value: "imafile",
				},
			},
		},
	})

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_File{
			File: &api.FileEvent{
				Filename: "imafile",
			},
		},
	}); !match {
		t.Error("No matching file event found for name 'imafile'")
	}

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_File{
			File: &api.FileEvent{
				Filename: "notafile",
			},
		},
	}); match {
		t.Error("Unexpected matching file event found for name 'notafile'")
	}
}

func TestFilterFileEventFilenamePattern(t *testing.T) {
	cf := NewEventFilter(&api.EventFilter{
		FileEvents: []*api.FileEventFilter{
			&api.FileEventFilter{
				FilenamePattern: &wrappers.StringValue{
					Value: "maybe*",
				},
			},
		},
	})

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_File{
			File: &api.FileEvent{
				Filename: "maybesuccess",
			},
		},
	}); !match {
		t.Error("No matching file event found for name 'maybesuccess'")
	}

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_File{
			File: &api.FileEvent{
				Filename: "nopefailure",
			},
		},
	}); match {
		t.Error("Unexpected matching file event found for name 'nopefailure'")
	}
}

func TestFilterFileEventOpenFlagsMask(t *testing.T) {
	cf := NewEventFilter(&api.EventFilter{
		FileEvents: []*api.FileEventFilter{
			&api.FileEventFilter{
				// TODO: O_RDONLY is 0 so we can't actually filter those out w/ a bitand op
				// problem should be fixed when we migrate to supplying operators in the protos
				// Mask for READ WRITE
				OpenFlagsMask: &wrappers.Int32Value{
					Value: int32(1),
				},
			},
		},
	})

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_File{
			File: &api.FileEvent{
				OpenFlags: int32(os.O_RDWR),
			},
		},
	}); !match {
		t.Error("No matching file event found for name open flags READ WRITE")
	}

	if match := cf.FilterFunc(&api.Event{
		Event: &api.Event_File{
			File: &api.FileEvent{
				OpenFlags: int32(os.O_WRONLY),
			},
		},
	}); match {
		t.Error("Unexpected matching file event found for open flags WRITE ONLY")
	}
}
