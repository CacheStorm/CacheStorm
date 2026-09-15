package store

import (
	"fmt"
	"testing"
	"time"
)

// Regression: Store.Get and the eviction path never recorded hit/miss/
// expiration/eviction counters, so /metrics hit/miss/evicted/expired (and the
// INFO hit rate) stayed 0 forever despite live traffic.

func TestGetRecordsHitMissExpiration(t *testing.T) {
	s := NewStore()

	hitsBefore := GlobalMetrics.TotalHits.Load()
	missesBefore := GlobalMetrics.TotalMisses.Load()
	expirationsBefore := GlobalMetrics.TotalExpirations.Load()

	// Miss: key never existed.
	if _, ok := s.Get("metrics:missing"); ok {
		t.Fatal("expected miss for non-existent key")
	}
	if got := GlobalMetrics.TotalMisses.Load() - missesBefore; got != 1 {
		t.Errorf("expected 1 miss recorded, got %d", got)
	}

	// Hit: live key.
	if err := s.Set("metrics:hit", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, ok := s.Get("metrics:hit"); !ok {
		t.Fatal("expected hit for live key")
	}
	if got := GlobalMetrics.TotalHits.Load() - hitsBefore; got != 1 {
		t.Errorf("expected 1 hit recorded, got %d", got)
	}

	// Expired lookup: counts as both a miss and an expiration.
	if err := s.Set("metrics:ttl", &StringValue{Data: []byte("v")}, SetOptions{TTL: 5 * time.Millisecond}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, ok := s.Get("metrics:ttl"); ok {
		t.Fatal("expected miss for expired key")
	}
	if got := GlobalMetrics.TotalMisses.Load() - missesBefore; got != 2 {
		t.Errorf("expected 2 misses recorded (missing + expired), got %d", got)
	}
	if got := GlobalMetrics.TotalExpirations.Load() - expirationsBefore; got != 1 {
		t.Errorf("expected 1 expiration recorded, got %d", got)
	}
}

func TestEvictionRecordsCounter(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionAllKeysLRU, 70, 85, 128)

	// Spread keys across shards so victim sampling finds candidates
	// (sampleSize 128 of 256 shards, 64 keys -> a victim is virtually certain).
	for i := 0; i < 64; i++ {
		if err := s.Set(fmt.Sprintf("evkey%d", i), &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}

	evictionsBefore := GlobalMetrics.TotalEvictions.Load()
	if n := s.Evictor().ForceEvict(2); n != 2 {
		t.Fatalf("ForceEvict evicted %d keys, want 2", n)
	}
	if got := GlobalMetrics.TotalEvictions.Load() - evictionsBefore; got != 2 {
		t.Errorf("expected 2 evictions recorded, got %d", got)
	}
}
