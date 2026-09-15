package store

import (
	"testing"
	"time"
)

// Regression: Store.DeleteBatch (the TAGINVALIDATE path) removed keys from
// shards and the versions map but never from the tag index, unlike
// Store.Delete which calls tagIndex.RemoveKey. A key tagged [t1, t2] that was
// invalidated via t1 stayed in t2's mapping forever: TAGKEYS t2 returned a
// deleted key and every SET-with-tags + INVALIDATE cycle leaked the index.

func TestDeleteBatchCleansTagIndex(t *testing.T) {
	s := NewStore()

	// Two keys sharing t1, each with a second tag of its own.
	if err := s.Set("k1", &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"t1", "t2"}}); err != nil {
		t.Fatalf("Set k1: %v", err)
	}
	if err := s.Set("k2", &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"t1", "t3"}}); err != nil {
		t.Fatalf("Set k2: %v", err)
	}

	// Sanity: the index maps both keys under t1.
	for _, k := range []string{"k1", "k2"} {
		if !contains(s.tagIndex.GetKeys("t1"), k) {
			t.Fatalf("setup: %q missing from tag t1", k)
		}
	}

	// The TAGINVALIDATE sequence: index lookup, then batch delete.
	keys := s.tagIndex.Invalidate("t1")
	if len(keys) != 2 {
		t.Fatalf("Invalidate(t1) returned %d keys, want 2", len(keys))
	}
	if n := s.DeleteBatch(keys); n != 2 {
		t.Fatalf("DeleteBatch deleted %d keys, want 2", n)
	}

	for _, k := range []string{"k1", "k2"} {
		if s.Exists(k) {
			t.Fatalf("key %q must be deleted", k)
		}
		if contains(s.tagIndex.GetKeys("t2"), k) {
			t.Fatalf("key %q was deleted but is still mapped under tag t2 (tag-index leak)", k)
		}
		if contains(s.tagIndex.GetKeys("t3"), k) {
			t.Fatalf("key %q was deleted but is still mapped under tag t3 (tag-index leak)", k)
		}
	}
}

// Deleting via Store.Delete must also clear the version entry (the lazy
// expiry path already does; parity check for the churn-leak contract).
func TestDeleteCleansVersionEntry(t *testing.T) {
	s := NewStore()

	if err := s.Set("vk", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	s.Delete("vk")

	s.versionMu.Lock()
	_, leaked := s.versions["vk"]
	s.versionMu.Unlock()
	if leaked {
		t.Fatal("version entry for a deleted key leaked (unbounded map growth under SET/DEL churn)")
	}
}

// SET-only churn must not grow the versions map without bound for expired
// keys: the lazy-expiry path cleans up (contract check on the existing path).
func TestLazyExpiryCleansVersionEntry(t *testing.T) {
	s := NewStore()

	if err := s.Set("ek", &StringValue{Data: []byte("v")}, SetOptions{TTL: time.Millisecond}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, found := s.Get("ek"); found {
		t.Fatal("expired key must not be returned")
	}

	s.versionMu.Lock()
	_, leaked := s.versions["ek"]
	s.versionMu.Unlock()
	if leaked {
		t.Fatal("version entry for a lazily-expired key leaked")
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
