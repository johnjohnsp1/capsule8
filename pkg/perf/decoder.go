package perf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
)

type TraceEventSampleData map[string]interface{}

type TraceEventDecoderFn func(*SampleRecord, TraceEventSampleData) (interface{}, error)

type traceEventDecoder struct {
	fields    map[string]TraceEventField
	decoderfn TraceEventDecoderFn
}

func decodeDataType(dataType int, rawData []byte) (interface{}, error) {
	switch dataType {
	case dtString:
		return nil, errors.New("internal error; got unexpected dtString")
	case dtS8:
		return int8(rawData[0]), nil
	case dtS16:
		return int16(binary.LittleEndian.Uint16(rawData)), nil
	case dtS32:
		return int32(binary.LittleEndian.Uint32(rawData)), nil
	case dtS64:
		return int64(binary.LittleEndian.Uint64(rawData)), nil
	case dtU8:
		return uint8(rawData[0]), nil
	case dtU16:
		return binary.LittleEndian.Uint16(rawData), nil
	case dtU32:
		return binary.LittleEndian.Uint32(rawData), nil
	case dtU64:
		return binary.LittleEndian.Uint64(rawData), nil
	}
	return nil, errors.New("internal error; undefined dataType")
}

func (d *traceEventDecoder) decodeRawData(rawData []byte) (TraceEventSampleData, error) {
	data := make(TraceEventSampleData)
	for _, field := range d.fields {
		var arraySize, dataLength, dataOffset int
		var err error

		if field.dataLocSize > 0 {
			switch field.dataLocSize {
			case 4:
				dataOffset = int(binary.LittleEndian.Uint16(rawData[field.Offset:]))
				dataLength = int(binary.LittleEndian.Uint16(rawData[field.Offset+2:]))
			case 8:
				dataOffset = int(binary.LittleEndian.Uint32(rawData[field.Offset:]))
				dataLength = int(binary.LittleEndian.Uint32(rawData[field.Offset+4:]))
			default:
				return nil, fmt.Errorf("__data_loc size is neither 4 nor 8 (got %d)", field.dataLocSize)
			}

			if field.dataType == dtString {
				if dataLength > 0 && rawData[dataOffset+dataLength-1] == 0 {
					dataLength--
				}
				data[field.FieldName] = string(rawData[dataOffset : dataOffset+dataLength])
				continue
			}
			arraySize = dataLength / field.dataTypeSize
		} else if field.arraySize == 0 {
			data[field.FieldName], err = decodeDataType(field.dataType, rawData[field.Offset:])
			if err != nil {
				return nil, err
			}
			continue
		} else {
			arraySize = field.arraySize
			dataOffset = field.Offset
			dataLength = arraySize * field.dataTypeSize
		}

		var array []interface{} = make([]interface{}, arraySize)
		for i := 0; i < arraySize; i++ {
			array[i], err = decodeDataType(field.dataType, rawData[dataOffset:])
			if err != nil {
				return nil, err
			}
			dataOffset += field.dataTypeSize
		}
		data[field.FieldName] = array
	}

	return data, nil
}

type TraceEventDecoderMap struct {
	sync.Mutex
	decoders map[uint16]*traceEventDecoder
	nameMap  map[string]uint16
}

func NewTraceEventDecoderMap() *TraceEventDecoderMap {
	return &TraceEventDecoderMap{
		decoders: make(map[uint16]*traceEventDecoder),
	}
}

func (l *TraceEventDecoderMap) AddDecoder(name string, fn TraceEventDecoderFn) (uint16, error) {
	id, fields, err := GetTraceEventFormat(name)
	if err != nil {
		return 0, err
	}

	l.Lock()
	defer l.Unlock()

	if l.decoders == nil {
		l.decoders = make(map[uint16]*traceEventDecoder)
	}
	l.decoders[id] = &traceEventDecoder{
		fields:    fields,
		decoderfn: fn,
	}

	if l.nameMap == nil {
		l.nameMap = make(map[string]uint16)
	}
	l.nameMap[name] = id

	return id, nil
}

func (l *TraceEventDecoderMap) RemoveDecoder(name string) {
	l.Lock()
	defer l.Unlock()

	if l.decoders != nil && l.nameMap != nil {
		id, ok := l.nameMap[name]
		if ok {
			delete(l.decoders, id)
			delete(l.nameMap, name)
		}
	}
}

func (l *TraceEventDecoderMap) getDecoder(eventType uint16) *traceEventDecoder {
	l.Lock()
	defer l.Unlock()

	if l.decoders == nil {
		return nil
	}
	return l.decoders[eventType]
}

func (l *TraceEventDecoderMap) DecodeSample(sample *SampleRecord) (interface{}, error) {
	eventType := uint16(binary.LittleEndian.Uint64(sample.RawData))
	decoder := l.getDecoder(eventType)
	if decoder == nil {
		// Not an error. There just isn't a decoder for this sample
		return nil, nil
	}

	data, err := decoder.decodeRawData(sample.RawData)
	if err != nil {
		return nil, err
	}

	return decoder.decoderfn(sample, data)
}
