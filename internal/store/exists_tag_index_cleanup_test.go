package store

import (
	"testing"
	"time"
)

// seedExpiredTagged sets key with a short TTL and attaches a tag, then waits for
// the TTL to lapse. Active expiry is deliberately NOT started, so the expired
// entry is deterministically still in the shard — the same state Exists finds in
// production between a TTL lapsing and the expiry wheel reaching the key's slot.
func seedExpiredTagged(t *testing.T, s *Store, key, tag string) {
	t.Helper()
	if err := s.Set(key, &StringValue{Data: []byte("v")}, SetOptions{
		TTL:  20 * time.Millisecond,
		Tags: []string{tag},
	}); err != nil {
		t.Fatalf("seed %q: %v", key, err)
	}
	time.Sleep(80 * time.Millisecond)
}

// Store.Exists reaps an expired key from the shard. Every other removal path in
// Store also drops the key's tag-index mappings (Get's lazy expiry, Delete,
// DeleteIfExpired, DeleteBatch); Exists omitted it, so a stale tag -> key
// mapping survived pointing at a key that no longer existed.
func TestStoreExistsExpiredKeyClearsTagIndex(t *testing.T) {
	s := NewStore()
	seedExpiredTagged(t, s, "ekey", "etag")

	if s.Exists("ekey") {
		t.Fatal("setup: the key should have expired")
	}
	if _, ok := s.GetShard("ekey").Get("ekey"); ok {
		t.Fatal("setup: Exists should have reaped the entry from the shard")
	}

	if keys := s.GetTagIndex().GetKeys("etag"); len(keys) != 0 {
		t.Fatalf("tag index still holds %v for a key Exists already reaped; every "+
			"other removal path drops these mappings", keys)
	}
}

// The severe consequence: with a stale mapping left behind, INVALIDATE ... CASCADE
// on that tag deletes a LIVE key that was recreated WITHOUT the tag, because
// Store.Set only reconciles the tags of the entry currently in the shard (by then
// the new, untagged entry).
func TestStaleTagIndexDoesNotDeleteRecreatedKey(t *testing.T) {
	s := NewStore()
	seedExpiredTagged(t, s, "ekey", "etag")

	if s.Exists("ekey") {
		t.Fatal("setup: the key should have expired")
	}

	// Recreate the key with no tags at all.
	if err := s.Set("ekey", &StringValue{Data: []byte("v2")}, SetOptions{}); err != nil {
		t.Fatalf("recreate: %v", err)
	}

	deleted := s.DeleteBatch(s.GetTagIndex().GetKeys("etag"))
	if deleted != 0 {
		t.Errorf("INVALIDATE-style cascade deleted %d key(s) that no longer carry the tag; "+
			"a live untagged key must survive", deleted)
	}

	entry, ok := s.Get("ekey")
	if !ok {
		t.Fatal("data loss: the recreated live key was deleted by a stale tag mapping")
	}
	if got := string(entry.Value.(*StringValue).Data); got != "v2" {
		t.Errorf("value = %q, want \"v2\"", got)
	}
}

// Control: EXISTS on a LIVE key must report true and must not disturb the tag
// index — the fix must only add cleanup on the reaping branch.
func TestStoreExistsLiveKeyKeepsTagIndex(t *testing.T) {
	s := NewStore()
	if err := s.Set("live", &StringValue{Data: []byte("v")}, SetOptions{
		Tags: []string{"ltag"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if !s.Exists("live") {
		t.Fatal("EXISTS on a live key must report true")
	}
	if keys := s.GetTagIndex().GetKeys("ltag"); len(keys) != 1 || keys[0] != "live" {
		t.Fatalf("EXISTS on a live key disturbed the tag index: %v", keys)
	}

	// Control: EXISTS on an absent key is a no-op and creates nothing.
	if s.Exists("nope") {
		t.Error("EXISTS on an absent key must report false")
	}
	if keys := s.GetTagIndex().GetKeys("ltag"); len(keys) != 1 {
		t.Errorf("EXISTS on an absent key disturbed the tag index: %v", keys)
	}
}
