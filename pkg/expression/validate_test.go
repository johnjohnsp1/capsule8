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
	"testing"

	api "github.com/capsule8/capsule8/api/v0"
)

func testInvalidIdentifier(t *testing.T, s string) {
	err := validateIdentifier(s)
	if err == nil { // YES ==
		t.Errorf("Invalid identifier %q accepted", s)
	}
}

func testValidIdentifier(t *testing.T, s string) {
	err := validateIdentifier(s)
	if err != nil {
		t.Errorf("Valid identifier %q; got %s", s, err)
	}
}

func TestValidateIdentifier(t *testing.T) {
	testInvalidIdentifier(t, "")
	testInvalidIdentifier(t, "CON$")
	testInvalidIdentifier(t, "1234")
	testInvalidIdentifier(t, "83foo")

	testValidIdentifier(t, "x")
	testValidIdentifier(t, "abc83")
	testValidIdentifier(t, "_")
	testValidIdentifier(t, "_83_")
}

func testValidateInvalidValue(t *testing.T, value *api.Value) {
	err := validateValue(value)
	if err == nil { // YES ==
		t.Errorf("Expected error for invalid value")
	}
}

func testValidateValidValue(t *testing.T, value *api.Value) {
	err := validateValue(value)
	if err != nil {
		t.Error(err)
	}
}

func TestValidateValue(t *testing.T) {
	testValidateInvalidValue(t, &api.Value{Type: api.ValueType_VALUETYPE_UNSPECIFIED})
	testValidateInvalidValue(t, &api.Value{Type: api.ValueType_STRING})
	testValidateInvalidValue(t, &api.Value{Type: api.ValueType_SINT8})
	testValidateInvalidValue(t, &api.Value{Type: api.ValueType_SINT16})
	testValidateInvalidValue(t, &api.Value{Type: api.ValueType_SINT32})
	testValidateInvalidValue(t, &api.Value{Type: api.ValueType_SINT64})
	testValidateInvalidValue(t, &api.Value{Type: api.ValueType_UINT8})
	testValidateInvalidValue(t, &api.Value{Type: api.ValueType_UINT16})
	testValidateInvalidValue(t, &api.Value{Type: api.ValueType_UINT32})
	testValidateInvalidValue(t, &api.Value{Type: api.ValueType_UINT64})
	testValidateInvalidValue(t, &api.Value{Type: api.ValueType_BOOL})
	testValidateInvalidValue(t, &api.Value{Type: api.ValueType_DOUBLE})
	testValidateInvalidValue(t, &api.Value{Type: api.ValueType_TIMESTAMP})

	testValidateValidValue(t, NewValue("capsule8"))
	testValidateValidValue(t, NewValue(int8(83)))
	testValidateValidValue(t, NewValue(int16(83)))
	testValidateValidValue(t, NewValue(int32(83)))
	testValidateValidValue(t, NewValue(int64(83)))
	testValidateValidValue(t, NewValue(uint8(83)))
	testValidateValidValue(t, NewValue(uint16(83)))
	testValidateValidValue(t, NewValue(uint32(83)))
	testValidateValidValue(t, NewValue(uint64(83)))
	testValidateValidValue(t, NewValue(true))
	testValidateValidValue(t, NewValue(false))
	testValidateValidValue(t, NewValue(8.3))
}

func testValidateExpr(t *testing.T, expr *api.Expression, normalPass, kernelPass, typesPass bool, types FieldTypeMap) {
	err := validateTree(expr)
	if normalPass {
		if err != nil {
			t.Errorf("%s -> Expected normal pass; got %s",
				expressionAsString(expr), err)
		}
	} else {
		if err == nil {
			t.Errorf("%s -> Expected normal validation failure; got pass",
				expressionAsString(expr))
		}
	}

	err = validateKernelFilterTree(expr)
	if kernelPass {
		if err != nil {
			t.Errorf("%s -> Expected kernel pass; got %s",
				expressionAsString(expr), err)
		}
	} else {
		if err == nil {
			t.Errorf("%s -> Expected kernel validation failure; got pass",
				expressionAsString(expr))
		}
	}

	_, err = validateTypes(expr, types)
	if typesPass {
		if err != nil {
			t.Errorf("%s -> Expected types pass; got %s",
				expressionAsString(expr), err)
		}
	} else {
		if err == nil {
			t.Errorf("%s -> Expected types validation failure; got pass",
				expressionAsString(expr))
		}
	}
}

func TestExpressionValidation(t *testing.T) {
	var expr *api.Expression

	types := FieldTypeMap{
		"port":     api.ValueType_UINT16,
		"filename": api.ValueType_STRING,
		"path":     api.ValueType_STRING,
		"service":  api.ValueType_STRING,
		"address":  api.ValueType_STRING,
		"flags":    api.ValueType_UINT32,
	}

	var binaryOps = []api.Expression_ExpressionType{
		api.Expression_EQ,
		api.Expression_NE,
		api.Expression_LT,
		api.Expression_LE,
		api.Expression_GT,
		api.Expression_GE,
	}

	for _, op := range binaryOps {
		expr = newBinaryExpr(op, Identifier("port"), Value(uint16(1024)))
		testValidateExpr(t, expr, true, true, true, types)
	}

	expr = Like(Identifier("filename"), Value("*passwd*"))
	testValidateExpr(t, expr, true, true, true, types)

	expr = IsNull(Identifier("path"))
	testValidateExpr(t, expr, true, false, true, types)

	expr = IsNotNull(Identifier("service"))
	testValidateExpr(t, expr, true, false, true, types)

	expr = NotEqual(
		BitwiseAnd(Identifier("flags"), Value(uint32(1234))),
		Value(uint32(0)))
	testValidateExpr(t, expr, true, true, true, types)

	expr = LogicalOr(
		Equal(Identifier("port"), Value(uint16(80))),
		Equal(Identifier("port"), Value(uint16(443))))
	testValidateExpr(t, expr, true, true, true, types)

	expr = LogicalAnd(
		NotEqual(Identifier("port"), Value(uint16(80))),
		NotEqual(Identifier("port"), Value(uint16(443))))
	testValidateExpr(t, expr, true, true, true, types)

	expr = LogicalAnd(
		Equal(Identifier("port"), Value(uint16(80))),
		LogicalOr(
			Equal(Identifier("address"), Value("192.168.1.4")),
			Equal(Identifier("address"), Value("127.0.0.1"))))
	testValidateExpr(t, expr, true, true, true, types)
}
