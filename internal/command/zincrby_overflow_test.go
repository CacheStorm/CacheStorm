package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// ZINCRBY follows Redis 7.4: an increment that overflows yields inf, an
// infinite increment is accepted, a NaN increment is "not a valid float",
// and only a NaN result (inf + -inf) is refused with "resulting score is not
// a number (NaN)", leaving the score unchanged.

func newZSetRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterSortedSetCommands(router)
	return router
}

func TestZIncrByAtMaxFloatOverflows(t *testing.T) {
	s := store.NewStore()
	router := newZSetRouter(s)

	runCmd(t, s, router, "ZADD", "z", "1e308", "m")
	if reply := runCmd(t, s, router, "ZINCRBY", "z", "1e308", "m"); reply != "$3\r\ninf\r\n" {
		t.Fatalf("overflowing ZINCRBY must reach inf, got %q", reply)
	}
	if got := runCmd(t, s, router, "ZSCORE", "z", "m"); got != "$3\r\ninf\r\n" {
		t.Fatalf("ZSCORE after overflow, got %q", got)
	}
}

func TestZIncrByNaNIncrementRejected(t *testing.T) {
	s := store.NewStore()
	router := newZSetRouter(s)

	runCmd(t, s, router, "ZADD", "z", "1", "m")
	if reply := runCmd(t, s, router, "ZINCRBY", "z", "NaN", "m"); reply != "-ERR value is not a valid float\r\n" {
		t.Fatalf("NaN increment must be rejected, got %q", reply)
	}
	if got := runCmd(t, s, router, "ZSCORE", "z", "m"); got != "$1\r\n1\r\n" {
		t.Fatalf("failed ZINCRBY must leave the score unchanged, got %q", got)
	}
}

func TestZIncrByNaNResultRejected(t *testing.T) {
	s := store.NewStore()
	router := newZSetRouter(s)

	if reply := runCmd(t, s, router, "ZINCRBY", "z", "Inf", "fresh"); reply != "$3\r\ninf\r\n" {
		t.Fatalf("Inf increment on a fresh member, got %q", reply)
	}
	if reply := runCmd(t, s, router, "ZINCRBY", "z", "-inf", "fresh"); reply != "-ERR resulting score is not a number (NaN)\r\n" {
		t.Fatalf("inf + -inf must be rejected, got %q", reply)
	}
	if got := runCmd(t, s, router, "ZSCORE", "z", "fresh"); got != "$3\r\ninf\r\n" {
		t.Fatalf("rejected ZINCRBY must leave the score unchanged, got %q", got)
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
