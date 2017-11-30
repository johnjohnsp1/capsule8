package expression

import (
	api "github.com/capsule8/api/v0"
)

// FieldTypeMap is a mapping of types for field names/identifiers
type FieldTypeMap map[string]api.ValueType

// FieldValueMap is a mapping of values for field names/identifiers.
type FieldValueMap map[string]interface{}

// Expression is a wrapper around expressions around the API. It may contain
// internal information that is used to better support the raw representation.
type Expression struct {
	tree *api.Expression
}

// NewExpression instantiates a new Expression instance. The expression tree
// that is passed is validated to ensure that it is well-formed.
func NewExpression(tree *api.Expression) (*Expression, error) {
	err := validateTree(tree)
	if err != nil {
		return nil, err
	}

	return &Expression{
		tree: tree,
	}, nil
}

// KernelFilterString returns a string representation of an expression that is
// suitable for setting a kernel perf_event filter. This is mostly the same as
// a normal string representation of the expression; however, a few adjustments
// are needed for the kernel.
func (expr *Expression) KernelFilterString() string {
	return expressionAsKernelFilterString(expr.tree)
}

// Return the string representation of an expression.
func (expr *Expression) String() string {
	return expressionAsString(expr.tree)
}

// Evaluate evaluates an expression using the specified type and value
// information, and returns the result of that evaluation or an error. Any
// identifier not present in the types map is considered to be an undefined
// field and any reference to it is an error. Any identifier present in the
// types map, but not present in the values map is considered to be NULL; all
// comparisons against NULL will always evaluate FALSE.
func (expr *Expression) Evaluate(types FieldTypeMap, values FieldValueMap) (*api.Value, error) {
	return evaluateExpression(expr.tree, types, values)
}

// Validate ensures that an expression is properly constructed with the
// specified type information. Any identifier not present in the types map is
// considered to be an undefined field and any reference to it is an error.
func (expr *Expression) Validate(types FieldTypeMap) error {
	_, err := validateTypes(expr.tree, types)
	return err
}

// ValidateKernelFilter determins whether an expression can be represented as
// a kernel filter string. If the result is nil, the kernel will most likely
// accept the expression as a filter. No check is done on the number of
// predicates in the expression, and some kernel versions do not support
// bitwise-and; however, this validator will accept bitwise-and because most
// do. Kernel limits on the number of predicates can vary, so it's not checked.
// If an expression passes this validation, it is not guaranteed that a given
// running kernel will absolutely accept it.
func (expr *Expression) ValidateKernelFilter() error {
	return validateKernelFilterTree(expr.tree)
}

// IsValueTrue determines whether a value's truth value is true or false.
// Strings are true if they contain one or more characters. Any numeric type
// is true if it is non-zero.
func IsValueTrue(value *api.Value) bool {
	switch value.GetType() {
	case api.ValueType_STRING:
		return len(value.GetStringValue()) > 0
	case api.ValueType_SINT8, api.ValueType_SINT16, api.ValueType_SINT32,
		api.ValueType_SINT64:

		return value.GetSignedValue() != 0

	case api.ValueType_UINT8, api.ValueType_UINT16, api.ValueType_UINT32,
		api.ValueType_UINT64:

		return value.GetUnsignedValue() != 0

	case api.ValueType_BOOL:
		return value.GetBoolValue()
	case api.ValueType_DOUBLE:
		return value.GetDoubleValue() != 0.0
	case api.ValueType_TIMESTAMP:
		return timestampValue(value.GetTimestampValue()) != 0
	}
	return false
}

// NewValue creates a new Value instance from a native Go type. If a Go type
// is used that does not have a Value equivalent, the return will be nil.
func NewValue(i interface{}) *api.Value {
	switch v := i.(type) {
	case string:
		return &api.Value{
			Type:  api.ValueType_STRING,
			Value: &api.Value_StringValue{StringValue: v},
		}
	case int8:
		return &api.Value{
			Type:  api.ValueType_SINT8,
			Value: &api.Value_SignedValue{SignedValue: int64(v)},
		}
	case int16:
		return &api.Value{
			Type:  api.ValueType_SINT16,
			Value: &api.Value_SignedValue{SignedValue: int64(v)},
		}
	case int32:
		return &api.Value{
			Type:  api.ValueType_SINT32,
			Value: &api.Value_SignedValue{SignedValue: int64(v)},
		}
	case int64:
		return &api.Value{
			Type:  api.ValueType_SINT64,
			Value: &api.Value_SignedValue{SignedValue: v},
		}
	case uint8:
		return &api.Value{
			Type:  api.ValueType_UINT8,
			Value: &api.Value_UnsignedValue{UnsignedValue: uint64(v)},
		}
	case uint16:
		return &api.Value{
			Type:  api.ValueType_UINT16,
			Value: &api.Value_UnsignedValue{UnsignedValue: uint64(v)},
		}
	case uint32:
		return &api.Value{
			Type:  api.ValueType_UINT32,
			Value: &api.Value_UnsignedValue{UnsignedValue: uint64(v)},
		}
	case uint64:
		return &api.Value{
			Type:  api.ValueType_UINT64,
			Value: &api.Value_UnsignedValue{UnsignedValue: v},
		}
	case bool:
		return &api.Value{
			Type:  api.ValueType_BOOL,
			Value: &api.Value_BoolValue{BoolValue: v},
		}
	case float64:
		return &api.Value{
			Type:  api.ValueType_DOUBLE,
			Value: &api.Value_DoubleValue{DoubleValue: v},
		}
	}

	return nil
}

