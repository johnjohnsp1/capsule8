package filter

import (
	"errors"
	"fmt"
	"unicode"

	api "github.com/capsule8/api/v0"
)

func validateIdentifier(ident string) error {
	if len(ident) == 0 {
		return errors.New("Invalid identifier: \"\"")
	}

	// First character must be letter or underscore
	if !(unicode.IsLetter(rune(ident[0])) || ident[0] == '_') {
		return errors.New("Identifiers must begin with letters or an underscore")
	}

	// Successive characters must be letters, digits, or underscore
	for _, r := range ident {
		if !(unicode.IsDigit(r) || unicode.IsLetter(r) || r == '_') {
			return errors.New("Identifiers must contain only letters, digits, or underscores")
		}
	}

	return nil
}

func validateValue(value *api.Value) error {
	switch value.GetType() {
	case api.ValueType_STRING:
		if _, ok := value.GetValue().(*api.Value_StringValue); !ok {
			return errors.New("STRING value has no StringValue set")
		}
	case api.ValueType_SINT8, api.ValueType_SINT16, api.ValueType_SINT32, api.ValueType_SINT64:
		if _, ok := value.GetValue().(*api.Value_SignedValue); !ok {
			return errors.New("SINT{8,16,32,64} value has no SignedValue set")
		}
	case api.ValueType_UINT8, api.ValueType_UINT16, api.ValueType_UINT32, api.ValueType_UINT64:
		if _, ok := value.GetValue().(*api.Value_UnsignedValue); !ok {
			return errors.New("UINT{8,16,32,64} value has no UnsignedValue set")
		}
	case api.ValueType_BOOL:
		if _, ok := value.GetValue().(*api.Value_BoolValue); !ok {
			return errors.New("BOOL value has no BoolValue set")
		}
	case api.ValueType_DOUBLE:
		if _, ok := value.GetValue().(*api.Value_DoubleValue); !ok {
			return errors.New("DOUBLE value has no DoubleValue set")
		}
	case api.ValueType_TIMESTAMP:
		if _, ok := value.GetValue().(*api.Value_TimestampValue); !ok {
			return errors.New("TIMESTAMP value has no TimestampValue set")
		}
	default:
		return fmt.Errorf("Unrecognized value type %d",
			value.GetType())
	}

	return nil
}

func validateNode(node *api.Expression, logical bool) error {
	switch node.GetType() {
	case api.Expression_IDENTIFIER:
		ident := node.GetIdentifier()
		if ident == "" {
			return errors.New("Identifier missing for IDENTIFIER node")
		}
		return validateIdentifier(ident)

	case api.Expression_VALUE:
		value := node.GetValue()
		if value == nil {
			return errors.New("Value missing for VALUE node")
		}
		return validateValue(value)

	case api.Expression_LOGICAL_AND, api.Expression_LOGICAL_OR:
		if !logical {
			return errors.New("Unexpected logical node")
		}
		operands := node.GetBinaryOp()
		if operands == nil {
			return errors.New("BinaryOp missing for logical AND/OR node")
		}
		if operands.Lhs == nil {
			return errors.New("BinaryOp missing lhs")
		}
		if operands.Rhs == nil {
			return errors.New("BinaryOp missing rhs")
		}
		err := validateNode(operands.Lhs, true)
		if err != nil {
			return err
		}
		return validateNode(operands.Rhs, true)

	case api.Expression_EQ, api.Expression_NE, api.Expression_LT,
		api.Expression_LE, api.Expression_GT, api.Expression_GE,
		api.Expression_LIKE:

		operands := node.GetBinaryOp()
		if operands == nil {
			return errors.New("BinaryOp missing for comparison node")
		}
		if operands.Lhs == nil {
			return errors.New("BinaryOp missing lhs")
		}
		if operands.Rhs == nil {
			return errors.New("BinaryOp missing rhs")
		}
		err := validateNode(operands.Lhs, false)
		if err != nil {
			return err
		}
		return validateNode(operands.Rhs, false)

	case api.Expression_IS_NULL, api.Expression_IS_NOT_NULL:
		operand := node.GetUnaryOp()
		if operand == nil {
			return errors.New("UnaryOp missing for unary compare")
		}
		return validateNode(operand, false)

	case api.Expression_BITWISE_AND:
		operands := node.GetBinaryOp()
		if operands == nil {
			return errors.New("BinaryOp missing for bitwise node")
		}
		if operands.Lhs == nil {
			return errors.New("BinaryOp missing lhs")
		}
		if operands.Rhs == nil {
			return errors.New("BinaryOp missing rhs")
		}
		err := validateNode(operands.Lhs, false)
		if err != nil {
			return err
		}
		return validateNode(operands.Rhs, false)

	default:
		return fmt.Errorf("Unrecognized expression type %d",
			node.GetType())
	}

	return nil
}

func validateTree(tree *api.Expression) error {
	return validateNode(tree, true)
}

