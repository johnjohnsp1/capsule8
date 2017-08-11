package sensor

import (
	"bytes"
	"encoding/binary"
	"log"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/perf"
)

/*
name: do_sys_open
ID: 573
format:
	field:unsigned short common_type;	offset:0;	size:2;	signed:0;
	field:unsigned char common_flags;	offset:2;	size:1;	signed:0;
	field:unsigned char common_preempt_count;	offset:3;	size:1;	signed:0;
	field:int common_pid;	offset:4;	size:4;	signed:1;

	field:__data_loc char[] filename;	offset:8;	size:4;	signed:1;
	field:int flags;	offset:12;	size:4;	signed:1;
	field:int mode;	offset:16;	size:4;	signed:1;

print fmt: ""%s" %x %o", __get_str(filename), REC->flags, REC->mode

name: do_sys_open
ID: 1601
format:
	field:unsigned short common_type;	offset:0;	size:2;	signed:0;
	field:unsigned char common_flags;	offset:2;	size:1;	signed:0;
	field:unsigned char common_preempt_count;	offset:3;	size:1;	signed:0;
	field:int common_pid;	offset:4;	size:4;	signed:1;

	field:unsigned long __probe_ip;	offset:8;	size:8;	signed:0;
	field:__data_loc char[] filename;	offset:16;	size:4;	signed:1;
	field:s32 flags;	offset:20;	size:4;	signed:1;
	field:s32 mode;	offset:24;	size:4;	signed:1;

print fmt: "(%lx) filename=\"%s\" flags=%d mode=%d", REC->__probe_ip, __get_str(filename), REC->flags, REC->mode
*/

type doSysOpenFormat struct {
	perf.TraceEvent
	FilenameOffset int16
	FilenameLength int16
	Flags          int32
	Mode           int32
}

type kpDoSysOpenFormat struct {
	perf.TraceEvent
	_              uint64
	FilenameOffset int16
	FilenameLength int16
	Flags          int32
	Mode           int32
}

func decodeKpDoSysOpen(rawData []byte) (interface{}, error) {
	reader := bytes.NewReader(rawData)

	format := kpDoSysOpenFormat{}
	err := binary.Read(reader, binary.LittleEndian, &format)
	if err != nil {
		return nil, err
	}

	fileName := make([]byte, format.FilenameLength)
	reader.ReadAt(fileName, int64(format.FilenameOffset))

	// Remove trailing NULL byte b/c Golang strings are 8-bit clean
	if fileName[len(fileName)-1] == 0 {
		fileName = fileName[:len(fileName)-1]
	}

	ev := newEventFromTraceEvent(&format.TraceEvent)
	ev.Event = &api.Event_File{
		File: &api.FileEvent{
			Type:      api.FileEventType_FILE_EVENT_TYPE_OPEN,
			Filename:  string(fileName),
			OpenFlags: format.Flags,
			OpenMode:  format.Mode,
		},
	}

	return ev, nil

}

func decodeDoSysOpen(rawData []byte) (interface{}, error) {
	reader := bytes.NewReader(rawData)

	format := doSysOpenFormat{}
	err := binary.Read(reader, binary.LittleEndian, &format)
	if err != nil {
		return nil, err
	}

	if uint(format.FilenameLength) > uint(len(rawData)) {
		//
		// In the kprobe format, the __probe_ip field is at the
		// same offset as FilenameLength and FilenameOffset.
		// These values are usually like ffffffffa51e8a90, so
		// the offset will be beyond the length of rawData.
		//
		return decodeKpDoSysOpen(rawData)
	}

	fileName := make([]byte, format.FilenameLength)
	reader.ReadAt(fileName, int64(format.FilenameOffset))

	// Remove trailing NULL byte b/c Golang strings are 8-bit clean
	if fileName[len(fileName)-1] == 0 {
		fileName = fileName[:len(fileName)-1]
	}

	ev := newEventFromTraceEvent(&format.TraceEvent)
	ev.Event = &api.Event_File{
		File: &api.FileEvent{
			Type:      api.FileEventType_FILE_EVENT_TYPE_OPEN,
			Filename:  string(fileName),
			OpenFlags: format.Flags,
			OpenMode:  format.Mode,
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

	eventName := "fs/do_sys_open"
	doSysOpenID, err := perf.GetTraceEventID(eventName)
	if err != nil {
		log.Printf("Tracepoint fs/do_sys_open not found, adding a kprobe to emulate")

		err := addKprobe()
		if err != nil {
			log.Printf("Couldn't add do_sys_open kprobe: %s", err)

			// Don't register decoder
			return
		}

		doSysOpenID, err = perf.GetTraceEventID(eventName)
		if err != nil {
			log.Printf("Couldn't get trace event ID for kprobe fs/do_sys_open")
		}
	}

	if doSysOpenID > 0 {
		sensor.registerDecoder(doSysOpenID, decodeDoSysOpen)
	}
}
