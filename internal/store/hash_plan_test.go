package store

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
)

func TestHashPlanPreservesIdentityAndMetadata(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(2048, EvictionNoEviction, 70, 90, 5)
	hash := &HashValue{Fields: map[string][]byte{"a": make([]byte, 600), "b": make([]byte, 600)}}
	if err := s.Set("h", hash, SetOptions{Tags: []string{"tag"}, ExpireAtMS: 4102444800000}); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Get("h")
	usage := s.MemUsage()
	err := s.ApplyHashFields("h", func(h *HashValue) (map[string][]byte, error) {
		return map[string][]byte{"a": make([]byte, 1200), "b": nil}, nil
	})
	after, _ := s.Get("h")
	if err != nil || after != before || after.Value != hash || s.MemUsage() != usage || after.ExpiresAt.Load() != 4102444800000 || len(after.TagList()) != 1 {
		t.Fatalf("replacement changed identity/metadata/budget: %v", err)
	}
	requireBatchAccountingX(t, s)
}

func TestHashPlanUnlocksOnEveryExit(t *testing.T) {
	for _, mode := range []string{"noop", "error", "oom", "wrongtype", "emptykey", "longkey"} {
		t.Run(mode, func(t *testing.T) {
			s := NewStore()
			s.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
			hash := &HashValue{Fields: map[string][]byte{"f": []byte("kept")}}
			key := "h"
			if mode == "wrongtype" {
				if err := s.Set(key, &StringValue{Data: []byte("kept")}, SetOptions{}); err != nil {
					t.Fatal(err)
				}
			} else if err := s.Set(key, hash, SetOptions{}); err != nil {
				t.Fatal(err)
			}
			if mode == "emptykey" {
				key = ""
			}
			if mode == "longkey" {
				key = string(make([]byte, MaxKeySize+1))
			}
			err := s.ApplyHashFields(key, func(h *HashValue) (map[string][]byte, error) {
				if mode == "noop" {
					return nil, nil
				}
				if mode == "error" {
					return nil, ErrKeyNotFound
				}
				n := 1
				if mode == "oom" {
					n = 2048
				}
				return map[string][]byte{"g": make([]byte, n)}, nil
			})
			if mode == "noop" && err != nil || mode != "noop" && err == nil {
				t.Fatalf("mode %s: %v", mode, err)
			}
			if !hash.mu.TryLock() {
				t.Fatal("hash lock leaked")
			}
			hash.mu.Unlock()
			if _, ok := s.Get("h"); !ok {
				t.Fatal("source changed")
			}
			requireBatchAccountingX(t, s)
		})
	}
}

