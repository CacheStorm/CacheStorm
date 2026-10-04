package command_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdXREAD's option loop started at index 1 and streamsIdx
// defaulted to 1 — it assumed the command name sat at Args[0], but
// CacheStorm's Context convention excludes the command. Every option in
// first position was therefore silently ignored: XREAD COUNT 2 ... returned
// unlimited results, XREAD BLOCK ... never blocked, unknown first tokens
// were accepted, and a negative BLOCK was silently treated as non-blocking.
// Stream IDs were also never normalized: a partial ID ("2") reached GetRange
// raw, which cannot parse it, so the read silently returned an empty reply
// instead of delivering everything after 2-0; a garbage ID did the same
// instead of erroring.
func TestXReadGrammar(t *testing.T) {
	seeded := func(t *testing.T) func(name string, args ...string) *resp.Value {
		t.Helper()
		s := store.NewStore()
		router := command.NewRouter()
		command.RegisterStreamCommands(router)
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
		for _, id := range []string{"1-0", "2-0", "3-0"} {
			if got := run("XADD", "s", id, "f", "v"); got.Type != resp.TypeBulkString || string(got.Bulk) != id {
				t.Fatalf("seed XADD replied %v %q, want %s", got.Type, got.Bulk, id)
			}
		}
		return run
	}
	entryIDs := func(t *testing.T, v *resp.Value) []string {
		t.Helper()
		if v.Type != resp.TypeArray {
			t.Fatalf("XREAD replied %v, want an array", v.Type)
		}
		ids := make([]string, 0, len(v.Array))
		for _, stream := range v.Array {
			if stream.Type != resp.TypeArray || len(stream.Array) != 2 {
				t.Fatalf("XREAD stream entry shape wrong: %v", stream.Type)
			}
			for _, e := range stream.Array[1].Array {
				ids = append(ids, string(e.Array[0].Bulk))
			}
		}
		return ids
	}

	t.Run("COUNT in first position is applied", func(t *testing.T) {
		run := seeded(t)
		ids := entryIDs(t, run("XREAD", "COUNT", "2", "STREAMS", "s", "0-0"))
		if len(ids) != 2 || ids[0] != "1-0" || ids[1] != "2-0" {
			t.Fatalf("XREAD COUNT 2 delivered %v, want 1-0 2-0", ids)
		}
	})

	t.Run("unknown option in first position is a syntax error", func(t *testing.T) {
		run := seeded(t)
		if got := run("XREAD", "FOO", "STREAMS", "s", "0-0"); got.Type != resp.TypeError {
			t.Fatalf("XREAD FOO ... replied %v with %d entries, want a syntax error", got.Type, len(got.Array))
		}
	})

	t.Run("negative BLOCK in first position is an error", func(t *testing.T) {
		run := seeded(t)
		if got := run("XREAD", "BLOCK", "-5", "STREAMS", "s", "0-0"); got.Type != resp.TypeError {
			t.Fatalf("XREAD BLOCK -5 replied %v, want an error", got.Type)
		}
	})

	t.Run("missing STREAMS keyword with valid arg count is a syntax error", func(t *testing.T) {
		run := seeded(t)
		if got := run("XREAD", "s", "t", "0-0", "0-0"); got.Type != resp.TypeError {
			t.Fatalf("XREAD without STREAMS replied %v, want a syntax error", got.Type)
		}
	})

	t.Run("partial ID excludes its boundary entry", func(t *testing.T) {
		run := seeded(t)
		ids := entryIDs(t, run("XREAD", "STREAMS", "s", "2"))
		if len(ids) != 1 || ids[0] != "3-0" {
			t.Fatalf("XREAD STREAMS s 2 delivered %v, want only 3-0", ids)
		}
	})

	t.Run("garbage ID is an error", func(t *testing.T) {
		run := seeded(t)
		if got := run("XREAD", "STREAMS", "s", "notanid"); got.Type != resp.TypeError {
			t.Fatalf("XREAD with garbage ID replied %v, want an error", got.Type)
		}
	})

	t.Run("BLOCK in first position blocks until data", func(t *testing.T) {
		s := store.NewStore()
		router := command.NewRouter()
		command.RegisterStreamCommands(router)
		exec := func(name string, cmdArgs ...string) *resp.Value {
			argv := make([][]byte, len(cmdArgs))
			for i, arg := range cmdArgs {
				argv[i] = []byte(arg)
			}
			var buf bytes.Buffer
			ctx := command.NewContext(name, argv, s, resp.NewWriter(&buf))
			if err := router.Execute(ctx); err != nil {
				t.Fatalf("%s execution: %v", name, err)
			}
			v, err := resp.NewReader(&buf).ReadValue()
			if err != nil {
				t.Fatalf("reply: %v", err)
			}
			return v
		}
		for _, id := range []string{"1-0", "2-0", "3-0"} {
			exec("XADD", "s", id, "f", "v")
		}
		go func() {
			time.Sleep(50 * time.Millisecond)
			exec("XADD", "s", "4-0", "f", "v")
		}()
		start := time.Now()
		got := exec("XREAD", "BLOCK", "2000", "STREAMS", "s", "$")
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("blocking XREAD took %v, unblock must be bounded", elapsed)
		}
		v := got
		if v.Type != resp.TypeArray {
			t.Fatalf("blocked XREAD replied %v after %v, want the new entry 4-0", v.Type, time.Since(start))
		}
		ids := entryIDs(t, v)
		if len(ids) != 1 || ids[0] != "4-0" {
			t.Fatalf("blocked XREAD delivered %v, want the new 4-0", ids)
		}
	})

	t.Run("full ID control", func(t *testing.T) {
		run := seeded(t)
		ids := entryIDs(t, run("XREAD", "STREAMS", "s", "2-0"))
		if len(ids) != 1 || ids[0] != "3-0" {
			t.Fatalf("full-ID control delivered %v, want only 3-0", ids)
		}
	})

	t.Run("keyword first control", func(t *testing.T) {
		run := seeded(t)
		ids := entryIDs(t, run("XREAD", "STREAMS", "s", "0-0"))
		if len(ids) != 3 {
			t.Fatalf("keyword-first control delivered %v, want all three", ids)
		}
	})

	t.Run("multi stream control", func(t *testing.T) {
		run := seeded(t)
		if got := run("XADD", "t", "9-0", "f", "w"); got.Type != resp.TypeBulkString {
			t.Fatalf("seed XADD t: %v", got.Type)
		}
		got := run("XREAD", "STREAMS", "s", "t", "2-0", "0")
		if got.Type != resp.TypeArray || len(got.Array) != 2 {
			t.Fatalf("multi-stream control returned %d streams, want 2", len(got.Array))
		}
	})
}
