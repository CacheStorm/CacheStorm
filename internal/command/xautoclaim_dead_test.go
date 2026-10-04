package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdXAUTOCLAIM's reply had only two elements where Redis 7
// returns three ([cursor, claimed, deleted-ids]), and PEL entries whose
// stream entry was removed by XDEL were silently skipped: they were claimed
// into the new consumer's PEL and stayed there forever, never reported.
// The start argument also reached GetPending's lexicographic guards raw, so
// a variable-width start mis-filtered the scan.
func TestXAutoClaimDeletedEntries(t *testing.T) {
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
	for _, id := range []string{"1-0", "2-0", "3-0", "9-0", "10-0", "11-0"} {
		if got := run("XADD", "s", id, "f", "v"); got.Type != resp.TypeBulkString || string(got.Bulk) != id {
			t.Fatalf("seed XADD replied %v %q, want %s", got.Type, got.Bulk, id)
		}
	}
	if got := run("XGROUP", "CREATE", "s", "g", "0"); got.Type != resp.TypeSimpleString {
		t.Fatalf("XGROUP CREATE replied %v, want OK", got.Type)
	}
	if got := run("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", ">"); got.Type != resp.TypeArray {
		t.Fatalf("deliver to c1: %v", got.Type)
	}
	if got := run("XDEL", "s", "2-0", "10-0"); got.Type != resp.TypeInteger || got.Int != 2 {
		t.Fatalf("XDEL replied %v %v, want 2", got.Type, got.Int)
	}

	// The scenario runs once: the first XAUTOCLAIM claims the live entries,
	// purges the dead ones, and hands the parent's captured reply to the
	// subtests that assert on it.
	first := run("XAUTOCLAIM", "s", "g", "c2", "0", "0-0")

	t.Run("reply carries the deleted-ids element", func(t *testing.T) {
		got := first
		if got.Type != resp.TypeArray || len(got.Array) != 3 {
			t.Fatalf("XAUTOCLAIM replied %d elements, want 3 [cursor, claimed, deleted]", len(got.Array))
		}
		if string(got.Array[0].Bulk) != "0-0" {
			t.Errorf("cursor %q, want 0-0 after a full scan", got.Array[0].Bulk)
		}
	})

	t.Run("dead entries are reported and live ones claimed", func(t *testing.T) {
		got := first
		if got.Type != resp.TypeArray || len(got.Array) != 3 {
			t.Fatalf("XAUTOCLAIM replied %d elements, want 3", len(got.Array))
		}
		claimed := got.Array[1]
		if claimed.Type != resp.TypeArray || len(claimed.Array) != 4 {
			t.Fatalf("claimed element has %v rows, want the 4 live entries", claimed.Type)
		}
		deleted := got.Array[2]
		if deleted.Type != resp.TypeArray || len(deleted.Array) != 2 {
			t.Fatalf("deleted element has %v entries, want the 2 XDEL'd ids", deleted.Type)
		}
		seen := map[string]bool{}
		for _, d := range deleted.Array {
			seen[string(d.Bulk)] = true
		}
		if !seen["2-0"] || !seen["10-0"] {
			t.Errorf("deleted ids %v, want 2-0 and 10-0", deleted.Array)
		}
	})

	t.Run("dead ids leave the pel", func(t *testing.T) {
		got := run("XPENDING", "s", "g")
		if got.Type != resp.TypeArray || len(got.Array) != 4 {
			t.Fatalf("XPENDING summary shape %v", got.Type)
		}
		if got.Array[0].Int != 4 {
			t.Fatalf("pending count %d, want 4 (dead entries must leave the PEL)", got.Array[0].Int)
		}
		detail := run("XPENDING", "s", "g", "-", "+", "100")
		for _, row := range detail.Array {
			if id := string(row.Array[0].Bulk); id == "2-0" || id == "10-0" {
				t.Errorf("deleted id %s still pending", id)
			}
		}
	})

	t.Run("claimed entries moved to the new consumer", func(t *testing.T) {
		got := run("XPENDING", "s", "g", "-", "+", "100", "c2")
		if got.Type != resp.TypeArray || len(got.Array) != 4 {
			t.Fatalf("c2 owns %v rows, want the 4 live entries", len(got.Array))
		}
	})

	t.Run("partial start filters numerically", func(t *testing.T) {
		got := run("XAUTOCLAIM", "s", "g", "c3", "0", "10-0")
		if got.Type != resp.TypeArray || len(got.Array) != 3 {
			t.Fatalf("XAUTOCLAIM replied %d elements, want 3", len(got.Array))
		}
		claimed := got.Array[1]
		if claimed.Type != resp.TypeArray || len(claimed.Array) != 1 {
			t.Fatalf("claimed %v rows from start 10-0, want only 11-0", claimed.Type)
		}
		if string(claimed.Array[0].Array[0].Bulk) != "11-0" {
			t.Fatalf("claimed %q, want 11-0", claimed.Array[0].Array[0].Bulk)
		}
	})

	t.Run("missing stream control", func(t *testing.T) {
		got := run("XAUTOCLAIM", "nosuch", "g", "c2", "0", "0-0")
		if got.Type != resp.TypeArray || len(got.Array) != 3 {
			t.Fatalf("missing-stream reply has %d elements, want 3", len(got.Array))
		}
		if string(got.Array[0].Bulk) != "0-0" {
			t.Errorf("missing-stream cursor %q, want 0-0", got.Array[0].Bulk)
		}
	})
}
