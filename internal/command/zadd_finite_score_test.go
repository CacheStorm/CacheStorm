package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: ZADD never enforced that a stored score is finite, so a
// sorted-set member could be persisted with a score of ±Inf.
//
// Two entry points shared that one root cause:
//
//  1. the INCR path added currentScore + score and stored the result with no
//     check, so an increment that overflowed stored +Inf;
//  2. the per-pair score parser rejected NaN but accepted the literals
//     "inf"/"+inf"/"-inf", because ParseFloat returns those without error.
//     (Out-of-range literals such as "1e400" already failed with ErrRange.)
//
// In-repo basis: cmdZINCRBY performs the same arithmetic and already refuses a
// non-finite result with ErrFloatOverflow.
//
// Impact is not just a wrong reply: ZSCORE then reports "+Inf" for the member,
// score-ordered iteration becomes degenerate, and GT/LT comparisons against it
// can never behave as intended.

// zaddFiniteRun drives a command through the real router and parses the reply.
func zaddFiniteRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) *resp.Value {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s: Execute returned error: %v", cmd, err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", cmd, buf.String(), err)
	}
	return v
}

func zaddFiniteRouter() *Router {
	router := NewRouter()
	RegisterSortedSetCommands(router)
	return router
}

// assertFiniteScore fails if a member's persisted score is ±Inf. Per the earlier
// ZADD round's lesson, assert the OBSERVABLE SCORE, not a bare return value.
func assertFiniteScore(t *testing.T, s *store.Store, router *Router, key, member string) {
	t.Helper()
	v := zaddFiniteRun(t, s, router, "ZSCORE", key, member)
	if v.Type == resp.TypeError {
		return // member absent: nothing non-finite was stored
	}
	switch string(v.Bulk) {
	case "+Inf", "-Inf", "Inf", "NaN":
		t.Fatalf("FAIL: member %q of %q has a non-finite score %q", member, key, string(v.Bulk))
	}
}

// TestZADDIncrRejectsScoreOverflow is the INCR entry point: two increments of
// 1e308 overflow to +Inf and must be refused.
func TestZADDIncrRejectsScoreOverflow(t *testing.T) {
	s := store.NewStore()
	router := zaddFiniteRouter()

	// The first INCR creates the member at a large but finite score.
	first := zaddFiniteRun(t, s, router, "ZADD", "z", "INCR", "1e308", "m")
	if first.Type == resp.TypeError {
		t.Fatalf("FAIL: first finite INCR was rejected: %q", string(first.Bulk))
	}

	second := zaddFiniteRun(t, s, router, "ZADD", "z", "INCR", "1e308", "m")
	if second.Type != resp.TypeError {
		t.Fatalf("FAIL: overflowing ZADD INCR returned type %v (%q), want an error —\n"+
			"currentScore + score is stored without an IsInf/IsNaN check",
			second.Type, string(second.Bulk))
	}
	assertFiniteScore(t, s, router, "z", "m")
}

// TestZADDRejectsNonFiniteLiteralScore is the literal entry point: "inf" and
// "-inf" parse successfully in Go and must still be refused.
func TestZADDRejectsNonFiniteLiteralScore(t *testing.T) {
	for _, lit := range []string{"inf", "+inf", "-inf", "Inf", "-Inf"} {
		s := store.NewStore()
		router := zaddFiniteRouter()

		v := zaddFiniteRun(t, s, router, "ZADD", "z", lit, "m")
		if v.Type != resp.TypeError {
			t.Fatalf("FAIL: ZADD z %s m was accepted (type %v) — a non-finite\n"+
				"score must be refused as Redis does", lit, v.Type)
		}
		assertFiniteScore(t, s, router, "z", "m")
	}
}

// TestZADDRejectsNonFiniteLiteralWithOptionsAndPairs covers the same rejection
// when options and multiple score/member pairs are present, so the guard is not
// bypassed by argument position.
func TestZADDRejectsNonFiniteLiteralWithOptionsAndPairs(t *testing.T) {
	cases := [][]string{
		{"z", "CH", "1", "a", "inf", "b"},
		{"z", "inf", "b", "1", "a"},
		{"z", "NX", "inf", "m"},
	}
	for _, args := range cases {
		s := store.NewStore()
		router := zaddFiniteRouter()

		if v := zaddFiniteRun(t, s, router, "ZADD", args...); v.Type != resp.TypeError {
			t.Fatalf("FAIL: ZADD %v was accepted (type %v), want an error for the non-finite pair",
				args, v.Type)
		}
	}
}

