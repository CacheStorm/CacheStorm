package store

import (
	"math"
	"testing"
	"time"
)

// Regression: SetTTL converted an absolute expiry instant to int64 NANOSECONDS
// with time.Time.UnixNano(), which is only defined between 1678 and 2262.
//
//	internal/store/store.go (before)
//	  expiresAt := time.Now().Add(ttl).UnixNano()
//
// A TTL longer than the ~236 years of headroom int64 nanoseconds actually have
// wrapped to a NEGATIVE value. Entry.IsExpired compares
// time.Now().UnixNano() > e.ExpiresAt, so a wrapped negative expiry reads as
// "already expired" and the key was silently deleted behind a :1 reply.
//
// This matters because the command layer DELIBERATELY accepts TTLs up to
// maxExpireAtSeconds = 9223372036 (292 years), so a perfectly valid, in-range
// TTL still destroyed the key one layer down.

func TestSetTTLFarFuturePreservesDuration(t *testing.T) {
	s := NewStore()
	if err := s.Set("k", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// 292 years == maxExpireAtSeconds, the largest TTL cmdEXPIRE accepts.
	if !s.SetTTL("k", 292*365*24*time.Hour) {
		t.Fatalf("SetTTL returned false for a far-future TTL")
	}

	if _, ok := s.Get("k"); !ok {
		t.Fatalf("the key was DELETED by a far-future TTL; time.Now().Add(ttl)." +
			"UnixNano() wrapped negative and IsExpired() read it as already expired")
	}
	if got, want := s.TTL("k"), 292*365*24*time.Hour; got < want-time.Second || got > want {
		t.Fatalf("TTL after far-future SetTTL = %v, want approximately %v", got, want)
	}
}

func TestExpiryDurationSaturation(t *testing.T) {
	for _, test := range []struct {
		ms   int64
		want time.Duration
	}{
		{math.MaxInt64, time.Duration(math.MaxInt64)},
		{math.MinInt64, time.Duration(math.MinInt64)},
		{0, 0},
		{90000, 90 * time.Second},
		{-90000, -90 * time.Second},
	} {
		if got := expiryDuration(test.ms); got != test.want {
			t.Fatalf("expiryDuration(%d) = %v, want %v", test.ms, got, test.want)
		}
	}
}

// TestSetTTLControlsUncontended is the control: ordinary TTLs still apply and
// report a sensible remaining TTL, and a negative TTL still expires the key
// (Redis's documented "delete now"). These pass before and after the fix.
func TestSetTTLControlsUncontended(t *testing.T) {
	s := NewStore()
	if err := s.Set("k", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !s.SetTTL("k", 100*time.Second) {
		t.Fatalf("SetTTL(100s) returned false")
	}
	ttl := s.TTL("k")
	if ttl <= 0 || ttl > 100*time.Second {
		t.Fatalf("TTL after SetTTL(100s) = %v, want (0, 100s]", ttl)
	}

	// A negative TTL expires the key.
	if !s.SetTTL("k", -time.Second) {
		t.Fatalf("SetTTL(-1s) returned false")
	}
	if _, ok := s.Get("k"); ok {
		t.Fatalf("a negative TTL must expire the key, but the key is still present")
	}
}
