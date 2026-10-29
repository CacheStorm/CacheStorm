package command

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: the typed/keyspace scan handlers accepted a NON-POSITIVE COUNT.
// A negative count flowed into
//
//	start := cursor
//	end   := start + count
//	result := make([]*resp.Value, 0, (end-start)*2)
//
// so a negative COUNT reached make() as a negative capacity and panicked with
// "runtime error: makeslice: cap out of range".
//
// This is NOT the negative-cursor defect fixed in round 34 (see
// scan_cursor_validation_test.go): cursor is a separate argument and is still
// rejected. COUNT is the work-size hint, and SSCAN already rejected count <= 0
// with ErrInvalidArg — HSCAN/ZSCAN/SCAN were never given the same guard.

// scanCountRun executes through the real router and turns a panic into a
// returned value, so the failure reason names the defect instead of crashing
// the run. A setup/parse failure is reported separately and is NOT a pass.
//
// The reply is held in a local, not a package-level variable: when a panic
// unwinds, a shared variable would still hold the PREVIOUS command's reply and
// a later assertion would read stale data instead of reporting the panic.
func scanCountRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) (*resp.Value, interface{}) {
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

// scanCountRouter registers every scan handler touched by this defect, plus
// the writers used to seed them (SET lives in the string module).
func scanCountRouter() *Router {
	router := NewRouter()
	RegisterHashCommands(router)
	RegisterSortedSetCommands(router)
	RegisterSetCommands(router)
	RegisterServerCommands(router)
	RegisterStringCommands(router)
	return router
}

func scanCountSeedHash(t *testing.T, s *store.Store, router *Router, n int) {
	t.Helper()
	fields := make([]string, 0, 2*n)
	for i := 0; i < n; i++ {
		fields = append(fields, fmt.Sprintf("f%02d", i), fmt.Sprintf("v%02d", i))
	}
	scanCountMustNotPanic(t, s, router, "HSET", append([]string{"h"}, fields...)...)
}

func scanCountSeedZset(t *testing.T, s *store.Store, router *Router, n int) {
	t.Helper()
	// ZADD takes score-member pairs, so the score comes first: append the
	// score "1" then member i, or every pair collapses onto member "1".
	members := make([]string, 0, 2*n)
	for i := 0; i < n; i++ {
		members = append(members, "1", fmt.Sprintf("m%02d", i))
	}
	scanCountMustNotPanic(t, s, router, "ZADD", append([]string{"z"}, members...)...)
}

func scanCountMustNotPanic(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) *resp.Value {
	t.Helper()
	v, p := scanCountRun(t, s, router, cmd, args...)
	if p != nil {
		t.Fatalf("FAIL %s %v panicked: %v — COUNT flows into end = start + count and\n"+
			"then into make() as a negative capacity", cmd, args, p)
	}
	return v
}

// scanCountRejects asserts the command answers with an error rather than
// panicking or silently succeeding.
func scanCountRejects(t *testing.T, s *store.Store, router *Router, cmd, key string, count string) {
	t.Helper()
	reply := scanCountMustNotPanic(t, s, router, cmd, key, "0", "COUNT", count)
	if reply.Type != resp.TypeError {
		t.Fatalf("FAIL %s %s 0 COUNT %s returned type %v, want an error", cmd, key, count, reply.Type)
	}
}

// TestScanVariantsRejectNonPositiveCount is the primary case: every scan
// handler must reject a non-positive COUNT instead of panicking.
func TestScanVariantsRejectNonPositiveCount(t *testing.T) {
	router := scanCountRouter()

	for _, count := range []string{"-1", "-5", "-2147483648", "0"} {
		s := store.NewStore()
		scanCountSeedHash(t, s, router, 4)
		scanCountSeedZset(t, s, router, 4)
		for i := 0; i < 4; i++ {
			scanCountMustNotPanic(t, s, router, "SET", fmt.Sprintf("k%02d", i), "v")
		}

		scanCountRejects(t, s, router, "HSCAN", "h", count)
		scanCountRejects(t, s, router, "ZSCAN", "z", count)

		// SCAN takes the cursor first and has no key argument.
		reply := scanCountMustNotPanic(t, s, router, "SCAN", "0", "COUNT", count)
		if reply.Type != resp.TypeError {
			t.Fatalf("FAIL SCAN 0 COUNT %s returned type %v, want an error", count, reply.Type)
		}
	}
}

// TestSSCANStillRejectsNonPositiveCount pins the pre-existing basis the fix
// mirrors, so that guard cannot silently regress.
func TestSSCANStillRejectsNonPositiveCount(t *testing.T) {
	router := scanCountRouter()
	s := store.NewStore()
	for _, m := range []string{"a", "b", "c"} {
		scanCountMustNotPanic(t, s, router, "SADD", "s", m)
	}
	scanCountRejects(t, s, router, "SSCAN", "s", "-5")
	scanCountRejects(t, s, router, "SSCAN", "s", "0")
}

// TestScanPositiveCountControls is the control: valid COUNT values and the
// no-COUNT default must keep working, so a rejection above is the fix and not a
// broken harness or an over-broad guard.
func TestScanPositiveCountControls(t *testing.T) {
	router := scanCountRouter()

	s := store.NewStore()
	scanCountSeedHash(t, s, router, 4)
	scanCountSeedZset(t, s, router, 4)
	for _, m := range []string{"a", "b", "c"} {
		scanCountMustNotPanic(t, s, router, "SADD", "s", m)
	}

	// HSCAN emits field AND value, so 4 fields == 8 elements.
	v := scanCountMustNotPanic(t, s, router, "HSCAN", "h", "0", "COUNT", "1000")
	if v.Type != resp.TypeArray || len(v.Array) != 2 {
		t.Fatalf("FAIL control: HSCAN COUNT 1000 returned %v, want [cursor, entries]", v)
	}
	if got := len(v.Array[1].Array); got != 8 {
		t.Fatalf("FAIL control: HSCAN COUNT 1000 returned %d elements, want 8", got)
	}

	// ZSCAN emits member AND score: 4 members == 8 elements.
	v = scanCountMustNotPanic(t, s, router, "ZSCAN", "z", "0", "COUNT", "1000")
	if got := len(v.Array[1].Array); got != 8 {
		t.Fatalf("FAIL control: ZSCAN COUNT 1000 returned %d elements, want 8", got)
	}

	// SSCAN emits members only: 3 members == 3 elements.
	v = scanCountMustNotPanic(t, s, router, "SSCAN", "s", "0", "COUNT", "1000")
	if got := len(v.Array[1].Array); got != 3 {
		t.Fatalf("FAIL control: SSCAN COUNT 1000 returned %d elements, want 3", got)
	}

	// The no-COUNT default form still works for each handler.
	for _, tc := range []struct{ cmd, key string }{{"HSCAN", "h"}, {"ZSCAN", "z"}, {"SSCAN", "s"}} {
		v = scanCountMustNotPanic(t, s, router, tc.cmd, tc.key, "0")
		if v.Type != resp.TypeArray || len(v.Array) != 2 {
			t.Fatalf("FAIL control: %s with no COUNT returned %v, want [cursor, entries]", tc.cmd, v)
		}
	}
	v = scanCountMustNotPanic(t, s, router, "SCAN", "0")
	if v.Type != resp.TypeArray || len(v.Array) != 2 {
		t.Fatalf("FAIL control: SCAN with no COUNT returned %v, want [cursor, keys]", v)
	}

	// A small collection comes back whole with cursor 0, like a Redis
	// listpack; a large one pages on a positive COUNT.
	v = scanCountMustNotPanic(t, s, router, "HSCAN", "h", "0", "COUNT", "2")
	if got := len(v.Array[1].Array); got != 8 || string(v.Array[0].Bulk) != "0" {
		t.Fatalf("FAIL control: HSCAN COUNT 2 over 4 fields returned %d elements, cursor %q; want 8, \"0\"", got, v.Array[0].Bulk)
	}
	large := store.NewStore()
	// More fields than hash-max-listpack-entries: a hash table, which pages.
	scanCountSeedHash(t, large, router, 600)
	v = scanCountMustNotPanic(t, large, router, "HSCAN", "h", "0", "COUNT", "2")
	if got := len(v.Array[1].Array); got != 4 {
		t.Fatalf("FAIL control: HSCAN COUNT 2 returned %d elements, want 4 (2 field/value pairs)", got)
	}
	if string(v.Array[0].Bulk) == "0" {
		t.Fatalf("FAIL control: HSCAN COUNT 2 over 600 fields should return a non-zero cursor, got 0")
	}
}
