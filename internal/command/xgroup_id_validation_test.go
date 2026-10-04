package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: XGROUP CREATE and XGROUP SETID stored the last-ID argument
// raw. A malformed id ("abc") created a group whose every '>' read parsed
// nothing and returned null forever, and a partial id ("5") was stored
// unnormalized so entries added after the group never arrived; Redis errors
// on the malformed form and expands the partial form. XGROUP SETID had the
// same raw-storage defect.
func TestXGroupIDValidation(t *testing.T) {
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

	t.Run("malformed create id is an error", func(t *testing.T) {
		run := seeded(t)
		if got := run("XGROUP", "CREATE", "s", "gX", "abc"); got.Type != resp.TypeError {
			t.Fatalf("XGROUP CREATE with a malformed id replied %v, want an error", got.Type)
		}
	})

	t.Run("malformed setid is an error", func(t *testing.T) {
		run := seeded(t)
		if got := run("XGROUP", "CREATE", "s", "g", "0-0"); got.Type != resp.TypeSimpleString {
			t.Fatalf("setup: %v", got.Type)
		}
		if got := run("XGROUP", "SETID", "s", "g", "abc"); got.Type != resp.TypeError {
			t.Fatalf("XGROUP SETID with a malformed id replied %v, want an error", got.Type)
		}
	})

	t.Run("partial create id expands and delivers new entries", func(t *testing.T) {
		run := seeded(t)
		if got := run("XGROUP", "CREATE", "s", "g5", "5"); got.Type != resp.TypeSimpleString {
			t.Fatalf("XGROUP CREATE 5 replied %v, want OK", got.Type)
		}
		if got := run("XADD", "s", "6-0", "f", "v"); got.Type != resp.TypeBulkString {
			t.Fatalf("XADD 6-0: %v", got.Type)
		}
		got := run("XREADGROUP", "GROUP", "g5", "c", "STREAMS", "s", ">")
		if got.Type != resp.TypeArray {
			t.Fatalf("read after partial-id create replied %v, want the new 6-0", got.Type)
		}
		if string(got.Array[0].Array[1].Array[0].Bulk) != "6-0" {
			t.Fatalf("delivered %q, want 6-0", got.Array[0].Array[1].Array[0].Bulk)
		}
	})

	t.Run("setid then read delivers new entries", func(t *testing.T) {
		run := seeded(t)
		if got := run("XGROUP", "CREATE", "s", "g", "0-0"); got.Type != resp.TypeSimpleString {
			t.Fatalf("setup: %v", got.Type)
		}
		if got := run("XGROUP", "SETID", "s", "g", "2-0"); got.Type != resp.TypeSimpleString {
			t.Fatalf("XGROUP SETID 2-0 replied %v, want OK", got.Type)
		}
		got := run("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", ">")
		if got.Type != resp.TypeArray {
			t.Fatalf("read after SETID replied %v, want entries after 2-0", got.Type)
		}
		if string(got.Array[0].Array[1].Array[0].Bulk) != "3-0" {
			t.Fatalf("delivered %q, want 3-0", got.Array[0].Array[1].Array[0].Bulk)
		}
	})

	t.Run("zero and dollar controls", func(t *testing.T) {
		run := seeded(t)
		if got := run("XGROUP", "CREATE", "s", "gz", "0"); got.Type != resp.TypeSimpleString {
			t.Fatalf("CREATE 0 replied %v, want OK", got.Type)
		}
		if got := run("XREADGROUP", "GROUP", "gz", "c", "STREAMS", "s", ">"); got.Type != resp.TypeArray {
			t.Fatalf("read after 0-create: %v", got.Type)
		}
		if got := run("XGROUP", "CREATE", "s", "gs", "$"); got.Type != resp.TypeSimpleString {
			t.Fatalf("CREATE $ replied %v, want OK", got.Type)
		}
		if got := run("XADD", "s", "9-0", "f", "v"); got.Type != resp.TypeBulkString {
			t.Fatalf("XADD 9-0: %v", got.Type)
		}
		got := run("XREADGROUP", "GROUP", "gs", "c", "STREAMS", "s", ">")
		if got.Type != resp.TypeArray {
			t.Fatalf("read after $-create: %v", got.Type)
		}
		if string(got.Array[0].Array[1].Array[0].Bulk) != "9-0" {
			t.Fatalf("delivered %q, want 9-0", got.Array[0].Array[1].Array[0].Bulk)
		}
	})
}
