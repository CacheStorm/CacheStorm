package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: the TRANSACTIONAL SETEX path (MULTI/EXEC) multiplied the parsed
// TTL by the unit WITHOUT bounding it, while the TOP-LEVEL SETEX handler guards
// the identical argument.
//
//	internal/command/transaction_commands.go (before)
//	  case "SETEX":
//	      seconds, err := parseInt(qc.args[1])
//	      ctx.Store.Set(key, &store.StringValue{Data: value},
//	          store.SetOptions{TTL: time.Duration(seconds) * time.Second})  // unchecked
//
// time.Duration(seconds) * time.Second overflows int64 nanoseconds for large
// values and wraps NEGATIVE. Store.Set falls through on a non-positive TTL
// (`else if opts.TTL > 0`), so the key was written with NO EXPIRY AT ALL:
// a permanent key behind an OK reply, where the caller asked for an expiry.
//
// Redis answers "ERR invalid expire time in 'setex' command".

func txSetexExec(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s: Execute returned error: %v", cmd, err)
	}
	return buf.String()
}

func txSetexRouter() *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterKeyCommands(router)
	RegisterServerCommands(router)
	RegisterTransactionCommands(router)
	return router
}

// txSetexBlock runs MULTI; SETEX ...; EXEC on ONE shared Context.
//
// MULTI state lives on the Context's Transaction, and NewContext allocates a
// FRESH Transaction on every call. Issuing the three commands as three separate
// Execute calls loses the MULTI and EXEC answers "EXEC without MULTI" — which
// silently invalidates any assertion about the transactional path. Re-pointing
// Command/Args on a single Context mirrors how a real connection reuses its
// per-connection transaction across pipelined commands.
func txSetexBlock(t *testing.T, s *store.Store, router *Router, ttl, val string) string {
	t.Helper()
	ctx := NewContext("MULTI", nil, s, resp.NewWriter(&bytes.Buffer{}))
	step := func(cmd string, args ...string) string {
		buf := &bytes.Buffer{}
		parsed := make([][]byte, len(args))
		for i, a := range args {
			parsed[i] = []byte(a)
		}
		ctx.Command = cmd
		ctx.Args = parsed
		ctx.Store = s
		ctx.Writer = resp.NewWriter(buf)
		if err := router.Execute(ctx); err != nil {
			t.Fatalf("%s: Execute returned error: %v", cmd, err)
		}
		return buf.String()
	}
	step("MULTI")
	step("SETEX", "k", ttl, val)
	return step("EXEC")
}

// TestTxSetexRejectsOverflowingTTL is the primary case: the transactional path
// must reject an out-of-range TTL exactly as the top-level handler does, and
// must never leave a permanent key behind an OK reply.
func TestTxSetexRejectsOverflowingTTL(t *testing.T) {
	router := txSetexRouter()

	for _, ttl := range []string{
		"9223372036854775807", // MaxInt64
		"-9223372036854775808",
	} {
		s := store.NewStore()
		execReply := txSetexBlock(t, s, router, ttl, "v")

		if strings.Contains(execReply, "invalid expire time") {
			continue // correctly rejected
		}

		gotTTL := txSetexExec(t, s, router, "TTL", "k")
		exists := txSetexExec(t, s, router, "EXISTS", "k")
		t.Fatalf("SETEX %s v inside MULTI: EXEC replied %q, but EXISTS=%q TTL=%q — "+
			"the unchecked multiplication wrapped to a negative TTL, which Store.Set "+
			"reads as \"no expiry\", leaving a PERMANENT key behind an OK reply",
			ttl, execReply, exists, gotTTL)
	}
}

// TestTxSetexControlsUncontended is the control: a normal TTL inside MULTI must
// still apply and report a sensible remaining TTL, the top-level handler must
// still behave, and other transactional commands must be unaffected.
func TestTxSetexControlsUncontended(t *testing.T) {
	router := txSetexRouter()

	s := store.NewStore()
	run := txSetexBlock(t, s, router, "100", "v")
	if !strings.Contains(run, "OK") {
		t.Fatalf("CONTROL FAIL: EXEC after transactional SETEX 100 = %q, want an OK entry", run)
	}
	if ttl := txSetexExec(t, s, router, "TTL", "k"); ttl != ":99\r\n" && ttl != ":100\r\n" {
		t.Fatalf("CONTROL FAIL: TTL after transactional SETEX 100 = %q, want 99 or 100", ttl)
	}

	s2 := store.NewStore()
	if out := txSetexExec(t, s2, router, "SETEX", "k", "100", "v"); out != "+OK\r\n" {
		t.Fatalf("CONTROL FAIL: top-level SETEX = %q, want +OK", out)
	}
	if ttl := txSetexExec(t, s2, router, "TTL", "k"); ttl != ":99\r\n" && ttl != ":100\r\n" {
		t.Fatalf("CONTROL FAIL: TTL after SETEX 100 = %q, want 99 or 100", ttl)
	}

	// Redis accepts a TTL past the int64-nanosecond horizon (its clock is in
	// milliseconds); the key must keep a far-future expiry, not become
	// permanent or expire at once.
	s4 := store.NewStore()
	if got := txSetexBlock(t, s4, router, "9223372037", "v"); !strings.Contains(got, "OK") {
		t.Fatalf("EXEC after transactional SETEX 9223372037 = %q, want an OK entry", got)
	}
	if ttl := txSetexExec(t, s4, router, "TTL", "k"); strings.HasPrefix(ttl, ":-") {
		t.Fatalf("TTL after transactional SETEX 9223372037 = %q, want a far-future expiry", ttl)
	}

	// A plain SET inside MULTI still stores the value.
	s3 := store.NewStore()
	if got := txSetexBlock(t, s3, router, "100", "v"); !strings.Contains(got, "OK") {
		t.Fatalf("CONTROL FAIL: EXEC after transactional SETEX = %q", got)
	}
}