func validateBitwiseAnd(expr *api.Expression) error {
	// lhs must be identifier; rhs must be an integer value
	operands := expr.GetBinaryOp()
	if operands.Lhs.GetType() != api.Expression_IDENTIFIER {
		return errors.New("BITWISE-AND lhs must be an identifier")
	}
	if operands.Rhs.GetType() != api.Expression_VALUE ||
		!isValueTypeInteger(operands.Rhs.GetValue().GetType()) {

		return errors.New("BITWISE-AND rhs must be an integer value")
	}
	err := validateKernelFilterNode(operands.Lhs)
	if err == nil {
		err = validateKernelFilterNode(operands.Rhs)
	}
	return err
}

func validateKernelFilterNode(node *api.Expression) error {
	switch node.GetType() {
	case api.Expression_IDENTIFIER:
		return nil

	case api.Expression_VALUE:
		value := node.GetValue()
		switch value.GetType() {
		case api.ValueType_STRING,
			api.ValueType_SINT8, api.ValueType_SINT16,
			api.ValueType_SINT32, api.ValueType_SINT64,
			api.ValueType_UINT8, api.ValueType_UINT16,
			api.ValueType_UINT32, api.ValueType_UINT64:

			return nil
		}
		return errors.New("Value must be string or integer")

	case api.Expression_LOGICAL_AND, api.Expression_LOGICAL_OR:
		operands := node.GetBinaryOp()
		err := validateKernelFilterTree(operands.Lhs)
		if err == nil {
			err = validateKernelFilterTree(operands.Rhs)
		}
		return err

	case api.Expression_EQ:
		// lhs must be identifier; rhs must be value, can be any type
		operands := node.GetBinaryOp()
		if operands.Lhs.GetType() != api.Expression_IDENTIFIER {
			return errors.New("Comparison lhs must be an identifier")
		}
		if operands.Rhs.GetType() != api.Expression_VALUE {
			return errors.New("Comparison rhs must be a value")
		}
		err := validateKernelFilterNode(operands.Lhs)
		if err == nil {
			err = validateKernelFilterNode(operands.Rhs)
		}
		return err

	case api.Expression_NE:
		// if lhs is bitwise-and, rhs must be integer value 0
		// if lhs is an identifier, rhs must be value of any type
		operands := node.GetBinaryOp()
		if operands.Lhs.GetType() == api.Expression_BITWISE_AND {
			err := validateBitwiseAnd(operands.Lhs)
			if err != nil {
				return err
			}

			if operands.Rhs.GetType() != api.Expression_VALUE {
				return errors.New("Rhs of comparison with bitwise-and must be 0")
			}
			value := operands.Rhs.GetValue()
			switch value.GetType() {
			case api.ValueType_SINT8, api.ValueType_SINT16,
				api.ValueType_SINT32, api.ValueType_SINT64:

				if value.GetSignedValue() != 0 {
					return errors.New("Rhs of comparison with bitwise-and must be 0")
				}
			case api.ValueType_UINT8, api.ValueType_UINT16,
				api.ValueType_UINT32, api.ValueType_UINT64:

				if value.GetUnsignedValue() != 0 {
					return errors.New("Rhs of comparison with bitwise-and must be 0")
				}
			default:
				return errors.New("Rhs of comparison with bitwise-and must be 0")
			}
			return nil
		}
		if operands.Lhs.GetType() != api.Expression_IDENTIFIER {
			return errors.New("Comparison lhs must be an identifier")
		}
		if operands.Rhs.GetType() != api.Expression_VALUE {
			return errors.New("Comparison rhs must be a value")
		}
		err := validateKernelFilterNode(operands.Lhs)
		if err == nil {
			err = validateKernelFilterNode(operands.Rhs)
		}
		return err

	case api.Expression_LT, api.Expression_LE,
		api.Expression_GT, api.Expression_GE:

		// lhs must be identifier; rhs must be value of integer type
		operands := node.GetBinaryOp()
		if operands.Lhs.GetType() != api.Expression_IDENTIFIER {
			return errors.New("Comparison lhs must be an identifier")
		}
		if operands.Rhs.GetType() != api.Expression_VALUE ||
			!isValueTypeInteger(operands.Rhs.GetValue().GetType()) {

			return errors.New("Comparison rhs must be an integer value")
		}
		err := validateKernelFilterNode(operands.Lhs)
		if err == nil {
			err = validateKernelFilterNode(operands.Rhs)
		}
		return err

	case api.Expression_LIKE:
		// lhs must be identifier; rhs must be string value
		operands := node.GetBinaryOp()
		if operands.Lhs.GetType() != api.Expression_IDENTIFIER {
			return errors.New("Comparison lhs must be an identifier")
		}
		if operands.Rhs.GetType() != api.Expression_VALUE ||
			!isValueTypeString(operands.Rhs.GetValue().GetType()) {

			return errors.New("Comparison rhs must be a string value")
		}
		err := validateKernelFilterNode(operands.Lhs)
		if err == nil {
			err = validateKernelFilterNode(operands.Rhs)
		}
		return err
	}

	return fmt.Errorf("Invalid expression type %d", node.GetType())
}

