package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdSET's EX and PX options multiplied the parsed TTL by the unit
// WITHOUT bounding it, so a large value overflowed to a NEGATIVE time.Duration.
//
//	internal/command/string_commands.go (before)
//	  case "EX": opts.TTL = time.Duration(sec) * time.Second     // unchecked
//	  case "PX": opts.TTL = time.Duration(ms) * time.Millisecond  // unchecked
//	  case "EXAT": if ts > maxExpireAtSeconds || ... { errInvalidExpireTime }  // GUARDED
//	  case "PXAT": if ts > maxExpireAtMillis  || ... { errInvalidExpireTime }  // GUARDED
//
// Store.Set falls through on a non-positive TTL (`else if opts.TTL > 0`), so the
// wrapped NEGATIVE value was read as "no expiry": the key became permanently
// live behind an +OK reply. Redis answers "ERR invalid expire time in 'set'" for
// every one of these inputs.
//
// The two absolute-instant options in the SAME function already bounded the
// argument; the two relative ones did not. Round 22 fixed the equivalent guard
// for cmdEXPIRE/cmdPEXPIRE, and cmdSETEX already had one.

// setExExec runs one command against the GIVEN store; the command name goes to
// NewContext's name, NOT into Args (including it there makes SET read the key as
// "SET"). Every case threads ONE store so the assertions observe the same key.
func setExExec(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
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

func setExRouter() *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterKeyCommands(router)
	RegisterServerCommands(router)
	return router
}

// TestSetExRejectsOverflowingTTL is the primary case: an out-of-range TTL must
// be rejected rather than silently producing a permanent key.
func TestSetExRejectsOverflowingTTL(t *testing.T) {
	router := setExRouter()

	for _, c := range []struct{ opt, arg, name string }{
		{"EX", "9223372036854775807", "SET EX MaxInt64"},
		{"EX", "-9223372036854775808", "SET EX MinInt64"},
		{"PX", "9223372036854775807", "SET PX MaxInt64"},
		{"PX", "-9223372036854775808", "SET PX MinInt64"},
	} {
		s := store.NewStore()
		out := setExExec(t, s, router, "SET", "k", "v", c.opt, c.arg)
		if !strings.HasPrefix(out, "-") {
			ttl := setExExec(t, s, router, "TTL", "k")
			t.Fatalf("%s: replied %q and TTL is now %q — the overflowing TTL became "+
				"a PERMANENT key; Redis answers \"ERR invalid expire time in 'set'\"",
				c.name, out, ttl)
		}
	}
}

// TestSetExControlsUncontended is the control: a normal EX/PX TTL must still
// apply and report a sensible remaining TTL, EXAT/PXAT must still reject
// out-of-range input, and a plain SET must still yield no expiry.
func TestSetExControlsUncontended(t *testing.T) {
	router := setExRouter()

	for _, c := range []struct {
		opt, arg string
		wantTTL  []string
	}{
		{"EX", "100", []string{":99\r\n", ":100\r\n"}},
		{"PX", "100000", []string{":99\r\n", ":100\r\n"}},
		{"EX", "86400", []string{":86399\r\n", ":86400\r\n"}},
	} {
		s := store.NewStore()
		if out := setExExec(t, s, router, "SET", "k", "v", c.opt, c.arg); out != "+OK\r\n" {
			t.Fatalf("CONTROL FAIL: SET k v %s %s = %q, want +OK", c.opt, c.arg, out)
		}
		got := setExExec(t, s, router, "TTL", "k")
		ok := false
		for _, w := range c.wantTTL {
			if got == w {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("CONTROL FAIL: TTL after SET %s %s = %q, want one of %v", c.opt, c.arg, got, c.wantTTL)
		}
	}

	for _, c := range []struct{ opt, name string }{
		{"EXAT", "EXAT"}, {"PXAT", "PXAT"},
	} {
		s := store.NewStore()
		if out := setExExec(t, s, router, "SET", "k", "v", c.opt, "9223372036854775807"); !strings.HasPrefix(out, "-") {
			t.Fatalf("CONTROL FAIL: SET %s MaxInt64 = %q, want an error", c.name, out)
		}
	}

	s := store.NewStore()
	setExExec(t, s, router, "SET", "k", "v")
	if got := setExExec(t, s, router, "TTL", "k"); got != ":-1\r\n" {
		t.Fatalf("CONTROL FAIL: TTL after a plain SET = %q, want :-1 (no expiry)", got)
	}
}
