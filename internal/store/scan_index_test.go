package store

import (
	"math/rand"
	"strconv"
	"testing"
)

// TestScanShardReturnsStableKeysOnce scans one crowded shard page by page
// while keys are added and deleted between pages: every key present for the
// whole scan comes back exactly once and nothing comes back twice.
func TestScanShardReturnsStableKeysOnce(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	s := NewStore()
	const shard = 7
	gen := 0
	newKey := func() string {
		for {
			gen++
			k := "k" + strconv.Itoa(gen)
			if s.shardIndex(k) == shard {
				return k
			}
		}
	}
	live := map[string]bool{}
	for range 3000 {
		k := newKey()
		if err := s.Set(k, &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
			t.Fatal(err)
		}
		live[k] = true
	}
	for round := range 30 {
		stable := map[string]bool{}
		for k := range live {
			stable[k] = true
		}
		seen := map[string]int{}
		pos := uint64(0)
		adds := r.Intn(8)
		for {
			keys, _, np := s.ScanShard(shard, pos, 1+r.Intn(40), false)
			for _, k := range keys {
				seen[k]++
			}
			for range r.Intn(adds + 1) {
				k := newKey()
				_ = s.Set(k, &StringValue{Data: []byte("v")}, SetOptions{})
				live[k] = true
			}
			for k := range live {
				if r.Intn(3) == 0 {
					s.Delete(k)
					delete(live, k)
					delete(stable, k)
				}
				break
			}
			if np == 0 {
				break
			}
			pos = np
		}
		for k := range stable {
			if seen[k] != 1 {
				t.Fatalf("round %d: stable key %s returned %d times", round, k, seen[k])
			}
		}
		for k, c := range seen {
			if c > 1 {
				t.Fatalf("round %d: %s returned %d times", round, k, c)
			}
		}
	}
}

// TestScanShardConcurrentWrites scans while another goroutine adds and
// deletes keys, for the race detector: the index is sorted outside the
// shard lock.
func TestScanShardConcurrentWrites(t *testing.T) {
	s := NewStore()
	for i := range 20000 {
		_ = s.Set("k"+strconv.Itoa(i), &StringValue{Data: []byte("v")}, SetOptions{})
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 20000 {
			k := "n" + strconv.Itoa(i)
			_ = s.Set(k, &StringValue{Data: []byte("v")}, SetOptions{})
			if i%3 == 0 {
				s.Delete(k)
			}
		}
	}()
	for shard := 0; shard < NumShards; shard++ {
		pos := uint64(0)
		for {
			_, _, np := s.ScanShard(shard, pos, 5, true)
			if np == 0 {
				break
			}
			pos = np
		}
	}
	<-done
}
