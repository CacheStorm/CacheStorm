package store

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// NewLoadedHash builds in one pass the hash that setting the fields one at a
// time and then fixing their order built: the same form, order, listpack
// conversion and accounted size.
func TestNewLoadedHashMatchesIncrementalBuild(t *testing.T) {
	for _, tc := range []struct {
		name   string
		n      int
		valLen int
	}{
		{"one", 1, 3},
		{"small", 10, 8},
		{"pairwise check limit", 16, 8},
		{"map check", 17, 8},
		{"packed limit", hashSmallMax, 8},
		{"past packed limit", hashSmallMax + 1, 8},
		{"large", 300, 8},
		{"value past listpack-value", 5, 100},
		{"past packed bytes", 40, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := make([]string, 0, 2*tc.n)
			for i := range tc.n {
				// Out of sorted order, so order is checked.
				items = append(items, fmt.Sprintf("field:%d", (i*7919)%tc.n), strings.Repeat("v", tc.valLen)+fmt.Sprint(i))
			}
			got, ok := NewLoadedHash(items)
			if !ok {
				t.Fatal("unique fields refused")
			}
			want := &HashValue{}
			order := make([]string, 0, tc.n)
			for i := 0; i < len(items); i += 2 {
				want.SetFieldKeepTTL(items[i], []byte(items[i+1]))
				order = append(order, items[i])
			}
			want.SetOrder(order)

			if (got.dict == nil) != (want.dict == nil) || got.Big() != want.Big() || got.Len() != want.Len() {
				t.Fatalf("form: packed %v/%v big %v/%v len %d/%d", got.dict == nil, want.dict == nil, got.Big(), want.Big(), got.Len(), want.Len())
			}
			if g, w := fieldOrder(got), fieldOrder(want); !slices.Equal(g, w) {
				t.Fatalf("order:\n got %v\nwant %v", g, w)
			}
			for i := 0; i < len(items); i += 2 {
				if val, ok := got.Get(items[i]); !ok || string(val) != items[i+1] {
					t.Fatalf("field %s: %q %v", items[i], val, ok)
				}
			}
			if got.SizeOf() != want.SizeOf() || got.SizeOf() != got.RecountSize() {
				t.Fatalf("size %d, incremental %d, recount %d", got.SizeOf(), want.SizeOf(), got.RecountSize())
			}
		})
	}
}

func TestNewLoadedHashRefusesRepeatedFields(t *testing.T) {
	for _, n := range []int{2, 16, 40} {
		items := make([]string, 0, 2*n)
		for i := range n {
			items = append(items, fmt.Sprint("f", i), "v")
		}
		items[2*(n-1)] = "f0"
		if _, ok := NewLoadedHash(items); ok {
			t.Fatalf("%d fields: a repeated field was accepted", n)
		}
	}
}

func fieldOrder(v *HashValue) []string {
	var out []string
	v.Range(func(f string, _ []byte) bool {
		out = append(out, f)
		return true
	})
	if v.Big() {
		slices.Sort(out)
	}
	return out
}
