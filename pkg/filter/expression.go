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
