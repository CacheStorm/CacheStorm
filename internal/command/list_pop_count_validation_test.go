package command

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: LMPOP and BLMPOP accepted a non-positive COUNT with no
// validation and passed it straight to
//
//	elements := make([]*resp.Value, 0, count)
//
// A negative COUNT became a negative make() capacity and panicked with
// "runtime error: makeslice: cap out of range"; COUNT 0 silently returned an
// empty element array while leaving the list untouched.
//
// The sorted-set siblings ZMPOP and BZMPOP have the identical
// numkeys/LEFT|RIGHT/COUNT grammar and already enforce `if count < 1`. LMPOP
// and BLMPOP were never given that guard.
//
// Note the threshold differs from the scan COUNT fixed in round 2: a scan
// COUNT is a work-size HINT where 0 is tolerated, whereas a pop COUNT is a
// hard element count that must be at least 1.

// listCountRun executes through the real router and turns a panic into a
// returned value so the reason names the defect instead of crashing the run.
// The reply is a local, not a package-level variable: when a panic unwinds a
// shared variable would still hold the PREVIOUS command's reply and a later
// assertion would read stale data instead of reporting the panic.
func listCountRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) (*resp.Value, interface{}) {
	t.Helper()

	var (
		reply    *resp.Value
		panicked interface{}
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = r
			}
		}()

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
		reply = v
	}()
	return reply, panicked
}

func listCountRouter() *Router {
	router := NewRouter()
	RegisterListCommands(router)
	RegisterSortedSetCommands(router)
	return router
}

// listCountSeedList pushes n elements so the handler reaches the make() under test.
func listCountSeedList(t *testing.T, s *store.Store, router *Router, key string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		listCountMustNotPanic(t, s, router, "RPUSH", key, fmt.Sprintf("e%d", i))
	}
}

func listCountMustNotPanic(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) *resp.Value {
	t.Helper()
	v, p := listCountRun(t, s, router, cmd, args...)
	if p != nil {
		t.Fatalf("FAIL %s %v panicked: %v — COUNT flows into\n"+
			"make([]*resp.Value, 0, count) as a negative capacity", cmd, args, p)
	}
	return v
}

func listCountRejects(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) {
	t.Helper()
	reply := listCountMustNotPanic(t, s, router, cmd, args...)
	if reply.Type != resp.TypeError {
		t.Fatalf("FAIL %s %v returned type %v, want an error", cmd, args, reply.Type)
	}
}

// TestListPopCommandsRejectNonPositiveCount is the primary case: LMPOP and
// BLMPOP must reject a non-positive COUNT instead of panicking (negative) or
// silently popping nothing (zero).
func TestListPopCommandsRejectNonPositiveCount(t *testing.T) {
	router := listCountRouter()

	for _, count := range []string{"-1", "-5", "-9223372036854775808", "0"} {
		s := store.NewStore()
		listCountSeedList(t, s, router, "l", 3)

		listCountRejects(t, s, router, "LMPOP", "1", "l", "LEFT", "COUNT", count)
		// The rejected LMPOP must not have popped anything.
		if got := listCountMustNotPanic(t, s, router, "LLEN", "l").Int; got != 3 {
			t.Fatalf("FAIL: LLEN after rejected LMPOP COUNT %s = %d, want 3", count, got)
		}

		// BLMPOP takes a leading timeout; arg index 2 is the numkeys token.
		listCountRejects(t, s, router, "BLMPOP", "0", "1", "1", "l", "LEFT", "COUNT", count)
		if got := listCountMustNotPanic(t, s, router, "LLEN", "l").Int; got != 3 {
			t.Fatalf("FAIL: LLEN after rejected BLMPOP COUNT %s = %d, want 3", count, got)
		}
	}
}

// TestSortedSetPopStillRejectsNonPositiveCount pins the in-repo basis the fix
// mirrors, so ZMPOP/BZMPOP's existing guard cannot silently regress.
func TestSortedSetPopStillRejectsNonPositiveCount(t *testing.T) {
	router := listCountRouter()

	s := store.NewStore()
	listCountMustNotPanic(t, s, router, "ZADD", "z", "1", "a", "2", "b", "3", "c")

	for _, count := range []string{"-1", "0"} {
		listCountRejects(t, s, router, "ZMPOP", "1", "z", "MIN", "COUNT", count)
		if got := listCountMustNotPanic(t, s, router, "ZCARD", "z").Int; got != 3 {
			t.Fatalf("FAIL: ZCARD after rejected ZMPOP COUNT %s = %d, want 3", count, got)
		}
	}
}

// TestListPopControls is the control: a valid positive COUNT must still pop
// exactly COUNT elements from the correct end, and the no-COUNT default must
// pop one. A rejection above is then the fix, not an over-broad guard.
func TestListPopControls(t *testing.T) {
	router := listCountRouter()

	s := store.NewStore()
	listCountSeedList(t, s, router, "l", 5)

	// LEFT, COUNT 2 pops e0 then e1.
	v := listCountMustNotPanic(t, s, router, "LMPOP", "1", "l", "LEFT", "COUNT", "2")
	if v.Type != resp.TypeArray || len(v.Array) != 2 {
		t.Fatalf("FAIL control: LMPOP returned %v, want [key, elements]", v)
	}
	if string(v.Array[0].Bulk) != "l" {
		t.Fatalf("FAIL control: LMPOP returned key %q, want \"l\"", v.Array[0].Bulk)
	}
	if got := len(v.Array[1].Array); got != 2 {
		t.Fatalf("FAIL control: LMPOP COUNT 2 popped %d elements, want 2", got)
	}
	if got := string(v.Array[1].Array[0].Bulk); got != "e0" {
		t.Fatalf("FAIL control: LEFT pop returned %q first, want e0", got)
	}
	if got := listCountMustNotPanic(t, s, router, "LLEN", "l").Int; got != 3 {
		t.Fatalf("FAIL control: LLEN after LMPOP COUNT 2 = %d, want 3", got)
	}

	// RIGHT pops from the tail: e4.
	v = listCountMustNotPanic(t, s, router, "LMPOP", "1", "l", "RIGHT", "COUNT", "1")
	if got := string(v.Array[1].Array[0].Bulk); got != "e4" {
		t.Fatalf("FAIL control: RIGHT pop returned %q, want e4", got)
	}

	// The no-COUNT default pops exactly one.
	v = listCountMustNotPanic(t, s, router, "LMPOP", "1", "l", "LEFT")
	if got := len(v.Array[1].Array); got != 1 {
		t.Fatalf("FAIL control: default LMPOP popped %d elements, want 1", got)
	}
	if got := listCountMustNotPanic(t, s, router, "LLEN", "l").Int; got != 1 {
		t.Fatalf("FAIL control: LLEN after default LMPOP = %d, want 1", got)
	}

	// COUNT larger than the list pops everything that remains.
	v = listCountMustNotPanic(t, s, router, "LMPOP", "1", "l", "LEFT", "COUNT", "100")
	if got := len(v.Array[1].Array); got != 1 {
		t.Fatalf("FAIL control: LMPOP COUNT 100 over 1 element popped %d, want 1", got)
	}
}
