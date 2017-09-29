package subscription

import (
	"testing"

	api "github.com/capsule8/api/v0"
)

func newSignedFilterPredicate(fieldName string, opType api.FilterPredicate_PredicateType, value int64) *api.FilterPredicate {
	return &api.FilterPredicate{
		FieldName: fieldName,
		Type:      opType,
		ValueType: api.FilterPredicate_SIGNED,
		Value: &api.FilterPredicate_SignedValue{
			SignedValue: value,
		},
	}
}

func newUnsignedFilterPredicate(fieldName string, opType api.FilterPredicate_PredicateType, value uint64) *api.FilterPredicate {
	return &api.FilterPredicate{
		FieldName: fieldName,
		Type:      opType,
		ValueType: api.FilterPredicate_UNSIGNED,
		Value: &api.FilterPredicate_UnsignedValue{
			UnsignedValue: value,
		},
	}
}

func newStringFilterPredicate(fieldName string, opType api.FilterPredicate_PredicateType, value string) *api.FilterPredicate {
	return &api.FilterPredicate{
		FieldName: fieldName,
		Type:      opType,
		ValueType: api.FilterPredicate_STRING,
		Value: &api.FilterPredicate_StringValue{
			StringValue: value,
		},
	}
}

func TestSignedFilterPredicate(t *testing.T) {
	var (
		fp *api.FilterPredicate
		s  string
		e  string
	)

	e = "-42"
	fp = newSignedFilterPredicate("foo", api.FilterPredicate_CONST, -42)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo == -42"
	fp = newSignedFilterPredicate("foo", api.FilterPredicate_EQ, -42)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo != -42"
	fp = newSignedFilterPredicate("foo", api.FilterPredicate_NE, -42)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo < -42"
	fp = newSignedFilterPredicate("foo", api.FilterPredicate_LT, -42)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo <= -42"
	fp = newSignedFilterPredicate("foo", api.FilterPredicate_LE, -42)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo > -42"
	fp = newSignedFilterPredicate("foo", api.FilterPredicate_GT, -42)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo >= -42"
	fp = newSignedFilterPredicate("foo", api.FilterPredicate_GE, -42)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = ""
	fp = newSignedFilterPredicate("foo", api.FilterPredicate_GLOB, -42)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}
}

func TestUnsignedFilterPredicate(t *testing.T) {
	var (
		fp *api.FilterPredicate
		s  string
		e  string
	)

	e = "8493"
	fp = newUnsignedFilterPredicate("foo", api.FilterPredicate_CONST, 8493)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo == 8493"
	fp = newUnsignedFilterPredicate("foo", api.FilterPredicate_EQ, 8493)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo != 8493"
	fp = newUnsignedFilterPredicate("foo", api.FilterPredicate_NE, 8493)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo < 8493"
	fp = newUnsignedFilterPredicate("foo", api.FilterPredicate_LT, 8493)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo <= 8493"
	fp = newUnsignedFilterPredicate("foo", api.FilterPredicate_LE, 8493)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo > 8493"
	fp = newUnsignedFilterPredicate("foo", api.FilterPredicate_GT, 8493)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo >= 8493"
	fp = newUnsignedFilterPredicate("foo", api.FilterPredicate_GE, 8493)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = ""
	fp = newUnsignedFilterPredicate("foo", api.FilterPredicate_GLOB, 8493)
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}
}

func TestStringFilterPredicate(t *testing.T) {
	var (
		fp *api.FilterPredicate
		s  string
		e  string
	)

	e = "\"bar\""
	fp = newStringFilterPredicate("foo", api.FilterPredicate_CONST, "bar")
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo == \"bar\""
	fp = newStringFilterPredicate("foo", api.FilterPredicate_EQ, "bar")
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo != \"bar\""
	fp = newStringFilterPredicate("foo", api.FilterPredicate_NE, "bar")
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = ""
	fp = newStringFilterPredicate("foo", api.FilterPredicate_LT, "bar")
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = ""
	fp = newStringFilterPredicate("foo", api.FilterPredicate_LE, "bar")
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = ""
	fp = newStringFilterPredicate("foo", api.FilterPredicate_GT, "bar")
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = ""
	fp = newStringFilterPredicate("foo", api.FilterPredicate_GE, "bar")
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "foo ~ \"bar\""
	fp = newStringFilterPredicate("foo", api.FilterPredicate_GLOB, "bar")
	s = predicateFilterString(fp)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}
}

func TestFilterExpression(t *testing.T) {
	var (
		fe *api.FilterExpression
		s  string
		e  string
	)

	e = "port > 1024"
	fe = &api.FilterExpression{
		Type:      api.FilterExpression_PREDICATE,
		Predicate: newUnsignedFilterPredicate("port", api.FilterPredicate_GT, 1024),
	}
	s = filterString(fe)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "port == 80 || port == 443"
	fe = &api.FilterExpression{
		Type: api.FilterExpression_OR,
		Lhs: &api.FilterExpression{
			Type:      api.FilterExpression_PREDICATE,
			Predicate: newUnsignedFilterPredicate("port", api.FilterPredicate_EQ, 80),
		},
		Rhs: &api.FilterExpression{
			Type:      api.FilterExpression_PREDICATE,
			Predicate: newUnsignedFilterPredicate("port", api.FilterPredicate_EQ, 443),
		},
	}
	s = filterString(fe)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "port != 80 && port != 443"
	fe = &api.FilterExpression{
		Type: api.FilterExpression_AND,
		Lhs: &api.FilterExpression{
			Type:      api.FilterExpression_PREDICATE,
			Predicate: newUnsignedFilterPredicate("port", api.FilterPredicate_NE, 80),
		},
		Rhs: &api.FilterExpression{
			Type:      api.FilterExpression_PREDICATE,
			Predicate: newUnsignedFilterPredicate("port", api.FilterPredicate_NE, 443),
		},
	}
	s = filterString(fe)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}

	e = "port == 80 && (address == \"192.168.1.4\" || address == \"127.0.0.1\")"
	fe = &api.FilterExpression{
		Type: api.FilterExpression_AND,
		Lhs: &api.FilterExpression{
			Type:      api.FilterExpression_PREDICATE,
			Predicate: newUnsignedFilterPredicate("port", api.FilterPredicate_EQ, 80),
		},
		Rhs: &api.FilterExpression{
			Type: api.FilterExpression_OR,
			Lhs: &api.FilterExpression{
				Type:      api.FilterExpression_PREDICATE,
				Predicate: newStringFilterPredicate("address", api.FilterPredicate_EQ, "192.168.1.4"),
			},
			Rhs: &api.FilterExpression{
				Type:      api.FilterExpression_PREDICATE,
				Predicate: newStringFilterPredicate("address", api.FilterPredicate_EQ, "127.0.0.1"),
			},
		},
	}
	s = filterString(fe)
	if s != e {
		t.Errorf("want: %q, got %q", e, s)
	}
}
