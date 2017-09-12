package perf

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type extractFileFn func(string, io.Reader) error

func extractFiles(filename string, fn extractFileFn) error {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	gzstream, err := gzip.NewReader(file)
	if err != nil {
		return err
	}

	tarstream := tar.NewReader(gzstream)
	for true {
		header, err := tarstream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag == tar.TypeReg {
			err = fn(header.Name, tarstream)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func testReadTraceEventID(name string, reader io.Reader) error {
	if filepath.Base(name) != "id" {
		return nil
	}

	_, err := ReadTraceEventID(filepath.Dir(name), reader)
	return err
}

func TestReadTraceEventID(t *testing.T) {
	err := extractFiles("testdata/events.tar.gz", testReadTraceEventID)
	if err != nil {
		t.Error(err)
	}
}

func testReadTraceEventFormat(name string, reader io.Reader) error {
	if filepath.Base(name) != "format" {
		return nil
	}

	_, _, err := ReadTraceEventFormat(filepath.Dir(name), reader)
	return err
}

func TestReadTraceEventFormat(t *testing.T) {
	err := extractFiles("testdata/events.tar.gz", testReadTraceEventFormat)
	if err != nil {
		t.Error(err)
	}
}
