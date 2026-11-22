package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// The sums and their text come from glibc's strtold and "%.17Lg" on x86-64,
// as AROP SUM computes them.
func TestArrayLongDoubleSum(t *testing.T) {
	for _, tc := range []struct {
		in   []string
		want string
	}{
		{[]string{"0.1", "0.2"}, "0.3"},
		{[]string{"1", "2", "3"}, "6"},
		{[]string{"1e20", "1"}, "1e+20"},
		{[]string{"123456789012345678", "0.5"}, "1.2345678901234568e+17"},
		{[]string{"-0.0001", "0.00001"}, "-9e-05"},
		{[]string{"1e-5"}, "1e-05"},
		{[]string{"99999999999999999.5"}, "1e+17"},
		{[]string{"3.14159265358979323846"}, "3.1415926535897932"},
		{[]string{"1e300", "1e300"}, "2e+300"},
		{[]string{"-2.5", "2.5"}, "0"},
		{[]string{"1e4000", "1e4000"}, "2e+4000"},
		{[]string{"0.000123456789"}, "0.000123456789"},
	} {
		var acc ldNum
		for i, s := range tc.in {
			n, _, _, ok := arrayNumber(s)
			if !ok {
				t.Fatalf("%q is not a number", s)
			}
			if i == 0 {
				acc = n
			} else {
				acc = acc.add(n)
			}
		}
		if got := acc.format(); got != tc.want {
			t.Errorf("sum of %v = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestClassifyArrayValue(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want store.ArrayKind
	}{
		{"3", store.ArrayInt}, {"-17", store.ArrayInt}, {"007", store.ArraySmallString},
		{"2305843009213693951", store.ArrayInt}, {"2305843009213693952", store.ArrayString},
		{"1.5", store.ArrayFloat}, {"1.0", store.ArrayFloat}, {"-0.0", store.ArrayFloat},
		{"0.1", store.ArraySmallString}, {"1e5", store.ArraySmallString}, {"hello world", store.ArrayString},
	} {
		if got, _, _ := store.ClassifyArrayValue(tc.in); got != tc.want {
			t.Errorf("ClassifyArrayValue(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