// Identifier creates a new IDENTIFIER Expression node.
func Identifier(name string) *api.Expression {
	return &api.Expression{
		Type: api.Expression_IDENTIFIER,
		Expr: &api.Expression_Identifier{
			Identifier: name,
		},
	}
}

// Value creates a new VALUE Expression node.
func Value(i interface{}) *api.Expression {
	return &api.Expression{
		Type: api.Expression_VALUE,
		Expr: &api.Expression_Value{Value: NewValue(i)},
	}
}

// IsNull creates a new IS_NULL unary Expression node
func IsNull(operand *api.Expression) *api.Expression {
	return &api.Expression{
		Type: api.Expression_IS_NULL,
		Expr: &api.Expression_UnaryOp{
			UnaryOp: operand,
		},
	}
}

// IsNotNull creates a new IS_NOT_NULL unary Expression node
func IsNotNull(operand *api.Expression) *api.Expression {
	return &api.Expression{
		Type: api.Expression_IS_NOT_NULL,
		Expr: &api.Expression_UnaryOp{
			UnaryOp: operand,
		},
	}
}

// LogicalAnd creates a new LOGICAL_AND binary Expression node. If either lhs
// or rhs is nil, the other will be returned
func LogicalAnd(lhs, rhs *api.Expression) *api.Expression {
	if lhs == nil {
		return rhs
	}
	if rhs == nil {
		return lhs
	}
	return newBinaryExpr(api.Expression_LOGICAL_AND, lhs, rhs)
}

// LogicalOr creates a new LOGICAL_OR binary Expression node. If either lhs
// or rhs is nil, the other will be returned
func LogicalOr(lhs, rhs *api.Expression) *api.Expression {
	if lhs == nil {
		return rhs
	}
	if rhs == nil {
		return lhs
	}
	return newBinaryExpr(api.Expression_LOGICAL_OR, lhs, rhs)
}

// BitwiseAnd creates a new BINARY_AND binary Expression node.
func BitwiseAnd(lhs, rhs *api.Expression) *api.Expression {
	return newBinaryExpr(api.Expression_BITWISE_AND, lhs, rhs)
}

// Equal creates a new EQ binary Expression node.
func Equal(lhs, rhs *api.Expression) *api.Expression {
	return newBinaryExpr(api.Expression_EQ, lhs, rhs)
}

// NotEqual creates a new NE binary Expression node.
func NotEqual(lhs, rhs *api.Expression) *api.Expression {
	return newBinaryExpr(api.Expression_NE, lhs, rhs)
}

// LessThan creates a new LT binary Expression node.
func LessThan(lhs, rhs *api.Expression) *api.Expression {
	return newBinaryExpr(api.Expression_LT, lhs, rhs)
}

// LessThanEqualTo creates a new LE binary Expression node.
func LessThanEqualTo(lhs, rhs *api.Expression) *api.Expression {
	return newBinaryExpr(api.Expression_LE, lhs, rhs)
}

// GreaterThan creates a new GT binary expression node.
func GreaterThan(lhs, rhs *api.Expression) *api.Expression {
	return newBinaryExpr(api.Expression_GT, lhs, rhs)
}

// GreaterThanEqualTo creates a new GE binary expression node.
func GreaterThanEqualTo(lhs, rhs *api.Expression) *api.Expression {
	return newBinaryExpr(api.Expression_GE, lhs, rhs)
}

// Like creates a new LIKE binary Expression node.
func Like(lhs, rhs *api.Expression) *api.Expression {
	return newBinaryExpr(api.Expression_LIKE, lhs, rhs)
}

func newBinaryExpr(op api.Expression_ExpressionType, lhs, rhs *api.Expression) *api.Expression {
	return &api.Expression{
		Type: op,
		Expr: &api.Expression_BinaryOp{
			BinaryOp: &api.BinaryOp{
				Lhs: lhs,
				Rhs: rhs,
			},
		},
	}
}
