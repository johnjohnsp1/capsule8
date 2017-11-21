package filter

import (
	api "github.com/capsule8/api/v0"
)

func isValueTypeInteger(t api.ValueType) bool {
	switch t {
	case api.ValueType_SINT8, api.ValueType_SINT16,
		api.ValueType_SINT32, api.ValueType_SINT64,
		api.ValueType_UINT8, api.ValueType_UINT16,
		api.ValueType_UINT32, api.ValueType_UINT64:

		return true
	default:
		return false
	}
}

func isValueTypeNumeric(t api.ValueType) bool {
	switch t {
	case api.ValueType_SINT8, api.ValueType_SINT16,
		api.ValueType_SINT32, api.ValueType_SINT64,
		api.ValueType_UINT8, api.ValueType_UINT16,
		api.ValueType_UINT32, api.ValueType_UINT64,
		api.ValueType_DOUBLE, api.ValueType_TIMESTAMP:

		return true
	default:
		return false
	}
}

func isValueTypeString(t api.ValueType) bool {
	if t == api.ValueType_STRING {
		return true
	}
	return false
}
