package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: SLOWLOG GET's sixth element is always empty.
//
// Redis SLOWLOG GET returns seven values per entry; element 6 is "Client name if
// set via the CLIENT SETNAME command", and Redis emits it as an EMPTY STRING
// when unset (docs example: 5) "127.0.0.1:52276" / 6) ""). Redis never omits
// the element. Element 7 (argument count) arrived in Redis 8.10, so a six-element
// reply is correct for earlier versions.
//
// cmdSLOWLOG emitted resp.BulkString("") -- a hardcoded literal -- so the shape
// was right but the value could never be non-empty, because the client name was
// recorded nowhere:
//
//   - CLIENT SETNAME replied +OK and DISCARDED the name (a stub);
//   - CLIENT GETNAME was hardcoded to null, so it could never echo one back;
//   - store.SlowLogEntry had no ClientName field and AddIfSlow took no name;
//   - server.Connection recorded only ctx.RemoteAddr.
//
// So a client that named itself was reported as anonymous in the slow log. The
// fix threads the name end to end: CLIENT SETNAME -> Context.ClientName ->
// Connection.clientName -> AddIfSlow -> SlowLogEntry.ClientName -> element 6.

func slowNameRun(t *testing.T, ctx *Context, router *Router, cmd string, args ...string) string {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	// Context.Args excludes the command name, and a fresh Context is built per
	// command in production, so reuse one Context to carry state across calls --
	// exactly how server.Connection does it (connection.go:133 / :174).
	ctx.Command = cmd
	ctx.Args = parsed
	buf := &bytes.Buffer{}
	ctx.Writer = resp.NewWriter(buf)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", cmd, err)
	}
	return buf.String()
}

func slowNameCtx(s *store.Store) (*Context, *Router) {
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterClientCommands(router)
	return NewContext("CLIENT", nil, s, resp.NewWriter(&bytes.Buffer{})), router
}

// slowNameEntry6 returns SLOWLOG GET's first entry and its sixth element.
func slowNameEntry6(t *testing.T, ctx *Context, router *Router, s *store.Store) *resp.Value {
	t.Helper()
	out := slowNameRun(t, ctx, router, "SLOWLOG", "GET")
	v, err := resp.NewReader(bytes.NewReader([]byte(out))).ReadValue()
	if err != nil {
		t.Fatalf("unparsable SLOWLOG GET reply %q: %v", out, err)
	}
	if v.Type != resp.TypeArray || len(v.Array) == 0 {
		t.Fatalf("SLOWLOG GET returned %q, want a non-empty array", out)
	}
	e := v.Array[0]
	if e.Type != resp.TypeArray || len(e.Array) < 6 {
		t.Fatalf("SLOWLOG entry has %d elements, want at least 6 (id, unix-time, duration, args, addr, name)", len(e.Array))
	}
	return e.Array[5]
}

func slowNameRecorder(t *testing.T) {
	t.Helper()
	sl := store.GlobalSlowLog
	sl.Clear()
	sl.SetEnabled(true)
	sl.SetThreshold(0)
	t.Cleanup(func() { sl.Clear(); sl.SetThreshold(10 * time.Millisecond) })
}

// TestClientSetNameRoundTrips is the primary case: CLIENT SETNAME stores the
// name, so CLIENT GETNAME echoes it instead of reporting "no name".
func TestClientSetNameRoundTrips(t *testing.T) {
	s := store.NewStore()
	ctx, router := slowNameCtx(s)

	if out := slowNameRun(t, ctx, router, "CLIENT", "SETNAME", "alice"); out != "+OK\r\n" {
		t.Fatalf("CLIENT SETNAME alice = %q, want +OK", out)
	}
	if out := slowNameRun(t, ctx, router, "CLIENT", "GETNAME"); out != "$5\r\nalice\r\n" {
		t.Fatalf("CLIENT GETNAME after SETNAME alice = %q, want \"alice\"; SETNAME was a "+
			"stub that replied +OK and discarded the name", out)
	}
}

// TestSlowLogCarriesClientName is the case the SLOWLOG field exists for: a named
// client's slow entries report that name in element 6.
func TestSlowLogCarriesClientName(t *testing.T) {
	slowNameRecorder(t)
	s := store.NewStore()
	ctx, router := slowNameCtx(s)

	slowNameRun(t, ctx, router, "CLIENT", "SETNAME", "reporting-service")
	// Mirror what internal/server/connection.go:158 records for a dispatched command.
	store.GlobalSlowLog.AddIfSlow(50*time.Millisecond, "GET", [][]byte{[]byte("k")}, "10.0.0.9:5300", ctx.ClientName)

	name := slowNameEntry6(t, ctx, router, s)
	if name.Type != resp.TypeBulkString {
		t.Fatalf("SLOWLOG element 6 has type %v, want a bulk string", name.Type)
	}
	if string(name.Bulk) != "reporting-service" {
		t.Fatalf("SLOWLOG element 6 = %q, want \"reporting-service\"; it was a hardcoded "+
			"empty literal, so a named client was reported anonymous", name.Bulk)
	}
}

// TestSlowLogClientNameControlsUncontended is the control: Redis emits "" for a
// client that never set a name, an empty SETNAME clears it, and the name is
// per-connection. These pass before and after the fix.
func TestSlowLogClientNameControlsUncontended(t *testing.T) {
	slowNameRecorder(t)
	s := store.NewStore()

	// A client that never called SETNAME reports no name.
	ctx, router := slowNameCtx(s)
	store.GlobalSlowLog.AddIfSlow(50*time.Millisecond, "GET", nil, "10.0.0.9:1", ctx.ClientName)
	if name := slowNameEntry6(t, ctx, router, s); string(name.Bulk) != "" {
		t.Fatalf("CONTROL FAIL: unnamed client reported name %q, want an empty string", name.Bulk)
	}

	// An empty SETNAME argument clears the name (Redis: CLIENT SETNAME "").
	ctx2, router2 := slowNameCtx(s)
	slowNameRun(t, ctx2, router2, "CLIENT", "SETNAME", "bob")
	if out := slowNameRun(t, ctx2, router2, "CLIENT", "GETNAME"); out != "$3\r\nbob\r\n" {
		t.Fatalf("CONTROL FAIL: after SETNAME bob, GETNAME = %q", out)
	}
	slowNameRun(t, ctx2, router2, "CLIENT", "SETNAME", "")
	if out := slowNameRun(t, ctx2, router2, "CLIENT", "GETNAME"); out != "$-1\r\n" {
		t.Fatalf("CONTROL FAIL: CLIENT SETNAME \"\" must clear the name, got %q", out)
	}

	// Names are per-connection: a second connection does not see the first's.
	ctx3, router3 := slowNameCtx(s)
	slowNameRun(t, ctx3, router3, "CLIENT", "SETNAME", "carol")
	if out := slowNameRun(t, ctx, router, "CLIENT", "GETNAME"); out != "$-1\r\n" {
		t.Fatalf("CONTROL FAIL: a client name leaked across connections: %q", out)
	}

	// Arity is still validated.
	ctx4, router4 := slowNameCtx(s)
	if out := slowNameRun(t, ctx4, router4, "CLIENT", "SETNAME"); out[0] != '-' {
		t.Fatalf("CONTROL FAIL: CLIENT SETNAME with no argument = %q, want an error", out)
	}
	if out := slowNameRun(t, ctx4, router4, "CLIENT", "SETNAME", "a", "b"); out[0] != '-' {
		t.Fatalf("CONTROL FAIL: CLIENT SETNAME with two arguments = %q, want an error", out)
	}
}
