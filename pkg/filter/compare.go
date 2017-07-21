package filter

import (
	"fmt"
	"reflect"

	"github.com/gobwas/glob"
)

var (
	ErrUnknownComparisonOperation = func(op string) error { return fmt.Errorf("unknown comparison operation: %s\n", op) }
)

type ComparisonOperation string

const (
	COMPARISON_EQUALITY     = "equality"
	COMPARISON_BITAND       = "bitand"
	COMPARISON_PATTERNMATCH = "patternmatch"
)

type MappedField struct {
	Name string
	Op   ComparisonOperation
}

// CompareFields checks if ALL fields in lhs struct match the rhs struct.
// Generally, the lhs is the filter and the rhs is the event.
func CompareFields(lhs, rhs interface{}, mapping map[string]*MappedField) (bool, error) {
	var err error
	allFieldsMatch := true
	// If ANY of the fields in a filter do not match, we return false
	lhsType := reflect.TypeOf(lhs).Elem()
compareLoop:
	for i := 0; i < lhsType.NumField(); i++ {
		lhsValue := reflect.ValueOf(lhs).Elem()
		rhsValue := reflect.ValueOf(rhs).Elem()
		lhsFieldName := lhsType.Field(i).Name

		var match bool
		if mappedField, ok := mapping[lhsFieldName]; ok {
			// Use mapping if present
			match, err = compare(
				lhsValue.FieldByName(lhsFieldName),
				rhsValue.FieldByName(mappedField.Name),
				mappedField.Op,
			)
		} else {
			// Otherwise assume that the field names match and default to equality comparison
			match, err = compare(
				lhsValue.FieldByName(lhsFieldName),
				rhsValue.FieldByName(lhsFieldName),
				COMPARISON_EQUALITY,
			)
		}

		if err != nil {
			break compareLoop
		}

		if !match {
			allFieldsMatch = false
			break compareLoop
		}
	}
	return allFieldsMatch, err
}

// Compare two reflected values via comparison operator
func compare(lhs, rhs reflect.Value, op ComparisonOperation) (bool, error) {
	// TODO: At some point all lhs values should be wrapped values w/ an operator
	// Some values are wrapped values
	if lhs.Kind() == reflect.Ptr {
		// Pass on comparing unset filter values
		if lhs.IsNil() {
			return true, nil
		}
		lhs = lhs.Elem().FieldByName("Value")
	}
	// Pass if no filter value specified
	if lhs.Interface() == nil {
		return true, nil
	}
	switch op {
	case COMPARISON_EQUALITY:
		return lhs.Interface() == rhs.Interface(), nil
	case COMPARISON_BITAND:
		// TODO: Maybe not every future flag is typed int32 and will break this
		return lhs.Interface().(int32)&rhs.Interface().(int32) == 0, nil
	case COMPARISON_PATTERNMATCH:
		g, err := glob.Compile(lhs.Interface().(string))
		if err != nil {
			return false, err
		}
		return g.Match(rhs.Interface().(string)), nil
	}

	return false, ErrUnknownComparisonOperation(fmt.Sprintf("%v", op))
}
