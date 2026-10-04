package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdXSETID accepted any string as the stream's last ID — a
// malformed ID silently replaced LastID, which broke the monotonicity guard
// every later XADD relies on (the equal-or-smaller check cannot parse the
// stored ID and passes everything). A smaller-than-top ID was accepted where
// Redis errors, partial IDs ("7") were stored unnormalized, and the
// MAXDELETEDID option always failed with a syntax error because its case
// skipped only the option token, feeding the value into the default branch.
func TestXSetIDValidation(t *testing.T) {
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

	t.Run("malformed id is an error", func(t *testing.T) {
		run := seeded(t)
		if got := run("XSETID", "s", "abc"); got.Type != resp.TypeError {
			t.Fatalf("XSETID with a malformed id replied %v, want an error", got.Type)
		}
	})

	t.Run("id smaller than top is an error", func(t *testing.T) {
		run := seeded(t)
		if got := run("XSETID", "s", "2-0"); got.Type != resp.TypeError {
			t.Fatalf("XSETID 2-0 with top 3-0 replied %v, want an error", got.Type)
		}
		if got := run("XADD", "s", "3-5", "f", "v"); got.Type != resp.TypeBulkString {
			t.Fatalf("XADD after rejected XSETID: %v, want 3-5 accepted", got.Type)
		}
	})

	t.Run("partial id normalizes and enforces monotonicity", func(t *testing.T) {
		run := seeded(t)
		if got := run("XSETID", "s", "7"); got.Type != resp.TypeSimpleString {
			t.Fatalf("XSETID 7 replied %v, want OK", got.Type)
		}
		if got := run("XADD", "s", "6-9", "f", "v"); got.Type != resp.TypeError {
			t.Fatalf("XADD 6-9 after XSETID 7 replied %v, want an error", got.Type)
		}
	})

	t.Run("equal id control", func(t *testing.T) {
		run := seeded(t)
		if got := run("XSETID", "s", "3-0"); got.Type != resp.TypeSimpleString {
			t.Fatalf("XSETID equal to top replied %v, want OK", got.Type)
		}
	})

	t.Run("greater id control", func(t *testing.T) {
		run := seeded(t)
		if got := run("XSETID", "s", "9-0"); got.Type != resp.TypeSimpleString {
			t.Fatalf("XSETID 9-0 replied %v, want OK", got.Type)
		}
		if got := run("XADD", "s", "4-0", "f", "v"); got.Type != resp.TypeError {
			t.Fatalf("XADD 4-0 after XSETID 9-0 replied %v, want an error", got.Type)
		}
	})

	t.Run("MAXDELETEDID with a valid id is accepted", func(t *testing.T) {
		run := seeded(t)
		if got := run("XSETID", "s", "8-0", "MAXDELETEDID", "5-0"); got.Type != resp.TypeSimpleString {
			t.Fatalf("XSETID MAXDELETEDID replied %v %q, want OK", got.Type, got.Str)
		}
	})

	t.Run("MAXDELETEDID with a malformed id is an error", func(t *testing.T) {
		run := seeded(t)
		if got := run("XSETID", "s", "8-0", "MAXDELETEDID", "junk"); got.Type != resp.TypeError {
			t.Fatalf("XSETID MAXDELETEDID junk replied %v, want an error", got.Type)
		}
	})

	t.Run("ENTRIESADDED control", func(t *testing.T) {
		run := seeded(t)
		if got := run("XSETID", "s", "8-0", "ENTRIESADDED", "5"); got.Type != resp.TypeSimpleString {
			t.Fatalf("XSETID ENTRIESADDED replied %v %q, want OK", got.Type, got.Str)
		}
	})

	t.Run("missing stream control", func(t *testing.T) {
		run := seeded(t)
		if got := run("XSETID", "nosuch", "1-0"); got.Type != resp.TypeError {
			t.Fatalf("XSETID on a missing stream replied %v, want an error", got.Type)
		}
	})
}
