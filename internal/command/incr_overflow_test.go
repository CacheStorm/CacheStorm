package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: incrBy computed current+incr with unchecked int64 arithmetic,
// so INCR at math.MaxInt64 wrapped to math.MinInt64 (and DECR at MinInt64
// wrapped positive) — the wrapped value was stored and returned as if it
// were the true result. Redis rejects the increment instead and leaves the
// value unchanged.

func newIncrRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	return router
}

func runIncr(t *testing.T, s *store.Store, router *Router, cmd string, key string, delta string) string {
	t.Helper()
	args := [][]byte{[]byte(key)}
	if delta != "" {
		args = append(args, []byte(delta))
	}
	ctx := newTestContext(cmd, args, s)
	buf := &bytes.Buffer{}
	ctx.Writer = resp.NewWriter(buf)
	_ = router.Execute(ctx)
	return buf.String()
}

func storedInt(t *testing.T, s *store.Store, key string) string {
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

func TestIncrAtMaxInt64Overflows(t *testing.T) {
	s := store.NewStore()
	router := newIncrRouter(s)
	const maxInt64 = "9223372036854775807"

	if err := s.Set("ctr", &store.StringValue{Data: []byte(maxInt64)}, store.SetOptions{}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	reply := runIncr(t, s, router, "INCR", "ctr", "")
	if !strings.Contains(reply, "would overflow") {
		t.Fatalf("INCR at MaxInt64 must be rejected, got %q", reply)
	}
	if got := storedInt(t, s, "ctr"); got != maxInt64 {
		t.Fatalf("failed INCR must leave the value unchanged, got %q", got)
	}
}

func TestDecrAtMinInt64Overflows(t *testing.T) {
	s := store.NewStore()
	router := newIncrRouter(s)
	const minInt64 = "-9223372036854775808"

	if err := s.Set("ctr", &store.StringValue{Data: []byte(minInt64)}, store.SetOptions{}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	reply := runIncr(t, s, router, "DECR", "ctr", "")
	if !strings.Contains(reply, "would overflow") {
		t.Fatalf("DECR at MinInt64 must be rejected, got %q", reply)
	}
	if got := storedInt(t, s, "ctr"); got != minInt64 {
		t.Fatalf("failed DECR must leave the value unchanged, got %q", got)
	}
}

func TestIncrByAcrossBoundaryOverflows(t *testing.T) {
	s := store.NewStore()
	router := newIncrRouter(s)

	// Near-boundary increments that fit are fine…
	if err := s.Set("ok", &store.StringValue{Data: []byte("9223372036854775797")}, store.SetOptions{}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if reply := runIncr(t, s, router, "INCRBY", "ok", "10"); !strings.Contains(reply, ":9223372036854775807") {
		t.Fatalf("INCRBY landing exactly on MaxInt64 must succeed, got %q", reply)
	}

	// …and one step further must be rejected, not wrapped.
	if reply := runIncr(t, s, router, "INCRBY", "ok", "10"); !strings.Contains(reply, "would overflow") {
		t.Fatalf("INCRBY past MaxInt64 must be rejected, got %q", reply)
	}
	if got := storedInt(t, s, "ok"); got != "9223372036854775807" {
		t.Fatalf("failed INCRBY must leave the value unchanged, got %q", got)
	}
}

// Sanity: ordinary increments are unaffected by the overflow guard.
func TestIncrNormalOperation(t *testing.T) {
	s := store.NewStore()
	router := newIncrRouter(s)

	if reply := runIncr(t, s, router, "INCR", "n", ""); !strings.Contains(reply, ":1") {
		t.Fatalf("INCR on a fresh key must yield 1, got %q", reply)
	}
	if reply := runIncr(t, s, router, "INCRBY", "n", "41"); !strings.Contains(reply, ":42") {
		t.Fatalf("INCRBY must accumulate, got %q", reply)
	}
	if reply := runIncr(t, s, router, "DECR", "n", ""); !strings.Contains(reply, ":41") {
		t.Fatalf("DECR must decrement, got %q", reply)
	}
	if got := storedInt(t, s, "n"); got != "41" {
		t.Fatalf("final value drifted, got %q", got)
	}
}

// The transaction replay path re-implements the increment family; it must
// handle negative stored values correctly and reject boundary overflows.
func TestIncrInsideMultiHandlesNegativeAndOverflow(t *testing.T) {
	s := store.NewStore()
	router := newTxRouter(s)
	tx := NewTransaction()

	if err := s.Set("neg", &store.StringValue{Data: []byte("-5")}, store.SetOptions{}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	multiCtx, _ := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(multiCtx)

	incrCtx, incrBuf := mkTxContext(tx, "INCR", [][]byte{[]byte("neg")}, s)
	_ = router.Execute(incrCtx)

	execCtx, execBuf := mkTxContext(tx, "EXEC", nil, s)
	_ = router.Execute(execCtx)

	if !strings.Contains(incrBuf.String(), "QUEUED") {
		t.Fatalf("INCR inside MULTI must queue, got %q", incrBuf.String())
	}
	if !strings.Contains(execBuf.String(), ":-4") {
		t.Fatalf("INCR of -5 inside MULTI must yield -4, got %q", execBuf.String())
	}
	if got := storedInt(t, s, "neg"); got != "-4" {
		t.Fatalf("replayed INCR stored %q, want -4", got)
	}

	// Overflow inside a transaction must surface as an error value and leave
	// the counter unchanged.
	if err := s.Set("big", &store.StringValue{Data: []byte("9223372036854775807")}, store.SetOptions{}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	multi2Ctx, _ := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(multi2Ctx)

	incr2Ctx, _ := mkTxContext(tx, "INCR", [][]byte{[]byte("big")}, s)
	_ = router.Execute(incr2Ctx)

	exec2Ctx, exec2Buf := mkTxContext(tx, "EXEC", nil, s)
	_ = router.Execute(exec2Ctx)

	if !strings.Contains(exec2Buf.String(), "would overflow") {
		t.Fatalf("INCR past MaxInt64 inside MULTI must error, got %q", exec2Buf.String())
	}
	if got := storedInt(t, s, "big"); got != "9223372036854775807" {
		t.Fatalf("failed replayed INCR must leave the value unchanged, got %q", got)
	}
}
