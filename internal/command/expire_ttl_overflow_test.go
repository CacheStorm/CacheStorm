package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdEXPIRE and cmdPEXPIRE multiplied the parsed TTL by the unit
// WITHOUT bounding it first, so the int64-nanosecond multiplication overflowed
// and wrapped to a duration with the WRONG SIGN.
//
//	internal/command/server_commands.go (before)
//	  newTTL := time.Duration(sec) * time.Second        // sec * 1e9, unchecked
//	  newTTL := time.Duration(ms) * time.Millisecond    // ms * 1e6, unchecked
//
// A far-future TTL therefore became a NEGATIVE duration, which Store.SetTTL
// stores as "already expired" — so EXPIRE key 9223372036854775807 replied :1
// and silently DELETED the key. Conversely a large negative TTL could wrap
// positive and keep a key alive that should have been deleted.
//
// The sibling TTL setters in the same package (cmdEXPIREAT, cmdPEXPIREAT,
// cmdSETEX) already bounded the argument with maxExpireAtSeconds /
// maxExpireAtMillis; these two were the only ones missing the guard.

// expireTTLExec runs one command against the GIVEN store. The command name is
// passed as NewContext's name and NOT as the first element of Args: including
// it in Args makes EXPIRE read the key as "EXPIRE". Every case threads ONE
// store through seed -> command -> EXISTS so the assertions observe the same key.
func expireTTLExec(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s %v: Execute returned error: %v", cmd, args, err)
	}
	return buf.String()
}

func expireTTLRouter() *Router {
	router := NewRouter()
	RegisterKeyCommands(router)
	RegisterServerCommands(router)
	RegisterStringCommands(router)
	return router
}

func expireTTLSeed(t *testing.T, router *Router) *store.Store {
	t.Helper()
	s := store.NewStore()
	expireTTLExec(t, s, router, "SET", "k", "v")
	return s
}

// TestExpireRejectsOverflowingTTL is the primary case: an out-of-range TTL must
// be rejected, and must NOT delete the key it was aimed at.
func TestExpireRejectsOverflowingTTL(t *testing.T) {
	router := expireTTLRouter()

	for _, c := range []struct{ cmd, arg, name string }{
		{"EXPIRE", "9223372036854775807", "EXPIRE MaxInt64"},
		{"EXPIRE", "-9223372036854775808", "EXPIRE MinInt64"},
		{"PEXPIRE", "9223372036854775807", "PEXPIRE MaxInt64"},
		{"PEXPIRE", "-9223372036854775808", "PEXPIRE MinInt64"},
	} {
		s := expireTTLSeed(t, router)
		out := expireTTLExec(t, s, router, c.cmd, "k", c.arg)
		if !strings.HasPrefix(out, "-") {
			t.Fatalf("%s: replied %q, want an error; the duration multiplication "+
				"overflowed instead of being bounded by maxExpireAt*", c.name, out)
		}
		if ex := expireTTLExec(t, s, router, "EXISTS", "k"); ex != ":1\r\n" {
			t.Fatalf("%s: replied an error but the key was DELETED (EXISTS = %q); "+
				"a rejected TTL must not destroy data", c.name, ex)
		}
	}
}

// TestExpireRejectsZeroTTL: Redis rejects a zero TTL outright. CacheStorm
// applied it, and its own expiry path treats 0 as "already expired".
func TestExpireRejectsZeroTTL(t *testing.T) {
	router := expireTTLRouter()

	for _, c := range []struct{ cmd, name string }{
		{"EXPIRE", "EXPIRE 0"},
		{"PEXPIRE", "PEXPIRE 0"},
	} {
		s := expireTTLSeed(t, router)
		out := expireTTLExec(t, s, router, c.cmd, "k", "0")
		if !strings.HasPrefix(out, "-") {
			t.Fatalf("%s: replied %q, want \"ERR invalid expire time\"; "+
				"a zero TTL must be rejected, not applied", c.name, out)
		}
		if ex := expireTTLExec(t, s, router, "EXISTS", "k"); ex != ":1\r\n" {
			t.Fatalf("%s: replied an error but the key was DELETED (EXISTS = %q)", c.name, ex)
		}
	}
}

// TestExpireControlsUncontended is the control: a normal TTL still applies, a
// negative TTL still deletes (Redis's documented "delete now"), a large
// in-range TTL still applies, and the NX condition still refuses to overwrite.
// These pass before and after the fix.
//
// The large TTL is deliberately 30 days, NOT maxExpireAtSeconds: a TTL near the
// year-2262 int64-nanosecond limit trips a SEPARATE overflow inside
// Store.SetTTL (time.Now().Add(ttl).UnixNano()), a different defect that must
// not be conflated with this one.
func TestExpireControlsUncontended(t *testing.T) {
	router := expireTTLRouter()

	s := expireTTLSeed(t, router)
	if out := expireTTLExec(t, s, router, "EXPIRE", "k", "100"); out != ":1\r\n" {
		t.Fatalf("CONTROL FAIL: EXPIRE k 100 = %q, want :1", out)
	}

	s = expireTTLSeed(t, router)
	if out := expireTTLExec(t, s, router, "EXPIRE", "k", "-1"); out != ":1\r\n" {
		t.Fatalf("CONTROL FAIL: EXPIRE k -1 = %q, want :1", out)
	}
	if ex := expireTTLExec(t, s, router, "EXISTS", "k"); ex != ":0\r\n" {
		t.Fatalf("CONTROL FAIL: EXISTS after EXPIRE -1 = %q, want :0 (key deleted)", ex)
	}

	s = expireTTLSeed(t, router)
	if out := expireTTLExec(t, s, router, "EXPIRE", "k", "2592000"); out != ":1\r\n" {
		t.Fatalf("CONTROL FAIL: EXPIRE k 2592000 = %q, want :1", out)
	}
	if ex := expireTTLExec(t, s, router, "EXISTS", "k"); ex != ":1\r\n" {
		t.Fatalf("CONTROL FAIL: a valid 30-day TTL deleted the key (EXISTS = %q)", ex)
	}

	s = expireTTLSeed(t, router)
	expireTTLExec(t, s, router, "EXPIRE", "k", "100")
	if out := expireTTLExec(t, s, router, "EXPIRE", "k", "200", "NX"); out != ":0\r\n" {
		t.Fatalf("CONTROL FAIL: EXPIRE NX over an existing TTL = %q, want :0", out)
	}
}
