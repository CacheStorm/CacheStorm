package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: Router.Execute never intercepted commands while MULTI was
// active — every command executed immediately, so transactions had no
// queuing, no atomicity, and DISCARD could not roll anything back; WATCH's
// optimistic lock was unreachable. cmdEXEC's replay machinery and the QUEUED
// reply were dead code behind the missing interception.

// mkTxContext builds a context with a buffer-backed writer sharing tx.
func mkTxContext(tx *Transaction, cmd string, args [][]byte, s *store.Store) (*Context, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	ctx := newTestContext(cmd, args, s)
	ctx.Writer = resp.NewWriter(buf)
	ctx.Transaction = tx
	return ctx, buf
}

func newTxRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterTransactionCommands(router)
	RegisterStringCommands(router)
	return router
}

func TestMultiQueuesCommandsAndDiscardRollsBack(t *testing.T) {
	s := store.NewStore()
	router := newTxRouter(s)
	tx := NewTransaction()

	ctx, _ := mkTxContext(tx, "MULTI", nil, s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("MULTI: %v", err)
	}

	setCtx, setBuf := mkTxContext(tx, "SET", [][]byte{[]byte("tk"), []byte("v")}, s)
	_ = router.Execute(setCtx)
	if !strings.Contains(setBuf.String(), "QUEUED") {
		t.Fatalf("SET inside MULTI answered %q, want +QUEUED — commands execute immediately instead of queuing", setBuf.String())
	}

	discardCtx, _ := mkTxContext(tx, "DISCARD", nil, s)
	_ = router.Execute(discardCtx)

	if _, exists := s.Get("tk"); exists {
		t.Fatal("DISCARD did not roll back: a command executed immediately inside MULTI stayed applied")
	}
}

func TestExecReplaysQueuedCommandsInOrder(t *testing.T) {
	s := store.NewStore()
	router := newTxRouter(s)
	tx := NewTransaction()

	ctx, _ := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(ctx)

	setACtx, setABuf := mkTxContext(tx, "SET", [][]byte{[]byte("ta"), []byte("1")}, s)
	_ = router.Execute(setACtx)
	if strings.Contains(setABuf.String(), "OK") && !strings.Contains(setABuf.String(), "QUEUED") {
		t.Fatalf("first queued command executed immediately (reply %q)", setABuf.String())
	}
	if _, exists := s.Get("ta"); exists {
		t.Fatal("queued command ran before EXEC — no atomicity")
	}

	setBCtx, _ := mkTxContext(tx, "SET", [][]byte{[]byte("tb"), []byte("2")}, s)
	_ = router.Execute(setBCtx)

	execCtx, execBuf := mkTxContext(tx, "EXEC", nil, s)
	_ = router.Execute(execCtx)

	if !strings.Contains(execBuf.String(), "*2") {
		t.Fatalf("EXEC reply must be a 2-element array of queued replies, got %q", execBuf.String())
	}
	if _, exists := s.Get("ta"); !exists {
		t.Fatal("EXEC must replay queued command ta")
	}
	if _, exists := s.Get("tb"); !exists {
		t.Fatal("EXEC must replay queued command tb")
	}
}

func TestWatchAbortsWhenWatchedKeyModified(t *testing.T) {
	s := store.NewStore()
	router := newTxRouter(s)
	tx := NewTransaction()

	if err := s.Set("w", &store.StringValue{Data: []byte("v1")}, store.SetOptions{}); err != nil {
		t.Fatalf("setup Set: %v", err)
	}

	watchCtx, _ := mkTxContext(tx, "WATCH", [][]byte{[]byte("w")}, s)
	if err := router.Execute(watchCtx); err != nil {
		t.Fatalf("WATCH: %v", err)
	}

	// Another client modifies the watched key immediately, before MULTI.
	if err := s.Set("w", &store.StringValue{Data: []byte("v-mid")}, store.SetOptions{}); err != nil {
		t.Fatalf("racing Set: %v", err)
	}

	multiCtx, _ := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(multiCtx)

	setCtx, _ := mkTxContext(tx, "SET", [][]byte{[]byte("w"), []byte("v2")}, s)
	_ = router.Execute(setCtx)

	execCtx, execBuf := mkTxContext(tx, "EXEC", nil, s)
	_ = router.Execute(execCtx)

	if execBuf.String() != "_\r\n" {
		t.Fatalf("EXEC must abort with a null reply (the writer's RESP3 _) when a watched key changed before EXEC, got %q", execBuf.String())
	}
	entry, exists := s.Get("w")
	if !exists {
		t.Fatal("aborted transaction must leave the watched key in place")
	}
	if sv, ok := entry.Value.(*store.StringValue); !ok || string(sv.Data) != "v-mid" {
		t.Fatal("watched key was overwritten by an aborted transaction — optimistic locking is not enforced")
	}
}

func TestExecFlushesWatchesOnSuccess(t *testing.T) {
	s := store.NewStore()
	router := newTxRouter(s)
	tx := NewTransaction()

	if err := s.Set("fw", &store.StringValue{Data: []byte("v1")}, store.SetOptions{}); err != nil {
		t.Fatalf("setup Set: %v", err)
	}

	watchCtx, _ := mkTxContext(tx, "WATCH", [][]byte{[]byte("fw")}, s)
	_ = router.Execute(watchCtx)

	multiCtx, _ := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(multiCtx)

	otherCtx, _ := mkTxContext(tx, "SET", [][]byte{[]byte("other"), []byte("x")}, s)
	_ = router.Execute(otherCtx)

	execCtx, _ := mkTxContext(tx, "EXEC", nil, s)
	_ = router.Execute(execCtx) // succeeds — fw untouched

	// The successful EXEC flushed the watch: modifying fw afterwards must
	// not abort the next transaction.
	if err := s.Set("fw", &store.StringValue{Data: []byte("v2")}, store.SetOptions{}); err != nil {
		t.Fatalf("untracked Set: %v", err)
	}

	multi2Ctx, _ := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(multi2Ctx)

	setCtx, _ := mkTxContext(tx, "SET", [][]byte{[]byte("zz"), []byte("z")}, s)
	_ = router.Execute(setCtx)

	exec2Ctx, _ := mkTxContext(tx, "EXEC", nil, s)
	_ = router.Execute(exec2Ctx)

	if _, exists := s.Get("zz"); !exists {
		t.Fatal("stale watch from a previous transaction aborted an unrelated transaction — EXEC must flush watches on success")
	}
}

func TestNestedMultiRejected(t *testing.T) {
	s := store.NewStore()
	router := newTxRouter(s)
	tx := NewTransaction()

	ctx, _ := mkTxContext(tx, "MULTI", nil, s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("MULTI: %v", err)
	}

	nestedCtx, nestedBuf := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(nestedCtx)
	if !strings.Contains(nestedBuf.String(), "MULTI calls can not be nested") {
		t.Fatalf("nested MULTI answered %q, want the nesting error", nestedBuf.String())
	}
}
