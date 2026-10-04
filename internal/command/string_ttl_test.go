package command_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: in-place string modifications dropped the key's TTL.
// Store.Set never read SetOptions.KeepTTL (a dead flag: SET k v KEEPTTL
// silently cleared the expiry), and the in-place modifiers — APPEND,
// SETRANGE, INCR/DECR/INCRBY/DECRBY, INCRBYFLOAT, and the EXEC replay of a
// queued INCR — passed SetOptions{}, which replaces the entry and erases the
// expiry. Redis preserves the TTL for in-place modifications and clears it
// only on plain SET.
func TestStringModifyPreservesTTL(t *testing.T) {
	newRouter := func(t *testing.T) (*store.Store, func(name string, args ...string) *resp.Value) {
		t.Helper()
		s := store.NewStore()
		router := command.NewRouter()
		command.RegisterStringCommands(router)
		command.RegisterTransactionCommands(router)
		run := func(name string, args ...string) *resp.Value {
			t.Helper()
			argv := make([][]byte, len(args))
			for i, arg := range args {
				argv[i] = []byte(arg)
			}
			var buf bytes.Buffer
			ctx := command.NewContext(name, argv, s, resp.NewWriter(&buf))
			if err := router.Execute(ctx); err != nil {
				t.Fatalf("%s execution: %v", name, err)
			}
			v, err := resp.NewReader(&buf).ReadValue()
			if err != nil {
				t.Fatalf("%s reply: %v", name, err)
			}
			return v
		}
		return s, run
	}
	ttlOf := func(t *testing.T, s *store.Store, key string) time.Duration {
		t.Helper()
		return s.GetTTL(key)
	}
	mustHaveTTL := func(t *testing.T, s *store.Store, key string) {
		t.Helper()
		if ttl := ttlOf(t, s, key); ttl <= 0 {
			t.Fatalf("TTL of %q = %v, want ~100s (the modification must preserve the expiry)", key, ttl)
		}
	}

	t.Run("APPEND preserves ttl", func(t *testing.T) {
		s, run := newRouter(t)
		run("SET", "k", "hello", "EX", "100")
		if got := run("APPEND", "k", "world"); got.Type != resp.TypeInteger || got.Int != 10 {
			t.Fatalf("APPEND replied %v, want 10", got)
		}
		mustHaveTTL(t, s, "k")
	})

	t.Run("SETRANGE preserves ttl", func(t *testing.T) {
		s, run := newRouter(t)
		run("SET", "k", "hello", "EX", "100")
		if got := run("SETRANGE", "k", "1", "X"); got.Type != resp.TypeInteger || got.Int != 5 {
			t.Fatalf("SETRANGE replied %v, want 5", got)
		}
		mustHaveTTL(t, s, "k")
	})

	t.Run("INCR preserves ttl", func(t *testing.T) {
		s, run := newRouter(t)
		run("SET", "k", "10", "EX", "100")
		if got := run("INCR", "k"); got.Type != resp.TypeInteger || got.Int != 11 {
			t.Fatalf("INCR replied %v, want 11", got)
		}
		mustHaveTTL(t, s, "k")
	})

	t.Run("INCRBYFLOAT preserves ttl", func(t *testing.T) {
		s, run := newRouter(t)
		run("SET", "k", "1.5", "EX", "100")
		if got := run("INCRBYFLOAT", "k", "1"); got.Type != resp.TypeBulkString {
			t.Fatalf("INCRBYFLOAT replied %v, want a bulk string", got.Type)
		}
		mustHaveTTL(t, s, "k")
	})

	t.Run("SET KEEPTTL preserves ttl", func(t *testing.T) {
		s, run := newRouter(t)
		run("SET", "k", "v", "EX", "100")
		if got := run("SET", "k", "v2", "KEEPTTL"); got.Type != resp.TypeSimpleString {
			t.Fatalf("SET KEEPTTL replied %v, want OK", got.Type)
		}
		mustHaveTTL(t, s, "k")
	})

	t.Run("queued INCR in EXEC preserves ttl", func(t *testing.T) {
		s, run := newRouter(t)
		run("SET", "k", "10", "EX", "100")
		// MULTI/INCR/EXEC must share one context: the transaction lives on
		// the Context, exactly like one connection's session.
		argv := [][]byte{[]byte("MULTI")}
		shared := command.NewContext("MULTI", argv, s, resp.NewWriter(&bytes.Buffer{}))
		router := command.NewRouter()
		command.RegisterStringCommands(router)
		command.RegisterTransactionCommands(router)
		execOn := func(name string, cmdArgs ...string) *resp.Value {
			var buf bytes.Buffer
			shared.Writer = resp.NewWriter(&buf)
			shared.Command = name
			shared.Args = nil
			for _, a := range cmdArgs {
				shared.Args = append(shared.Args, []byte(a))
			}
			if err := router.Execute(shared); err != nil {
				t.Fatalf("%s execution: %v", name, err)
			}
			v, err := resp.NewReader(&buf).ReadValue()
			if err != nil {
				t.Fatalf("%s reply: %v", name, err)
			}
			return v
		}
		if got := execOn("MULTI"); got.Type != resp.TypeSimpleString {
			t.Fatalf("MULTI replied %v, want OK", got)
		}
		if got := execOn("INCR", "k"); got.Type != resp.TypeSimpleString || got.Str != "QUEUED" {
			t.Fatalf("queued INCR replied %v, want QUEUED", got)
		}
		if got := execOn("EXEC"); got.Type != resp.TypeArray {
			t.Fatalf("EXEC replied %v, want an array", got)
		}
		mustHaveTTL(t, s, "k")
	})

	t.Run("plain SET still clears ttl", func(t *testing.T) {
		s, run := newRouter(t)
		run("SET", "k", "v", "EX", "100")
		run("SET", "k", "v2")
		if ttl := ttlOf(t, s, "k"); ttl >= 0 {
			t.Fatalf("TTL after plain SET = %v, want negative (SET clears the expiry)", ttl)
		}
	})

	t.Run("fresh key has no ttl control", func(t *testing.T) {
		s, run := newRouter(t)
		if got := run("INCR", "fresh"); got.Type != resp.TypeInteger || got.Int != 1 {
			t.Fatalf("INCR fresh replied %v, want 1", got)
		}
		if ttl := ttlOf(t, s, "fresh"); ttl >= 0 {
			t.Fatalf("TTL of fresh INCR key = %v, want negative", ttl)
		}
	})

	t.Run("seeded ttl control", func(t *testing.T) {
		s, run := newRouter(t)
		run("SET", "k", "v", "EX", "100")
		if ttl := ttlOf(t, s, "k"); ttl <= 0 {
			t.Fatalf("seed TTL = %v, want ~100s", ttl)
		}
	})
}
