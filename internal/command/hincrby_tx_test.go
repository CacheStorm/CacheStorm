package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: executeQueuedCommand had no HINCRBY case — HINCRBY inside
// MULTI queued successfully but failed at EXEC with "ERR command not
// supported in transaction", while every other increment command replayed
// correctly. Redis queues and executes HINCRBY at EXEC like any other
// command.

func newHashTxRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterTransactionCommands(router)
	RegisterHashCommands(router)
	return router
}

func TestHIncrByInsideMultiReplays(t *testing.T) {
	s := store.NewStore()
	router := newHashTxRouter(s)
	tx := NewTransaction()

	multiCtx, _ := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(multiCtx)

	incrCtx, incrBuf := mkTxContext(tx, "HINCRBY", [][]byte{[]byte("h"), []byte("f"), []byte("5")}, s)
	_ = router.Execute(incrCtx)

	execCtx, execBuf := mkTxContext(tx, "EXEC", nil, s)
	_ = router.Execute(execCtx)

	if !strings.Contains(incrBuf.String(), "QUEUED") {
		t.Fatalf("HINCRBY inside MULTI must queue, got %q", incrBuf.String())
	}
	if !strings.Contains(execBuf.String(), ":5") {
		t.Fatalf("replayed HINCRBY must yield :5, got %q", execBuf.String())
	}
	if got := runCmd(t, s, router, "HGET", "h", "f"); !strings.Contains(got, "5") {
		t.Fatalf("replayed HINCRBY stored %q, want 5", got)
	}
}

func TestHIncrByInsideMultiAccumulates(t *testing.T) {
	s := store.NewStore()
	router := newHashTxRouter(s)
	tx := NewTransaction()

	if reply := runCmd(t, s, router, "HSET", "h", "f", "10"); reply == "" {
		t.Fatal("HSET setup failed")
	}

	multiCtx, _ := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(multiCtx)

	incrCtx, _ := mkTxContext(tx, "HINCRBY", [][]byte{[]byte("h"), []byte("f"), []byte("-3")}, s)
	_ = router.Execute(incrCtx)

	execCtx, execBuf := mkTxContext(tx, "EXEC", nil, s)
	_ = router.Execute(execCtx)

	if !strings.Contains(execBuf.String(), ":7") {
		t.Fatalf("replayed HINCRBY must yield :7, got %q", execBuf.String())
	}
	if got := runCmd(t, s, router, "HGET", "h", "f"); !strings.Contains(got, "7") {
		t.Fatalf("field drifted, got %q", got)
	}
}

func TestHIncrByInsideMultiOverflowAborts(t *testing.T) {
	s := store.NewStore()
	router := newHashTxRouter(s)
	tx := NewTransaction()
	const maxInt64 = "9223372036854775807"

	if reply := runCmd(t, s, router, "HSET", "h", "f", maxInt64); reply == "" {
		t.Fatal("HSET setup failed")
	}

	multiCtx, _ := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(multiCtx)

	incrCtx, _ := mkTxContext(tx, "HINCRBY", [][]byte{[]byte("h"), []byte("f"), []byte("1")}, s)
	_ = router.Execute(incrCtx)

	execCtx, execBuf := mkTxContext(tx, "EXEC", nil, s)
	_ = router.Execute(execCtx)

	if !strings.Contains(execBuf.String(), "would overflow") {
		t.Fatalf("overflowing HINCRBY replay must error inside the array, got %q", execBuf.String())
	}
	if got := runCmd(t, s, router, "HGET", "h", "f"); !strings.Contains(got, maxInt64) {
		t.Fatalf("aborted increment must leave the field unchanged, got %q", got)
	}
}
