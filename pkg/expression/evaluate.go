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
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	api "github.com/capsule8/capsule8/api/v0"
	google_protobuf2 "github.com/golang/protobuf/ptypes/timestamp"
)

// nullValueType is only used internally
const nullValueType api.ValueType = api.ValueType_VALUETYPE_UNSPECIFIED

func timestampValue(stamp *google_protobuf2.Timestamp) uint64 {
	return (uint64(stamp.Seconds) * uint64(time.Second)) +
		uint64(stamp.Nanos)
}

func compareEqual(lhs, rhs api.Value) (bool, error) {
	switch t := lhs.GetType(); t {
	case api.ValueType_STRING:
		return lhs.GetStringValue() == rhs.GetStringValue(), nil
	case api.ValueType_SINT8, api.ValueType_SINT16, api.ValueType_SINT32,
		api.ValueType_SINT64:

		return lhs.GetSignedValue() == rhs.GetSignedValue(), nil
	case api.ValueType_UINT8, api.ValueType_UINT16, api.ValueType_UINT32,
		api.ValueType_UINT64:

		return lhs.GetUnsignedValue() == rhs.GetUnsignedValue(), nil
	case api.ValueType_BOOL:
		return lhs.GetBoolValue() == rhs.GetBoolValue(), nil
	case api.ValueType_DOUBLE:
		return lhs.GetDoubleValue() == rhs.GetDoubleValue(), nil
	case api.ValueType_TIMESTAMP:
		return timestampValue(lhs.GetTimestampValue()) ==
			timestampValue(rhs.GetTimestampValue()), nil
	default:
		return false, fmt.Errorf("Unknown value type %d", t)
	}
}

func compareNotEqual(lhs, rhs api.Value) (bool, error) {
	switch t := lhs.GetType(); t {
	case api.ValueType_STRING:
		return lhs.GetStringValue() != rhs.GetStringValue(), nil
	case api.ValueType_SINT8, api.ValueType_SINT16, api.ValueType_SINT32,
		api.ValueType_SINT64:

		return lhs.GetSignedValue() != rhs.GetSignedValue(), nil
	case api.ValueType_UINT8, api.ValueType_UINT16, api.ValueType_UINT32,
		api.ValueType_UINT64:

		return lhs.GetUnsignedValue() != rhs.GetUnsignedValue(), nil
	case api.ValueType_BOOL:
		return lhs.GetBoolValue() != rhs.GetBoolValue(), nil
	case api.ValueType_DOUBLE:
		return lhs.GetDoubleValue() != rhs.GetDoubleValue(), nil
	case api.ValueType_TIMESTAMP:
		return timestampValue(lhs.GetTimestampValue()) !=
			timestampValue(rhs.GetTimestampValue()), nil
	default:
		return false, fmt.Errorf("Unknown value type %d", t)
	}
}

func compareLessThan(lhs, rhs api.Value) (bool, error) {
	switch t := lhs.GetType(); t {
	case api.ValueType_STRING, api.ValueType_BOOL:
		return false,
			fmt.Errorf("Cannot compare %s types", api.ValueType_name[int32(t)])
	case api.ValueType_SINT8, api.ValueType_SINT16, api.ValueType_SINT32,
		api.ValueType_SINT64:

		return lhs.GetSignedValue() < rhs.GetSignedValue(), nil
	case api.ValueType_UINT8, api.ValueType_UINT16, api.ValueType_UINT32,
		api.ValueType_UINT64:

		return lhs.GetUnsignedValue() < rhs.GetUnsignedValue(), nil
	case api.ValueType_DOUBLE:
		return lhs.GetDoubleValue() < rhs.GetDoubleValue(), nil
	case api.ValueType_TIMESTAMP:
		return timestampValue(lhs.GetTimestampValue()) <
			timestampValue(rhs.GetTimestampValue()), nil
	default:
		return false, fmt.Errorf("Unknown value type %d", t)
	}
}

func compareLessThanEqualTo(lhs, rhs api.Value) (bool, error) {
	switch t := lhs.GetType(); t {
	case api.ValueType_STRING, api.ValueType_BOOL:
		return false,
			fmt.Errorf("Cannot compare %s types", api.ValueType_name[int32(t)])
	case api.ValueType_SINT8, api.ValueType_SINT16, api.ValueType_SINT32,
		api.ValueType_SINT64:

		return lhs.GetSignedValue() <= rhs.GetSignedValue(), nil
	case api.ValueType_UINT8, api.ValueType_UINT16, api.ValueType_UINT32,
		api.ValueType_UINT64:

		return lhs.GetUnsignedValue() <= rhs.GetUnsignedValue(), nil
	case api.ValueType_DOUBLE:
		return lhs.GetDoubleValue() <= rhs.GetDoubleValue(), nil
	case api.ValueType_TIMESTAMP:
		return timestampValue(lhs.GetTimestampValue()) <=
			timestampValue(rhs.GetTimestampValue()), nil
	default:
		return false, fmt.Errorf("Unknown value type %d", t)
	}
}

