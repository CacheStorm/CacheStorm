package command_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdXPENDING's detail form was never aligned with the XPENDING
// contract. Bounds were passed raw into GetPending's lexicographic guards
// (pending 9-0/10-0/11-0 with start 9-0 dropped 10-0 and 11-0 because
// "10-0" >= "9-0" is false as strings), column 3 reported the absolute
// delivery timestamp instead of idle milliseconds, the consumer filter ran
// after the count cap (XPENDING s g - + 2 c2 could return nothing even
// though c2 owns entries), and entries came out in random map order instead
// of ID order.
func TestXPendingDetail(t *testing.T) {
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
	for _, id := range []string{"1-0", "2-0", "3-0", "4-0", "5-0", "6-0", "8-0", "9-0", "10-0", "11-0"} {
		if got := run("XADD", "s", id, "f", "v"); got.Type != resp.TypeBulkString || string(got.Bulk) != id {
			t.Fatalf("seed XADD replied %v %q, want %s", got.Type, got.Bulk, id)
		}
	}
	if got := run("XGROUP", "CREATE", "s", "g", "0"); got.Type != resp.TypeSimpleString {
		t.Fatalf("XGROUP CREATE replied %v, want OK", got.Type)
	}
	if got := run("XREADGROUP", "GROUP", "g", "c1", "COUNT", "3", "STREAMS", "s", ">"); got.Type != resp.TypeArray {
		t.Fatalf("deliver to c1: %v", got.Type)
	}
	if got := run("XREADGROUP", "GROUP", "g", "c2", "COUNT", "3", "STREAMS", "s", ">"); got.Type != resp.TypeArray {
		t.Fatalf("deliver to c2: %v", got.Type)
	}
	if got := run("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", ">"); got.Type != resp.TypeArray {
		t.Fatalf("deliver rest to c1: %v", got.Type)
	}
	time.Sleep(60 * time.Millisecond)

	pendingIDs := func(t *testing.T, v *resp.Value) []string {
		t.Helper()
		if v.Type != resp.TypeArray {
			t.Fatalf("XPENDING replied %v, want an array", v.Type)
		}
		ids := make([]string, 0, len(v.Array))
		for _, row := range v.Array {
			if row.Type != resp.TypeArray || len(row.Array) != 4 {
				t.Fatalf("XPENDING row shape wrong: %v", row.Type)
			}
			ids = append(ids, string(row.Array[0].Bulk))
		}
		return ids
	}

	t.Run("numeric bounds with variable-width ids", func(t *testing.T) {
		ids := pendingIDs(t, run("XPENDING", "s", "g", "9-0", "+", "10"))
		want := []string{"9-0", "10-0", "11-0"}
		if len(ids) != len(want) {
			t.Fatalf("XPENDING s g 9-0 + 10 delivered %v, want %v", ids, want)
		}
		for i := range want {
			if ids[i] != want[i] {
				t.Fatalf("XPENDING s g 9-0 + 10 delivered %v, want %v", ids, want)
			}
		}
	})

	t.Run("consumer filter applies before count", func(t *testing.T) {
		ids := pendingIDs(t, run("XPENDING", "s", "g", "-", "+", "2", "c2"))
		if len(ids) != 2 || ids[0] != "4-0" || ids[1] != "5-0" {
			t.Fatalf("XPENDING ... 2 c2 delivered %v, want 4-0 5-0", ids)
		}
	})

	t.Run("column 3 is idle milliseconds not a timestamp", func(t *testing.T) {
		v := run("XPENDING", "s", "g", "-", "+", "10")
		if v.Type != resp.TypeArray || len(v.Array) == 0 {
			t.Fatalf("XPENDING replied %v, want rows", v.Type)
		}
		row := v.Array[0]
		if row.Array[2].Type != resp.TypeInteger {
			t.Fatalf("idle column type %v, want integer", row.Array[2].Type)
		}
		idle := row.Array[2].Int
		if idle < 0 || idle > 5000 {
			t.Fatalf("idle column = %d, want idle milliseconds (< 5000 here)", idle)
		}
	})

	t.Run("entries come back in id order", func(t *testing.T) {
		ids := pendingIDs(t, run("XPENDING", "s", "g", "-", "+", "100"))
		want := []string{"1-0", "2-0", "3-0", "4-0", "5-0", "6-0", "8-0", "9-0", "10-0", "11-0"}
		if len(ids) != len(want) {
			t.Fatalf("XPENDING delivered %d entries, want %d", len(ids), len(want))
		}
		for i := range want {
			if ids[i] != want[i] {
				t.Fatalf("XPENDING delivered %v, want sorted %v", ids, want)
			}
		}
	})

	t.Run("garbage start is an error", func(t *testing.T) {
		if got := run("XPENDING", "s", "g", "notanid", "+", "10"); got.Type != resp.TypeError {
			t.Fatalf("XPENDING with garbage start replied %v, want an error", got.Type)
		}
	})

	t.Run("partial triple is a wrong arity", func(t *testing.T) {
		if got := run("XPENDING", "s", "g", "-"); got.Type != resp.TypeError {
			t.Fatalf("XPENDING with start only replied %v, want an error", got.Type)
		}
	})

	t.Run("non-positive count is an error", func(t *testing.T) {
		if got := run("XPENDING", "s", "g", "-", "+", "0"); got.Type != resp.TypeError {
			t.Fatalf("XPENDING with count 0 replied %v, want an error", got.Type)
		}
	})

	t.Run("summary control", func(t *testing.T) {
		v := run("XPENDING", "s", "g")
		if v.Type != resp.TypeArray || len(v.Array) != 4 {
			t.Fatalf("summary shape %v, want 4 elements", v.Type)
		}
		if v.Array[0].Int != 10 {
			t.Fatalf("summary count = %d, want 10", v.Array[0].Int)
		}
		if string(v.Array[1].Bulk) != "1-0" || string(v.Array[2].Bulk) != "11-0" {
			t.Fatalf("summary range %q..%q, want 1-0..11-0", v.Array[1].Bulk, v.Array[2].Bulk)
		}
	})

	t.Run("lexicographically safe bounds control", func(t *testing.T) {
		ids := pendingIDs(t, run("XPENDING", "s", "g", "-", "2-0", "10"))
		if len(ids) != 2 || ids[0] != "1-0" || ids[1] != "2-0" {
			t.Fatalf("bounds control delivered %v, want 1-0 2-0", ids)
		}
	})
}
