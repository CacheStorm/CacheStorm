package store

import (
	"testing"
	"time"
)

// The store must fire OnEvict when a key is evicted, passing the key and its
// value, so the composition root can bridge the event to plugin consumers.
func TestStoreHooksOnEvictFiresForForcedEviction(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1<<20, EvictionAllKeysLRU, 80, 90, 5)

	evicted := make([]string, 0, 1)
	s.SetHooks(StoreHooks{OnEvict: func(key string, value interface{}) {
		evicted = append(evicted, key)
	}})

	if err := s.Set("hooks:evict", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if n := s.Evictor().ForceEvict(1); n != 1 {
		t.Fatalf("ForceEvict evicted %d keys, want 1", n)
	}
	if len(evicted) != 1 || evicted[0] != "hooks:evict" {
		t.Fatalf("OnEvict got %v, want [hooks:evict]", evicted)
	}
}

// The store must fire OnExpire when a key is lazily removed at read time.
func TestStoreHooksOnExpireFiresOnLazyExpiry(t *testing.T) {
	s := NewStore()

	var expired []string
	s.SetHooks(StoreHooks{OnExpire: func(key string, value interface{}) {
		expired = append(expired, key)
	}})

	if err := s.Set("hooks:expire", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !s.SetExpiresAt("hooks:expire", time.Now().Add(-time.Second).UnixMilli()) {
		t.Fatal("SetExpiresAt reported failure")
	}
	if _, ok := s.Get("hooks:expire"); ok {
		t.Fatal("expected the lazy expiry to remove the expired key")
	}
	if len(expired) != 1 || expired[0] != "hooks:expire" {
		t.Fatalf("OnExpire got %v, want [hooks:expire]", expired)
	}
}

// The store must fire OnTagInvalidate with the tag and the keys that the
// invalidation removed from the tag index.
func TestStoreHooksOnTagInvalidateFires(t *testing.T) {
	s := NewStore()

	var firedTag string
	var firedKeys []string
	s.SetHooks(StoreHooks{OnTagInvalidate: func(tag string, keys []string) {
		firedTag = tag
		firedKeys = keys
	}})

	s.GetTagIndex().AddTags("hooks:tagged", []string{"t1"})
	keys := s.GetTagIndex().Invalidate("t1")
	if len(keys) != 1 || keys[0] != "hooks:tagged" {
		t.Fatalf("Invalidate returned %v, want [hooks:tagged]", keys)
	}
	if firedTag != "t1" || len(firedKeys) != 1 || firedKeys[0] != "hooks:tagged" {
		t.Fatalf("OnTagInvalidate got (%q, %v), want (\"t1\", [hooks:tagged])", firedTag, firedKeys)
	}
}
