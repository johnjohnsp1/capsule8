package subscription

import (
	"fmt"

	api "github.com/capsule8/api/v0"
)

func filterString(f *api.FilterExpression) string {
	var joiner string

	switch f.Type {
	case api.FilterExpression_PREDICATE:
		if f.Predicate == nil {
			return ""
		}
		return predicateFilterString(f.Predicate)
	case api.FilterExpression_AND:
		joiner = "&&"
		break
	case api.FilterExpression_OR:
		joiner = "||"
		break
	default:
		return ""
	}

	if f.Lhs == nil || f.Rhs == nil {
		return ""
	}
	lhs := filterString(f.Lhs)
	if lhs == "" {
		return ""
	}
	rhs := filterString(f.Rhs)
	if rhs == "" {
		return ""
	}
	if f.Rhs.Type != api.FilterExpression_PREDICATE {
		// AND and OR are left-associative by default, so if there's
		// rhs recursion, wrap the resultant string in parenthesis to
		// make it right-associative.
		rhs = fmt.Sprintf("(%s)", rhs)
	}

	return fmt.Sprintf("%s %s %s", lhs, joiner, rhs)
}

func predicateFilterString(p *api.FilterPredicate) string {
	var value string

	switch p.ValueType {
	case api.FilterPredicate_SIGNED:
		value = fmt.Sprintf("%d", p.Value.(*api.FilterPredicate_SignedValue).SignedValue)
	case api.FilterPredicate_UNSIGNED:
		value = fmt.Sprintf("%d", p.Value.(*api.FilterPredicate_UnsignedValue).UnsignedValue)
	case api.FilterPredicate_STRING:
		value = fmt.Sprintf("%q", p.Value.(*api.FilterPredicate_StringValue).StringValue)
	default:
		return ""
	}

	switch p.Type {
	case api.FilterPredicate_CONST:
		return value
	case api.FilterPredicate_EQ:
		return fmt.Sprintf("%s == %s", p.FieldName, value)
	case api.FilterPredicate_NE:
		return fmt.Sprintf("%s != %s", p.FieldName, value)
	case api.FilterPredicate_LT:
		if p.ValueType != api.FilterPredicate_SIGNED &&
			p.ValueType != api.FilterPredicate_UNSIGNED {

			return ""
		}
		return fmt.Sprintf("%s < %s", p.FieldName, value)
	case api.FilterPredicate_LE:
		if p.ValueType != api.FilterPredicate_SIGNED &&
			p.ValueType != api.FilterPredicate_UNSIGNED {

			return ""
		}
		return fmt.Sprintf("%s <= %s", p.FieldName, value)
	case api.FilterPredicate_GT:
		if p.ValueType != api.FilterPredicate_SIGNED &&
			p.ValueType != api.FilterPredicate_UNSIGNED {

			return ""
		}
		return fmt.Sprintf("%s > %s", p.FieldName, value)
	case api.FilterPredicate_GE:
		if p.ValueType != api.FilterPredicate_SIGNED &&
			p.ValueType != api.FilterPredicate_UNSIGNED {

			return ""
		}
		return fmt.Sprintf("%s >= %s", p.FieldName, value)
	case api.FilterPredicate_GLOB:
		if p.ValueType != api.FilterPredicate_STRING {
			return ""
		}
		return fmt.Sprintf("%s ~ %s", p.FieldName, value)
	}

	return ""
}
