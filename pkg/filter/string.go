package filter

import (
	"fmt"
	"time"

	api "github.com/capsule8/api/v0"
)

func valueAsString(value *api.Value) string {
	switch value.GetType() {
	case api.ValueType_STRING:
		return fmt.Sprintf("%q", value.GetStringValue())
	case api.ValueType_SINT8, api.ValueType_SINT16,
		api.ValueType_SINT32, api.ValueType_SINT64:

		return fmt.Sprintf("%d", value.GetSignedValue())

	case api.ValueType_UINT8, api.ValueType_UINT16,
		api.ValueType_UINT32, api.ValueType_UINT64:

		return fmt.Sprintf("%d", value.GetUnsignedValue())

	case api.ValueType_BOOL:
		if value.GetBoolValue() {
			return "TRUE"
		} else {
			return "FALSE"
		}

	case api.ValueType_DOUBLE:
		return fmt.Sprintf("%f", value.GetDoubleValue())

	case api.ValueType_TIMESTAMP:
		v := value.GetTimestampValue()
		return fmt.Sprintf("TIMESTAMP(%d)",
			time.Duration(v.Seconds)+(time.Duration(v.Nanos)*time.Second))
	}

	return "<<invalid>>"
}

var operatorStrings = map[api.Expression_ExpressionType]string{
	api.Expression_LOGICAL_AND: "&&",
	api.Expression_LOGICAL_OR:  "||",
	api.Expression_EQ:          "==",
	api.Expression_NE:          "!=",
	api.Expression_LT:          "<",
	api.Expression_LE:          "<=",
	api.Expression_GT:          ">",
	api.Expression_GE:          ">=",
	api.Expression_LIKE:        "LIKE",
	api.Expression_BITWISE_AND: "&",
}

func expressionAsString(expr *api.Expression) string {
	switch t := expr.GetType(); t {
	case api.Expression_IDENTIFIER:
		return expr.GetIdentifier()

	case api.Expression_VALUE:
		return valueAsString(expr.GetValue())

	case api.Expression_LOGICAL_AND, api.Expression_LOGICAL_OR:
		operands := expr.GetBinaryOp()
		lhs := expressionAsString(operands.Lhs)
		rhs := expressionAsString(operands.Rhs)
		if operands.Rhs.GetType() == api.Expression_LOGICAL_AND ||
			operands.Rhs.GetType() == api.Expression_LOGICAL_OR {

			rhs = fmt.Sprintf("(%s)", rhs)
		}
		return fmt.Sprintf("%s %s %s", lhs, operatorStrings[t], rhs)

	case api.Expression_EQ, api.Expression_NE, api.Expression_LT,
		api.Expression_LE, api.Expression_GT, api.Expression_GE,
		api.Expression_LIKE:

		operands := expr.GetBinaryOp()
		lhs := expressionAsString(operands.Lhs)
		rhs := expressionAsString(operands.Rhs)
		return fmt.Sprintf("%s %s %s", lhs, operatorStrings[t], rhs)

	case api.Expression_IS_NULL:
		operand := expressionAsString(expr.GetUnaryOp())
		return fmt.Sprintf("%s IS NULL", operand)

	case api.Expression_IS_NOT_NULL:
		operand := expressionAsString(expr.GetUnaryOp())
		return fmt.Sprintf("%s IS NOT NULL", operand)

	case api.Expression_BITWISE_AND:
		operands := expr.GetBinaryOp()
		lhs := expressionAsString(operands.Lhs)
		rhs := expressionAsString(operands.Rhs)
		if operands.Rhs.GetType() == api.Expression_BITWISE_AND {
			rhs = fmt.Sprintf("(%s)", rhs)
		}
		return fmt.Sprintf("%s %s %s", lhs, operatorStrings[t], rhs)
	}

	return ""
}

func expressionAsKernelFilterString(expr *api.Expression) string {
	// This is basically the same as expressionAsString except for special
	// handling for BITWISE_AND and an alternate operator representation
	// for LIKE
	switch t := expr.GetType(); t {
	case api.Expression_IDENTIFIER:
		return expr.GetIdentifier()

	case api.Expression_VALUE:
		return valueAsString(expr.GetValue())

	case api.Expression_LOGICAL_AND, api.Expression_LOGICAL_OR:
		operands := expr.GetBinaryOp()
		lhs := expressionAsString(operands.Lhs)
		rhs := expressionAsString(operands.Rhs)
		if operands.Rhs.GetType() == api.Expression_LOGICAL_AND ||
			operands.Rhs.GetType() == api.Expression_LOGICAL_OR {

			rhs = fmt.Sprintf("(%s)", rhs)
		}
		return fmt.Sprintf("%s %s %s", lhs, operatorStrings[t], rhs)

	case api.Expression_NE:
		operands := expr.GetBinaryOp()
		lhs := expressionAsString(operands.Lhs)
		if operands.Lhs.GetType() == api.Expression_BITWISE_AND {
			// Assume that Rhs is 0 because prior validation
			// should ensure that to be the case
			return lhs
		}
		rhs := expressionAsString(operands.Rhs)
		return fmt.Sprintf("%s %s %s", lhs, operatorStrings[t], rhs)

	case api.Expression_EQ, api.Expression_LT, api.Expression_LE,
		api.Expression_GT, api.Expression_GE:

		operands := expr.GetBinaryOp()
		lhs := expressionAsString(operands.Lhs)
		rhs := expressionAsString(operands.Rhs)
		return fmt.Sprintf("%s %s %s", lhs, operatorStrings[t], rhs)

	case api.Expression_LIKE:
		operands := expr.GetBinaryOp()
		lhs := expressionAsString(operands.Lhs)
		rhs := expressionAsString(operands.Rhs)
		return fmt.Sprintf("%s ~ %s", lhs, rhs)

	case api.Expression_BITWISE_AND:
		operands := expr.GetBinaryOp()
		lhs := expressionAsString(operands.Lhs)
		rhs := expressionAsString(operands.Rhs)
		return fmt.Sprintf("%s %s %s", lhs, operatorStrings[t], rhs)
	}

	return ""
}
