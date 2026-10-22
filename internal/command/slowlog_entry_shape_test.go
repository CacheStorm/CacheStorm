package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Deliberate version-gating decision, recorded 2026-10-09: SLOWLOG GET emits SIX
// elements, matching Redis < 8.10. This is intentional, not an oversight.
//
// Redis documents seven values per entry (SLOWLOG GET docs, "Return information"),
// where element 7 is:
//
//	"The total number of arguments in the command, including the command name.
//	 Added in Redis 8.10."
//
// Its stated purpose is narrow and entirely about TRUNCATION:
//
//	"The logged argument array (element 4) is truncated when the command has more
//	 arguments than the slowlog-max-argc configuration directive allows (32 by
//	 default). In that case, the last logged element is replaced with a string of
//	 the form ... (N more arguments). The total argument count (element 7) always
//	 reflects the command's ORIGINAL argument count, captured before truncation,
//	 so it can be read directly without parsing that string."
//
// Element 7 is the escape hatch for a truncated element 4 -- it exists so a
// client need not parse the "... (N more arguments)" marker. CacheStorm has no
// `slowlog-max-argc` directive and never truncates element 4: `cmdSLOWLOG` emits
// every recorded argument. Verified empirically -- a SADD carrying 200 arguments
// yields element 4 with 201 entries and no truncation marker.
//
// Therefore an element 7 here would be arithmetically IDENTICAL to len(element 4)
// (element 4 already begins with the command name, which cmdSLOWLOG prepends).
// It would carry zero information while breaking every client that indexes the
// array positionally against the pre-8.10 six-element layout.
//
// If CacheStorm later adopts slowlog-max-argc truncation, this decision must be
// revisited in the SAME change: that is the only condition under which element 7
// becomes meaningful. Emitting element 7 without truncation would be pure
// duplication, and emitting it alone would be a wire-format break with no upside.

// slowlogGateRecorder puts the shared recorder in a known state and restores it.
func slowlogGateRecorder(t *testing.T) {
	t.Helper()
	sl := store.GlobalSlowLog
	sl.Clear()
	sl.SetEnabled(true)
	sl.SetThreshold(0)
	t.Cleanup(func() { sl.Clear(); sl.SetThreshold(10 * time.Millisecond) })
}

// slowlogGateEntry records one entry with nArgs arguments and returns the
// parsed SLOWLOG GET entry.
func slowlogGateEntry(t *testing.T, nArgs int) *resp.Value {
	t.Helper()
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	args := make([][]byte, nArgs)
	for i := range args {
		args[i] = []byte("m")
	}
	store.GlobalSlowLog.AddIfSlow(60*time.Millisecond, "SADD", args, "10.0.0.1:1", "gate")

	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext("SLOWLOG", [][]byte{[]byte("GET")}, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("SLOWLOG GET: %v", err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil || v.Type != resp.TypeArray || len(v.Array) == 0 {
		t.Fatalf("SLOWLOG GET produced no entry: %v %q", err, buf.String())
	}
	return v.Array[0]
}

// TestSlowlogEntryShapeIsSixElements pins the deliberate six-element layout for
// the Redis < 8.10 compatibility target: id, unix-time (seconds), duration
// (microseconds), args, client addr, client name.
func TestSlowlogEntryShapeIsSixElements(t *testing.T) {
	slowlogGateRecorder(t)
	e := slowlogGateEntry(t, 2)

	if len(e.Array) != 6 {
		t.Fatalf("SLOWLOG entry has %d elements, want 6 (Redis <8.10 layout: id, unix-time, "+
			"duration, args, client-addr, client-name). Redis 8.10 added a 7th element "+
			"(original argument count); it is deliberately NOT emitted because CacheStorm "+
			"does not truncate element 4, so it would only duplicate len(element 4).",
			len(e.Array))
	}
	if e.Array[0].Type != resp.TypeInteger || e.Array[1].Type != resp.TypeInteger ||
		e.Array[2].Type != resp.TypeInteger {
		t.Fatalf("elements 1-3 must be integers (id, unix-time, duration), got %v/%v/%v",
			e.Array[0].Type, e.Array[1].Type, e.Array[2].Type)
	}
	if e.Array[3].Type != resp.TypeArray {
		t.Fatalf("element 4 must be the args array, got type %v", e.Array[3].Type)
	}
	if e.Array[4].Type != resp.TypeBulkString || e.Array[5].Type != resp.TypeBulkString {
		t.Fatalf("elements 5-6 must be bulk strings (client addr, client name), got %v/%v",
			e.Array[4].Type, e.Array[5].Type)
	}
}

// TestSlowlogArgsAreNotTruncated is the reason element 7 is withheld: element 4
// carries every argument, far past Redis's default slowlog-max-argc of 32, with
// no "... (N more arguments)" marker. Consequently len(element 4) already IS the
// original argument count, and an element 7 would be pure duplication.
func TestSlowlogArgsAreNotTruncated(t *testing.T) {
	slowlogGateRecorder(t)

	// 200 arguments: far beyond Redis's default slowlog-max-argc (32).
	const nArgs = 200
	e := slowlogGateEntry(t, nArgs)

	argsArr := e.Array[3]
	// Element 4 is [command name, arg1..argN] -- cmdSLOWLOG prepends the command.
	wantLen := nArgs + 1
	if len(argsArr.Array) != wantLen {
		t.Fatalf("element 4 has %d entries, want %d (no truncation is applied); if "+
			"CacheStorm adopts slowlog-max-argc, this test must be revisited together "+
			"with the element-7 decision", len(argsArr.Array), wantLen)
	}
	if string(argsArr.Array[0].Bulk) != "SADD" {
		t.Fatalf("element 4 must begin with the command name, got %q", argsArr.Array[0].Bulk)
	}
	last := string(argsArr.Array[len(argsArr.Array)-1].Bulk)
	if len(last) > 3 && last[:3] == "..." {
		t.Fatalf("element 4 ends with a truncation marker %q; if truncation is now applied, "+
			"Redis 8.10's element 7 (original argument count) becomes meaningful and must be "+
			"added in the same change", last)
	}
}
