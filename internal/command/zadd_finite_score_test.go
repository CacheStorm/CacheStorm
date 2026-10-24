package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Redis 7.4 stores infinite sorted-set scores: ZADD accepts "inf", "+inf"
// and "-inf", and an increment that overflows yields inf. Only NaN is
// refused, both as a literal ("value is not a valid float") and as the
// result of an increment such as inf + -inf ("resulting score is not a
// number (NaN)"). These tests pin that behaviour; earlier versions of them
// pinned a rejection of infinite scores that Redis does not make.

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

func zaddBulk(t *testing.T, v *resp.Value, want string) {
	t.Helper()
	if v.Type == resp.TypeError || string(v.Bulk) != want {
		t.Fatalf("got type %v %q, want %q", v.Type, string(v.Bulk), want)
	}
}

// TestZADDIncrOverflowsToInf: two increments of 1e308 overflow to inf, as in
// Redis.
func TestZADDIncrOverflowsToInf(t *testing.T) {
	s := store.NewStore()
	router := zaddFiniteRouter()

	zaddBulk(t, zaddFiniteRun(t, s, router, "ZADD", "z", "INCR", "1e308", "m"), "1e+308")
	zaddBulk(t, zaddFiniteRun(t, s, router, "ZADD", "z", "INCR", "1e308", "m"), "inf")
	zaddBulk(t, zaddFiniteRun(t, s, router, "ZSCORE", "z", "m"), "inf")
}

// TestZADDAcceptsInfiniteLiteralScore: infinite literals are stored and
// reported as inf / -inf.
func TestZADDAcceptsInfiniteLiteralScore(t *testing.T) {
	for lit, want := range map[string]string{"inf": "inf", "+inf": "inf", "-inf": "-inf", "Inf": "inf", "-Inf": "-inf"} {
		s := store.NewStore()
		router := zaddFiniteRouter()

		if v := zaddFiniteRun(t, s, router, "ZADD", "z", lit, "m"); v.Type != resp.TypeInteger || v.Int != 1 {
			t.Fatalf("ZADD z %s m returned type %v int %d, want :1", lit, v.Type, v.Int)
		}
		zaddBulk(t, zaddFiniteRun(t, s, router, "ZSCORE", "z", "m"), want)
	}
}

// TestZADDRejectsNaN: a NaN literal is refused before anything is stored, and
// an increment that would produce NaN is refused and leaves the score alone.
func TestZADDRejectsNaN(t *testing.T) {
	cases := [][]string{
		{"z", "nan", "m"},
		{"z", "CH", "1", "a", "nan", "b"},
		{"z", "NX", "NaN", "m"},
	}
	for _, args := range cases {
		s := store.NewStore()
		router := zaddFiniteRouter()

		v := zaddFiniteRun(t, s, router, "ZADD", args...)
		if v.Type != resp.TypeError || v.Err != "ERR value is not a valid float" {
			t.Fatalf("ZADD %v: got type %v %q, want the not-a-valid-float error", args, v.Type, v.Err)
		}
		if _, exists := s.Get("z"); exists {
			t.Fatalf("ZADD %v created the key", args)
		}
	}

	s := store.NewStore()
	router := zaddFiniteRouter()
	zaddFiniteRun(t, s, router, "ZADD", "z", "inf", "m")
	v := zaddFiniteRun(t, s, router, "ZADD", "z", "INCR", "-inf", "m")
	if v.Type != resp.TypeError || v.Err != "ERR resulting score is not a number (NaN)" {
		t.Fatalf("NaN-producing INCR: got type %v %q", v.Type, v.Err)
	}
	zaddBulk(t, zaddFiniteRun(t, s, router, "ZSCORE", "z", "m"), "inf")
}

// TestZINCRBYOverflowAndNaN: ZINCRBY overflows to inf and refuses NaN like
// ZADD INCR.
func TestZINCRBYOverflowAndNaN(t *testing.T) {
	s := store.NewStore()
	router := zaddFiniteRouter()

	zaddBulk(t, zaddFiniteRun(t, s, router, "ZINCRBY", "z", "1e308", "m"), "1e+308")
	zaddBulk(t, zaddFiniteRun(t, s, router, "ZINCRBY", "z", "1e308", "m"), "inf")
	v := zaddFiniteRun(t, s, router, "ZINCRBY", "z", "-inf", "m")
	if v.Type != resp.TypeError || v.Err != "ERR resulting score is not a number (NaN)" {
		t.Fatalf("NaN-producing ZINCRBY: got type %v %q", v.Type, v.Err)
	}
	zaddBulk(t, zaddFiniteRun(t, s, router, "ZSCORE", "z", "m"), "inf")
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
