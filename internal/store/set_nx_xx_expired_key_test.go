package store

import (
	"testing"
	"time"
)

// storeSetWithShortTTL seeds key with a TTL that has already lapsed but is not
// yet reaped by active expiry. Active expiry is deliberately NOT started, so
// the expired entry is guaranteed to still sit in the shard exactly as it does
// in production during the window between a TTL lapsing and the wheel reaching
// the key's slot. getExpiredKey reads the expiry through the public Store.Get
// path (Store.Get reaps and treats an expired key as absent).
func storeSetWithShortTTL(t *testing.T, s *Store, key string) {
	t.Helper()
	if err := s.Set(key, &StringValue{Data: []byte("v")}, SetOptions{TTL: 20 * time.Millisecond}); err != nil {
		t.Fatalf("seed %q: %v", key, err)
	}
	time.Sleep(80 * time.Millisecond)
}

// NX must answer "does this key exist?" with the same rule as Store.Get: an
// expired key does not exist. The NX check used a raw shard lookup that ignored
// expiry, so SETNX / SET ... NX refused a write the caller was entitled to make
// and the value was silently dropped.
func TestSetNXTreatsExpiredKeyAsAbsent(t *testing.T) {
	s := NewStore()
	storeSetWithShortTTL(t, s, "nxkey")

	// Control: the key really is logically expired per the public read path.
	if _, ok := s.Get("nxkey"); ok {
		t.Fatal("setup: the key should read as expired/absent")
	}

	err := s.Set("nxkey", &StringValue{Data: []byte("v2")}, SetOptions{NX: true})
	if err != nil {
		t.Fatalf("SETNX on an expired key failed with %v; an expired key does not exist, "+
			"so NX must allow the write", err)
	}
	entry, ok := s.Get("nxkey")
	if !ok {
		t.Fatal("the value was not stored")
	}
	if got := string(entry.Value.(*StringValue).Data); got != "v2" {
		t.Fatalf("stored value = %q, want \"v2\"", got)
	}
}

// The mirror case: XX must not revive a key that no longer exists.
func TestSetXXDoesNotReviveExpiredKey(t *testing.T) {
	s := NewStore()
	storeSetWithShortTTL(t, s, "xxkey")

	if _, ok := s.Get("xxkey"); ok {
		t.Fatal("setup: the key should read as expired/absent")
	}

	err := s.Set("xxkey", &StringValue{Data: []byte("v2")}, SetOptions{XX: true})
	if err != ErrKeyNotFound {
		t.Fatalf("SET XX on an expired key = %v, want ErrKeyNotFound; XX must not "+
			"resurrect a key that no longer exists", err)
	}
	// The same test instance must not have been overwritten with the new value.
	if entry, ok := s.GetShard("xxkey").Get("xxkey"); ok {
		if got := string(entry.Value.(*StringValue).Data); got != "v2" {
			t.Fatalf("the expired entry was overwritten with %q", got)
		}
	}
}

// Control: the non-expired behaviours must be unchanged. SETNX on a live key
// still refuses, SET XX on a live key still succeeds.
func TestConditionalSetUnaffectedForLiveKeys(t *testing.T) {
	s := NewStore()

	if err := s.Set("live", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.Set("live", &StringValue{Data: []byte("v2")}, SetOptions{NX: true}); err != ErrKeyExists {
		t.Errorf("SETNX on a live key = %v, want ErrKeyExists", err)
	}
	if err := s.Set("live", &StringValue{Data: []byte("v2")}, SetOptions{XX: true}); err != nil {
		t.Errorf("SET XX on a live key = %v, want nil", err)
	}
	if err := s.Set("absent", &StringValue{Data: []byte("v")}, SetOptions{XX: true}); err != ErrKeyNotFound {
		t.Errorf("SET XX on an absent key = %v, want ErrKeyNotFound", err)
	}
	if err := s.Set("absent", &StringValue{Data: []byte("v")}, SetOptions{NX: true}); err != nil {
		t.Errorf("SETNX on an absent key = %v, want nil", err)
	}
}

// A key whose TTL has not yet lapsed must still block NX, so the fix did not
// simply treat every existing key as absent.
func TestSetNXStillRefusedForUnexpiredKey(t *testing.T) {
	s := NewStore()
	if err := s.Set("fresh", &StringValue{Data: []byte("v")}, SetOptions{TTL: time.Minute}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.Set("fresh", &StringValue{Data: []byte("v2")}, SetOptions{NX: true}); err != ErrKeyExists {
		t.Errorf("SETNX on an unexpired key = %v, want ErrKeyExists", err)
	}
}
