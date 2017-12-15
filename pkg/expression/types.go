// Copyright 2017 Capsule8, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package expression

import (
	api "github.com/capsule8/capsule8/api/v0"
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
