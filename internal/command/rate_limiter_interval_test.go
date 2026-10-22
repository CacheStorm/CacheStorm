package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// rlTestExecute runs a command through the real router and returns the reply,
// plus the value of a panic that escaped the handler (nil when the command
// completed normally).
func rlTestExecute(t *testing.T, router *Router, s *store.Store, cmd string, args ...string) (reply string, panicked any) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			panicked = r
		}
	}()

	ctx, buf := bufCtx(cmd, bytesArgs(args...), s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s returned an error: %v", cmd, err)
	}
	return buf.String(), nil
}

// TestRateLimiterRejectsNonPositiveInterval pins the contract that a rate
// limiter cannot be created with a refill interval of zero (or an argument
// that is not a number at all).
//
// The bug this pins: RL.CREATE parsed its interval with the shared lenient
// parseInt64 helper, which returns 0 for any input it cannot read, and never
// checked that the interval was usable. A zero interval was stored, and the
// next RL.ALLOW divided by it in store.RateLimiter.Allow and panicked the
// connection handler with "integer divide by zero".
//
// A rate limiter is a guard: it must never be configurable into a state where
// using it crashes the server.
func TestRateLimiterRejectsNonPositiveInterval(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityCommands(router)

	for _, tc := range []struct {
		name     string
		interval string
	}{
		{"zero interval", "0"},
		{"negative interval", "-1000"},
		{"non-numeric interval", "abc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := "rl-" + tc.name

			createReply, createPanic := rlTestExecute(t, router, s,
				"RL.CREATE", key, "10", "5", tc.interval)
			if createPanic != nil {
				t.Fatalf("RL.CREATE panicked: %v", createPanic)
			}

			// The limiter must not be usable with a zero interval: this is
			// the command that divides by it.
			_, allowPanic := rlTestExecute(t, router, s, "RL.ALLOW", key, "1")
			if allowPanic != nil {
				t.Fatalf("RL.ALLOW panicked after RL.CREATE with a %s: %v — the stored "+
					"refill interval of 0 makes the refill arithmetic divide by zero",
					tc.name, allowPanic)
			}

			// The configuration must have been refused, not stored.
			if !strings.HasPrefix(createReply, "-") {
				t.Fatalf("RL.CREATE accepted a %s (replied %q); it must refuse a zero "+
					"refill interval with an error reply", tc.name, createReply)
			}

			// And nothing must be left behind for the key: a refused
			// RL.CREATE must not have stored a limiter, so RL.GET must
			// report the key as absent (a RESP null, '_').
			getReply, getPanic := rlTestExecute(t, router, s, "RL.GET", key)
			if getPanic != nil {
				t.Fatalf("RL.GET panicked: %v", getPanic)
			}
			if !strings.HasPrefix(getReply, "_") {
				t.Fatalf("RL.CREATE with a %s stored a rate limiter for %q (RL.GET replied "+
					"%q); the unusable limiter must not be kept", tc.name, key, getReply)
			}
		})
	}
}

// TestRateLimiterValidIntervalStillWorks is the control: a rate limiter
// created with a valid interval must still be created, must still answer
// RL.ALLOW, and must still be visible through RL.GET. It passes before and
// after the fix, so it proves the harness is sound and that the failure above
// is the invalid-interval path, not the harness.
func TestRateLimiterValidIntervalStillWorks(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityCommands(router)

	key := "rl-valid"

	createReply, createPanic := rlTestExecute(t, router, s,
		"RL.CREATE", key, "10", "5", "1000")
	if createPanic != nil {
		t.Fatalf("control failed: RL.CREATE panicked: %v", createPanic)
	}
	if !strings.HasPrefix(createReply, "+") {
		t.Fatalf("control failed: RL.CREATE with a valid interval replied %q, want +OK",
			createReply)
	}

	allowReply, allowPanic := rlTestExecute(t, router, s, "RL.ALLOW", key, "1")
	if allowPanic != nil {
		t.Fatalf("control failed: RL.ALLOW panicked: %v", allowPanic)
	}
	// The reply is an array of [allowed, remaining, resetTime]; the first
	// element is 1 when the request was allowed.
	if !strings.HasPrefix(allowReply, "*3\r\n:1\r\n") {
		t.Fatalf("control failed: RL.ALLOW replied %q, want an array whose first element "+
			"is 1 (allowed)", allowReply)
	}

	getReply, getPanic := rlTestExecute(t, router, s, "RL.GET", key)
	if getPanic != nil {
		t.Fatalf("control failed: RL.GET panicked: %v", getPanic)
	}
	if !strings.HasPrefix(getReply, "*") {
		t.Fatalf("control failed: RL.GET replied %q, want an array describing the limiter",
			getReply)
	}
}
