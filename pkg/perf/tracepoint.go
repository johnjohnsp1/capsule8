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

const (
	dtString int = iota
	dtS8
	dtS16
	dtS32
	dtS64
	dtU8
	dtU16
	dtU32
	dtU64
)

type TraceEventField struct {
	FieldName string
	TypeName  string
	Offset    int
	Size      int
	IsSigned  bool

	dataType     int // data type constant from above
	dataTypeSize int
	arraySize    int // -1 == not an array, 0 == [] array, >0 == # elements
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

func parseTypeName(s string, size int, isSigned bool) (int, int, int, error) {
	if strings.HasPrefix(s, "__data_loc") {
		s = s[11:]
		if s == "char[]" {
			return dtString, 1, 0, nil
		}
	}

	if strings.HasSuffix(s, "[]") {
		dataType, dataTypeSize, _, err := parseTypeName(s[:len(s)-2], size, isSigned)
		return dataType, dataTypeSize, 0, err
	}
	if strings.HasSuffix(s, "]") {
		x := strings.Index(s, "[")
		if x < 0 {
			return 0, 0, 0, errors.New("malformed type name")
		}
		dataType, dataTypeSize, _, err := parseTypeName(s[:x], size, isSigned)
		if err != nil {
			return 0, 0, 0, err
		}
		arraySize, err := strconv.Atoi(s[x+1 : len(s)-1])
		return dataType, dataTypeSize, arraySize, err
	}

	// Except for prefix and suffix information, ignore the type name.
	// For kprobes and uprobes, the type names will be standard, but for
	// tracepoints the type names will be whatever is used in the kernel
	// source. Handling all possibilities is not feasible, so consider only
	// the size and signed flag
	switch size {
	case 1:
		if isSigned {
			return dtS8, 1, -1, nil
		}
		return dtU8, 1, -1, nil
	case 2:
		if isSigned {
			return dtS16, 2, -1, nil
		}
		return dtU16, 2, -1, nil
	case 4:
		if isSigned {
			return dtS32, 4, -1, nil
		}
		return dtU32, 4, -1, nil
	case 8:
		if isSigned {
			return dtS64, 8, -1, nil
		}
		return dtU64, 8, -1, nil
	}
	return 0, 0, 0, errors.New(fmt.Sprintf("unrecognized type name \"%s\"", s))
}

func parseTraceEventField(line string) (*TraceEventField, error) {
	var err error

	field := &TraceEventField{}
	fields := strings.Split(strings.TrimSpace(line), ";")
	for i := 0; i < len(fields); i++ {
		if fields[i] == "" {
			continue
		}
		parts := strings.Split(fields[i], ":")
		if len(parts) != 2 {
			return nil, errors.New("malformed format field")
		}

		switch strings.TrimSpace(parts[0]) {
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

	field.dataType, field.dataTypeSize, field.arraySize, err = parseTypeName(field.TypeName, field.Size, field.IsSigned)
	return field, nil
}

func GetTraceEventFormat(name string) (uint16, map[string]TraceEventField, error) {
	filename := filepath.Join(getTraceFs(), "events", name, "format")
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		log.Printf("Couldn't open trace event %s: %v",
			filename, err)
		return 0, nil, err
	}
	defer file.Close()

	var eventID uint16

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
				return 0, nil, err
			}
			fields[field.FieldName] = *field
		} else if strings.HasPrefix(line, "format:") {
			inFormat = true
		} else if strings.HasPrefix(line, "ID:") {
			value := strings.TrimSpace(line[3:])
			parsedValue, err := strconv.Atoi(value)
			if err != nil {
				log.Printf("Couldn't parse trace event ID: %v", err)
				return 0, nil, err
			}
			eventID = uint16(parsedValue)
		}
	}
	err = scanner.Err()
	if err != nil {
		return 0, nil, err
	}

	return eventID, fields, err
}
