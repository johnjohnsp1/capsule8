package perf

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/capsule8/reactive8/pkg/config"
)

func getTraceFs() string {
	return config.Sensor.TraceFs
}

func AddKprobe(definition string) error {
	keFilename := filepath.Join(getTraceFs(), "kprobe_events")
	keFile, err := os.OpenFile(keFilename, os.O_WRONLY, 0)
	if err != nil {
		return err
	}

	defer keFile.Close()

	_, err = keFile.Write([]byte(definition))
	if err != nil {
		return err
	}

	return nil
}

func RemoveKprobe(name string) error {
	keFilename := filepath.Join(getTraceFs(), "kprobe_events")
	keFile, err := os.OpenFile(keFilename, os.O_APPEND, 0)
	if err != nil {
		return err
	}

	defer keFile.Close()

	_, err = keFile.Seek(0, os.SEEK_END)
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf("-:%s", name)
	_, err = keFile.Write([]byte(cmd))
	if err != nil {
		return err
	}

	return nil
}

func GetAvailableTraceEvents() ([]string, error) {
	var events []string

	filename := filepath.Join(getTraceFs(), "available_events")
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		events = append(events, scanner.Text())
	}
	err = scanner.Err()
	if err != nil {
		return nil, err
	}

	return events, nil
}

func GetTraceEventID(name string) (uint16, error) {
	filename := filepath.Join(getTraceFs(), "events", name, "id")
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		log.Printf("Couldn't open trace event %s: %v",
			filename, err)
		return 0, err
	}

	//
	// The tracepoint id is a uint16, so we can assume it'll be
	// no longer than 5 characters plus a newline.
	//
	var buf [6]byte
	_, err = file.Read(buf[:])
	if err != nil {
		log.Printf("Couldn't read trace event id: %v", err)
		return 0, err
	}

	idStr := strings.TrimRight(string(buf[:]), "\n\x00")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Printf("Couldn't parse trace event id %s: %v",
			string(buf[:]), err)
		return 0, err
	}

	return uint16(id), nil
}

func GetTraceEventFormat(name string) (string, error) {
	filename := filepath.Join(getTraceFs(), "events", name, "format")
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}

	var buf [4096]byte
	n, err := file.Read(buf[:])
	if err != nil {
		return "", err
	}

	return string(buf[:n]), nil

}

func GetTraceEventFormatSHA256(name string) (string, error) {
	format, err := GetTraceEventFormat(name)
	if err != nil {
		return "", err
	}

	sha := sha256.Sum256([]byte(format))
	return hex.EncodeToString(sha[:]), nil
}
