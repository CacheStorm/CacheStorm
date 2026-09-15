package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: INCRBYFLOAT and HINCRBYFLOAT computed current+incr with no
// NaN/Inf check, and Go's ParseFloat accepts "NaN"/"Inf" as the increment —
// so overflowing increments (or literal Inf/NaN) were formatted and stored
// as "+Inf"/"NaN" text, permanently poisoning the key or hash field. Redis
// rejects the increment instead and leaves the value unchanged.

func newFloatRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterHashCommands(router)
	return router
}

func runCmd(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	ctx := newTestContext(cmd, parsed, s)
	buf := &bytes.Buffer{}
	ctx.Writer = resp.NewWriter(buf)
	_ = router.Execute(ctx)
	return buf.String()
}

func storedStr(t *testing.T, s *store.Store, key string) string {
	t.Helper()
	entry, exists := s.Get(key)
	if !exists {
		t.Fatalf("key %q disappeared", key)
	}
	sv, ok := entry.Value.(*store.StringValue)
	if !ok {
		t.Fatalf("key %q is not a string value", key)
	}
	return string(sv.Data)
}

func TestIncrByFloatInfRejected(t *testing.T) {
	s := store.NewStore()
	router := newFloatRouter(s)

	if err := s.Set("f", &store.StringValue{Data: []byte("1e308")}, store.SetOptions{}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	reply := runCmd(t, s, router, "INCRBYFLOAT", "f", "1e308")
	if !strings.Contains(reply, "NaN or Infinity") {
		t.Fatalf("overflowing INCRBYFLOAT must be rejected, got %q", reply)
	}
	if got := storedStr(t, s, "f"); got != "1e308" {
		t.Fatalf("rejected INCRBYFLOAT must leave the value unchanged, got %q", got)
	}
}

func TestIncrByFloatNaNIncrementRejected(t *testing.T) {
	s := store.NewStore()
	router := newFloatRouter(s)

	if err := s.Set("n", &store.StringValue{Data: []byte("0")}, store.SetOptions{}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	reply := runCmd(t, s, router, "INCRBYFLOAT", "n", "NaN")
	if !strings.Contains(reply, "NaN or Infinity") {
		t.Fatalf("NaN increment must be rejected, got %q", reply)
	}
	if got := storedStr(t, s, "n"); got != "0" {
		t.Fatalf("rejected INCRBYFLOAT must leave the value unchanged, got %q", got)
	}
}

func TestIncrByFloatFreshKeyInfRejected(t *testing.T) {
	s := store.NewStore()
	router := newFloatRouter(s)

	reply := runCmd(t, s, router, "INCRBYFLOAT", "fresh", "Inf")
	if !strings.Contains(reply, "NaN or Infinity") {
		t.Fatalf("Inf increment on a fresh key must be rejected, got %q", reply)
	}
	if _, exists := s.Get("fresh"); exists {
		t.Fatal("rejected INCRBYFLOAT must not create the key")
	}
}

func TestHIncrByFloatOverflowRejected(t *testing.T) {
	s := store.NewStore()
	router := newFloatRouter(s)

	// First increment on a fresh field lands exactly on 1e308 (legal).
	first := runCmd(t, s, router, "HINCRBYFLOAT", "h", "fld", "1e308")
	if strings.Contains(first, "Inf") || strings.Contains(first, "NaN") {
		t.Fatalf("first HINCRBYFLOAT must succeed, got %q", first)
	}

	// The second overflows: must be rejected, field unchanged.
	reply := runCmd(t, s, router, "HINCRBYFLOAT", "h", "fld", "1e308")
	if !strings.Contains(reply, "NaN or Infinity") {
		t.Fatalf("overflowing HINCRBYFLOAT must be rejected, got %q", reply)
	}
	if hget := runCmd(t, s, router, "HGET", "h", "fld"); hget != first {
		t.Fatalf("rejected HINCRBYFLOAT must leave the field unchanged, got %q want %q", hget, first)
	}
}

// Sanity: ordinary float increments are unaffected by the guard.
func TestIncrByFloatNormalOperation(t *testing.T) {
	s := store.NewStore()
	router := newFloatRouter(s)

	if reply := runCmd(t, s, router, "INCRBYFLOAT", "k", "10"); !strings.Contains(reply, "10") {
		t.Fatalf("fresh-key INCRBYFLOAT must create the value, got %q", reply)
	}
	if reply := runCmd(t, s, router, "INCRBYFLOAT", "k", "1.5"); !strings.Contains(reply, "11.5") {
		t.Fatalf("INCRBYFLOAT must accumulate, got %q", reply)
	}
}
