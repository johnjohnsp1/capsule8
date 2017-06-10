package perf

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const tracefs = "/sys/kernel/debug/tracing"

func GetAvailableTraceEvents() ([]string, error) {
	var events []string

	filename := filepath.Join(tracefs, "available_events")
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
	filename := filepath.Join(tracefs, "events", name, "id")
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
	filename := filepath.Join(tracefs, "events", name, "format")
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}

	var buf [4096]byte
	_, err = file.Read(buf[:])
	if err != nil {
		return "", err
	}

	return string(buf[:]), nil

}