// TestZINCRBYStillRejectsScoreOverflow pins the in-repo basis the fix mirrors,
// so that sibling guard cannot silently regress.
func TestZINCRBYStillRejectsScoreOverflow(t *testing.T) {
	s := store.NewStore()
	router := zaddFiniteRouter()

	if v := zaddFiniteRun(t, s, router, "ZINCRBY", "z", "1e308", "m"); v.Type == resp.TypeError {
		t.Fatalf("FAIL: first finite ZINCRBY was rejected: %q", string(v.Bulk))
	}
	if v := zaddFiniteRun(t, s, router, "ZINCRBY", "z", "1e308", "m"); v.Type != resp.TypeError {
		t.Fatalf("FAIL: overflowing ZINCRBY returned type %v, want an error", v.Type)
	}
	assertFiniteScore(t, s, router, "z", "m")
}

// TestZADDFiniteScoreControls is the control: ordinary scores, ordinary
// increments and the option forms must all keep working, so a rejection above
// is the fix and not an over-broad guard. Values are checked through ZSCORE.
func TestZADDFiniteScoreControls(t *testing.T) {
	s := store.NewStore()
	router := zaddFiniteRouter()

	// Plain ZADD stores the score, verified observably.
	if v := zaddFiniteRun(t, s, router, "ZADD", "z", "1.5", "a"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Fatalf("CONTROL FAIL: ZADD z 1.5 a returned type %v int %d, want :1", v.Type, v.Int)
	}
	if v := zaddFiniteRun(t, s, router, "ZSCORE", "z", "a"); string(v.Bulk) != "1.5" {
		t.Fatalf("CONTROL FAIL: ZSCORE a = %q, want 1.5", string(v.Bulk))
	}

	// INCR on an existing member accumulates.
	if v := zaddFiniteRun(t, s, router, "ZADD", "z", "INCR", "2.25", "a"); string(v.Bulk) != "3.75" {
		t.Fatalf("CONTROL FAIL: ZADD INCR on existing = %q, want 3.75", string(v.Bulk))
	}
	if v := zaddFiniteRun(t, s, router, "ZSCORE", "z", "a"); string(v.Bulk) != "3.75" {
		t.Fatalf("CONTROL FAIL: ZSCORE a after INCR = %q, want 3.75", string(v.Bulk))
	}

	// INCR on a new member creates it at the increment value.
	if v := zaddFiniteRun(t, s, router, "ZADD", "z", "INCR", "10", "fresh"); string(v.Bulk) != "10" {
		t.Fatalf("CONTROL FAIL: ZADD INCR on new member = %q, want 10", string(v.Bulk))
	}
	if v := zaddFiniteRun(t, s, router, "ZSCORE", "z", "fresh"); string(v.Bulk) != "10" {
		t.Fatalf("CONTROL FAIL: ZSCORE fresh = %q, want 10", string(v.Bulk))
	}

	// A large but finite increment is still accepted and stays finite.
	if v := zaddFiniteRun(t, s, router, "ZADD", "z2", "INCR", "1e307", "big"); v.Type == resp.TypeError {
		t.Fatalf("CONTROL FAIL: finite large INCR was rejected: %q", string(v.Bulk))
	}
	assertFiniteScore(t, s, router, "z2", "big")

	// A large but finite literal is still accepted.
	if v := zaddFiniteRun(t, s, router, "ZADD", "z4", "1e307", "lit"); v.Type == resp.TypeError {
		t.Fatalf("CONTROL FAIL: finite large literal was rejected: %q", string(v.Bulk))
	}
	assertFiniteScore(t, s, router, "z4", "lit")

	// Negative finite scores are legal and must not be rejected.
	if v := zaddFiniteRun(t, s, router, "ZADD", "z5", "-2.5", "n"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Fatalf("CONTROL FAIL: ZADD z5 -2.5 n returned type %v int %d, want :1", v.Type, v.Int)
	}
	if v := zaddFiniteRun(t, s, router, "ZSCORE", "z5", "n"); string(v.Bulk) != "-2.5" {
		t.Fatalf("CONTROL FAIL: ZSCORE n = %q, want -2.5", string(v.Bulk))
	}

	// Zero and a negative INCR still work.
	if v := zaddFiniteRun(t, s, router, "ZADD", "z6", "INCR", "5", "m"); string(v.Bulk) != "5" {
		t.Fatalf("CONTROL FAIL: ZADD z6 INCR 5 m = %q, want 5", string(v.Bulk))
	}
	if v := zaddFiniteRun(t, s, router, "ZADD", "z6", "INCR", "-3", "m"); string(v.Bulk) != "2" {
		t.Fatalf("CONTROL FAIL: ZADD z6 INCR -3 m = %q, want 2", string(v.Bulk))
	}

	// Multi-member ZADD with CH (options precede the pairs).
	if v := zaddFiniteRun(t, s, router, "ZADD", "z7", "CH", "1", "a", "2", "b"); v.Type != resp.TypeInteger || v.Int != 2 {
		t.Fatalf("CONTROL FAIL: ZADD z7 CH 1 a 2 b returned type %v int %d, want :2", v.Type, v.Int)
	}
}
