package expression

import (
	"testing"

	api "github.com/capsule8/api/v0"
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

	testValidateValidValue(t, NewValueExpr("swarley").GetValue())
	testValidateValidValue(t, NewValueExpr(int8(83)).GetValue())
	testValidateValidValue(t, NewValueExpr(int16(83)).GetValue())
	testValidateValidValue(t, NewValueExpr(int32(83)).GetValue())
	testValidateValidValue(t, NewValueExpr(int64(83)).GetValue())
	testValidateValidValue(t, NewValueExpr(uint8(83)).GetValue())
	testValidateValidValue(t, NewValueExpr(uint16(83)).GetValue())
	testValidateValidValue(t, NewValueExpr(uint32(83)).GetValue())
	testValidateValidValue(t, NewValueExpr(uint64(83)).GetValue())
	testValidateValidValue(t, NewValueExpr(true).GetValue())
	testValidateValidValue(t, NewValueExpr(false).GetValue())
	testValidateValidValue(t, NewValueExpr(8.3).GetValue())
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
		expr = NewBinaryExpr(op,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(1024)))
		testValidateExpr(t, expr, true, true, true, types)
	}

	expr = NewBinaryExpr(api.Expression_LIKE,
		NewIdentifierExpr("filename"),
		NewValueExpr("*passwd*"))
	testValidateExpr(t, expr, true, true, true, types)

	expr = NewUnaryExpr(api.Expression_IS_NULL,
		NewIdentifierExpr("path"))
	testValidateExpr(t, expr, true, false, true, types)

	expr = NewUnaryExpr(api.Expression_IS_NOT_NULL,
		NewIdentifierExpr("service"))
	testValidateExpr(t, expr, true, false, true, types)

	expr = NewBinaryExpr(api.Expression_NE,
		NewBinaryExpr(api.Expression_BITWISE_AND,
			NewIdentifierExpr("flags"),
			NewValueExpr(uint32(1234))),
		NewValueExpr(uint32(0)))
	testValidateExpr(t, expr, true, true, true, types)

	expr = NewBinaryExpr(api.Expression_LOGICAL_OR,
		NewBinaryExpr(api.Expression_EQ,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(80))),
		NewBinaryExpr(api.Expression_EQ,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(443))))
	testValidateExpr(t, expr, true, true, true, types)

	expr = NewBinaryExpr(api.Expression_LOGICAL_AND,
		NewBinaryExpr(api.Expression_NE,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(80))),
		NewBinaryExpr(api.Expression_NE,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(443))))
	testValidateExpr(t, expr, true, true, true, types)

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
	testValidateExpr(t, expr, true, true, true, types)
}
