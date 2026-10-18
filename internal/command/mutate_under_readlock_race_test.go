package command

import (
	"sync"
	"testing"
)

// newRouterForRace builds a router with the integration families registered.
func newRouterForRace() *Router {
	router := NewRouter()
	RegisterIntegrationCommands(router)
	return router
}

// The CIRCUITBREAKER and RATELIMIT families took only a READ lock, copied the
// map's *pointer* value, released the lock, and then mutated the shared struct
// behind it. circuitBreakersExt and rateLimiters are map[string]*Struct, so the
// RLock protected only the map lookup; every field touched afterwards was
// mutated with no lock held.
//
// Two consequences:
//   - a data race (and for RATELIMIT, a concurrent Go map read+write, which is
//     an unrecoverable runtime throw that kills the process);
//   - LOST INCREMENTS, because cb.Failures++ and rl.Requests[window]++ are
//     non-atomic read-modify-writes. That is a silent control failure: the
//     breaker may never reach its threshold and never open, and the limiter may
//     never reach its limit and never throttle -- exactly the behaviour those
//     commands exist to provide.

// TestCircuitBreakerTripCounterDoesNotLoseIncrements asserts the DETERMINISTIC
// half of the defect: N trips issued from G goroutines must record exactly N.
// Lost increments are observable without the race detector, so this test is
// meaningful under plain `go test` too.
func TestCircuitBreakerTripCounterDoesNotLoseIncrements(t *testing.T) {
	const (
		goroutines = 8
		perRoutine = 500
		want       = goroutines * perRoutine
	)

	router := newRouterForRace()

	t.Cleanup(func() {
		circuitBreakersExtMu.Lock()
		delete(circuitBreakersExt, "counter-test")
		circuitBreakersExtMu.Unlock()
	})

	// A threshold above the expected total keeps the breaker from opening, so
	// the assertion observes the counter alone.
	if err := runArrayCmd(router, "CIRCUITBREAKER.CREATE", "counter-test", "1000000", "60000"); err != nil {
		t.Fatalf("CIRCUITBREAKER.CREATE errored: %v", err)
	}

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perRoutine; i++ {
				// Errors are ignored off the test goroutine: t.Fatalf is not
				// legal there. Seeding above already asserts validity.
				_ = runArrayCmd(router, "CIRCUITBREAKER.TRIP", "counter-test")
			}
		}()
	}
	wg.Wait()

	circuitBreakersExtMu.Lock()
	cb := circuitBreakersExt["counter-test"]
	var got int64
	if cb != nil {
		got = cb.Failures
	}
	circuitBreakersExtMu.Unlock()

	if got != want {
		t.Errorf("recorded failures = %d after %d concurrent trips, want %d; the "+
			"non-atomic cb.Failures++ lost increments", got, want, want)
	}
}

// TestRateLimiterCheckCounterDoesNotLoseIncrements is the RATELIMIT analogue:
// N checks must record exactly N in the current window.
func TestRateLimiterCheckCounterDoesNotLoseIncrements(t *testing.T) {
	const (
		goroutines = 8
		perRoutine = 500
		want       = goroutines * perRoutine
	)

	router := newRouterForRace()

	t.Cleanup(func() {
		rateLimitersMu.Lock()
		delete(rateLimiters, "counter-test")
		rateLimitersMu.Unlock()
	})

	// A limit above the expected total keeps the limiter from refusing, so the
	// assertion observes the counter alone.
	if err := runArrayCmd(router, "RATELIMIT.CREATE", "counter-test", "1000000", "60000"); err != nil {
		t.Fatalf("RATELIMIT.CREATE errored: %v", err)
	}

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perRoutine; i++ {
				_ = runArrayCmd(router, "RATELIMIT.CHECK", "counter-test", "x")
			}
		}()
	}
	wg.Wait()

	rateLimitersMu.Lock()
	rl := rateLimiters["counter-test"]
	var total int64
	if rl != nil {
		for _, v := range rl.Requests {
			total += v
		}
	}
	rateLimitersMu.Unlock()

	if total != want {
		t.Errorf("recorded requests = %d after %d concurrent checks, want %d; the "+
			"non-atomic rl.Requests[window]++ lost increments", total, want, want)
	}
}

// TestResilienceFamiliesDoNotRaceUnderLoad hammers both families. Only
// meaningful under -race, which is the project's CI gate
// (`go test -race ./internal/... -count=1`).
func TestResilienceFamiliesDoNotRaceUnderLoad(t *testing.T) {
	router := newRouterForRace()

	t.Cleanup(func() {
		circuitBreakersExtMu.Lock()
		delete(circuitBreakersExt, "load-test")
		circuitBreakersExtMu.Unlock()
		rateLimitersMu.Lock()
		delete(rateLimiters, "load-test")
		rateLimitersMu.Unlock()
	})

	for _, args := range [][]string{
		{"CIRCUITBREAKER.CREATE", "load-test", "1000000", "60000"},
		{"RATELIMIT.CREATE", "load-test", "1000000", "60000"},
	} {
		if err := runArrayCmd(router, args...); err != nil {
			t.Fatalf("%v errored: %v", args, err)
		}
	}

	readers := [][]string{
		{"CIRCUITBREAKER.STATE", "load-test"},
		{"RATELIMIT.CHECK", "load-test", "x"},
	}
	writers := [][]string{
		{"CIRCUITBREAKER.TRIP", "load-test"},
		{"RATELIMIT.CHECK", "load-test", "x"},
		{"CIRCUITBREAKER.RESET", "load-test"},
		{"RATELIMIT.RESET", "load-test"},
	}

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				for _, cmd := range readers {
					_ = runArrayCmd(router, cmd...)
				}
			}
		}()
	}
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				for _, cmd := range writers {
					_ = runArrayCmd(router, cmd...)
				}
			}
		}()
	}
	wg.Wait()
}
