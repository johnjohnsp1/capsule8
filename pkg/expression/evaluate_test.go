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

func testEvaluateExpr(t *testing.T, expr *api.Expression, types FieldTypeMap, values FieldValueMap, want interface{}) {
	gotValue, err := evaluateExpression(expr, types, values)
	if err != nil {
		t.Errorf("%s -> unexpected evaluation error: %s",
			expressionAsString(expr), err)
		return
	}

	wantValue := NewValue(want)
	if gotValue.GetType() != wantValue.GetType() {
		t.Errorf("%s -> unexpected result: %v",
			expressionAsString(expr), valueAsString(gotValue))
		return
	}

	eq, err := compareEqual(*gotValue, *wantValue)
	if err != nil {
		t.Errorf("%s -> unable to compare result: %s",
			expressionAsString(expr), err)
		return
	}
	if !eq {
		t.Errorf("%s -> unexpected result: %v",
			expressionAsString(expr), valueAsString(gotValue))
		return
	}
}

func TestExpressionEvaluation(t *testing.T) {
	var expr *api.Expression

	types := FieldTypeMap{
		"port":     api.ValueType_UINT16,
		"address":  api.ValueType_STRING,
		"flags":    api.ValueType_UINT32,
		"filename": api.ValueType_STRING,
		"path":     api.ValueType_STRING,
		"service":  api.ValueType_STRING,
	}

	values := FieldValueMap{
		"port":     uint16(80),
		"address":  "127.0.0.1",
		"flags":    uint32(31337),
		"filename": "/etc/passwd",
		"path":     "/var/run/capsule8",
		// "service" is intentionally omitted
	}

	var binaryOps = map[api.Expression_ExpressionType]bool{
		api.Expression_EQ: false,
		api.Expression_NE: true,
		api.Expression_LT: true,
		api.Expression_LE: true,
		api.Expression_GT: false,
		api.Expression_GE: false,
	}

	for op, result := range binaryOps {
		expr = newBinaryExpr(op, Identifier("port"), Value(uint16(1024)))
		testEvaluateExpr(t, expr, types, values, result)
	}

	expr = Like(Identifier("filename"), Value("*passwd*"))
	testEvaluateExpr(t, expr, types, values, true)

	expr = IsNull(Identifier("path"))
	testEvaluateExpr(t, expr, types, values, false)

	expr = IsNotNull(Identifier("service"))
	testEvaluateExpr(t, expr, types, values, false)

	expr = NotEqual(
		BitwiseAnd(Identifier("flags"), Value(uint32(1234))),
		Value(uint32(0)))
	testEvaluateExpr(t, expr, types, values, true)

	expr = LogicalOr(
		Equal(Identifier("port"), Value(uint16(80))),
		Equal(Identifier("port"), Value(uint16(443))))
	testEvaluateExpr(t, expr, types, values, true)

	expr = LogicalAnd(
		NotEqual(Identifier("port"), Value(uint16(80))),
		NotEqual(Identifier("port"), Value(uint16(443))))
	testEvaluateExpr(t, expr, types, values, false)

	expr = LogicalAnd(
		Equal(Identifier("port"), Value(uint16(80))),
		LogicalOr(
			Equal(Identifier("address"), Value("192.168.1.4")),
			Equal(Identifier("address"), Value("127.0.0.1"))))
	testEvaluateExpr(t, expr, types, values, true)
}
