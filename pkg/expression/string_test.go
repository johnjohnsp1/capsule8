package expression

import (
	"fmt"
	"testing"

	api "github.com/capsule8/capsule8/api/v0"
)

func testValueAsString(t *testing.T, value *api.Value, want string) {
	got := valueAsString(value)
	if got != want {
		t.Errorf("want: %q, got %q", want, got)
	}
}

func TestValueAsString(t *testing.T) {
	testValueAsString(t, NewValue("capsule8"), "\"capsule8\"")
	testValueAsString(t, NewValue(int8(83)), "83")
	testValueAsString(t, NewValue(int16(83)), "83")
	testValueAsString(t, NewValue(int32(83)), "83")
	testValueAsString(t, NewValue(int64(83)), "83")
	testValueAsString(t, NewValue(uint8(83)), "83")
	testValueAsString(t, NewValue(uint16(83)), "83")
	testValueAsString(t, NewValue(uint32(83)), "83")
	testValueAsString(t, NewValue(uint64(83)), "83")
	testValueAsString(t, NewValue(true), "TRUE")
	testValueAsString(t, NewValue(false), "FALSE")
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
		expr = newBinaryExpr(op, Identifier("port"), Value(uint16(1024)))

		want = fmt.Sprintf("port %s 1024", s[0])
		testExpressionAsString(t, expr, want)

		want = fmt.Sprintf("port %s 1024", s[1])
		testExpressionAsKernelFilterString(t, expr, want)
	}

	expr = Like(Identifier("filename"), Value("*passwd*"))
	testExpressionAsString(t, expr, "filename LIKE \"*passwd*\"")
	testExpressionAsKernelFilterString(t, expr, "filename ~ \"*passwd*\"")

	expr = IsNull(Identifier("path"))
	testExpressionAsString(t, expr, "path IS NULL")
	testExpressionAsKernelFilterString(t, expr, "")

	expr = IsNotNull(Identifier("service"))
	testExpressionAsString(t, expr, "service IS NOT NULL")
	testExpressionAsKernelFilterString(t, expr, "")

	expr = NotEqual(
		BitwiseAnd(Identifier("flags"), Value(uint32(1234))),
		Value(uint32(0)))
	testExpressionAsString(t, expr, "flags & 1234 != 0")
	testExpressionAsKernelFilterString(t, expr, "flags & 1234")

	expr = LogicalOr(
		Equal(Identifier("port"), Value(uint16(80))),
		Equal(Identifier("port"), Value(uint16(443))))
	testExpressionAsString(t, expr,
		"port = 80 OR port = 443")
	testExpressionAsKernelFilterString(t, expr,
		"port == 80 || port == 443")

	expr = LogicalAnd(
		NotEqual(Identifier("port"), Value(uint16(80))),
		NotEqual(Identifier("port"), Value(uint16(443))))
	testExpressionAsString(t, expr,
		"port != 80 AND port != 443")
	testExpressionAsKernelFilterString(t, expr,
		"port != 80 && port != 443")

	expr = LogicalAnd(
		Equal(Identifier("port"), Value(uint16(80))),
		LogicalOr(
			Equal(Identifier("address"), Value("192.168.1.4")),
			Equal(Identifier("address"), Value("127.0.0.1"))))
	testExpressionAsString(t, expr,
		"port = 80 AND (address = \"192.168.1.4\" OR address = \"127.0.0.1\")")
	testExpressionAsKernelFilterString(t, expr,
		"port == 80 && (address == \"192.168.1.4\" || address == \"127.0.0.1\")")
}
