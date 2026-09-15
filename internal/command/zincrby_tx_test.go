package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: executeQueuedCommand had no ZINCRBY case — ZINCRBY inside
// MULTI queued successfully but failed at EXEC with "ERR command not
// supported in transaction", while every other increment command replayed
// correctly. Redis queues and executes ZINCRBY at EXEC like any other
// command.

func newZSetTxRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterTransactionCommands(router)
	RegisterSortedSetCommands(router)
	return router
}

func TestZIncrByInsideMultiReplays(t *testing.T) {
	s := store.NewStore()
	router := newZSetTxRouter(s)
	tx := NewTransaction()

	multiCtx, _ := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(multiCtx)

	incrCtx, incrBuf := mkTxContext(tx, "ZINCRBY", [][]byte{[]byte("z"), []byte("2.5"), []byte("m")}, s)
	_ = router.Execute(incrCtx)

	execCtx, execBuf := mkTxContext(tx, "EXEC", nil, s)
	_ = router.Execute(execCtx)

	if !strings.Contains(incrBuf.String(), "QUEUED") {
		t.Fatalf("ZINCRBY inside MULTI must queue, got %q", incrBuf.String())
	}
	if !strings.Contains(execBuf.String(), "2.5") {
		t.Fatalf("replayed ZINCRBY must yield 2.5, got %q", execBuf.String())
	}
	if got := runCmd(t, s, router, "ZSCORE", "z", "m"); !strings.Contains(got, "2.5") {
		t.Fatalf("replayed ZINCRBY stored %q, want 2.5", got)
	}
}

func TestZIncrByInsideMultiAccumulates(t *testing.T) {
	s := store.NewStore()
	router := newZSetTxRouter(s)
	tx := NewTransaction()

	if reply := runCmd(t, s, router, "ZADD", "z", "1", "m"); reply == "" {
		t.Fatal("ZADD setup failed")
	}

	multiCtx, _ := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(multiCtx)

	incrCtx, _ := mkTxContext(tx, "ZINCRBY", [][]byte{[]byte("z"), []byte("2"), []byte("m")}, s)
	_ = router.Execute(incrCtx)

	execCtx, execBuf := mkTxContext(tx, "EXEC", nil, s)
	_ = router.Execute(execCtx)

	if !strings.Contains(execBuf.String(), "3") {
		t.Fatalf("replayed ZINCRBY must yield 3, got %q", execBuf.String())
	}
	if got := runCmd(t, s, router, "ZSCORE", "z", "m"); !strings.Contains(got, "3") {
		t.Fatalf("score drifted, got %q", got)
	}
}

func TestZIncrByInsideMultiOverflowAborts(t *testing.T) {
	s := store.NewStore()
	router := newZSetTxRouter(s)
	tx := NewTransaction()

	if reply := runCmd(t, s, router, "ZADD", "z", "1e308", "m"); reply == "" {
		t.Fatal("ZADD setup failed")
	}
	before := runCmd(t, s, router, "ZSCORE", "z", "m")

	multiCtx, _ := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(multiCtx)

	incrCtx, _ := mkTxContext(tx, "ZINCRBY", [][]byte{[]byte("z"), []byte("1e308"), []byte("m")}, s)
	_ = router.Execute(incrCtx)

	execCtx, execBuf := mkTxContext(tx, "EXEC", nil, s)
	_ = router.Execute(execCtx)

	if !strings.Contains(execBuf.String(), "would produce NaN") {
		t.Fatalf("overflowing ZINCRBY replay must error inside the array, got %q", execBuf.String())
	}
	if after := runCmd(t, s, router, "ZSCORE", "z", "m"); after != before {
		t.Fatalf("aborted increment must leave the score unchanged, got %q, want %q", after, before)
	}
}
