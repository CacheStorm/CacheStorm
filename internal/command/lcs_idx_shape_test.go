package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdLCS honored the IDX option only on the normal path. When
// either key was missing or held an empty string, the early-return paths
// replied with the plain bulk-string form and silently ignored IDX — a
// client requesting the [matches, len] shape got a bare bulk string instead.
func TestLCSIdxReplyShape(t *testing.T) {
	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterStringCommands(router)
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

	assertIdxShape := func(t *testing.T, v *resp.Value, wantMatches int, wantLen int64) {
		t.Helper()
		if v.Type != resp.TypeArray || len(v.Array) != 4 {
			t.Fatalf("IDX reply has %v/%d elements, want the 4-element [matches, ..., len, ...] shape", v.Type, len(v.Array))
		}
		if string(v.Array[0].Bulk) != "matches" || string(v.Array[2].Bulk) != "len" {
			t.Fatalf("IDX reply markers %q/%q, want matches/len", v.Array[0].Bulk, v.Array[2].Bulk)
		}
		if v.Array[1].Type != resp.TypeArray || len(v.Array[1].Array) != wantMatches {
			t.Fatalf("matches has %d rows, want %d", len(v.Array[1].Array), wantMatches)
		}
		if v.Array[3].Int != wantLen {
			t.Fatalf("len = %d, want %d", v.Array[3].Int, wantLen)
		}
	}

	t.Run("IDX with both keys missing", func(t *testing.T) {
		assertIdxShape(t, run("LCS", "m1", "m2", "IDX"), 0, 0)
	})

	t.Run("IDX with an empty-string key", func(t *testing.T) {
		if got := run("SET", "empty", ""); got.Type != resp.TypeSimpleString {
			t.Fatalf("SET empty replied %v, want OK", got.Type)
		}
		assertIdxShape(t, run("LCS", "empty", "m2", "IDX"), 0, 0)
	})

	t.Run("normal IDX control", func(t *testing.T) {
		run("SET", "s1", "abcabc")
		run("SET", "s2", "abc")
		got := run("LCS", "s1", "s2", "IDX")
		assertIdxShape(t, got, 1, 3)
		match := got.Array[1].Array[0]
		// The backtrack from the end lands on s1's second "abc" (indices
		// 3-5) — a valid maximal match for this implementation.
		if match.Array[0].Array[0].Int != 3 || match.Array[0].Array[1].Int != 5 {
			t.Fatalf("A-range %v, want [3, 5]", match.Array[0])
		}
		if match.Array[1].Array[0].Int != 0 || match.Array[1].Array[1].Int != 2 {
			t.Fatalf("B-range %v, want [0, 2]", match.Array[1])
		}
	})

	t.Run("MINMATCHLEN control", func(t *testing.T) {
		assertIdxShape(t, run("LCS", "s1", "s2", "IDX", "MINMATCHLEN", "5"), 0, 3)
	})

	t.Run("WITHMATCHLEN control", func(t *testing.T) {
		got := run("LCS", "s1", "s2", "IDX", "WITHMATCHLEN")
		if got.Type != resp.TypeArray || len(got.Array) != 4 {
			t.Fatalf("WITHMATCHLEN reply shape %v/%d, want 4 elements", got.Type, len(got.Array))
		}
		row := got.Array[1].Array[0]
		if len(row.Array) != 3 {
			t.Fatalf("match row has %d elements, want 3 with WITHMATCHLEN", len(row.Array))
		}
		if row.Array[2].Int != 3 {
			t.Fatalf("match length = %d, want 3", row.Array[2].Int)
		}
	})

	t.Run("missing keys without options control", func(t *testing.T) {
		got := run("LCS", "m1", "m2")
		if got.Type != resp.TypeBulkString || len(got.Bulk) != 0 {
			t.Fatalf("plain LCS on missing keys replied %v %q, want empty bulk", got.Type, got.Bulk)
		}
	})

	t.Run("missing keys LEN control", func(t *testing.T) {
		got := run("LCS", "m1", "m2", "LEN")
		if got.Type != resp.TypeInteger || got.Int != 0 {
			t.Fatalf("LEN on missing keys replied %v %d, want 0", got.Type, got.Int)
		}
	})

	t.Run("plain LCS control", func(t *testing.T) {
		got := run("LCS", "s1", "s2")
		if got.Type != resp.TypeBulkString || string(got.Bulk) != "abc" {
			t.Fatalf("plain LCS replied %q, want abc", got.Bulk)
		}
	})
}
