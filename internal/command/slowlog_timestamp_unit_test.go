package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: SLOWLOG GET reported the event timestamp in MICROSECONDS, while
// Redis documents field 2 as a Unix timestamp in SECONDS (field 3, the duration,
// IS microseconds).
//
//	internal/command/server_commands.go (before)
//	  resp.IntegerValue(entry.Timestamp.UnixMicro())   <- microseconds
//	  resp.IntegerValue(entry.Duration.Microseconds())  <- microseconds (correct)
//
// That inflated the timestamp by 1e6, so a client rendering it as a wall-clock
// date landed roughly 50,000 years in the future.
//
// The assertion pins the value to the current wall clock, so it fails for the
// unit defect and not for a fixture assumption.

func slowlogUnitRun(t *testing.T, router *Router, args ...string) string {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext("SLOWLOG", parsed, store.NewStore(), resp.NewWriter(buf))); err != nil {
		t.Fatalf("SLOWLOG %v: Execute returned error: %v", args, err)
	}
	return buf.String()
}

// slowlogUnitSetup installs an enabled, zero-threshold recorder and restores it.
func slowlogUnitSetup(t *testing.T) {
	t.Helper()
	store.GlobalSlowLog.Clear()
	store.GlobalSlowLog.SetEnabled(true)
	store.GlobalSlowLog.SetThreshold(0)
	t.Cleanup(func() {
		store.GlobalSlowLog.Clear()
		store.GlobalSlowLog.SetThreshold(10 * time.Millisecond)
	})
}

func slowlogUnitRouter() *Router {
	router := NewRouter()
	RegisterServerCommands(router)
	return router
}

// TestSlowlogTimestampIsUnixSeconds is the defect: Redis reports field 2 as a
// Unix timestamp in seconds, so it must fall in a small window around now.
func TestSlowlogTimestampIsUnixSeconds(t *testing.T) {
	slowlogUnitSetup(t)
	router := slowlogUnitRouter()

	before := time.Now().Unix()
	store.GlobalSlowLog.Add(15*time.Millisecond, "PING", [][]byte{}, "127.0.0.1:52120")
	after := time.Now().Unix()

	out := slowlogUnitRun(t, router, "GET")
	v, err := resp.NewReader(bytes.NewReader([]byte(out))).ReadValue()
	if err != nil {
		t.Fatalf("unparsable SLOWLOG GET reply %q: %v", out, err)
	}
	if v.Type != resp.TypeArray || len(v.Array) != 1 || v.Array[0].Type != resp.TypeArray {
		t.Fatalf("SLOWLOG GET returned %q, want one entry array", out)
	}
	ts := v.Array[0].Array[1].Int
	if ts < before || ts > after {
		t.Fatalf("SLOWLOG GET timestamp = %d, want a Unix second in [%d, %d]; "+
			"Redis reports unix-time in SECONDS, not microseconds", ts, before, after)
	}
}

// TestSlowlogEntryShapeControls covers the rest of the documented reply, which
// must stay correct so a red above is the timestamp unit and nothing else:
// exactly 6 fields, an integer id, duration in microseconds, command+args, and
// the client addr/name pair.
func TestSlowlogEntryShapeControls(t *testing.T) {
	slowlogUnitSetup(t)
	router := slowlogUnitRouter()

	store.GlobalSlowLog.Add(15*time.Millisecond, "PING", [][]byte{[]byte("foo")}, "127.0.0.1:52120")

	out := slowlogUnitRun(t, router, "GET")
	v, err := resp.NewReader(bytes.NewReader([]byte(out))).ReadValue()
	if err != nil {
		t.Fatalf("unparsable SLOWLOG GET reply %q: %v", out, err)
	}
	if len(v.Array) != 1 {
		t.Fatalf("CONTROL FAIL: expected 1 slowlog entry, got %d", len(v.Array))
	}
	e := v.Array[0].Array

	// Redis emits exactly 6 fields.
	if len(e) != 6 {
		t.Fatalf("CONTROL FAIL: entry has %d fields, want 6 (id, unix-time, duration, args, client addr, client name)", len(e))
	}
	if e[0].Type != resp.TypeInteger {
		t.Fatalf("CONTROL FAIL: field 1 (id) is %v, want an integer", e[0].Type)
	}
	// Field 3 (duration) IS microseconds in Redis: 15ms == 15000us.
	if e[2].Int != 15000 {
		t.Fatalf("CONTROL FAIL: duration = %d, want 15000 microseconds (15ms)", e[2].Int)
	}
	// Field 4 is the command followed by its args: ["PING", "foo"].
	args := e[3].Array
	if len(args) != 2 {
		t.Fatalf("CONTROL FAIL: args array has %d entries, want 2 (command + 1 arg)", len(args))
	}
	if string(args[0].Bulk) != "PING" {
		t.Fatalf("CONTROL FAIL: args[0] = %q, want the command name PING", args[0].Bulk)
	}
	if string(args[1].Bulk) != "foo" {
		t.Fatalf("CONTROL FAIL: args[1] = %q, want the recorded argument foo", args[1].Bulk)
	}
	// Field 5 is the client addr; field 6 the client name (empty here).
	if string(e[4].Bulk) != "127.0.0.1:52120" {
		t.Fatalf("CONTROL FAIL: client addr = %q, want 127.0.0.1:52120", e[4].Bulk)
	}
	if e[5].Type != resp.TypeBulkString {
		t.Fatalf("CONTROL FAIL: client name is %v, want a bulk string", e[5].Type)
	}
}
