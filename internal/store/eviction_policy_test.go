package store

import (
	"testing"
	"time"
)

func TestParseEvictionPolicyRoundTrip(t *testing.T) {
	names := []string{"noeviction", "allkeys-lru", "allkeys-lfu", "allkeys-random", "volatile-lru", "volatile-lfu", "volatile-random", "volatile-ttl"}
	for _, name := range names {
		p, ok := ParseEvictionPolicy(name)
		if !ok || p.String() != name {
			t.Errorf("ParseEvictionPolicy(%q) = %v, %v", name, p, ok)
		}
	}
	if _, ok := ParseEvictionPolicy("allkeys-everything"); ok {
		t.Error("an unknown policy name was accepted")
	}
}

func fillForEviction(t *testing.T, s *Store) {
	t.Helper()
	for i := 0; i < 50; i++ {
		key := "persistent:" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		if err := s.Set(key, &StringValue{Data: make([]byte, 100)}, SetOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 50; i++ {
		key := "volatile:" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		ttl := time.Duration(i+1) * time.Hour
		if err := s.Set(key, &StringValue{Data: make([]byte, 100)}, SetOptions{TTL: ttl}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVolatilePoliciesOnlyEvictKeysWithTTL(t *testing.T) {
	for _, policy := range []EvictionPolicy{EvictionVolatileLRU, EvictionVolatileLFU, EvictionVolatileRandom, EvictionVolatileTTL} {
		t.Run(policy.String(), func(t *testing.T) {
			s := NewStore()
			s.ConfigureMemory(0, policy, 70, 85, 10)
			fillForEviction(t, s)
			for i := 0; i < 20; i++ {
				if !s.evictor.evictOne() {
					t.Fatalf("eviction %d found no victim", i)
				}
			}
			persistent := 0
			for i := 0; i < NumShards; i++ {
				sh := s.shards[i]
				sh.mu.RLock()
				for key := range sh.data {
					if len(key) > 10 && key[:11] == "persistent:" {
						persistent++
					}
				}
				sh.mu.RUnlock()
			}
			if persistent != 50 {
				t.Fatalf("%s evicted %d keys without a TTL", policy, 50-persistent)
			}
		})
	}
}

func TestVolatileTTLPrefersSoonestExpiry(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(0, EvictionVolatileTTL, 70, 85, 64)
	fillForEviction(t, s)
	before := int64(0)
	for i := 0; i < 10; i++ {
		key := s.evictor.selectVictim()
		e, ok := s.shards[s.shardIndex(key)].Get(key)
		if !ok {
			t.Fatalf("victim %q is missing", key)
		}
		before += e.ExpiresAt.Load()
	}
	avgVictim := before / 10
	var sum, n int64
	for i := 0; i < NumShards; i++ {
		sh := s.shards[i]
		sh.mu.RLock()
		for _, e := range sh.data {
			if at := e.ExpiresAt.Load(); at > 0 {
				sum += at
				n++
			}
		}
		sh.mu.RUnlock()
	}
	if avgVictim >= sum/n {
		t.Fatalf("volatile-ttl victims expire on average at %d, not sooner than the keyspace average %d", avgVictim, sum/n)
	}
}

func TestSetMemoryLimitAppliesToSiblingDatabases(t *testing.T) {
	dbs := NewDatabases(NewStore(), 4)
	dbs.DB(0).ConfigureMemory(0, EvictionNoEviction, 70, 85, 5)
	other := dbs.DB(2)
	if !dbs.DB(0).SetMemoryLimit(1<<20, EvictionAllKeysLFU, 7) {
		t.Fatal("SetMemoryLimit refused a configured store")
	}
	limit, policy := other.MemoryLimit()
	if limit != 1<<20 || policy != EvictionAllKeysLFU || other.evictor.SampleSize() != 7 {
		t.Fatalf("sibling database sees limit=%d policy=%s samples=%d", limit, policy, other.evictor.SampleSize())
	}
	if !other.UsesLFU() {
		t.Fatal("UsesLFU did not follow the runtime policy change")
	}
}
