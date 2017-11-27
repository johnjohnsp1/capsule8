package filter

import (
	api "github.com/capsule8/api/v0"
)

type FieldTypeMap map[string]api.ValueType
type FieldValueMap map[string]interface{}

type Expression struct {
	tree *api.Expression
}

func NewExpression(tree *api.Expression) (*Expression, error) {
	err := validateTree(tree)
	if err != nil {
		return nil, err
	}

	return &Expression{
		tree: tree,
	}, nil
}

// Return a string representation of an expression that is suitable for setting
// a kernel filter. This is mostly the same as anormal string represnetation of
// the expression; however, bitwise-and is handled specially for the kernel's
// odd syntax.
func (expr *Expression) KernelFilterString() string {
	return expressionAsKernelFilterString(expr.tree)
}

// Return the string representation of an expression.
func (expr *Expression) String() string {
	return expressionAsString(expr.tree)
}

func (expr *Expression) Evaluate(types FieldTypeMap, values FieldValueMap) (api.Value, error) {
	return evaluateExpression(expr.tree, types, values)
}

func (expr *Expression) Validate(types FieldTypeMap) error {
	_, err := validateTypes(expr.tree, types)
	return err
}

// Determine whether an expression can be represented as a kernel filter.
// If the result is nil, the kernel will most likely accept the expression as
// a filter. No check is done on the number of predicates in the expression,
// and some kernel versions do not support bitwise-and; however, this validator
// will accept bitwise-and.
func (expr *Expression) ValidateKernelFilter() error {
	return validateKernelFilterTree(expr.tree)
}

//
// Convenience functions for creating new expressions
//

func NewIdentifierExpr(name string) *api.Expression {
	return &api.Expression{
		Type: api.Expression_IDENTIFIER,
		Expr: &api.Expression_Identifier{
			Identifier: name,
		},
	}
}

func NewValueExpr(i interface{}) *api.Expression {
	var value *api.Value

	switch v := i.(type) {
	case string:
		value = &api.Value{
			Type:  api.ValueType_STRING,
			Value: &api.Value_StringValue{StringValue:v},
		}
	case int8:
		value = &api.Value{
			Type:  api.ValueType_SINT8,
			Value: &api.Value_SignedValue{SignedValue:int64(v)},
		}
	case int16:
		value = &api.Value{
			Type:  api.ValueType_SINT16,
			Value: &api.Value_SignedValue{SignedValue:int64(v)},
		}
	case int32:
		value = &api.Value{
			Type:  api.ValueType_SINT32,
			Value: &api.Value_SignedValue{SignedValue:int64(v)},
		}
	case int64:
		value = &api.Value{
			Type:  api.ValueType_SINT64,
			Value: &api.Value_SignedValue{SignedValue:v},
		}
	case uint8:
		value = &api.Value{
			Type:  api.ValueType_UINT8,
			Value: &api.Value_UnsignedValue{UnsignedValue:uint64(v)},
		}
	case uint16:
		value = &api.Value{
			Type:  api.ValueType_UINT16,
			Value: &api.Value_UnsignedValue{UnsignedValue:uint64(v)},
		}
	case uint32:
		value = &api.Value{
			Type:  api.ValueType_UINT32,
			Value: &api.Value_UnsignedValue{UnsignedValue:uint64(v)},
		}
	case uint64:
		value = &api.Value{
			Type:  api.ValueType_UINT64,
			Value: &api.Value_UnsignedValue{UnsignedValue:v},
		}
	case bool:
		value = &api.Value{
			Type:  api.ValueType_BOOL,
			Value: &api.Value_BoolValue{BoolValue:v},
		}
	case float64:
		value = &api.Value{
			Type:  api.ValueType_DOUBLE,
			Value: &api.Value_DoubleValue{DoubleValue:v},
		}
	}

	return &api.Expression{
		Type: api.Expression_VALUE,
		Expr: &api.Expression_Value{Value: value},
	}
}

func NewBinaryExpr(op api.Expression_ExpressionType, lhs, rhs *api.Expression) *api.Expression {
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

func NewUnaryExpr(op api.Expression_ExpressionType, operand *api.Expression) *api.Expression {
	return &api.Expression{
		Type: op,
		Expr: &api.Expression_UnaryOp{
			UnaryOp: operand,
		},
	}
}

func LinkExprs(op api.Expression_ExpressionType, lhs, rhs *api.Expression) *api.Expression {
	if lhs == nil {
		return rhs
	}
	if rhs == nil {
		return lhs
	}
	return NewBinaryExpr(op, lhs, rhs)
}
