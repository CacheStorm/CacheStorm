package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: the increment family ignored Store.Set's error return. Under
// max_memory pressure with noeviction, Store.Set rejects the write with
// ErrMemoryLimit, but INCR/INCRBYFLOAT replied with the new value and the
// MULTI/EXEC replay surfaced it as a success — the client believed the
// counter advanced while the store still held the old value. cmdSET already
// propagated the error; the increment handlers must too.

// newOOMStore builds a saturated store: a small noeviction limit configured
// before any writes, a counter at 41, and enough filler keys to push tracked
// usage to the limit so any further write is rejected.
func newOOMStore(t *testing.T) (*store.Store, *Router) {
	t.Helper()
	s := store.NewStore()
	// The limit must hold the counter plus the filler keys, and be small
	// enough that the fill below saturates it.
	s.ConfigureMemory(70000, store.EvictionNoEviction, 80, 90, 5)

	router := newTxRouter(s)

	// The counter under test, before the fill.
	runCmd(t, s, router, "SET", "counter", "41")

	// One large filler, then many small distinct-key fillers (overwrites do
	// not grow tracked usage). 300 x ~100 tracked bytes saturates the limit;
	// failed fillers are harmless noise.
	runCmd(t, s, router, "SET", "filler-big", strings.Repeat("x", 65536))
	for i := 0; i < 300; i++ {
		runCmd(t, s, router, "SET", "f"+itoa(i), "yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy")
	}
	return s, router
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for ; i > 0; i /= 10 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
	}
	return string(digits)
}

func TestIncrReportsOOMRejection(t *testing.T) {
	s, router := newOOMStore(t)

	reply := runIncr(t, s, router, "INCR", "counter", "")
	if !strings.Contains(reply, "OOM") {
		t.Fatalf("INCR under OOM rejection must propagate the error, got %q", reply)
	}
	if got := storedStr(t, s, "counter"); got != "41" {
		t.Fatalf("rejected INCR must leave the counter unchanged, got %q", got)
	}
}

func TestIncrByFloatReportsOOMRejection(t *testing.T) {
	s, router := newOOMStore(t)

	reply := runCmd(t, s, router, "INCRBYFLOAT", "counter", "1")
	if !strings.Contains(reply, "OOM") {
		t.Fatalf("INCRBYFLOAT under OOM rejection must propagate the error, got %q", reply)
	}
	if got := storedStr(t, s, "counter"); got != "41" {
		t.Fatalf("rejected INCRBYFLOAT must leave the counter unchanged, got %q", got)
	}
}

func TestReplayedIncrReportsOOMRejection(t *testing.T) {
	s, router := newOOMStore(t)
	tx := NewTransaction()

	multiCtx, _ := mkTxContext(tx, "MULTI", nil, s)
	_ = router.Execute(multiCtx)

	incrCtx, _ := mkTxContext(tx, "INCR", [][]byte{[]byte("counter")}, s)
	_ = router.Execute(incrCtx)

	execCtx, execBuf := mkTxContext(tx, "EXEC", nil, s)
	_ = router.Execute(execCtx)

	if !strings.Contains(execBuf.String(), "OOM") {
		t.Fatalf("replayed INCR under OOM rejection must surface the error, got %q", execBuf.String())
	}
	if got := storedStr(t, s, "counter"); got != "41" {
		t.Fatalf("rejected replayed INCR must leave the counter unchanged, got %q", got)
	}
}
