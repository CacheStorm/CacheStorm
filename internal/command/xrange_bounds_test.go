package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: StreamValue.GetRange compared entry IDs lexicographically
// ("100-1" < "50-1" as strings, though 100 > 50 numerically) and had no
// partial-ID handling (a bare-ms end bound excluded every entry in that
// millisecond). XRANGE results were wrong for any IDs of differing
// digit-length. Redis compares (ms, seq) numerically and treats a bare-ms
// end bound as inclusive of the whole millisecond.

func newXRangeRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterStreamCommands(router)
	return router
}

func TestXRangeNumericOrderingAcrossDigitLengths(t *testing.T) {
	s := store.NewStore()
	router := newXRangeRouter(s)

	// Both IDs are valid and increasing (round-23 monotonicity allows them).
	if reply := runCmd(t, s, router, "XADD", "k", "50-1", "f", "v"); !strings.Contains(reply, "50-1") {
		t.Fatalf("XADD setup failed, got %q", reply)
	}
	if reply := runCmd(t, s, router, "XADD", "k", "100-1", "f", "v"); !strings.Contains(reply, "100-1") {
		t.Fatalf("XADD setup failed, got %q", reply)
	}

	reply := runCmd(t, s, router, "XRANGE", "k", "50-1", "+")
	if !strings.Contains(reply, "50-1") || !strings.Contains(reply, "100-1") {
		t.Fatalf("XRANGE must compare IDs numerically: both entries expected, got %q", reply)
	}
}

func TestXRangePartialIDBounds(t *testing.T) {
	s := store.NewStore()
	router := newXRangeRouter(s)

	if reply := runCmd(t, s, router, "XADD", "k", "100-1", "f", "v"); !strings.Contains(reply, "100-1") {
		t.Fatalf("XADD setup failed, got %q", reply)
	}
	if reply := runCmd(t, s, router, "XADD", "k", "100-2", "f", "v"); !strings.Contains(reply, "100-2") {
		t.Fatalf("XADD setup failed, got %q", reply)
	}

	// A bare-ms end bound is inclusive of the whole millisecond in Redis.
	reply := runCmd(t, s, router, "XRANGE", "k", "100", "100")
	if !strings.Contains(reply, "100-1") || !strings.Contains(reply, "100-2") {
		t.Fatalf("partial-ID bounds must include the whole millisecond, got %q", reply)
	}
}

// Sanity: the full range and COUNT were already correct — they must stay so.
func TestXRangeFullRangeAndCountUnchanged(t *testing.T) {
	s := store.NewStore()
	router := newXRangeRouter(s)

	for _, id := range []string{"100-1", "200-1", "300-1"} {
		if reply := runCmd(t, s, router, "XADD", "k", id, "f", "v"); !strings.Contains(reply, id) {
			t.Fatalf("XADD setup failed, got %q", reply)
		}
	}

	full := runCmd(t, s, router, "XRANGE", "k", "-", "+")
	for _, id := range []string{"100-1", "200-1", "300-1"} {
		if !strings.Contains(full, id) {
			t.Fatalf("full range lost entry %q, got %q", id, full)
		}
	}

	capped := runCmd(t, s, router, "XRANGE", "k", "-", "+", "COUNT", "2")
	if strings.Contains(capped, "300-1") {
		t.Fatalf("COUNT 2 must cap the result at two entries, got %q", capped)
	}
}
