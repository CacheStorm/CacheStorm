package command

import (
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression tests for the probabilistic size arguments.
//
// BF.RESERVE, CMS.INIT and TOPK.RESERVE read a size from the client and
// used to hand it straight to the store constructor with no validation. The
// shared parseInt64 helper returns 0 for any input it cannot read, so a
// typo'd size was indistinguishable from a legitimate zero — and zero is
// exactly the value these structures cannot survive:
//
//   - a zero-width bloom filter divides by its zero modulus in
//     store.BloomFilter.Add, so the panic happened on the NEXT command
//     touching the key, not on the reserve itself;
//   - a zero-width count-min sketch divides by its zero modulus in
//     store.CountMinSketch.Add;
//   - a negative top-k makes store.TopK.List allocate with a negative
//     capacity.
//
// A negative size was worse still: it became a huge uint, the constructor
// allocated an impossible slice and panicked, and because BF.RESERVE built
// the filter INSIDE the global probabilisticMu critical section, that panic
// stranded the mutex — every later probabilistic command in the server then
// blocked forever.
//
// The sibling command CF.RESERVE in the same file already refuses a
// non-positive capacity with an error reply, and RedisBloom requires a
// positive capacity/topk, so the expected behaviour is an error reply.

// probSizeRun executes one command through the real router and reports the
// reply, or the panic value if the handler crashed.
func probSizeRun(t *testing.T, router *Router, s *store.Store, cmd string, args ...string) (out string, panicked any) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			panicked = r
		}
	}()
	ctx, buf := bufCtx(cmd, bytesArgs(args...), s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s %v returned a transport error: %v", cmd, args, err)
	}
	return buf.String(), nil
}

// probSizeRunBounded runs a command in its own goroutine and waits at most
// timeout for it to finish. A command that never returns is a failure in its
// own right, and the test must be able to REPORT it rather than hang.
func probSizeRunBounded(t *testing.T, router *Router, s *store.Store, timeout time.Duration, cmd string, args ...string) (out string, panicked any, timedOut bool) {
	t.Helper()
	type result struct {
		out      string
		panicked any
	}
	resCh := make(chan result, 1)
	go func() {
		r := result{}
		defer func() {
			if rec := recover(); rec != nil {
				r.panicked = rec
			}
			resCh <- r
		}()
		ctx, buf := bufCtx(cmd, bytesArgs(args...), s)
		if err := router.Execute(ctx); err != nil {
			return
		}
		r.out = buf.String()
	}()
	select {
	case r := <-resCh:
		return r.out, r.panicked, false
	case <-time.After(timeout):
		return "", nil, true
	}
}

