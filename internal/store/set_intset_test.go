package store

import (
	"math"
	"math/rand"
	"slices"
	"testing"
)

// The packed intset agrees with a sorted []int64 through random inserts and
// removals across every width, widens when a member needs it and carries no
// spare capacity.
func TestIntsetMatchesSortedSlice(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	pick := []func() int64{
		func() int64 { return int64(r.Intn(200) - 100) },
		func() int64 { return int64(r.Intn(1<<20) - 1<<19) },
		func() int64 { return r.Int63() - math.MaxInt64/2 },
		func() int64 {
			return []int64{math.MinInt64, math.MaxInt64, -1 << 15, 1<<15 - 1, -1 << 31, 1<<31 - 1}[r.Intn(6)]
		},
	}
	for round := 0; round < 200; round++ {
		var s intset
		var ref []int64
		for op := 0; op < 300; op++ {
			n := pick[r.Intn(len(pick))]()
			if r.Intn(3) == 0 && len(ref) > 0 {
				n = ref[r.Intn(len(ref))]
				i, ok := s.search(n)
				j, refOk := slices.BinarySearch(ref, n)
				if !ok || !refOk || i != j {
					t.Fatalf("search(%d) = %d,%v want %d,%v", n, i, ok, j, refOk)
				}
				s = s.remove(i)
				ref = slices.Delete(ref, j, j+1)
			} else if i, ok := s.search(n); !ok {
				s = s.insert(i, n)
				j, _ := slices.BinarySearch(ref, n)
				ref = slices.Insert(ref, j, n)
			}
			if s.Len() != len(ref) {
				t.Fatalf("Len %d want %d", s.Len(), len(ref))
			}
			if len(s) > 0 && cap(s) != len(s) {
				t.Fatalf("intset has spare capacity %d > %d", cap(s), len(s))
			}
		}
		for i, want := range ref {
			if got := s.at(i); got != want {
				t.Fatalf("at(%d) = %d want %d", i, got, want)
			}
		}
	}

	var s intset
	for _, n := range []int64{1, 2, 3} {
		i, _ := s.search(n)
		s = s.insert(i, n)
	}
	if s[0] != 2 || len(s) != 1+3*2 {
		t.Fatalf("three small members use width %d and %d bytes", s[0], len(s))
	}
	i, _ := s.search(1 << 40)
	s = s.insert(i, 1<<40)
	if s[0] != 8 || s.at(0) != 1 || s.at(3) != 1<<40 {
		t.Fatal("a large member did not widen every member")
	}
}
