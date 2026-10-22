package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Proof for the MULTI/EXEC replay path's missing expiry bounds.
//
// internal/command/transaction_commands.go (before the fix)
//	  case "EXPIRE":
//	      seconds, err := parseInt(qc.args[1])
//	      ...
//	      ctx.Store.SetTTL(key, time.Duration(seconds)*time.Second)   // UNBOUNDED
//	  case "PEXPIRE":
//	      ms, err := parseInt(qc.args[1])
//	      ...
//	      ctx.Store.SetTTL(key, time.Duration(ms)*time.Millisecond)    // UNBOUNDED
//
// The TOP-LEVEL handlers for the very same commands already bound the
// argument:
//
//	internal/command/server_commands.go
//	  cmdEXPIRE  (server_commands.go:399)
//	    if sec == 0 || sec > maxExpireAtSeconds || sec < -maxExpireAtSeconds {
//	        return ctx.WriteError(errInvalidExpireTime)
//	    }
//	  cmdPEXPIRE (server_commands.go:429)
//	    if ms == 0 || ms > maxExpireAtMillis || ms < -maxExpireAtMillis {
//	        return ctx.WriteError(errInvalidExpireTime)
//	    }
//
// and the QUEUED SETEX case in the very same switch already does the same
// (transaction_commands.go:451), with a comment saying "exactly as the
// top-level cmdSETEX handler does".
//
// So the established contract is: an expiry that is zero or outside the
// int64-nanosecond range is refused with "ERR invalid expire time". The
// replay dispatcher re-implements EXPIRE and PEXPIRE instead of dispatching to
// the handlers, so it never inherited that guard:
//
//   - sec*1e9 / ms*1e6 overflow int64 nanoseconds and wrap NEGATIVE (or, for
//     MinInt64, wrap to exactly 0), and Store.SetTTL turns that into an expiry
//     instant in the PAST -- the key is destroyed behind a :1 reply.
//
// The trigger is specific to the transaction path: the same command OUTSIDE
// MULTI/EXEC is correctly refused, so EXPIRE/PEXPIRE behave differently
// depending on whether they were queued.
//
// A NEGATIVE expiry is NOT part of this defect: the top-level guard admits it
// and both paths then delete the key (Redis' "negative TTL deletes now").
// The probe below pins that behaviour as a control.

// queuedExpireRun drives a MULTI/EXEC cycle through the real router and
// returns the raw EXEC reply.
//
// Every command in one transaction must share ONE Context: the transaction
// lives on the Context, exactly like one connection's session. cmds must NOT
// include EXEC -- this helper runs it.
func queuedTTLRun(t *testing.T, s *store.Store, router *Router, cmds [][]string) string {
	t.Helper()
	shared := NewContext("MULTI", [][]byte{[]byte("MULTI")}, s, resp.NewWriter(&bytes.Buffer{}))
	for _, c := range cmds {
		var buf bytes.Buffer
		shared.Writer = resp.NewWriter(&buf)
		shared.Command = c[0]
		shared.Args = nil
		for _, a := range c[1:] {
			shared.Args = append(shared.Args, []byte(a))
		}
		if err := router.Execute(shared); err != nil {
			t.Fatalf("%v: Execute returned error: %v", c, err)
		}
	}
	var buf bytes.Buffer
	shared.Writer = resp.NewWriter(&buf)
	shared.Command = "EXEC"
	shared.Args = nil
	if err := router.Execute(shared); err != nil {
		t.Fatalf("EXEC returned error: %v", err)
	}
	return buf.String()
}

func queuedTTLRouter() *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterKeyCommands(router)
	RegisterTransactionCommands(router)
	return router
}

// queuedExpireExecValue parses the EXEC reply array and returns the first
// element, so the assertion can check its RESP TYPE (an error element vs an
// integer).
func queuedTTLExecValue(t *testing.T, raw string) *resp.Value {
	t.Helper()
	v, err := resp.NewReader(strings.NewReader(raw)).ReadValue()
	if err != nil {
		t.Fatalf("parsing EXEC reply %q: %v", raw, err)
	}
	if v.Type != resp.TypeArray || len(v.Array) != 1 {
		t.Fatalf("EXEC replied %q, want a 1-element array", raw)
	}
	return v.Array[0]
}