func compareGreaterThan(lhs, rhs api.Value) (bool, error) {
	switch t := lhs.GetType(); t {
	case api.ValueType_STRING, api.ValueType_BOOL:
		return false,
			fmt.Errorf("Cannot compare %s types", api.ValueType_name[int32(t)])
	case api.ValueType_SINT8, api.ValueType_SINT16, api.ValueType_SINT32,
		api.ValueType_SINT64:

		return lhs.GetSignedValue() > rhs.GetSignedValue(), nil
	case api.ValueType_UINT8, api.ValueType_UINT16, api.ValueType_UINT32,
		api.ValueType_UINT64:

		return lhs.GetUnsignedValue() > rhs.GetUnsignedValue(), nil
	case api.ValueType_DOUBLE:
		return lhs.GetDoubleValue() > rhs.GetDoubleValue(), nil
	case api.ValueType_TIMESTAMP:
		return timestampValue(lhs.GetTimestampValue()) >
			timestampValue(rhs.GetTimestampValue()), nil
	default:
		return false, fmt.Errorf("Unknown value type %d", t)
	}
}

func compareGreaterThanEqualTo(lhs, rhs api.Value) (bool, error) {
	switch t := lhs.GetType(); t {
	case api.ValueType_STRING, api.ValueType_BOOL:
		return false,
			fmt.Errorf("Cannot compare %s types", api.ValueType_name[int32(t)])
	case api.ValueType_SINT8, api.ValueType_SINT16, api.ValueType_SINT32,
		api.ValueType_SINT64:

		return lhs.GetSignedValue() >= rhs.GetSignedValue(), nil
	case api.ValueType_UINT8, api.ValueType_UINT16, api.ValueType_UINT32,
		api.ValueType_UINT64:

		return lhs.GetUnsignedValue() >= rhs.GetUnsignedValue(), nil
	case api.ValueType_DOUBLE:
		return lhs.GetDoubleValue() >= rhs.GetDoubleValue(), nil
	case api.ValueType_TIMESTAMP:
		return timestampValue(lhs.GetTimestampValue()) >=
			timestampValue(rhs.GetTimestampValue()), nil
	default:
		return false, fmt.Errorf("Unknown value type %d", t)
	}
}

func compareLike(lhs, rhs api.Value) (bool, error) {
	switch t := lhs.GetType(); t {
	case api.ValueType_SINT8, api.ValueType_SINT16,
		api.ValueType_SINT32, api.ValueType_SINT64, api.ValueType_UINT8,
		api.ValueType_UINT16, api.ValueType_UINT32,
		api.ValueType_UINT64, api.ValueType_BOOL, api.ValueType_DOUBLE,
		api.ValueType_TIMESTAMP:

		return false,
			fmt.Errorf("Cannot compare %s types", api.ValueType_name[int32(t)])
	case api.ValueType_STRING:
		var result bool

		s := lhs.GetStringValue()
		pattern := rhs.GetStringValue()
		if strings.HasPrefix(pattern, "*") {
			if strings.HasSuffix(pattern, "*") {
				result = strings.Contains(s, pattern[1:len(pattern)-1])
			} else {
				result = strings.HasSuffix(s, pattern[1:])
			}
		} else if strings.HasSuffix(pattern, "*") {
			result = strings.HasPrefix(s, pattern[:len(pattern)-1])
		} else {
			result = s == pattern
		}
		return result, nil

	default:
		return false, fmt.Errorf("Unknown value type %d", t)
	}
}

type evalContext struct {
	types  FieldTypeMap
	values FieldValueMap
	stack  []api.Value
}

var typeStrings = map[api.ValueType]string{
	api.ValueType_STRING:    "string",
	api.ValueType_SINT8:     "int8",
	api.ValueType_SINT16:    "int16",
	api.ValueType_SINT32:    "int32",
	api.ValueType_SINT64:    "int64",
	api.ValueType_UINT8:     "uint8",
	api.ValueType_UINT16:    "uint16",
	api.ValueType_UINT32:    "uint32",
	api.ValueType_UINT64:    "uint64",
	api.ValueType_BOOL:      "bool",
	api.ValueType_DOUBLE:    "float64",
	api.ValueType_TIMESTAMP: "uint64",
}

