package store

import (
	"strconv"
	"testing"
)

func TestHeapRatioScalesTheLimit(t *testing.T) {
	mt := NewMemoryTracker(1000, 70, 85)
	if mt.Limit() != 1000 {
		t.Fatalf("limit %d before any ratio", mt.Limit())
	}
	for i := 0; i < 30; i++ {
		mt.SetHeapRatio(1.25)
	}
	if got := mt.Limit(); got < 795 || got > 800 {
		t.Fatalf("limit %d with the heap at 1.25x usage, want about 800", got)
	}
	if mt.Max() != 1000 {
		t.Fatalf("max %d, want the configured 1000", mt.Max())
	}
	mt.Add(900)
	if mt.CanAllocate(1) {
		t.Fatal("usage 900 allowed to grow past a limit scaled to 800")
	}
	mt.Sub(900)
	for i := 0; i < 30; i++ {
		mt.SetHeapRatio(10)
	}
	if got := mt.Limit(); got < 500 || got > 505 {
		t.Fatalf("limit %d, want it held at half of maxmemory", got)
	}
	for i := 0; i < 30; i++ {
		mt.SetHeapRatio(0.5)
	}
	if got := mt.Limit(); got < 995 {
		t.Fatalf("limit %d, want maxmemory again once the heap is under usage", got)
	}
	mt.Add(900)
	if !mt.CanAllocate(1) {
		t.Fatal("usage 900 refused under the full limit of 1000")
	}
}

func TestVisitShardsReachesEveryShardOnce(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(0, EvictionAllKeysLRU, 70, 85, 5)
	for round := 0; round < 50; round++ {
		seen := make(map[*Shard]int)
		s.evictor.visitShards(func(sh *Shard) bool {
			seen[sh]++
			return true
		})
		if len(seen) != NumShards {
			t.Fatalf("round %d: visited %d shards, want %d", round, len(seen), NumShards)
		}
	}
}

func TestSamplingDoesNotAllocatePerShard(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(0, EvictionAllKeysLRU, 70, 85, 5)
	for i := 0; i < 2000; i++ {
		s.Set("k"+strconv.Itoa(i), &StringValue{Data: []byte("v")}, SetOptions{})
	}
	// The candidates themselves are the one allocation.
	if n := testing.AllocsPerRun(200, func() { s.evictor.selectLRU() }); n > 1 {
		t.Fatalf("selectLRU allocates %v times per call", n)
	}
	if n := testing.AllocsPerRun(200, func() { s.evictor.selectRandom() }); n > 1 {
		t.Fatalf("selectRandom allocates %v times per call", n)
	}
}

func TestTagIndexCountsTags(t *testing.T) {
	ti := NewTagIndex()
	ti.AddTags("a", []string{"x", "y"})
	ti.AddTags("b", []string{"x"})
	if n := ti.tags.Load(); n != 2 {
		t.Fatalf("tags %d, want 2", n)
	}
	ti.RemoveTags("a", []string{"y"})
	if n := ti.tags.Load(); n != 1 {
		t.Fatalf("tags %d after y emptied, want 1", n)
	}
	ti.RemoveTagsBatchAllShards([]string{"a"})
	if got := ti.GetKeys("x"); len(got) != 1 || got[0] != "b" {
		t.Fatalf("x holds %v, want [b]", got)
	}
	ti.Invalidate("x")
	if n := ti.tags.Load(); n != 0 {
		t.Fatalf("tags %d after invalidating x, want 0", n)
	}
	ti.Invalidate("missing")
	if n := ti.tags.Load(); n != 0 {
		t.Fatalf("tags %d after invalidating a missing tag", n)
	}
}
