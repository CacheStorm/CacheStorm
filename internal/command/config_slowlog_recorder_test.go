package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: CONFIG SET slowlog-log-slower-than and slowlog-max-len wrote to
// command.globalSlowLog — the orphan SlowLog that round 20's sibling fix proved
// nothing in production ever reads — so both runtime knobs silently did nothing.
//
// Startup configured the LIVE recorder correctly from the config file
// (internal/server/server.go calls store.GlobalSlowLog.SetThreshold/SetMaxSize),
// but every later CONFIG SET landed on the orphan. An operator who lowered the
// threshold to "record everything" and then ran SLOWLOG GET saw nothing, while
// CONFIG GET echoed the new value back — the round trip looked correct and the
// behaviour never changed.
//
// There is no getter for the recorder's threshold, so these tests assert
// OBSERVABLE recording behaviour rather than internal fields.

func slowlogConfigRouter() *Router {
	// RegisterConfigCommands must be called explicitly: no other Register*
	// function pulls it in, and omitting it makes CONFIG report
	// "unknown command" even for the default user.
	router := NewRouter()
	RegisterConfigCommands(router)
	RegisterServerCommands(router)
	return router
}

// resetSlowlogRecorder puts the live recorder in a known state and restores
// documented defaults on cleanup, so a later test is not left with a 1s
// threshold or a shrunken cap.
func resetSlowlogRecorder(t *testing.T) {
	t.Helper()
	restore := func() {
		store.GlobalSlowLog.Clear()
		store.GlobalSlowLog.SetEnabled(true)
		store.GlobalSlowLog.SetThreshold(10 * time.Millisecond)
		store.GlobalSlowLog.SetMaxSize(128)
	}
	restore()
	t.Cleanup(restore)
}

func slowlogConfigRun(t *testing.T, s *store.Store, router *Router, args ...string) *resp.Value {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(args[0], parsed[1:], s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%v: Execute returned error: %v", args, err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%v: could not parse RESP reply %q: %v", args, buf.String(), err)
	}
	return v
}

// recordIfSlow mirrors the request path's record attempt for a command that
// took d, and reports whether it was recorded.
func recordIfSlow(d time.Duration) bool {
	before := store.GlobalSlowLog.Len()
	store.GlobalSlowLog.AddIfSlow(d, "PROBE", [][]byte{[]byte("x")}, "127.0.0.1:1", "")
	return store.GlobalSlowLog.Len() > before
}

// TestConfigSetSlowerThanAppliesToRecorder is the primary case: lowering the
// threshold at runtime must make the live recorder capture commands.
func TestConfigSetSlowerThanAppliesToRecorder(t *testing.T) {
	resetSlowlogRecorder(t)
	store.GlobalSlowLog.SetThreshold(time.Second) // nothing short of 1s records

	s := store.NewStore()
	router := slowlogConfigRouter()

	if recordIfSlow(time.Millisecond) {
		t.Fatalf("SETUP FAIL: a 1ms command was recorded under a 1s threshold")
	}

	reply := slowlogConfigRun(t, s, router, "CONFIG", "SET", "slowlog-log-slower-than", "0")
	if reply.Type == resp.TypeError {
		t.Fatalf("SETUP FAIL: CONFIG SET was rejected: %q", reply.Str)
	}

	// The behaviour that matters: the 1ms command must now be recorded.
	if !recordIfSlow(time.Millisecond) {
		got := slowlogConfigRun(t, s, router, "CONFIG", "GET", "slowlog-log-slower-than")
		t.Fatalf("FAIL: after CONFIG SET slowlog-log-slower-than 0 a 1ms command is\n"+
			"still not recorded, yet CONFIG GET reports %q — the threshold landed on\n"+
			"command.globalSlowLog, which the request path never reads", got.Str)
	}
}