// execRaw runs one command through the real router and returns the raw reply.
func queuedTTLRaw(t *testing.T, s *store.Store, router *Router, name string, args ...string) string {
	t.Helper()
	argv := make([][]byte, len(args))
	for i, a := range args {
		argv[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(name, argv, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s %v: Execute returned error: %v", name, args, err)
	}
	return buf.String()
}

// TestQueuedExpireRejectsOutOfRangeTTL is the primary case: EXPIRE and
// PEXPIRE queued inside MULTI/EXEC must be refused exactly like the same
// commands are outside a transaction, and the key must survive.
//
// Before the fix the queued path accepted the value, replied :1, and left the
// key with an expiry in the past -- the data was destroyed behind a success.
func TestQueuedExpireRejectsOutOfRangeTTL(t *testing.T) {
	router := queuedTTLRouter()

	cases := []struct {
		cmd, arg, name string
	}{
		{"EXPIRE", "0", "queued EXPIRE 0"},
		{"EXPIRE", "9223372036854775807", "queued EXPIRE MaxInt64"},
		{"EXPIRE", "-9223372036854775808", "queued EXPIRE MinInt64"},
		{"EXPIRE", "9223372037", "queued EXPIRE just over the int64-nanosecond limit"},
		{"PEXPIRE", "0", "queued PEXPIRE 0"},
		{"PEXPIRE", "9223372036854775807", "queued PEXPIRE MaxInt64"},
		{"PEXPIRE", "-9223372036854775808", "queued PEXPIRE MinInt64"},
		{"PEXPIRE", "9223372036855", "queued PEXPIRE just over the int64-nanosecond limit"},
	}

	for _, c := range cases {
		s := store.NewStore()
		// A live key with a value, so a destroyed key is observable.
		queuedTTLRaw(t, s, router, "SET", "k", "v")

		raw := queuedTTLRun(t, s, router, [][]string{
			{"MULTI"},
			{c.cmd, "k", c.arg},
		})
		elem := queuedTTLExecValue(t, raw)

		if elem.Type != resp.TypeError {
			// It accepted the value. Now show the damage.
			ttl := queuedTTLRaw(t, s, router, "TTL", "k")
			got := queuedTTLRaw(t, s, router, "GET", "k")
			t.Fatalf("FAIL: %s: the queued %s replied %v instead of an error; TTL is now %q and "+
				"GET returns %q -- the unbounded argument wrapped the expiry into the past, so the key "+
				"was destroyed behind a success reply. The same command OUTSIDE a transaction is "+
				"refused with \"ERR invalid expire time\"",
				c.name, c.cmd, elem.Type, ttl, got)
		}

		// The key must still be there after the refusal.
		got := queuedTTLRaw(t, s, router, "GET", "k")
		if got != "$1\r\nv\r\n" {
			t.Fatalf("FAIL: %s: the key was destroyed by a REFUSED queued %s: GET returns %q, want %q",
				c.name, c.cmd, got, "$1\r\nv\r\n")
		}
	}
}

// TestQueuedExpireControlsUncontended is the control.
//
//  1. The TOP-LEVEL EXPIRE/PEXPIRE already refuse a zero or out-of-range
//     value. That is the contract the queued path must match, and it must
//     keep refusing.
//  2. A NEGATIVE expiry is accepted by the top-level handler and deletes the
//     key (Redis' "negative TTL deletes now"). The queued path must behave
//     the SAME way -- it is not part of this defect.
//  3. A valid EXPIRE/PEXPIRE queued inside MULTI/EXEC must still be accepted
//     and must still apply a positive TTL.
func TestQueuedExpireControlsUncontended(t *testing.T) {
	router := queuedTTLRouter()

	// (1) The direct path already refuses zero and out-of-range values.
	for _, c := range []struct {
		cmd, arg, name string
	}{
		{"EXPIRE", "0", "direct EXPIRE 0"},
		{"EXPIRE", "9223372036854775807", "direct EXPIRE MaxInt64"},
		{"PEXPIRE", "0", "direct PEXPIRE 0"},
		{"PEXPIRE", "9223372036854775807", "direct PEXPIRE MaxInt64"},
	} {
		s := store.NewStore()
		queuedTTLRaw(t, s, router, "SET", "k", "v")
		out := queuedTTLRaw(t, s, router, c.cmd, "k", c.arg)
		if !strings.HasPrefix(out, "-") {
			t.Fatalf("CONTROL FAIL: %s: replied %q, want an error", c.name, out)
		}
		if got := queuedTTLRaw(t, s, router, "GET", "k"); got != "$1\r\nv\r\n" {
			t.Fatalf("CONTROL FAIL: %s: GET returns %q, want %q -- a refused expiry must not "+
				"destroy the key", c.name, got, "$1\r\nv\r\n")
		}
	}

	// (2) A negative expiry deletes the key in BOTH paths, and both reply :1.
	for _, c := range []struct {
		cmd, arg, name string
	}{
		{"EXPIRE", "-1", "direct EXPIRE -1"},
		{"PEXPIRE", "-1", "direct PEXPIRE -1"},
	} {
		s := store.NewStore()
		queuedTTLRaw(t, s, router, "SET", "k", "v")
		out := queuedTTLRaw(t, s, router, c.cmd, "k", c.arg)
		if out != ":1\r\n" {
			t.Fatalf("CONTROL FAIL: %s: replied %q, want :1", c.name, out)
		}
		if got := queuedTTLRaw(t, s, router, "GET", "k"); got != "$-1\r\n" {
			t.Fatalf("CONTROL FAIL: %s: GET returns %q, want a null -- a negative expiry deletes "+
				"the key in the direct path", c.name, got)
		}
	}
	for _, c := range []struct {
		cmd, arg, name string
	}{
		{"EXPIRE", "-1", "queued EXPIRE -1"},
		{"PEXPIRE", "-1", "queued PEXPIRE -1"},
	} {
		s := store.NewStore()
		queuedTTLRaw(t, s, router, "SET", "k", "v")
		raw := queuedTTLRun(t, s, router, [][]string{
			{"MULTI"},
			{c.cmd, "k", c.arg},
		})
		elem := queuedTTLExecValue(t, raw)
		if elem.Type != resp.TypeInteger || elem.Int != 1 {
			t.Fatalf("CONTROL FAIL: %s: the queued %s replied %v, want the integer 1 -- a negative "+
				"expiry is accepted by the direct path, so the queued path must accept it too",
				c.name, c.cmd, elem)
		}
		if got := queuedTTLRaw(t, s, router, "GET", "k"); got != "$-1\r\n" {
			t.Fatalf("CONTROL FAIL: %s: GET returns %q, want a null -- a negative expiry deletes "+
				"the key in the queued path too", c.name, got)
		}
	}

	// (3) A valid expiry queued inside MULTI/EXEC must be accepted and apply a
	// positive TTL, and must not destroy the key.
	for _, c := range []struct {
		cmd, arg, name string
	}{
		{"EXPIRE", "100", "queued EXPIRE 100"},
		{"PEXPIRE", "100000", "queued PEXPIRE 100000"},
	} {
		s := store.NewStore()
		queuedTTLRaw(t, s, router, "SET", "k", "v")

		raw := queuedTTLRun(t, s, router, [][]string{
			{"MULTI"},
			{c.cmd, "k", c.arg},
		})
		elem := queuedTTLExecValue(t, raw)
		if elem.Type != resp.TypeInteger || elem.Int != 1 {
			t.Fatalf("CONTROL FAIL: %s: the queued %s replied %v, want the integer 1",
				c.name, c.cmd, elem)
		}
		ttl := queuedTTLRaw(t, s, router, "TTL", "k")
		if !strings.HasPrefix(ttl, ":") || strings.HasPrefix(ttl, ":-") {
			t.Fatalf("CONTROL FAIL: %s: TTL replied %q, want a POSITIVE expiry -- a valid queued "+
				"expiry must still set a TTL", c.name, ttl)
		}
		if got := queuedTTLRaw(t, s, router, "GET", "k"); got != "$1\r\nv\r\n" {
			t.Fatalf("CONTROL FAIL: %s: GET returns %q, want %q -- a valid queued expiry must not "+
				"destroy the key", c.name, got, "$1\r\nv\r\n")
		}
	}
}
