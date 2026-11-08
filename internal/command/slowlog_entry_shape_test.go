package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// SLOWLOG GET emits SIX elements per entry, the Redis 7.4 layout CacheStorm
// targets. Like Redis 7.4 the logged arguments (element 4) are trimmed to 32
// words, the last replaced by "... (N more arguments)", and each argument to
// 128 bytes plus "... (N more bytes)". Redis 8.10 later added a seventh
// element carrying the untrimmed argument count; a 7.4 client indexes the
// six-element array positionally, so it is not emitted.

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

// TestSlowlogArgsAreTrimmed checks element 4 against Redis 7.4: a SADD with
// 200 arguments (201 words) keeps 31 words and a marker for the other 170.
func TestSlowlogArgsAreTrimmed(t *testing.T) {
	slowlogGateRecorder(t)

	const nArgs = 200
	e := slowlogGateEntry(t, nArgs)

	argsArr := e.Array[3]
	if len(argsArr.Array) != 32 {
		t.Fatalf("element 4 has %d entries, want 32", len(argsArr.Array))
	}
	if string(argsArr.Array[0].Bulk) != "SADD" {
		t.Fatalf("element 4 must begin with the command name, got %q", argsArr.Array[0].Bulk)
	}
	if last := string(argsArr.Array[31].Bulk); last != "... (170 more arguments)" {
		t.Fatalf("element 4 ends with %q, want the Redis marker for 170 more arguments", last)
	}
	if len(e.Array) != 6 {
		t.Fatalf("entry has %d elements, want the Redis 7.4 six", len(e.Array))
	}
}
