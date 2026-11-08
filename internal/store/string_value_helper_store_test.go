package store

import (
	"testing"
)

// mustStringValue returns a string value in either stored form as a
// *StringValue, failing the test for any other type.
func mustStringValue(t testing.TB, v Value) *StringValue {
	t.Helper()
	sv, ok := AsStringValue(v)
	if !ok {
		t.Fatalf("value is %T, not a string", v)
	}
	return sv
}
