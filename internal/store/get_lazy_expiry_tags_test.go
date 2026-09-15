package store

import (
	"testing"
)

// A tagged key that expires and is then read (the lazy-expiry path in
// Store.Get) must be removed from the tag index exactly like the
// Delete and DeleteIfExpired paths do — otherwise TAGKEYS and INVALIDATE
// report phantom keys forever.
func TestGetLazyExpiryCleansTagIndex(t *testing.T) {
	s := NewStore()
	if err := s.Set("k", &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"t1"}}); err != nil {
		t.Fatal(err)
	}

	// Expire the key, then read it: Store.Get's inline lazy-expiry path runs.
	if !s.SetExpiresAt("k", 1) {
		t.Fatal("could not expire key")
	}
	if _, exists := s.Get("k"); exists {
		t.Fatal("an expired key must not be returned by Get")
	}

	// The tag index must be clean after the lazy expiry.
	if leaked := s.GetTagIndex().Invalidate("t1"); len(leaked) != 0 {
		t.Fatalf("tag index leaked %d phantom keys after a lazy-expired read: %v", len(leaked), leaked)
	}
}

// The same cleanup contract on the explicit DeleteIfExpired path (already
// implemented) — locked here so both expiry paths stay symmetric.
func TestDeleteIfExpiredCleansTagIndex(t *testing.T) {
	s := NewStore()
	if err := s.Set("k2", &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"t2"}}); err != nil {
		t.Fatal(err)
	}

	if !s.SetExpiresAt("k2", 1) {
		t.Fatal("could not expire key")
	}
	if !s.DeleteIfExpired("k2") {
		t.Fatal("DeleteIfExpired must report the expired key as removed")
	}

	if leaked := s.GetTagIndex().Invalidate("t2"); len(leaked) != 0 {
		t.Fatalf("tag index leaked %d phantom keys after DeleteIfExpired: %v", len(leaked), leaked)
	}
}