func (c *evalContext) pushIdentifier(ident string) error {
	t, ok := c.types[ident]
	if !ok {
		return fmt.Errorf("Undefined identifier %q", ident)
	}
	v, ok := c.values[ident]
	if !ok {
		t = nullValueType
	} else if reflect.TypeOf(v).String() != typeStrings[t] {
		return fmt.Errorf("Data type mismatch for %q (expected %s; got %s)",
			ident, typeStrings[t], reflect.TypeOf(v))
	}

	value := api.Value{
		Type: t,
	}

	switch t {
	case api.ValueType_STRING:
		value.Value = &api.Value_StringValue{StringValue: v.(string)}
	case api.ValueType_SINT8:
		value.Value = &api.Value_SignedValue{SignedValue: int64(v.(int8))}
	case api.ValueType_SINT16:
		value.Value = &api.Value_SignedValue{SignedValue: int64(v.(int16))}
	case api.ValueType_SINT32:
		value.Value = &api.Value_SignedValue{SignedValue: int64(v.(int32))}
	case api.ValueType_SINT64:
		value.Value = &api.Value_SignedValue{SignedValue: v.(int64)}
	case api.ValueType_UINT8:
		value.Value = &api.Value_UnsignedValue{UnsignedValue: uint64(v.(uint8))}
	case api.ValueType_UINT16:
		value.Value = &api.Value_UnsignedValue{UnsignedValue: uint64(v.(uint16))}
	case api.ValueType_UINT32:
		value.Value = &api.Value_UnsignedValue{UnsignedValue: uint64(v.(uint32))}
	case api.ValueType_UINT64:
		value.Value = &api.Value_UnsignedValue{UnsignedValue: v.(uint64)}
	case api.ValueType_BOOL:
		value.Value = &api.Value_BoolValue{BoolValue: v.(bool)}
	case api.ValueType_DOUBLE:
		value.Value = &api.Value_DoubleValue{DoubleValue: v.(float64)}
	case api.ValueType_TIMESTAMP:
		ns := v.(uint64)
		value.Value = &api.Value_TimestampValue{
			TimestampValue: &google_protobuf2.Timestamp{
				Seconds: int64(ns / uint64(time.Second)),
				Nanos:   int32(ns % uint64(time.Second)),
			},
		}
	}

	c.stack = append(c.stack, value)
	return nil
}

