package sensor

import (
	"log"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/perf"
)

func decodeDoSysOpen(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	ev := newEventFromFieldData(data)
	ev.Event = &api.Event_File{
		File: &api.FileEvent{
			Type:      api.FileEventType_FILE_EVENT_TYPE_OPEN,
			Filename:  data["filename"].(string),
			OpenFlags: int32(data["flags"].(uint64)),
			OpenMode:  int32(data["mode"].(uint64)),
		},
	}

	return ev, nil
}

// -----------------------------------------------------------------------------

func addKprobe() error {
	return perf.AddKprobe("fs/do_sys_open", "do_sys_open", false, "filename=+0(%si):string flags=%dx:s32 mode=%cx:s32")
}

func init() {
	sensor := getSensor()

	err := sensor.registerDecoder("fs/do_sys_open", decodeDoSysOpen)
	if err != nil {
		log.Printf("Tracepoint fs/do_sys_open not found, adding a kprobe to emulate")

		err = addKprobe()
		if err != nil {
			log.Printf("Couldn't add do_sys_open kprobe: %v", err)
			return
		}
		err = sensor.registerDecoder("fs/do_sys_open", decodeDoSysOpen)
		if err != nil {
			log.Printf("Couldn't register observer for fs/do_sys_open kprobe: %v", err)
			return
		}
	}
}