// TestConfigSetSlowerThanRejectsNegative covers the boundary: a negative
// threshold must clamp to 0 (record everything), not become a negative
// duration that no command can satisfy.
func TestConfigSetSlowerThanRejectsNegative(t *testing.T) {
	resetSlowlogRecorder(t)
	store.GlobalSlowLog.SetThreshold(time.Hour)

	s := store.NewStore()
	router := slowlogConfigRouter()

	slowlogConfigRun(t, s, router, "CONFIG", "SET", "slowlog-log-slower-than", "-1")

	if !recordIfSlow(time.Microsecond) {
		t.Fatal("FAIL: a negative slowlog-log-slower-than left the recorder\n" +
			"rejecting every command; it must clamp to 0")
	}
}

// TestConfigSetMaxLenAppliesToRecorder is the sibling knob: shrinking the cap
// at runtime must bound the live recorder immediately.
func TestConfigSetMaxLenAppliesToRecorder(t *testing.T) {
	resetSlowlogRecorder(t)
	store.GlobalSlowLog.SetMaxSize(128)

	s := store.NewStore()
	router := slowlogConfigRouter()

	for i := 0; i < 5; i++ {
		store.GlobalSlowLog.Add(50*time.Millisecond, "CMD", [][]byte{[]byte("k")}, "127.0.0.1:1")
	}
	if before := store.GlobalSlowLog.Len(); before != 5 {
		t.Fatalf("SETUP FAIL: recorder holds %d entries, want 5", before)
	}

	reply := slowlogConfigRun(t, s, router, "CONFIG", "SET", "slowlog-max-len", "2")
	if reply.Type == resp.TypeError {
		t.Fatalf("SETUP FAIL: CONFIG SET was rejected: %q", reply.Str)
	}

	if got := store.GlobalSlowLog.Len(); got > 2 {
		t.Fatalf("FAIL: recorder still holds %d entries after CONFIG SET slowlog-max-len 2 —\n"+
			"the cap landed on command.globalSlowLog, which the request path never reads", got)
	}

	// The cap must keep bounding on later records too.
	store.GlobalSlowLog.Add(50*time.Millisecond, "CMD", [][]byte{[]byte("k")}, "127.0.0.1:1")
	if got := store.GlobalSlowLog.Len(); got > 2 {
		t.Fatalf("FAIL: after one more record the recorder holds %d entries, want at most 2", got)
	}
}

// TestSlowlogConfigControls is the control: the live recorder must honour its
// threshold and cap on its own, independent of CONFIG SET. If these fail, the
// tests above would be passing for an unrelated reason.
func TestSlowlogConfigControls(t *testing.T) {
	resetSlowlogRecorder(t)

	if recordIfSlow(time.Millisecond) {
		t.Fatalf("CONTROL FAIL: a 1ms command was recorded under a 1s threshold")
	}
	store.GlobalSlowLog.SetThreshold(time.Millisecond)
	if !recordIfSlow(50 * time.Millisecond) {
		t.Fatal("CONTROL FAIL: a 50ms command was NOT recorded under a 1ms threshold")
	}

	store.GlobalSlowLog.Clear()
	store.GlobalSlowLog.SetMaxSize(3)
	for i := 0; i < 6; i++ {
		store.GlobalSlowLog.Add(50*time.Millisecond, "CMD", [][]byte{[]byte("k")}, "127.0.0.1:1")
	}
	if got := store.GlobalSlowLog.Len(); got != 3 {
		t.Fatalf("CONTROL FAIL: recorder holds %d entries under MaxSize 3, want 3", got)
	}

	// A non-positive cap must not wedge the recorder at zero entries.
	store.GlobalSlowLog.SetMaxSize(0)
	for i := 0; i < 3; i++ {
		store.GlobalSlowLog.Add(50*time.Millisecond, "CMD", [][]byte{[]byte("k")}, "127.0.0.1:1")
	}
	if got := store.GlobalSlowLog.Len(); got != 1 {
		t.Fatalf("CONTROL FAIL: recorder holds %d entries after SetMaxSize(0), want 1", got)
	}
}