func TestProbabilisticSizeArgumentsAreValidated(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterProbabilisticCommands(router)

	// --- BF.RESERVE capacity ---
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"zero capacity", []string{"prob-bf-zero", "0"}},
		{"non-numeric capacity", []string{"prob-bf-nan", "abc"}},
	} {
		out, p := probSizeRun(t, router, s, "BF.RESERVE", tc.args...)
		if p != nil {
			t.Errorf("BF.RESERVE %v panicked: %v", tc.args, p)
			continue
		}
		if !strings.HasPrefix(out, "-ERR") {
			t.Errorf("BF.RESERVE %v was accepted (reply %q); a capacity that is not a positive integer must be refused", tc.args, out)
		}
		// A refused BF.RESERVE must not leave a broken filter behind that
		// panics the next command touching the same key.
		if _, p2 := probSizeRun(t, router, s, "BF.ADD", tc.args[0], "item"); p2 != nil {
			t.Errorf("BF.ADD after BF.RESERVE %v panicked: %v", tc.args, p2)
		}
	}

	// --- CMS.INIT depth / width ---
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"zero depth", []string{"prob-cms-depth0", "0", "100"}},
		{"zero width", []string{"prob-cms-width0", "5", "0"}},
		{"non-numeric depth", []string{"prob-cms-nan", "abc", "100"}},
	} {
		out, p := probSizeRun(t, router, s, "CMS.INIT", tc.args...)
		if p != nil {
			t.Errorf("CMS.INIT %v panicked: %v", tc.args, p)
			continue
		}
		if !strings.HasPrefix(out, "-ERR") {
			t.Errorf("CMS.INIT %v was accepted (reply %q); depth and width must be positive integers", tc.args, out)
		}
		// A refused CMS.INIT must not leave a sketch that panics the next
		// command touching the same key.
		if _, p2 := probSizeRun(t, router, s, "CMS.INCRBY", tc.args[0], "item", "1"); p2 != nil {
			t.Errorf("CMS.INCRBY after CMS.INIT %v panicked: %v", tc.args, p2)
		}
	}

	// --- TOPK.RESERVE topk ---
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"zero topk", []string{"prob-topk-zero", "0"}},
		{"non-numeric topk", []string{"prob-topk-nan", "abc"}},
	} {
		out, p := probSizeRun(t, router, s, "TOPK.RESERVE", tc.args...)
		if p != nil {
			t.Errorf("TOPK.RESERVE %v panicked: %v", tc.args, p)
			continue
		}
		if !strings.HasPrefix(out, "-ERR") {
			t.Errorf("TOPK.RESERVE %v was accepted (reply %q); topk must be a positive integer", tc.args, out)
		}
		// A refused TOPK.RESERVE must not leave a top-k that panics the next
		// command touching the same key.
		if _, p2 := probSizeRun(t, router, s, "TOPK.LIST", tc.args[0]); p2 != nil {
			t.Errorf("TOPK.LIST after TOPK.RESERVE %v panicked: %v", tc.args, p2)
		}
	}

	// --- CF.RESERVE capacity / bucket size ---
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"zero capacity", []string{"prob-cf-cap0", "0"}},
		{"non-numeric capacity", []string{"prob-cf-nan", "abc"}},
		{"zero bucket size", []string{"prob-cf-bucket0", "1000", "0"}},
	} {
		out, p := probSizeRun(t, router, s, "CF.RESERVE", tc.args...)
		if p != nil {
			t.Errorf("CF.RESERVE %v panicked: %v", tc.args, p)
			continue
		}
		if !strings.HasPrefix(out, "-ERR") {
			t.Errorf("CF.RESERVE %v was accepted (reply %q); capacity and bucket size must be positive integers", tc.args, out)
		}
		// A refused CF.RESERVE must not leave a broken filter behind that
		// panics the next command touching the same key.
		if _, p2 := probSizeRun(t, router, s, "CF.ADD", tc.args[0], "item"); p2 != nil {
			t.Errorf("CF.ADD after CF.RESERVE %v panicked: %v", tc.args, p2)
		}
	}

	// --- control: the ordinary valid-input path must keep working ---
	out, p := probSizeRun(t, router, s, "BF.RESERVE", "prob-bf-ok", "1000")
	if p != nil {
		t.Errorf("control BF.RESERVE panicked: %v", p)
	} else if !strings.HasPrefix(out, "+OK") {
		t.Errorf("control BF.RESERVE replied %q, want +OK", out)
	}
	out, p = probSizeRun(t, router, s, "BF.ADD", "prob-bf-ok", "item")
	if p != nil {
		t.Errorf("control BF.ADD panicked: %v", p)
	} else if !strings.HasPrefix(out, ":1") {
		t.Errorf("control BF.ADD replied %q, want :1", out)
	}
	out, p = probSizeRun(t, router, s, "BF.EXISTS", "prob-bf-ok", "item")
	if p != nil {
		t.Errorf("control BF.EXISTS panicked: %v", p)
	} else if !strings.HasPrefix(out, ":1") {
		t.Errorf("control BF.EXISTS replied %q, want :1", out)
	}
}

// TestBFReserveNegativeCapacityIsRefused is kept separate because the
// unfixed BF.RESERVE panics while holding the global probabilisticMu, which
// strands the mutex for every other probabilistic command in the process.
// Running it alone keeps that blast radius contained to this test.
func TestBFReserveNegativeCapacityIsRefused(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterProbabilisticCommands(router)

	out, p, timedOut := probSizeRunBounded(t, router, s, 5*time.Second, "BF.RESERVE", "prob-bf-neg", "-1")
	if timedOut {
		t.Fatalf("BF.RESERVE with a negative capacity never returned")
	}
	if p != nil {
		t.Errorf("BF.RESERVE with a negative capacity panicked: %v", p)
	} else if !strings.HasPrefix(out, "-ERR") {
		t.Errorf("BF.RESERVE with a negative capacity was accepted (reply %q); it must be refused", out)
	}

	// The refused reserve must not have stranded the probabilistic mutex:
	// the next probabilistic command still has to answer.
	out, p, timedOut = probSizeRunBounded(t, router, s, 5*time.Second, "BF.RESERVE", "prob-bf-neg-2", "1000")
	if timedOut {
		t.Fatalf("BF.RESERVE blocked forever after a refused negative capacity; the probabilistic mutex was stranded")
	}
	if p != nil {
		t.Errorf("BF.RESERVE after a refused negative capacity panicked: %v", p)
	} else if !strings.HasPrefix(out, "+OK") {
		t.Errorf("BF.RESERVE after a refused negative capacity replied %q, want +OK", out)
	}
}
