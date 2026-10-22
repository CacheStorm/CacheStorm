package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: SLOWLOG GET / LEN / RESET read a command-package SlowLog that
// nothing in production ever appends to, so the whole command was dead.
//
// The handler used command.globalSlowLog (a second SlowLog type declared in
// this package), while the request path records into a different type entirely:
//
//	internal/server/connection.go
//	store.GlobalSlowLog.AddIfSlow(elapsed,cmd,ctx.Args,ctx.RemoteAddr,"")
//
// globalSlowLog.Add was called from tests only. So after a command was recorded
// as slow, SLOWLOG GET still returned an empty array, SLOWLOG LEN returned 0,
// and SLOWLOG RESET cleared nothing the operator could see — the recorded
// entries stayed in the recorder forever.
//
// The proof records through the exact call connection.go makes, then reads back
// through the real router, so the disconnect is exercised end to end.

func slowlogRaceRouter() *Router {
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterConfigCommands(router)
	return router
}

// slowlogResetRecorder restores the shared recorder's documented defaults after
// the test, so a later test is not left with threshold 0.
func slowlogResetRecorder(t *testing.T) {
	t.Helper()
	store.GlobalSlowLog.Clear()
	store.GlobalSlowLog.SetEnabled(true)
	store.GlobalSlowLog.SetThreshold(10 * time.Millisecond)
	t.Cleanup(func() {
		store.GlobalSlowLog.Clear()
		store.GlobalSlowLog.SetEnabled(true)
		store.GlobalSlowLog.SetThreshold(10 * time.Millisecond)
	})
}

// recordSlowCommand performs exactly what internal/server/connection.go does
// once a command has completed.
func recordSlowCommand(cmd string, args [][]byte) {
	store.GlobalSlowLog.AddIfSlow(50*time.Millisecond, cmd, args, "10.0.0.7:5555", "")
}

// TestSLOWLOGCommandsReadTheRecorder is the primary case: a recorded slow
// command must be visible to SLOWLOG LEN and GET.
func TestSLOWLOGCommandsReadTheRecorder(t *testing.T) {
	slowlogResetRecorder(t)
	store.GlobalSlowLog.SetThreshold(0) // record everything

	s := store.NewStore()
	router := slowlogRaceRouter()

	recordSlowCommand("SLOWCOMMAND", [][]byte{[]byte("alpha"), []byte("beta")})

	recorded := store.GlobalSlowLog.Len()
	if recorded != 1 {
		t.Fatalf("SETUP FAIL: store.GlobalSlowLog recorded %d entries, want 1", recorded)
	}

	lenReply := slowlogRaceRun(t, s, router, "SLOWLOG", "LEN")
	if lenReply.Type != resp.TypeInteger {
		t.Fatalf("FAIL: SLOWLOG LEN returned type %v, want an integer", lenReply.Type)
	}
	if lenReply.Int != int64(recorded) {
		t.Fatalf("FAIL: SLOWLOG LEN = %d but the recorder holds %d entries — SLOWLOG\n"+
			"must read store.GlobalSlowLog, the recorder the request path writes to",
			lenReply.Int, recorded)
	}

	getReply := slowlogRaceRun(t, s, router, "SLOWLOG", "GET")
	if getReply.Type != resp.TypeArray || len(getReply.Array) != 1 {
		t.Fatalf("FAIL: SLOWLOG GET returned %d entries, want 1 — the recorded slow\n"+
			"command is invisible to the client", len(getReply.Array))
	}
	// The entry must carry the command and its arguments. connection.go
	// records ctx.Args, which EXCLUDES the command name, so the handler
	// prepends entry.Command — matching Redis, where args[0] is the command.
	entry := getReply.Array[0]
	if len(entry.Array) < 4 || entry.Array[3].Type != resp.TypeArray {
		t.Fatalf("FAIL: SLOWLOG GET entry has shape %v, want at least 4 fields with args at index 3", entry)
	}
	if got := string(entry.Array[3].Array[0].Bulk); got != "SLOWCOMMAND" {
		t.Fatalf("FAIL: SLOWLOG GET args[0] = %q, want SLOWCOMMAND", got)
	}
	wantArgs := []string{"SLOWCOMMAND", "alpha", "beta"}
	if len(entry.Array[3].Array) != len(wantArgs) {
		t.Fatalf("FAIL: SLOWLOG GET reported %d args, want %d (%v)",
			len(entry.Array[3].Array), len(wantArgs), wantArgs)
	}
	for i, want := range wantArgs {
		if got := string(entry.Array[3].Array[i].Bulk); got != want {
			t.Fatalf("FAIL: SLOWLOG GET args[%d] = %q, want %q", i, got, want)
		}
	}
}

