package perf

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGetTraceEventID(t *testing.T) {
	_, err := GetTraceEventID("fs/do_sys_open")
	if err != nil {
		t.Error(err)
		return
	}
}

func collectFormatFiles(path string) ([]string, error) {
	f, err := os.Open(filepath.Join(getTraceFs(), "events", path))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	files, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(files))
	for i := 0; i < len(files); i++ {
		if files[i].IsDir() {
			subfiles, err := collectFormatFiles(filepath.Join(path, files[i].Name()))
			if err != nil {
				return nil, err
			}
			for j := 0; j < len(subfiles); j++ {
				result = append(result, subfiles[j])
			}
		}
		if files[i].Name() == "format" {
			result = append(result, path)
		}
	}

	return result, nil
}

func TestGetTraceEventFormat(t *testing.T) {
	// Parse every "format" file found under /sys/kernel/debug/tracing/events
	formatFiles, err := collectFormatFiles("")
	if err != nil {
		t.Error(err)
		return
	}

	for i := 0; i < len(formatFiles); i++ {
		fmt.Printf("%s\n", formatFiles[i])
		_, err := GetTraceEventFormat(formatFiles[i])
		if err != nil {
			t.Error(err)
			return
		}
	}
}

/*
var fields map[string]TraceEventField

func handlePerfEvent(sample *Sample, err error) {
	if err != nil {
		fmt.Printf("handlePerfEvent error: %v", err)
		return
	}

	fmt.Printf("Got sample: type %d misc %d size %d\n", sample.Type, sample.Misc, sample.Size)
	switch sample.Type {
	case PERF_RECORD_SAMPLE:
		record := sample.Record.(*SampleRecord)

		fmt.Printf("pid %d, tid %d, time %d, size %d\n", record.Pid, record.Tid, record.Time, len(record.RawData))
		data, err := DecodeTraceEvent(record.RawData, fields)
		if err != nil {
			fmt.Printf("DecodeTraceEvent error %v\n", err)
		} else {
			for k, v := range(data) {
				fmt.Printf("    %s = %v\n", k, v)
			}
		}
	}
}

func TestDecodeTraceEvent(t *testing.T) {
	id, err := GetTraceEventID("fs/do_sys_open")
	if err != nil {
		t.Error(err)
		return
	}

	fields, err = GetTraceEventFormat("fs/do_sys_open")
	if err != nil {
		t.Error(err)
		return
	}

	fmt.Printf("Event ID %d\n", id)
	fmt.Printf("Formatted fields:\n")
	for k, v := range(fields) {
		fmt.Printf("\t%s: %s offset %d size %d\n", k, v.TypeName, v.Offset, v.Size)
	}

	ea := &EventAttr{}
	ea.Type = PERF_TYPE_TRACEPOINT
	ea.Config = uint64(id)
	ea.SampleType = PERF_SAMPLE_TID | PERF_SAMPLE_TIME | PERF_SAMPLE_RAW
	//	ea.Disabled = true
	ea.SamplePeriod = 1
	ea.Watermark = true
	ea.WakeupEvents = 1

	attrs := []*EventAttr{ea}

	perf, err := New(attrs, nil)
	if err != nil {
		t.Error(err)
		return
	}
	defer perf.Close()

	err = perf.Run(handlePerfEvent)
	if err != nil {
		t.Error(err)
		return
	}
}
*/
