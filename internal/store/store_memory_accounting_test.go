package store

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// Regression: MemoryTracker.Add/Sub had no production callers, so tracked
// usage stayed 0 forever — CanAllocate always succeeded, eviction never fired,
// and ErrMemoryLimit was unreachable. Configured max_memory was silently
// unenforced. Shard Set/Delete/Flush return exact byte deltas; every store
// mutation path must feed them to the tracker.

func TestSetDeleteFeedMemoryTracker(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1<<20, EvictionAllKeysLRU, 70, 85, 128)

	for i := 0; i < 3; i++ {
		if err := s.Set(fmt.Sprintf("acct:%d", i), &StringValue{Data: make([]byte, 100)}, SetOptions{}); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	if s.MemoryTracker().Usage() <= 0 {
		t.Fatalf("expected tracker usage > 0 after sets, got %d", s.MemoryTracker().Usage())
	}
	if s.MemoryTracker().Usage() != s.MemUsage() {
		t.Fatalf("tracker usage %d out of sync with shard accounting %d",
			s.MemoryTracker().Usage(), s.MemUsage())
	}

	for i := 0; i < 3; i++ {
		s.Delete(fmt.Sprintf("acct:%d", i))
	}
	if got := s.MemoryTracker().Usage(); got != 0 {
		t.Fatalf("expected usage 0 after deleting all keys, got %d", got)
	}
}

func TestLazyExpiryFreesTrackedMemory(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1<<20, EvictionAllKeysLRU, 70, 85, 128)

	for _, key := range []string{"exp:get", "exp:exists"} {
		if err := s.Set(key, &StringValue{Data: make([]byte, 100)}, SetOptions{TTL: 5 * time.Millisecond}); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	time.Sleep(20 * time.Millisecond)

	if _, ok := s.Get("exp:get"); ok {
		t.Fatal("expected expired key to be gone via Get")
	}
	if s.Exists("exp:exists") {
		t.Fatal("expected expired key to be gone via Exists")
	}
	if got := s.MemoryTracker().Usage(); got != 0 {
		t.Fatalf("expected lazy expiry to free tracked memory, got %d", got)
	}
}

func TestDeleteBatchFreesTrackedMemory(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1<<20, EvictionAllKeysLRU, 70, 85, 128)

	keys := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("batch:%d", i)
		keys = append(keys, key)
		if err := s.Set(key, &StringValue{Data: make([]byte, 100)}, SetOptions{}); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	if n := s.DeleteBatch(keys); n != 3 {
		t.Fatalf("DeleteBatch removed %d keys, want 3", n)
	}
	if got := s.MemoryTracker().Usage(); got != 0 {
		t.Fatalf("expected DeleteBatch to free tracked memory, got %d", got)
	}
}

func TestFlushFreesTrackedMemory(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1<<20, EvictionAllKeysLRU, 70, 85, 128)

	for i := 0; i < 3; i++ {
		if err := s.Set(fmt.Sprintf("flush:%d", i), &StringValue{Data: make([]byte, 100)}, SetOptions{}); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	s.Flush()
	if got := s.MemoryTracker().Usage(); got != 0 {
		t.Fatalf("expected Flush to free tracked memory, got %d", got)
	}
}

// End-to-end contract: under allkeys-lru every write is accepted and the cap
// bounds live memory via evictions.
func TestEvictionEnforcesMaxMemoryAllKeysLRU(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(32*1024, EvictionAllKeysLRU, 70, 85, 128)

	evictionsBefore := GlobalMetrics.TotalEvictions.Load()
	for i := 0; i < 20; i++ {
		if err := s.Set(fmt.Sprintf("big:%d", i), &StringValue{Data: make([]byte, 2048)}, SetOptions{}); err != nil {
			t.Fatalf("SET %d under allkeys-lru must succeed: %v", i, err)
		}
	}
	if got := GlobalMetrics.TotalEvictions.Load() - evictionsBefore; got < 1 {
		t.Fatalf("expected evictions under pressure, got %d", got)
	}
	if got := s.MemoryTracker().Usage(); got > 32*1024 {
		t.Fatalf("usage %d exceeds configured cap %d", got, 32*1024)
	}
}

// Boundary: noeviction must reach ErrMemoryLimit (previously unreachable
// because usage was never tracked).
func TestNoEvictionPolicyReturnsErrMemoryLimit(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(4*1024, EvictionNoEviction, 70, 85, 128)

	if err := s.Set("oom:1", &StringValue{Data: make([]byte, 2048)}, SetOptions{}); err != nil {
		t.Fatalf("first Set must succeed: %v", err)
	}
	err := s.Set("oom:2", &StringValue{Data: make([]byte, 2048)}, SetOptions{})
	if !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("expected ErrMemoryLimit under noeviction, got %v", err)
	}
	if _, exists := s.Get("oom:2"); exists {
		t.Fatal("rejected write must not be stored")
	}
}
