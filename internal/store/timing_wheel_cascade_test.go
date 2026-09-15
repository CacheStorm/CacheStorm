package store

import (
	"testing"
	"time"
)

// Regression: the timing wheel's higher levels advanced one slot per parent
// revolution instead of sweeping the parent's coverage window. cascade(1)
// fired hourly but advanced level 1 (1-minute slots) by ONE slot, so a 2h TTL
// was visited only after ~120 hours (60x late); level 2 advanced only when
// level 1 wrapped (~60 days), so a 2-day TTL fired after ~8 years. Unread
// expired keys lingered for months counting toward max_memory.
//
// The tests drive the store's own wheel (s.expiry — the instance Set
// schedules into) with synthetic clock values. To keep the wheel clock and
// the store's real-time IsExpired check consistent, the entries carry a
// real-past expiry and the wheel schedule is injected directly (the pattern
// the store_full_coverage_test.go wheel tests use).

func TestCascadeSweepsLevel1Coverage(t *testing.T) {
	s := NewStore()

	if err := s.Set("ttl:key", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !s.SetExpiresAt("ttl:key", time.Now().Add(-time.Second).UnixNano()) {
		t.Fatal("SetExpiresAt must succeed on a live key")
	}

	// Schedule the key 120 level-1 slots (2h) ahead of the current pointer.
	l1 := s.expiry.levels[1]
	l1.mu.Lock()
	l1.slots[120].keys["ttl:key"] = time.Now().Add(2 * time.Hour).UnixNano()
	l1.mu.Unlock()

	// cascadeLevel sweeps the current slot then advances, so slot 120 is
	// swept during the third hourly batch.
	now := time.Now().UnixNano()
	s.expiry.cascadeHour(now + int64(time.Hour))
	s.expiry.cascadeHour(now + 2*int64(time.Hour))
	s.expiry.cascadeHour(now + 3*int64(time.Hour))

	if s.Exists("ttl:key") {
		t.Fatal("expired key must be removed once its level-1 slot is swept (within three hourly wraps); the old cascade advanced one slot per wrap")
	}
}

func TestCascadeSweepsLevel2Coverage(t *testing.T) {
	s := NewStore()

	if err := s.Set("ttl:day", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !s.SetExpiresAt("ttl:day", time.Now().Add(-time.Second).UnixNano()) {
		t.Fatal("SetExpiresAt must succeed on a live key")
	}

	// Schedule the key 25 level-2 slots (25h) ahead of the current pointer.
	l2 := s.expiry.levels[2]
	l2.mu.Lock()
	l2.slots[25].keys["ttl:day"] = time.Now().Add(25 * time.Hour).UnixNano()
	l2.mu.Unlock()

	// Level 2 advances one 1-hour slot per hourly wrap; slot 25 is swept by
	// wrap 25. Pre-fix, level 2 only advanced when level 1 wrapped.
	now := time.Now().UnixNano()
	for i := 1; i <= 25; i++ {
		s.expiry.cascadeHour(now + int64(i)*int64(time.Hour))
	}

	if s.Exists("ttl:day") {
		t.Fatal("expired key must be removed once its level-2 slot is swept (wrap 25); the old cascade advanced level 2 only when level 1 wrapped")
	}
}

// A key without a TTL is never scheduled and must survive every sweep.
func TestCascadeLeavesControlKeyAlive(t *testing.T) {
	s := NewStore()

	if err := s.Set("ctl", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("Set control: %v", err)
	}

	now := time.Now().UnixNano()
	for i := 1; i <= 30; i++ {
		s.expiry.cascadeHour(now + int64(i)*int64(time.Hour))
	}

	if !s.Exists("ctl") {
		t.Fatal("control key without TTL must survive the cascade")
	}
}
