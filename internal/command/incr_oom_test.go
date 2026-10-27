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

func newOOMStore(t *testing.T) (*store.Store, *Router) {
	t.Helper()
	s := store.NewStore()
	s.ConfigureMemory(10000, store.EvictionNoEviction, 80, 90, 5)
	if err := s.Set("counter", &store.StringValue{Data: []byte("99")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("filler", &store.StringValue{}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	length := int(9499 - s.MemUsage())
	if length <= 0 {
		t.Fatal("memory-pressure setup changed")
	}
	if err := s.Set("filler", &store.StringValue{Data: make([]byte, length)}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	if s.MemoryTracker().Usage() != 9499 {
		t.Fatal("memory-pressure setup changed")
	}
	return s, newTxRouter(s)
}

func TestIncrReportsOOMRejection(t *testing.T) {
	s, router := newOOMStore(t)

	reply := runIncr(t, s, router, "INCR", "counter", "")
	if !strings.Contains(reply, "OOM") {
		t.Fatalf("INCR under OOM rejection must propagate the error, got %q", reply)
	}
	if got := storedStr(t, s, "counter"); got != "99" {
		t.Fatalf("rejected INCR must leave the counter unchanged, got %q", got)
	}
}

func TestIncrByFloatReportsOOMRejection(t *testing.T) {
	s, router := newOOMStore(t)

	reply := runCmd(t, s, router, "INCRBYFLOAT", "counter", "1")
	if !strings.Contains(reply, "OOM") {
		t.Fatalf("INCRBYFLOAT under OOM rejection must propagate the error, got %q", reply)
	}
	if got := storedStr(t, s, "counter"); got != "99" {
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
	if got := storedStr(t, s, "counter"); got != "99" {
		t.Fatalf("rejected replayed INCR must leave the counter unchanged, got %q", got)
	}
}

func TestSameSizeCounterWriteDoesNotNeedAdditionalMemory(t *testing.T) {
	s, r := newOOMStore(t)
	before := s.MemUsage()
	if got := runCmd(t, s, r, "DECR", "counter"); got != ":98\r\n" {
		t.Fatalf("same-size decrement=%q", got)
	}
	if s.MemUsage() != before || s.MemoryTracker().Usage() != before {
		t.Fatal("same-size write changed accounting")
	}
}
