package expression

import (
	"testing"

	api "github.com/capsule8/api/v0"
)

func testEvaluateExpr(t *testing.T, expr *api.Expression, types FieldTypeMap, values FieldValueMap, want interface{}) {
	gotValue, err := evaluateExpression(expr, types, values)
	if err != nil {
		t.Errorf("%s -> unexpected evaluation error: %s",
			expressionAsString(expr), err)
		return
	}

	wantValue := NewValueExpr(want).GetValue()
	if gotValue.GetType() != wantValue.GetType() {
		t.Errorf("%s -> unexpected result: %v",
			expressionAsString(expr), valueAsString(&gotValue))
		return
	}

	eq, err := compareEqual(gotValue, *wantValue)
	if err != nil {
		t.Errorf("%s -> unable to compare result: %s",
			expressionAsString(expr), err)
		return
	}
	if !eq {
		t.Errorf("%s -> unexpected result: %v",
			expressionAsString(expr), valueAsString(&gotValue))
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
		expr = NewBinaryExpr(op,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(1024)))
		testEvaluateExpr(t, expr, types, values, result)
	}

	expr = NewBinaryExpr(api.Expression_LIKE,
		NewIdentifierExpr("filename"),
		NewValueExpr("*passwd*"))
	testEvaluateExpr(t, expr, types, values, true)

	expr = NewUnaryExpr(api.Expression_IS_NULL,
		NewIdentifierExpr("path"))
	testEvaluateExpr(t, expr, types, values, false)

	expr = NewUnaryExpr(api.Expression_IS_NOT_NULL,
		NewIdentifierExpr("service"))
	testEvaluateExpr(t, expr, types, values, false)

	expr = NewBinaryExpr(api.Expression_NE,
		NewBinaryExpr(api.Expression_BITWISE_AND,
			NewIdentifierExpr("flags"),
			NewValueExpr(uint32(1234))),
		NewValueExpr(uint32(0)))
	testEvaluateExpr(t, expr, types, values, true)

	expr = NewBinaryExpr(api.Expression_LOGICAL_OR,
		NewBinaryExpr(api.Expression_EQ,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(80))),
		NewBinaryExpr(api.Expression_EQ,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(443))))
	testEvaluateExpr(t, expr, types, values, true)

	expr = NewBinaryExpr(api.Expression_LOGICAL_AND,
		NewBinaryExpr(api.Expression_NE,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(80))),
		NewBinaryExpr(api.Expression_NE,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(443))))
	testEvaluateExpr(t, expr, types, values, false)

	expr = NewBinaryExpr(api.Expression_LOGICAL_AND,
		NewBinaryExpr(api.Expression_EQ,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(80))),
		NewBinaryExpr(api.Expression_LOGICAL_OR,
			NewBinaryExpr(api.Expression_EQ,
				NewIdentifierExpr("address"),
				NewValueExpr("192.168.1.4")),
			NewBinaryExpr(api.Expression_EQ,
				NewIdentifierExpr("address"),
				NewValueExpr("127.0.0.1"))))
	testEvaluateExpr(t, expr, types, values, true)
}
