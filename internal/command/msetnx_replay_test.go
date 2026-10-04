package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: executeQueuedCommand had no MSETNX case, so a MSETNX queued
// inside MULTI failed at EXEC with "ERR command not supported in
// transaction" while the live command worked. Redis executes MSETNX in
// transactions: the replay must run the same all-or-nothing semantics
// (set every key when none exists, set nothing and reply 0 otherwise).
func TestQueuedMSetNXReplay(t *testing.T) {
	newSession := func(t *testing.T) (*store.Store, *command.Router, func(name string, args ...string) *resp.Value, func(name string, args ...string) *resp.Value) {
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
		// MULTI/QUEUED-commands/EXEC must share one context: the
		// transaction lives on the Context, like one connection's session.
		var shared *command.Context
		txn := func(name string, args ...string) *resp.Value {
			t.Helper()
			var buf bytes.Buffer
			if shared == nil {
				argv := make([][]byte, len(args))
				for i, arg := range args {
					argv[i] = []byte(arg)
				}
				shared = command.NewContext(name, argv, s, resp.NewWriter(&buf))
			} else {
				argv := make([][]byte, len(args))
				for i, arg := range args {
					argv[i] = []byte(arg)
				}
				shared.Command = name
				shared.Args = argv
				shared.Writer = resp.NewWriter(&buf)
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
		return s, router, run, txn
	}

	t.Run("live MSETNX control", func(t *testing.T) {
		s, _, run, _ := newSession(t)
		if got := run("MSETNX", "a", "1", "b", "2"); got.Type != resp.TypeInteger || got.Int != 1 {
			t.Fatalf("MSETNX replied %v, want 1", got)
		}
		if got := run("MSETNX", "a", "x", "c", "3"); got.Type != resp.TypeInteger || got.Int != 0 {
			t.Fatalf("MSETNX over existing replied %v, want 0", got)
		}
		if _, exists := s.Get("c"); exists {
			t.Fatal("c was set despite MSETNX returning 0")
		}
	})

	t.Run("queued MSETNX replays with all-or-nothing semantics", func(t *testing.T) {
		s, _, run, txn := newSession(t)
		run("SET", "guard", "v")
		if got := txn("MULTI"); got.Type != resp.TypeSimpleString {
			t.Fatalf("MULTI replied %v, want OK", got)
		}
		if got := txn("MSETNX", "q1", "1", "q2", "2"); got.Type != resp.TypeSimpleString || got.Str != "QUEUED" {
			t.Fatalf("queued MSETNX replied %v, want QUEUED", got)
		}
		got := txn("EXEC")
		if got.Type != resp.TypeArray || len(got.Array) != 1 {
			t.Fatalf("EXEC replied %v, want a 1-element array", got)
		}
		if got.Array[0].Type != resp.TypeInteger || got.Array[0].Int != 1 {
			t.Fatalf("replayed MSETNX element = %v, want integer 1 (got error %q)",
				got.Array[0], got.Array[0].Err)
		}
		if _, exists := s.Get("q1"); !exists {
			t.Fatal("q1 missing after EXEC")
		}
		if _, exists := s.Get("q2"); !exists {
			t.Fatal("q2 missing after EXEC")
		}
	})

	t.Run("queued MSETNX over existing key sets nothing", func(t *testing.T) {
		s, _, run, txn := newSession(t)
		run("SET", "z", "v")
		if got := txn("MULTI"); got.Type != resp.TypeSimpleString {
			t.Fatalf("MULTI replied %v, want OK", got)
		}
		if got := txn("MSETNX", "z", "x", "w", "2"); got.Str != "QUEUED" {
			t.Fatalf("queued MSETNX replied %v, want QUEUED", got)
		}
		got := txn("EXEC")
		if got.Type != resp.TypeArray || len(got.Array) != 1 {
			t.Fatalf("EXEC replied %v, want a 1-element array", got)
		}
		if got.Array[0].Type != resp.TypeInteger || got.Array[0].Int != 0 {
			t.Fatalf("replayed MSETNX element = %v, want integer 0", got.Array[0])
		}
		if _, exists := s.Get("w"); exists {
			t.Fatal("w was set despite the replayed MSETNX returning 0")
		}
	})

	t.Run("queued MSET control", func(t *testing.T) {
		s, _, _, txn := newSession(t)
		if got := txn("MULTI"); got.Type != resp.TypeSimpleString {
			t.Fatalf("MULTI replied %v, want OK", got)
		}
		txn("MSET", "m1", "1", "m2", "2")
		got := txn("EXEC")
		if got.Type != resp.TypeArray || len(got.Array) != 1 {
			t.Fatalf("EXEC replied %v, want a 1-element array", got)
		}
		if _, exists := s.Get("m1"); !exists {
			t.Fatal("MSET replay control failed: m1 missing")
		}
	})
}