// TestSLOWLOGResetClearsTheRecorder is the other direction of the same defect:
// RESET must clear what GET reads.
func TestSLOWLOGResetClearsTheRecorder(t *testing.T) {
	slowlogResetRecorder(t)
	store.GlobalSlowLog.SetThreshold(0)

	s := store.NewStore()
	router := slowlogRaceRouter()

	recordSlowCommand("RESETME", [][]byte{[]byte("x")})
	if store.GlobalSlowLog.Len() != 1 {
		t.Fatalf("SETUP FAIL: recorder holds %d entries, want 1", store.GlobalSlowLog.Len())
	}

	slowlogRaceRun(t, s, router, "SLOWLOG", "RESET")

	if got := store.GlobalSlowLog.Len(); got != 0 {
		t.Fatalf("FAIL: after SLOWLOG RESET the recorder still holds %d entries —\n"+
			"RESET must clear store.GlobalSlowLog", got)
	}
}

// TestSLOWLOGControlEmptyRecorder is the control: with nothing recorded,
// SLOWLOG GET must return an empty array and LEN zero — which is also the
// count=0 contract round 20 pinned, so delegating to store.GlobalSlowLog.Get
// (whose n <= 0 means "all") must not turn it into a full listing.
func TestSLOWLOGControlEmptyRecorder(t *testing.T) {
	slowlogResetRecorder(t)
	store.GlobalSlowLog.SetThreshold(0)

	s := store.NewStore()
	router := slowlogRaceRouter()

	if got := store.GlobalSlowLog.Len(); got != 0 {
		t.Fatalf("SETUP FAIL: recorder should be empty, holds %d", got)
	}
	if v := slowlogRaceRun(t, s, router, "SLOWLOG", "GET"); v.Type != resp.TypeArray || len(v.Array) != 0 {
		t.Fatalf("CONTROL FAIL: SLOWLOG GET on an empty recorder returned type %v with %d entries, want an empty array",
			v.Type, len(v.Array))
	}
	if v := slowlogRaceRun(t, s, router, "SLOWLOG", "LEN"); v.Type != resp.TypeInteger || v.Int != 0 {
		t.Fatalf("CONTROL FAIL: SLOWLOG LEN on an empty recorder = type %v %d, want :0", v.Type, v.Int)
	}

	// With entries present, an explicit count of 0 must still be an empty
	// array, not the full listing.
	recordSlowCommand("ZERO", [][]byte{[]byte("a")})
	recordSlowCommand("ONE", [][]byte{[]byte("b")})
	if v := slowlogRaceRun(t, s, router, "SLOWLOG", "GET", "0"); v.Type != resp.TypeArray || len(v.Array) != 0 {
		t.Fatalf("CONTROL FAIL: SLOWLOG GET 0 with entries present returned %d entries, want 0",
			len(v.Array))
	}
	if v := slowlogRaceRun(t, s, router, "SLOWLOG", "GET", "-1"); v.Type != resp.TypeArray || len(v.Array) != 2 {
		t.Fatalf("CONTROL FAIL: SLOWLOG GET -1 returned %d entries, want 2", len(v.Array))
	}
	if v := slowlogRaceRun(t, s, router, "SLOWLOG", "GET"); v.Type != resp.TypeArray || len(v.Array) != 2 {
		t.Fatalf("CONTROL FAIL: SLOWLOG GET (default) returned %d entries, want 2", len(v.Array))
	}
}

func slowlogRaceRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) *resp.Value {
	t.Helper()
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
	return v
}