func TestHashPlanHoldsValueLockThroughAdmission(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
	hash := &HashValue{Fields: map[string][]byte{"f": []byte("old")}}
	if err := s.Set("h", hash, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- s.ApplyHashFields("h", func(h *HashValue) (map[string][]byte, error) {
			close(entered)
			<-release
			return map[string][]byte{"f": append(append([]byte(nil), h.Fields["f"]...), []byte(" new")...)}, nil
		})
	}()
	<-entered
	if hash.mu.TryLock() {
		hash.mu.Unlock()
		t.Error("other writer entered during preparation")
	}
	shard := s.shards[s.shardIndex("h")]
	if shard.mu.TryRLock() {
		shard.mu.RUnlock()
		t.Error("entry could be replaced during preparation")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	entry, _ := s.Get("h")
	if entry.Value != hash || string(hash.Fields["f"]) != "old new" {
		t.Fatal("identity/data changed")
	}
	if !hash.mu.TryLock() {
		t.Fatal("lock retained after commit")
	}
	hash.mu.Unlock()
	requireBatchAccountingX(t, s)
}

func TestHashPlanReservesSharedBudgetAcrossDatabases(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(2048, EvictionNoEviction, 70, 90, 5)
	dbs := NewDatabases(s, 2)
	start := make(chan struct{})
	results := make(chan error, 16)
	var applied atomic.Int64
	for i := 0; i < 16; i++ {
		go func(i int) {
			<-start
			results <- dbs.DB(i%2).ApplyHashFields(fmt.Sprintf("h%d", i), func(*HashValue) (map[string][]byte, error) {
				applied.Add(1)
				return map[string][]byte{"f": make([]byte, 600)}, nil
			})
		}(i)
	}
	close(start)
	wins := 0
	for i := 0; i < 16; i++ {
		if err := <-results; err == nil {
			wins++
		} else if !errors.Is(err, ErrMemoryLimit) {
			t.Fatal(err)
		}
	}
	if wins != 2 || applied.Load() != 16 {
		t.Fatalf("wins=%d preparations=%d", wins, applied.Load())
	}
	total := dbs.DB(0).MemUsage() + dbs.DB(1).MemUsage()
	if total != s.MemoryTracker().Usage() || !s.MemoryTracker().withinLimit(total) {
		t.Fatalf("total=%d tracker=%d", total, s.MemoryTracker().Usage())
	}
}

func TestHashPlanEvictionReentersAndReselects(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionAllKeysLRU, 70, 90, 128)
	if err := s.Set("h", &HashValue{Fields: map[string][]byte{"f": []byte("old")}}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("victim", &StringValue{Data: make([]byte, 400)}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	victim, _ := s.Get("victim")
	victim.LastAccess.Store(0)
	callbacks := 0
	s.SetHooks(StoreHooks{OnEvict: func(key string, _ interface{}) {
		callbacks++
		if key != "victim" {
			t.Errorf("victim=%s", key)
		}
		if err := s.ApplyHashFields("h", func(*HashValue) (map[string][]byte, error) { return map[string][]byte{"f": []byte("callback")}, nil }); err != nil {
			t.Error(err)
		}
	}})
	preparations := 0
	err := s.ApplyHashFields("h", func(h *HashValue) (map[string][]byte, error) {
		preparations++
		if preparations > 1 && string(h.Fields["f"]) != "callback" {
			t.Error("stale hash after callback")
		}
		return map[string][]byte{"g": make([]byte, 400)}, nil
	})
	if err != nil || callbacks != 1 || preparations < 2 || s.Exists("victim") {
		t.Fatalf("err=%v callbacks=%d prepare=%d", err, callbacks, preparations)
	}
	requireBatchAccountingX(t, s)
}

func TestApplyHashPairsLastValueWinsAndCountsNewFields(t *testing.T) {
	s := NewStore()
	args := [][]byte{[]byte("f"), []byte("1"), []byte("g"), []byte("2"), []byte("f"), []byte("3")}
	if added, err := s.ApplyHashPairs("h", false, args); err != nil || added != 2 {
		t.Fatalf("added = %d, %v; want 2", added, err)
	}
	var many [][]byte
	for i := 0; i < 20; i++ {
		many = append(many, []byte("k"+string(rune('a'+i%5))), []byte{byte('0' + i%10)})
	}
	if added, err := s.ApplyHashPairs("h", false, many); err != nil || added != 5 {
		t.Fatalf("added with 20 pairs over 5 fields = %d, %v; want 5", added, err)
	}
	e, _ := s.Get("h")
	h := e.Value.(*HashValue)
	h.RLock()
	f, ka := string(h.Fields["f"]), string(h.Fields["ka"])
	h.RUnlock()
	if f != "3" || ka != "5" {
		t.Fatalf("f=%q ka=%q, want the last values 3 and 5", f, ka)
	}
	if added, _ := s.ApplyHashPairs("h", true, [][]byte{[]byte("f"), []byte("x")}); added != 0 {
		t.Fatal("NX overwrote an existing field")
	}
	var want int64
	h.RLock()
	for k, v := range h.Fields {
		want += hashFieldSize(k, v)
	}
	h.RUnlock()
	if got, ok := h.size.load(); !ok || got != want {
		t.Fatalf("tracked size %d, recomputed %d", got, want)
	}
}
