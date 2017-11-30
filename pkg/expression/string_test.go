package expression

import (
	"fmt"
	"testing"

	api "github.com/capsule8/api/v0"
)

func testValueAsString(t *testing.T, value *api.Value, want string) {
	got := valueAsString(value)
	if got != want {
		t.Errorf("want: %q, got %q", want, got)
	}
}

func TestValueAsString(t *testing.T) {
	testValueAsString(t, NewValueExpr("capsule8").GetValue(), "\"capsule8\"")
	testValueAsString(t, NewValueExpr(int8(83)).GetValue(), "83")
	testValueAsString(t, NewValueExpr(int16(83)).GetValue(), "83")
	testValueAsString(t, NewValueExpr(int32(83)).GetValue(), "83")
	testValueAsString(t, NewValueExpr(int64(83)).GetValue(), "83")
	testValueAsString(t, NewValueExpr(uint8(83)).GetValue(), "83")
	testValueAsString(t, NewValueExpr(uint16(83)).GetValue(), "83")
	testValueAsString(t, NewValueExpr(uint32(83)).GetValue(), "83")
	testValueAsString(t, NewValueExpr(uint64(83)).GetValue(), "83")
	testValueAsString(t, NewValueExpr(true).GetValue(), "TRUE")
	testValueAsString(t, NewValueExpr(false).GetValue(), "FALSE")
}

func testExpressionAsString(t *testing.T, expr *api.Expression, want string) {
	got := expressionAsString(expr)
	if got != want {
		t.Errorf("want: %q, got %q", want, got)
	}
}

func testExpressionAsKernelFilterString(t *testing.T, expr *api.Expression, want string) {
	got := expressionAsKernelFilterString(expr)
	if got != want {
		t.Errorf("want: %q, got %q", want, got)
	}
}

func TestExpressionStrings(t *testing.T) {
	var (
		expr *api.Expression
		want string
	)

	var binaryOps = map[api.Expression_ExpressionType][]string{
		api.Expression_EQ: []string{"=", "=="},
		api.Expression_NE: []string{"!=", "!="},
		api.Expression_LT: []string{"<", "<"},
		api.Expression_LE: []string{"<=", "<="},
		api.Expression_GT: []string{">", ">"},
		api.Expression_GE: []string{">=", ">="},
	}

	for op, s := range binaryOps {
		expr = NewBinaryExpr(op,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(1024)))

		want = fmt.Sprintf("port %s 1024", s[0])
		testExpressionAsString(t, expr, want)

		want = fmt.Sprintf("port %s 1024", s[1])
		testExpressionAsKernelFilterString(t, expr, want)
	}

	expr = NewBinaryExpr(api.Expression_LIKE,
		NewIdentifierExpr("filename"),
		NewValueExpr("*passwd*"))
	testExpressionAsString(t, expr, "filename LIKE \"*passwd*\"")
	testExpressionAsKernelFilterString(t, expr, "filename ~ \"*passwd*\"")

	expr = NewUnaryExpr(api.Expression_IS_NULL,
		NewIdentifierExpr("path"))
	testExpressionAsString(t, expr, "path IS NULL")
	testExpressionAsKernelFilterString(t, expr, "")

	expr = NewUnaryExpr(api.Expression_IS_NOT_NULL,
		NewIdentifierExpr("service"))
	testExpressionAsString(t, expr, "service IS NOT NULL")
	testExpressionAsKernelFilterString(t, expr, "")

	expr = NewBinaryExpr(api.Expression_NE,
		NewBinaryExpr(api.Expression_BITWISE_AND,
			NewIdentifierExpr("flags"),
			NewValueExpr(uint32(1234))),
		NewValueExpr(uint32(0)))
	testExpressionAsString(t, expr, "flags & 1234 != 0")
	testExpressionAsKernelFilterString(t, expr, "flags & 1234")

	expr = NewBinaryExpr(api.Expression_LOGICAL_OR,
		NewBinaryExpr(api.Expression_EQ,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(80))),
		NewBinaryExpr(api.Expression_EQ,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(443))))
	testExpressionAsString(t, expr,
		"port = 80 OR port = 443")
	testExpressionAsKernelFilterString(t, expr,
		"port == 80 || port == 443")

	expr = NewBinaryExpr(api.Expression_LOGICAL_AND,
		NewBinaryExpr(api.Expression_NE,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(80))),
		NewBinaryExpr(api.Expression_NE,
			NewIdentifierExpr("port"),
			NewValueExpr(uint16(443))))
	testExpressionAsString(t, expr,
		"port != 80 AND port != 443")
	testExpressionAsKernelFilterString(t, expr,
		"port != 80 && port != 443")

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
	testExpressionAsString(t, expr,
		"port = 80 AND (address = \"192.168.1.4\" OR address = \"127.0.0.1\")")
	testExpressionAsKernelFilterString(t, expr,
		"port == 80 && (address == \"192.168.1.4\" || address == \"127.0.0.1\")")
}
