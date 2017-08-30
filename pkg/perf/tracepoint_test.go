package perf

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGetTraceEventID(t *testing.T) {
	// Root is required to access the tracing fs
	if os.Geteuid() != 0 {
		t.Skip("root privileges are required")
		return
	}

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
	// Root is required to access the tracing fs
	if os.Geteuid() != 0 {
		t.Skip("root privileges are required")
		return
	}

	// Parse every "format" file found under /sys/kernel/debug/tracing/events
	formatFiles, err := collectFormatFiles("")
	if err != nil {
		t.Error(err)
		return
	}

	for i := 0; i < len(formatFiles); i++ {
		fmt.Printf("%s\n", formatFiles[i])
		_, _, err := GetTraceEventFormat(formatFiles[i])
		if err != nil {
			t.Error(err)
			return
		}
	}
}