func validateKernelFilterTree(expr *api.Expression) error {
	if expr.GetType() == api.Expression_BITWISE_AND {
		return validateBitwiseAnd(expr)
	}
	return validateKernelFilterNode(expr)
}

func validateTypes(expr *api.Expression, types FieldTypeMap) (api.ValueType, error) {
	switch op := expr.GetType(); op {
	case api.Expression_IDENTIFIER:
		ident := expr.GetIdentifier()
		t, ok := types[ident]
		if !ok {
			return 0, fmt.Errorf("Undefined identifier %q", ident)
		}
		return t, nil
	case api.Expression_VALUE:
		return expr.GetValue().GetType(), nil
	case api.Expression_LOGICAL_AND, api.Expression_LOGICAL_OR:
		operands := expr.GetBinaryOp()
		lhs, err := validateTypes(operands.Lhs, types)
		if err != nil {
			return 0, err
		}
		if lhs != api.ValueType_BOOL {
			err = fmt.Errorf("Lhs of AND/OR must be type BOOL; got %s",
				api.ValueType_name[int32(lhs)])
			return 0, err
		}
		rhs, err := validateTypes(operands.Rhs, types)
		if err != nil {
			return 0, err
		}
		if rhs != api.ValueType_BOOL {
			err = fmt.Errorf("Rhs of AND/OR must be type BOOL; got %s",
				api.ValueType_name[int32(rhs)])
			return 0, err
		}
		return api.ValueType_BOOL, nil
	case api.Expression_EQ, api.Expression_NE:
		operands := expr.GetBinaryOp()
		lhs, err := validateTypes(operands.Lhs, types)
		if err != nil {
			return 0, err
		}
		rhs, err := validateTypes(operands.Rhs, types)
		if err != nil {
			return 0, err
		}
		if lhs != rhs {
			err = fmt.Errorf("Type mismatch (%s vs. %s)",
				api.ValueType_name[int32(lhs)],
				api.ValueType_name[int32(rhs)])
			return 0, err
		}
		return api.ValueType_BOOL, nil

	case api.Expression_LT, api.Expression_LE,
		api.Expression_GT, api.Expression_GE:

		operands := expr.GetBinaryOp()
		lhs, err := validateTypes(operands.Lhs, types)
		if err != nil {
			return 0, err
		}
		if !isValueTypeNumeric(lhs) {
			err = fmt.Errorf("Type for %s must be numeric; got %s",
				operatorStrings[op],
				api.ValueType_name[int32(lhs)])
			return 0, err
		}
		rhs, err := validateTypes(operands.Rhs, types)
		if err != nil {
			return 0, err
		}
		if lhs != rhs {
			err = fmt.Errorf("Type mismatch (%s vs. %s)",
				api.ValueType_name[int32(lhs)],
				api.ValueType_name[int32(rhs)])
			return 0, err
		}
		return api.ValueType_BOOL, nil

	case api.Expression_LIKE:
		operands := expr.GetBinaryOp()
		lhs, err := validateTypes(operands.Lhs, types)
		if err != nil {
			return 0, err
		}
		if !isValueTypeString(lhs) {
			err = fmt.Errorf("Type for LIKE must be STRING; got %s",
				api.ValueType_name[int32(lhs)])
			return 0, err
		}
		rhs, err := validateTypes(operands.Rhs, types)
		if err != nil {
			return 0, err
		}
		if lhs != rhs {
			err = fmt.Errorf("Type mismatch (%s vs. %s)",
				api.ValueType_name[int32(lhs)],
				api.ValueType_name[int32(rhs)])
			return 0, err
		}
		return api.ValueType_BOOL, nil

	case api.Expression_IS_NULL, api.Expression_IS_NOT_NULL:
		_, err := validateTypes(expr.GetUnaryOp(), types)
		if err != nil {
			return 0, err
		}
		return api.ValueType_BOOL, nil

	case api.Expression_BITWISE_AND:
		operands := expr.GetBinaryOp()
		lhs, err := validateTypes(operands.Lhs, types)
		if err != nil {
			return 0, err
		}
		if !isValueTypeInteger(lhs) {
			err = fmt.Errorf("Type for %s must be an integer; got %s",
				operatorStrings[op],
				api.ValueType_name[int32(lhs)])
			return 0, err
		}
		rhs, err := validateTypes(operands.Rhs, types)
		if err != nil {
			return 0, err
		}
		if lhs != rhs {
			err = fmt.Errorf("Type mismatch (%s vs. %s)",
				api.ValueType_name[int32(lhs)],
				api.ValueType_name[int32(rhs)])
			return 0, err
		}
		return api.ValueType_BOOL, nil
	}

	return 0, nil
}
