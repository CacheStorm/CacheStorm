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

// TestSetTTLFarFutureSaturatesInsteadOfWrapping is the primary case: a TTL far
// beyond the int64-nanosecond range must leave the key alive, not delete it.
func TestSetTTLFarFutureSaturatesInsteadOfWrapping(t *testing.T) {
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
	if got := s.TTL("k"); got <= 0 {
		t.Fatalf("TTL after a far-future SetTTL = %v, want a positive remaining TTL", got)
	}
}

// TestSetTTLNSSaturationUnit pins nowNSPlus directly at both int64 bounds, so
// the saturation logic is checked without waiting on a 292-year TTL.
func TestSetTTLNSSaturationUnit(t *testing.T) {
	// Saturated high: now + MaxInt64 would overflow, so the result must be
	// MaxInt64 rather than a wrapped NEGATIVE value.
	if got := nowNSPlus(math.MaxInt64); got != math.MaxInt64 {
		t.Fatalf("nowNSPlus(MaxInt64) = %d, want MaxInt64 (%d)", got, int64(math.MaxInt64))
	}
	// A delta that does NOT overflow must be exact, not saturated.
	now := time.Now().UnixNano()
	if got := nowNSPlus(0); got < now || got-now > int64(5*time.Second) {
		t.Fatalf("nowNSPlus(0) = %d, want ~now (%d) within a few seconds", got, now)
	}
	if got := nowNSPlus(int64(90 * time.Second)); got < now || got-now > int64(95*time.Second) {
		t.Fatalf("nowNSPlus(90s) = %d, want now+~90s", got)
	}
	// 1<<62 does NOT overflow (now + 4.6e18 is still under MaxInt64), so it must
	// be passed through rather than saturated -- a guard that saturated too
	// eagerly would report the wrong remaining TTL for large-but-legal values.
	// nowNSPlus samples its OWN time.Now(), so allow a small sampling window
	// rather than demanding exact equality against the `now` captured above.
	want := now + (1 << 62)
	if got := nowNSPlus(1 << 62); got < want || got-want > int64(5*time.Second) {
		t.Fatalf("nowNSPlus(1<<62) = %d, want ~now+2^62 (%d): a non-overflowing delta "+
			"must not be saturated", got, want)
	}

	// The negative branch is intentionally untested: time.Now().UnixNano() is
	// positive (~1.8e18), so no int64 delta can underflow now+d past MinInt64.
	// The branch is defensive, not reachable from SetTTL's guarded inputs.
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
