package sensor

import (
	"bytes"
	"encoding/binary"
	"log"

	"github.com/capsule8/reactive8/pkg/api/event"
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
*/

type doSysOpenFormat struct {
	CommonType         uint16
	CommonFlags        uint8
	CommonPreemptCount uint8
	CommonPid          int32
	FilenameOffset     int16
	FilenameLength     int16
	Flags              int32
	Mode               int32
}

func decodeDoSysOpen(rawData []byte) (interface{}, error) {
	reader := bytes.NewReader(rawData)

	format := doSysOpenFormat{}
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

	containerID, err := pidMapGetContainerID(format.CommonPid)

	return &event.Event{
		ContainerId: containerID,

		Event: &event.Event_File{
			File: &event.FileEvent{
				Type:      event.FileEventType_FILE_EVENT_TYPE_OPEN,
				Filename:  string(fileName),
				OpenFlags: format.Flags,
				OpenMode:  format.Mode,
			},
		},
	}, nil
}

// -----------------------------------------------------------------------------

func init() {
	sensor := getSensor()

	eventName := "fs/do_sys_open"
	doSysOpenID, err := perf.GetTraceEventID(eventName)
	if err != nil {
		log.Printf("Couldn't get %s event id: %v", eventName, err)
	}

	sensor.registerDecoder(doSysOpenID, decodeDoSysOpen)
}
