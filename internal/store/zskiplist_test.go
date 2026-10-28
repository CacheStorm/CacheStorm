package store

import (
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"testing"
)

// The skiplist index agrees with a sorted copy of Members through random
// adds, score changes and removals.
func TestSortedSetIndexMatchesSortedMembers(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	v := &SortedSetValue{}
	for step := 0; step < 20000; step++ {
		m := "m" + strconv.Itoa(rng.Intn(400))
		switch rng.Intn(3) {
		case 0, 1:
			v.Add(m, float64(rng.Intn(50)))
		default:
			v.Remove(m)
		}
		if step%500 != 0 {
			continue
		}
		want := make([]SortedEntry, 0, len(v.Members))
		for m, s := range v.Members {
			want = append(want, SortedEntry{Member: m, Score: s})
		}
		sort.Sort(sortedEntries(want))
		got := v.GetAllEntries()
		if len(got) != len(want) {
			t.Fatalf("step %d: %d entries, want %d", step, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("step %d: entry %d is %v, want %v", step, i, got[i], want[i])
			}
			if r := v.Rank(want[i].Member, false); r != i {
				t.Fatalf("step %d: rank of %s is %d, want %d", step, want[i].Member, r, i)
			}
			if r := v.Rank(want[i].Member, true); r != len(want)-1-i {
				t.Fatalf("step %d: reverse rank of %s is %d", step, want[i].Member, r)
			}
		}
		rev := v.RangeByRank(1, 5, true)
		for i, e := range rev {
			if e != want[len(want)-2-i] {
				t.Fatalf("step %d: reverse rank %d is %v", step, i+1, e)
			}
		}
		lo, hi := float64(rng.Intn(50)), float64(rng.Intn(50))
		first, last := v.ScoreRanks(lo, true, hi, false)
		n := 0
		for _, e := range want {
			if e.Score > lo && e.Score <= hi {
				n++
			}
		}
		if last-first+1 != n && (n != 0 || last >= first) {
			t.Fatalf("step %d: (%v,%v] holds %d..%d, want %d members", step, lo, hi, first, last, n)
		}
	}
}

// Members filled directly, as the snapshot loader does, get their order on
// first use.
func TestSortedSetIndexBuiltFromMembers(t *testing.T) {
	v := &SortedSetValue{Members: map[string]float64{"c": 1, "a": 2, "b": 1}}
	got := v.GetAllEntries()
	if len(got) != 3 || got[0].Member != "b" || got[1].Member != "c" || got[2].Member != "a" {
		t.Fatalf("order %v", got)
	}
	v.Add("d", 0)
	if r := v.Rank("d", false); r != 0 {
		t.Fatalf("rank of d %d", r)
	}
	c := v.Clone().(*SortedSetValue)
	c.Remove("d")
	if v.Rank("d", false) != 0 || c.Rank("d", false) != -1 || c.Card() != 3 {
		t.Fatal("clone shares the index")
	}
}

// Readers holding the read lock may race to build the order of a set filled
// directly; they build it once and all see it.
func TestSortedSetIndexBuiltByConcurrentReaders(t *testing.T) {
	v := &SortedSetValue{Members: map[string]float64{}}
	for i := 0; i < 1000; i++ {
		v.Members["m"+strconv.Itoa(i)] = float64(i % 37)
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v.RLock()
			defer v.RUnlock()
			if got := v.GetAllEntries(); len(got) != 1000 {
				t.Errorf("%d entries", len(got))
			}
		}()
	}
	wg.Wait()
}
