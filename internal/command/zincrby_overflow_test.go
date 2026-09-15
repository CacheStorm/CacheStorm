package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdZINCRBY computed current+incr with unchecked float
// arithmetic, and strconv.ParseFloat accepted "NaN"/"Inf" as the increment —
// an overflowing increment stored +Inf as the member's score (and a literal
// NaN/Inf increment poisoned a fresh member directly), returned to the
// client as score text. Redis rejects with "ERR increment would produce NaN
// or Infinity" and leaves the score unchanged.

func newZSetRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterSortedSetCommands(router)
	return router
}

func TestZIncrByAtMaxFloatOverflows(t *testing.T) {
	s := store.NewStore()
	router := newZSetRouter(s)

	if reply := runCmd(t, s, router, "ZADD", "z", "1e308", "m"); reply == "" {
		t.Fatal("ZADD setup failed")
	}
	before := runCmd(t, s, router, "ZSCORE", "z", "m")

	reply := runCmd(t, s, router, "ZINCRBY", "z", "1e308", "m")
	if !strings.Contains(reply, "would produce NaN") {
		t.Fatalf("overflowing ZINCRBY must be rejected, got %q", reply)
	}
	if after := runCmd(t, s, router, "ZSCORE", "z", "m"); after != before {
		t.Fatalf("failed ZINCRBY must leave the score unchanged, got %q, want %q", after, before)
	}
}

func TestZIncrByNaNIncrementRejected(t *testing.T) {
	s := store.NewStore()
	router := newZSetRouter(s)

	if reply := runCmd(t, s, router, "ZADD", "z", "1", "m"); reply == "" {
		t.Fatal("ZADD setup failed")
	}

	reply := runCmd(t, s, router, "ZINCRBY", "z", "NaN", "m")
	if !strings.Contains(reply, "would produce NaN") {
		t.Fatalf("NaN increment must be rejected, got %q", reply)
	}
	if got := runCmd(t, s, router, "ZSCORE", "z", "m"); !strings.Contains(got, "1") {
		t.Fatalf("failed ZINCRBY must leave the score unchanged, got %q", got)
	}
}

func TestZIncrByFreshMemberInfRejected(t *testing.T) {
	s := store.NewStore()
	router := newZSetRouter(s)

	reply := runCmd(t, s, router, "ZINCRBY", "z", "Inf", "fresh")
	if !strings.Contains(reply, "would produce NaN") {
		t.Fatalf("Inf increment on a fresh member must be rejected, got %q", reply)
	}
	if got := runCmd(t, s, router, "ZSCORE", "z", "fresh"); strings.Contains(got, "Inf") {
		t.Fatalf("rejected ZINCRBY must not create the member, got %q", got)
	}
}

// Sanity: ordinary increments are unaffected by the guard.
func TestZIncrByNormalOperation(t *testing.T) {
	s := store.NewStore()
	router := newZSetRouter(s)

	if reply := runCmd(t, s, router, "ZADD", "z", "1", "m"); reply == "" {
		t.Fatal("ZADD setup failed")
	}
	if reply := runCmd(t, s, router, "ZINCRBY", "z", "2", "m"); !strings.Contains(reply, "3") {
		t.Fatalf("ZINCRBY must accumulate, got %q", reply)
	}
	if got := runCmd(t, s, router, "ZSCORE", "z", "m"); !strings.Contains(got, "3") {
		t.Fatalf("score drifted, got %q", got)
	}
}
