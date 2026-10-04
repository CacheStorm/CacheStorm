package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func configSetRun(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterConfigCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], configSetBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

func configSetBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

func configSetReports(t *testing.T, s *store.Store, param, want string) bool {
	t.Helper()
	got := configSetRun(t, s, "CONFIG", "GET", param)
	if got.Type != resp.TypeArray {
		t.Fatalf("CONFIG GET %s returned type %v, want an array", param, got.Type)
	}
	for _, e := range got.Array {
		if string(e.Bulk) == want {
			return true
		}
	}
	return false
}

// CONFIG SET must apply a key/value pair.
//
// cmdConfigSet rejected `ArgCount()%2 != 0`, but its own pair loop starts at
// index 1 because index 0 holds the "SET" subcommand. Every well-formed call
// therefore has an ODD argument count and was rejected with
// "ERR wrong number of arguments" — the command could never succeed, so no
// runtime setting could ever be changed.
func TestConfigSetAppliesKeyValuePair(t *testing.T) {
	s := store.NewStore()

	got := configSetRun(t, s, "CONFIG", "SET", "maxmemory", "1234")
	if got.Type == resp.TypeError {
		t.Fatalf("CONFIG SET maxmemory 1234 returned %q, want +OK", got.Err)
	}
	if got.Type != resp.TypeSimpleString || got.Str != "OK" {
		t.Fatalf("CONFIG SET returned type %v %q, want +OK", got.Type, got.Str)
	}
	if !configSetReports(t, s, "maxmemory", "1234") {
		t.Fatal("CONFIG SET reported +OK but CONFIG GET does not show the new value")
	}
}

// The pair loop is written for multiple pairs, so several in one call must work.
func TestConfigSetAppliesMultiplePairs(t *testing.T) {
	s := store.NewStore()

	got := configSetRun(t, s, "CONFIG", "SET", "maxmemory", "2048", "timeout", "77")
	if got.Type == resp.TypeError {
		t.Fatalf("CONFIG SET with two pairs returned %q, want +OK", got.Err)
	}
	if !configSetReports(t, s, "maxmemory", "2048") {
		t.Fatal("first pair was not applied")
	}
	if !configSetReports(t, s, "timeout", "77") {
		t.Fatal("second pair was not applied")
	}
}

// The inverted guard was wrong in both directions: even counts — which mean a
// dangling key — were ACCEPTED, and the loop then read ctx.ArgString(i+1) past
// the end of the slice and silently did nothing while replying +OK.
func TestConfigSetRejectsMalformedArity(t *testing.T) {
	s := store.NewStore()

	if v := configSetRun(t, s, "CONFIG", "SET", "maxmemory"); v.Type != resp.TypeError {
		t.Fatalf("CONFIG SET maxmemory (no value) returned type %v, want an error", v.Type)
	}
	if v := configSetRun(t, s, "CONFIG", "SET", "maxmemory", "1", "timeout", "2", "maxmemory-policy"); v.Type != resp.TypeError {
		t.Fatalf("CONFIG SET with a trailing key returned type %v, want an error", v.Type)
	}
	if v := configSetRun(t, s, "CONFIG", "SET"); v.Type != resp.TypeError {
		t.Fatalf("bare CONFIG SET returned type %v, want an error", v.Type)
	}
}

// Unknown parameters must be rejected instead of being silently ignored behind
// +OK. This includes security-relevant names that are not runtime-configurable.
func TestConfigSetRejectsUnknownParameters(t *testing.T) {
	s := store.NewStore()

	if v := configSetRun(t, s, "CONFIG", "SET", "requirepass", ""); v.Type != resp.TypeError {
		t.Fatalf("CONFIG SET requirepass \"\" returned type %v, want an error", v.Type)
	}
	if v := configSetRun(t, s, "CONFIG", "SET", "bogusparam", "1"); v.Type != resp.TypeError {
		t.Fatalf("CONFIG SET bogusparam 1 returned type %v, want an error", v.Type)
	}
}

// Control: an invalid VALUE is still rejected on its own merits, so the arity
// fix does not bypass value validation.
func TestConfigSetInvalidValueStillRejectedControl(t *testing.T) {
	s := store.NewStore()

	if v := configSetRun(t, s, "CONFIG", "SET", "maxmemory", "-5"); v.Type != resp.TypeError {
		t.Fatalf("CONFIG SET maxmemory -5 returned type %v, want an error", v.Type)
	}
}

// Control: CONFIG GET already had the correct arity check and is unaffected.
func TestConfigGetUnaffectedControl(t *testing.T) {
	s := store.NewStore()

	if v := configSetRun(t, s, "CONFIG", "GET", "maxmemory"); v.Type != resp.TypeArray {
		t.Fatalf("CONFIG GET maxmemory returned type %v, want an array", v.Type)
	}
	if v := configSetRun(t, s, "CONFIG", "GET"); v.Type != resp.TypeError {
		t.Fatalf("bare CONFIG GET returned type %v, want an error", v.Type)
	}
}

// Control: RESETSTAT (a separate fix) and GET keep working alongside SET.
func TestConfigOtherSubCommandsStillWorkControl(t *testing.T) {
	s := store.NewStore()

	if v := configSetRun(t, s, "CONFIG", "RESETSTAT"); v.Type != resp.TypeSimpleString {
		t.Fatalf("CONFIG RESETSTAT returned type %v, want a simple string", v.Type)
	}
	if v := configSetRun(t, s, "CONFIG", "NOSUCHTHING"); v.Type != resp.TypeError {
		t.Fatalf("CONFIG NOSUCHTHING returned type %v, want an error", v.Type)
	}
}
