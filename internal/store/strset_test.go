package store

import (
	"math/rand"
	"slices"
	"strconv"
	"testing"
)

// strSet behaves as a map[string]struct{} through random adds and removes
// that cross the small and hashed forms both ways, the empty key included.
func TestStrSetMatchesMapX(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for round := 0; round < 40; round++ {
		s := newStrSet()
		ref := map[string]struct{}{}
		universe := 1 + r.Intn(3000)
		for op := 0; op < 20000; op++ {
			key := ""
			if r.Intn(500) != 0 {
				key = "k" + strconv.Itoa(r.Intn(universe))
			}
			_, had := ref[key]
			if r.Intn(3) == 0 {
				if got := s.Remove(key); got != had {
					t.Fatalf("round %d: Remove(%q) = %v, had %v", round, key, got, had)
				}
				delete(ref, key)
			} else {
				if got := s.Add(key); got == had {
					t.Fatalf("round %d: Add(%q) = %v, had %v", round, key, got, had)
				}
				ref[key] = struct{}{}
			}
			if s.Len() != len(ref) {
				t.Fatalf("round %d: Len = %d, want %d", round, s.Len(), len(ref))
			}
		}
		for k := range ref {
			if !s.Has(k) {
				t.Fatalf("round %d: lost %q", round, k)
			}
		}
		got := s.AppendTo(nil)
		want := make([]string, 0, len(ref))
		for k := range ref {
			want = append(want, k)
		}
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("round %d: members differ", round)
		}
		n := 0
		s.Range(func(string) bool { n++; return true })
		if n != len(ref) {
			t.Fatalf("round %d: Range visited %d of %d", round, n, len(ref))
		}
	}
}

// Removing everything from a large set brings it back to the small form,
// and a member costs far less than in a Go map.
func TestStrSetShrinksX(t *testing.T) {
	s := newStrSet()
	for i := 0; i < 5000; i++ {
		s.Add("k" + strconv.Itoa(i))
	}
	if !s.hashed || len(s.slots) > 5000*2 {
		t.Fatalf("hashed=%v slots=%d for 5000 members", s.hashed, len(s.slots))
	}
	for i := 0; i < 4998; i++ {
		s.Remove("k" + strconv.Itoa(i))
	}
	if s.hashed || s.Len() != 2 || !s.Has("k4998") || !s.Has("k4999") {
		t.Fatalf("after removing: hashed=%v len=%d", s.hashed, s.Len())
	}
}
