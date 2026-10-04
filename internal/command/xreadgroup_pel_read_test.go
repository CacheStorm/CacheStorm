package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdXREADGROUP answered a non-> ID by scanning the raw stream
// (XREADGROUP GROUP g c STREAMS s 0 returned every stream entry, or nothing
// at all when the ID did not parse). Redis: a non-> ID returns the entries
// PENDING for the requesting consumer — delivered to it and not yet ACKed —
// with IDs greater than the given one, in ID order.
func TestXReadGroupPelRead(t *testing.T) {
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
	entryIDs := func(v *resp.Value) []string {
		t.Helper()
		if v == nil || v.Type != resp.TypeArray {
			t.Fatalf("replied %v, want an entry array", v)
		}
		var ids []string
		for _, tuple := range v.Array {
			if tuple == nil || tuple.Type != resp.TypeArray || len(tuple.Array) < 2 {
				continue
			}
			entries := tuple.Array[1]
			if entries == nil || entries.Type != resp.TypeArray {
				continue
			}
			for i := 0; i+1 < len(entries.Array); i += 2 {
				ids = append(ids, string(entries.Array[i].Bulk))
			}
		}
		return ids
	}
	wantIDs := func(got []string, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("delivered %v, want %v", got, want)
			return
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("delivered %v, want %v", got, want)
				return
			}
		}
	}

	if got := run("XADD", "s", "1-0", "f", "v"); got.Type != resp.TypeBulkString || string(got.Bulk) != "1-0" {
		t.Fatalf("seed XADD replied %v %q, want 1-0", got.Type, got.Bulk)
	}
	if got := run("XADD", "s", "2-0", "f", "v"); got.Type != resp.TypeBulkString || string(got.Bulk) != "2-0" {
		t.Fatalf("seed XADD replied %v %q, want 2-0", got.Type, got.Bulk)
	}
	if got := run("XADD", "s", "3-0", "f", "v"); got.Type != resp.TypeBulkString || string(got.Bulk) != "3-0" {
		t.Fatalf("seed XADD replied %v %q, want 3-0", got.Type, got.Bulk)
	}
	if got := run("XGROUP", "CREATE", "s", "g", "0"); got.Type != resp.TypeSimpleString || got.Str != "OK" {
		t.Fatalf("XGROUP CREATE replied %v %q, want OK", got.Type, got.Str)
	}

	// Deliver all three entries to consumer c (control; passes with the r50 fix).
	if got := run("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", ">"); got.Type != resp.TypeArray {
		t.Fatalf("first > delivered %v, want entries 1-0, 2-0 and 3-0", got)
	}

	// ACK 2-0: it must disappear from c's pending history.
	if got := run("XACK", "s", "g", "2-0"); got.Type != resp.TypeInteger || got.Int != 1 {
		t.Fatalf("XACK replied %v %v, want 1", got.Type, got.Int)
	}

	if got := run("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", "0"); got.Type != resp.TypeArray {
		t.Fatalf("PEL read 0 delivered %v, want entries 1-0 and 3-0 (2-0 was ACKed)", got)
	} else {
		wantIDs(entryIDs(got), "1-0", "3-0")
	}

	// Consumer isolation: 4-0 belongs to c2 and must not leak into c's PEL read.
	if got := run("XADD", "s", "4-0", "f", "v"); got.Type != resp.TypeBulkString || string(got.Bulk) != "4-0" {
		t.Fatalf("XADD replied %v %q, want 4-0", got.Type, got.Bulk)
	}
	if got := run("XREADGROUP", "GROUP", "g", "c2", "STREAMS", "s", ">"); got.Type != resp.TypeArray {
		t.Fatalf("c2 > delivered %v, want entry 4-0", got)
	}
	if got := run("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", "0"); got.Type != resp.TypeArray {
		t.Fatalf("PEL read 0 delivered %v, want entries 1-0 and 3-0 only (4-0 is c2's)", got)
	} else {
		wantIDs(entryIDs(got), "1-0", "3-0")
	}

	// COUNT caps the PEL read.
	if got := run("XREADGROUP", "GROUP", "g", "c", "COUNT", "1", "STREAMS", "s", "0"); got.Type != resp.TypeArray {
		t.Fatalf("PEL read COUNT 1 delivered %v, want entry 1-0 only", got)
	} else {
		wantIDs(entryIDs(got), "1-0")
	}

	// > with nothing new still answers null (control).
	if got := run("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", ">"); got.Type != resp.TypeNull {
		t.Fatalf("> with nothing new replied %v, want null", got)
	}
}
