package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdHINCRBY computed currentInt+incr with unchecked int64
// arithmetic, so HINCRBY at math.MaxInt64 wrapped to math.MinInt64 (and at
// MinInt64 wrapped positive) — the wrapped value was stored into the hash
// field and returned as the true result. Redis rejects the increment
// instead and leaves the field unchanged.

func newHashRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterHashCommands(router)
	return router
}

func hgetField(t *testing.T, s *store.Store, router *Router, key, field string) string {
	t.Helper()
	return runCmd(t, s, router, "HGET", key, field)
}

func TestHIncrByAtMaxInt64Overflows(t *testing.T) {
	s := store.NewStore()
	router := newHashRouter(s)
	const maxInt64 = "9223372036854775807"

	if reply := runCmd(t, s, router, "HSET", "h", "f", maxInt64); reply == "" {
		t.Fatal("HSET setup failed")
	}

	reply := runCmd(t, s, router, "HINCRBY", "h", "f", "1")
	if !strings.Contains(reply, "would overflow") {
		t.Fatalf("HINCRBY at MaxInt64 must be rejected, got %q", reply)
	}
	if got := hgetField(t, s, router, "h", "f"); !strings.Contains(got, maxInt64) {
		t.Fatalf("failed HINCRBY must leave the field unchanged, got %q", got)
	}
}

func TestHIncrByAtMinInt64Overflows(t *testing.T) {
	s := store.NewStore()
	router := newHashRouter(s)
	const minInt64 = "-9223372036854775808"

	if reply := runCmd(t, s, router, "HSET", "h", "f", minInt64); reply == "" {
		t.Fatal("HSET setup failed")
	}

	reply := runCmd(t, s, router, "HINCRBY", "h", "f", "-1")
	if !strings.Contains(reply, "would overflow") {
		t.Fatalf("HINCRBY at MinInt64 must be rejected, got %q", reply)
	}
	if got := hgetField(t, s, router, "h", "f"); !strings.Contains(got, minInt64) {
		t.Fatalf("failed HINCRBY must leave the field unchanged, got %q", got)
	}
}

// Sanity: ordinary increments are unaffected by the overflow guard.
func TestHIncrByNormalOperation(t *testing.T) {
	s := store.NewStore()
	router := newHashRouter(s)

	if reply := runCmd(t, s, router, "HINCRBY", "h", "n", "5"); !strings.Contains(reply, ":5") {
		t.Fatalf("HINCRBY on a fresh field must yield 5, got %q", reply)
	}
	if reply := runCmd(t, s, router, "HINCRBY", "h", "n", "37"); !strings.Contains(reply, ":42") {
		t.Fatalf("HINCRBY must accumulate, got %q", reply)
	}
	if got := hgetField(t, s, router, "h", "n"); !strings.Contains(got, "42") {
		t.Fatalf("field drifted, got %q", got)
	}
}