func (c *evalContext) evaluateNode(node *api.Expression) error {
	switch op := node.GetType(); op {
	case api.Expression_IDENTIFIER:
		return c.pushIdentifier(node.GetIdentifier())

	case api.Expression_VALUE:
		c.stack = append(c.stack, *node.GetValue())
		return nil

	case api.Expression_LOGICAL_AND:
		operands := node.GetBinaryOp()
		err := c.evaluateNode(operands.Lhs)
		if err != nil {
			return err
		}
		v := &c.stack[len(c.stack)-1]
		if !IsValueTrue(v) {
			// Leave the false value on the stack and return
			// There's no need to evaluate the rhs; it won't change
			// the result.
			return nil
		}
		// Pop the lhs result, evaluate the rhs, and leave its result
		// behind as the result of this op
		c.stack = c.stack[0 : len(c.stack)-1]
		return c.evaluateNode(operands.Rhs)

	case api.Expression_LOGICAL_OR:
		operands := node.GetBinaryOp()
		err := c.evaluateNode(operands.Lhs)
		if err != nil {
			return err
		}
		v := &c.stack[len(c.stack)-1]
		if IsValueTrue(v) {
			// Leave the true value on the stack and return
			// There's no need to evaluate the lhs; it won't change
			// the result.
			return nil
		}
		// Pop the lhs result, evaluate the rhs, and leave its result
		// behind as the result of this op
		c.stack = c.stack[0 : len(c.stack)-1]
		return c.evaluateNode(operands.Rhs)

	case api.Expression_EQ, api.Expression_NE, api.Expression_LT,
		api.Expression_LE, api.Expression_GT, api.Expression_GE,
		api.Expression_LIKE:

		operands := node.GetBinaryOp()
		err := c.evaluateNode(operands.Lhs)
		if err != nil {
			return err
		}
		err = c.evaluateNode(operands.Rhs)
		if err != nil {
			return err
		}
		lhs := c.stack[len(c.stack)-2]
		rhs := c.stack[len(c.stack)-1]
		c.stack = c.stack[0 : len(c.stack)-2]

		var result bool

		// If either side of the comparison is NULL, the result is FALSE
		if lhs.GetType() == nullValueType || rhs.GetType() == nullValueType {
			result = false
		} else {
			if lhs.GetType() != rhs.GetType() {
				return fmt.Errorf("Type mismatch in comparison: %s vs. %s",
					api.ValueType_name[int32(lhs.GetType())],
					api.ValueType_name[int32(rhs.GetType())])
			}

			switch op {
			case api.Expression_EQ:
				result, err = compareEqual(lhs, rhs)
			case api.Expression_NE:
				result, err = compareNotEqual(lhs, rhs)
			case api.Expression_LT:
				result, err = compareLessThan(lhs, rhs)
			case api.Expression_LE:
				result, err = compareLessThanEqualTo(lhs, rhs)
			case api.Expression_GT:
				result, err = compareGreaterThan(lhs, rhs)
			case api.Expression_GE:
				result, err = compareGreaterThanEqualTo(lhs, rhs)
			case api.Expression_LIKE:
				result, err = compareLike(lhs, rhs)
			}
			if err != nil {
				return err
			}
		}

		v := api.Value{
			Type:  api.ValueType_BOOL,
			Value: &api.Value_BoolValue{BoolValue: result},
		}
		c.stack = append(c.stack, v)
		return nil

	case api.Expression_IS_NULL:
		err := c.evaluateNode(node.GetUnaryOp())
		if err != nil {
			return err
		}
		v := &c.stack[len(c.stack)-1]
		result := v.GetType() == nullValueType
		v.Type = api.ValueType_BOOL
		v.Value = &api.Value_BoolValue{BoolValue: result}
		return nil

	case api.Expression_IS_NOT_NULL:
		err := c.evaluateNode(node.GetUnaryOp())
		if err != nil {
			return err
		}
		v := &c.stack[len(c.stack)-1]
		result := v.GetType() != nullValueType
		v.Type = api.ValueType_BOOL
		v.Value = &api.Value_BoolValue{BoolValue: result}
		return nil

	case api.Expression_BITWISE_AND:
		operands := node.GetBinaryOp()
		err := c.evaluateNode(operands.Lhs)
		if err != nil {
			return err
		}
		err = c.evaluateNode(operands.Rhs)
		if err != nil {
			return err
		}
		lhs := c.stack[len(c.stack)-2]
		rhs := c.stack[len(c.stack)-1]
		c.stack = c.stack[0 : len(c.stack)-2]

		t := lhs.GetType()
		if t != rhs.GetType() {
			return fmt.Errorf("Type mismatch for &: %s vs. %s",
				api.ValueType_name[int32(lhs.GetType())],
				api.ValueType_name[int32(rhs.GetType())])
		}

		v := api.Value{Type: t}
		switch t {
		case api.ValueType_STRING, api.ValueType_BOOL,
			api.ValueType_DOUBLE, api.ValueType_TIMESTAMP:

			return fmt.Errorf("Type for & must be an integer; got %s",
				api.ValueType_name[int32(t)])
		case api.ValueType_SINT8, api.ValueType_SINT16,
			api.ValueType_SINT32, api.ValueType_SINT64:

			l := lhs.GetSignedValue()
			r := rhs.GetSignedValue()
			v.Value = &api.Value_SignedValue{SignedValue: (l & r)}
		case api.ValueType_UINT8, api.ValueType_UINT16,
			api.ValueType_UINT32, api.ValueType_UINT64:

			l := lhs.GetUnsignedValue()
			r := rhs.GetUnsignedValue()
			v.Value = &api.Value_UnsignedValue{UnsignedValue: (l & r)}
		}

		c.stack = append(c.stack, v)
		return nil
	}

	return errors.New("internal error: unreachable condition")
}

func evaluateExpression(expr *api.Expression, types FieldTypeMap, values FieldValueMap) (*api.Value, error) {
	c := evalContext{
		types:  types,
		values: values,
		stack:  make([]api.Value, 0, 8),
	}

	err := c.evaluateNode(expr)
	if err == nil {
		if len(c.stack) == 0 {
			err = errors.New("internal error: empty stack after expression evaluation")
		} else if len(c.stack) > 1 {
			err = errors.New("internal error: more than one value on the stack after expression evaluation")
		}
	}

	if err != nil {
		return nil, err
	}
	return &c.stack[0], nil
}
