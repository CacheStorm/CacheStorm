package store

import (
	"fmt"
	"testing"
	"time"
)

// Regression: the timing wheel was never constructed or started in production,
// so keys with a TTL that were never read again were never removed — they
// persisted forever (and, with live memory accounting, kept counting toward
// configured max_memory until a read touched them).

func TestActiveExpirationRemovesUnreadKeys(t *testing.T) {
	s := NewStore()
	s.StartExpiry()
	defer s.StopExpiry()

	if err := s.Set("ctl:alive", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("Set control: %v", err)
	}
	expirationsBefore := GlobalMetrics.TotalExpirations.Load()
	for i := 0; i < 3; i++ {
		if err := s.Set(fmt.Sprintf("exp:%d", i), &StringValue{Data: []byte("v")}, SetOptions{TTL: 50 * time.Millisecond}); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}

	time.Sleep(400 * time.Millisecond) // TTL 50ms, wheel sweeps every 100ms

	if got := s.KeyCount(); got != 1 {
		t.Fatalf("expected only the control key to remain, got %d keys", got)
	}
	if !s.Exists("ctl:alive") {
		t.Fatal("control key must survive active expiration")
	}
	if got := GlobalMetrics.TotalExpirations.Load() - expirationsBefore; got < 3 {
		t.Fatalf("expected >=3 expirations recorded, got %d", got)
	}
}

// A stale schedule (key rescheduled after it was queued) must never delete a
// live key.
func TestActiveExpirySkipsRescheduledKey(t *testing.T) {
	s := NewStore()
	s.StartExpiry()
	defer s.StopExpiry()

	if err := s.Set("ttl:key", &StringValue{Data: []byte("v")}, SetOptions{TTL: 150 * time.Millisecond}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !s.SetTTL("ttl:key", 10*time.Second) {
		t.Fatal("SetTTL must succeed on a live key")
	}
	time.Sleep(400 * time.Millisecond) // the original 150ms schedule has fired

	if !s.Exists("ttl:key") {
		t.Fatal("stale schedule must not delete a rescheduled key")
	}
	if got := s.GetTTL("ttl:key"); got <= 4*time.Second {
		t.Fatalf("rescheduled key should keep its new TTL, got %v", got)
	}
}

// PERSIST clears the expiry; a stale schedule must not delete the key.
func TestActiveExpirySkipsPersistedKey(t *testing.T) {
	s := NewStore()
	s.StartExpiry()
	defer s.StopExpiry()

	if err := s.Set("ttl:persist", &StringValue{Data: []byte("v")}, SetOptions{TTL: 150 * time.Millisecond}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !s.Persist("ttl:persist") {
		t.Fatal("Persist must succeed on a live key")
	}
	time.Sleep(400 * time.Millisecond)

	if !s.Exists("ttl:persist") {
		t.Fatal("stale schedule must not delete a PERSISTed key")
	}
	// Store.GetTTL's no-expiry sentinel is -1 second (Entry.TTL's is -1ns).
	if got := s.GetTTL("ttl:persist"); got != -1*time.Second {
		t.Fatalf("persisted key should have no expiry (-1s), got %v", got)
	}
}
