package perf

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/capsule8/reactive8/pkg/config"
)

type TraceEventField struct {
	FieldName string
	TypeName  string
	Offset    int
	Size      int
	IsSigned  bool
}

func getTraceFs() string {
	return config.Sensor.TraceFs
}

func AddKprobe(name string, address string, onReturn bool, output string) error {
	filename := filepath.Join(getTraceFs(), "kprobe_events")
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	var cmd string
	if onReturn {
		cmd = fmt.Sprintf("r:%s %s %s", name, address, output)
	} else {
		cmd = fmt.Sprintf("p:%s %s %s", name, address, output)
	}
	_, err = file.Write([]byte(cmd))
	if err != nil {
		return err
	}

	return nil
}

func RemoveKprobe(name string) error {
	filename := filepath.Join(getTraceFs(), "kprobe_events")
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	cmd := fmt.Sprintf("-:%s", name)
	_, err = file.Write([]byte(cmd))
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
	defer file.Close()

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
	defer file.Close()

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

func parseTraceEventField(line string) (*TraceEventField, error) {
	field := &TraceEventField{}
	fields := strings.Split(strings.TrimSpace(line), ";")
	for i := 0; i < len(fields); i++ {
		parts := strings.Split(fields[i], ":")
		if len(parts) != 2 {
			return nil, errors.New("malformed format field")
		}

		var err error
		switch parts[0] {
		case "field":
			x := strings.LastIndexFunc(parts[1], unicode.IsSpace)
			if x < 0 {
				err = errors.New("malformed format field")
			} else {
				field.FieldName = strings.TrimSpace(string(parts[1][x+1:]))
				field.TypeName = strings.TrimSpace(string(parts[1][:x]))
			}
		case "offset":
			field.Offset, err = strconv.Atoi(parts[1])
		case "size":
			field.Size, err = strconv.Atoi(parts[1])
		case "signed":
			field.IsSigned, err = strconv.ParseBool(parts[1])
		}
		if err != nil {
			return nil, err
		}
	}

	return field, nil
}

func GetTraceEventFormat(name string) (map[string]TraceEventField, error) {
	filename := filepath.Join(getTraceFs(), "events", name, "format")
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		log.Printf("Couldn't open trace event %s: %v",
			filename, err)
		return nil, err
	}
	defer file.Close()

	inFormat := false
	fields := make(map[string]TraceEventField)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		if inFormat {
			if !unicode.IsSpace(rune(rawLine[0])) {
				inFormat = false
				continue
			}
			field, err := parseTraceEventField(line)
			if err != nil {
				log.Printf("Couldn't parse trace event format: %v", err)
				return nil, err
			}
			fields[field.FieldName] = *field
		} else if strings.HasPrefix(line, "format:") {
			inFormat = true
		}
	}
	err = scanner.Err()
	if err != nil {
		return nil, err
	}

	return fields, err
}
