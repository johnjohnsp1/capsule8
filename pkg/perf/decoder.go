package perf

import (
	"encoding/binary"
	"errors"
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
		return uint64(rawData[0]), nil
	case dtS16:
		return uint64(binary.LittleEndian.Uint16(rawData)), nil
	case dtS32:
		return uint64(binary.LittleEndian.Uint32(rawData)), nil
	case dtS64:
		return binary.LittleEndian.Uint64(rawData), nil
	case dtU8:
		return uint64(rawData[0]), nil
	case dtU16:
		return uint64(binary.LittleEndian.Uint16(rawData)), nil
	case dtU32:
		return uint64(binary.LittleEndian.Uint32(rawData)), nil
	case dtU64:
		return binary.LittleEndian.Uint64(rawData), nil
	}
	return nil, errors.New("internal error; undefined dataType")
}

func (d *traceEventDecoder) decodeRawData(rawData []byte) (TraceEventSampleData, error) {
	data := make(map[string]interface{})
	for _, field := range d.fields {
		var err error

		if field.dataType == dtString {
			dataOffset := binary.LittleEndian.Uint16(rawData[field.Offset:])
			dataLength := binary.LittleEndian.Uint16(rawData[field.Offset+2:])
			data[field.FieldName] = string([]byte(rawData[dataOffset : dataOffset+dataLength-1]))
			continue
		}

		if field.arraySize == -1 {
			data[field.FieldName], err = decodeDataType(field.dataType, rawData[field.Offset:])
			if err != nil {
				return nil, err
			}
			continue
		}

		var arraySize, dataOffset int
		if field.arraySize == 0 {
			dataOffset = int(binary.LittleEndian.Uint16(rawData[field.Offset:]))
			dataLength := int(binary.LittleEndian.Uint16(rawData[field.Offset+2:]))
			arraySize = dataLength / field.dataTypeSize
		} else {
			dataOffset = field.Offset
			arraySize = field.arraySize
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

type TraceEventDecoderList struct {
	sync.Mutex
	decoders   map[uint16]*traceEventDecoder
}

func (l *TraceEventDecoderList) AddDecoder(name string, fn TraceEventDecoderFn) (uint16, error) {
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
		fields: fields,
		decoderfn: fn,
	}

	return id, nil
}

func (l *TraceEventDecoderList) getDecoder(eventType uint16) *traceEventDecoder {
	l.Lock()
	defer l.Unlock()

	if l.decoders == nil {
		return nil
	}
	return l.decoders[eventType]
}

func (l *TraceEventDecoderList) DecodeSample(sample *SampleRecord) (interface{}, error) {
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
