package store

import (
	"testing"
	"time"
)

// TestRateLimiterAllowWithNonPositiveInterval pins the store-level invariant
// that RateLimiter.Allow never divides by a non-positive interval.
//
// The bug this pins: the refill arithmetic computed elapsed/Interval, so a
// limiter installed with a zero interval panicked with "integer divide by
// zero" the first time it was used. The command layer now refuses such a
// configuration up front, but the store is the layer that owns the division,
// so it must not panic if a limiter with a non-positive interval is ever
// installed here directly.
//
// A limiter with a non-positive interval cannot be refilled, so the expected
// behaviour is that the request is simply judged against the tokens already
// available, with no refill and no crash.
func TestRateLimiterAllowWithNonPositiveInterval(t *testing.T) {
	for _, tc := range []struct {
		name     string
		interval time.Duration
	}{
		{"zero interval", 0},
		{"negative interval", -time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rl := NewRateLimiter()
			rl.Create("key", 5, 1, tc.interval)

			// The first request must be judged against the available
			// tokens. It panics if the refill arithmetic divides by the
			// interval.
			allowed, remaining, _ := rl.Allow("key", 3)
			if !allowed {
				t.Errorf("the first request should be allowed")
			}
			if remaining != 2 {
				t.Errorf("remaining = %d, want 2", remaining)
			}

			// The limiter must still be usable afterwards.
			allowed, remaining, _ = rl.Allow("key", 5)
			if allowed {
				t.Errorf("the request should not be allowed (not enough tokens)")
			}
			if remaining != 2 {
				t.Errorf("remaining = %d, want 2", remaining)
			}
		})
	}
}

// TestRateLimiterAllowRefillsOverTime is the control: a limiter with a
// positive interval must still refill once the interval has elapsed. It
// proves the guard in Allow did not disable the refill path.
func TestRateLimiterAllowRefillsOverTime(t *testing.T) {
	rl := NewRateLimiter()
	rl.Create("key", 5, 10, time.Millisecond)

	// Drain the bucket.
	allowed, remaining, _ := rl.Allow("key", 5)
	if !allowed {
		t.Fatalf("the first request should be allowed")
	}
	if remaining != 0 {
		t.Fatalf("remaining = %d, want 0", remaining)
	}

	// Wait for the interval to elapse so the refill applies.
	time.Sleep(5 * time.Millisecond)

	allowed, remaining, _ = rl.Allow("key", 5)
	if !allowed {
		t.Errorf("the request should be allowed after the refill interval elapsed")
	}
	if remaining != 0 {
		t.Errorf("remaining = %d, want 0", remaining)
	}
}
