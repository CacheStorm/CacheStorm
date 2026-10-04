package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestXGroupCreateConsumer(t *testing.T) {
	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterStreamCommands(router)
	run := func(args ...string) *resp.Value {
		t.Helper()
		argv := make([][]byte, len(args))
		for i, a := range args {
			argv[i] = []byte(a)
		}
		var buf bytes.Buffer
		ctx := command.NewContext(args[0], argv[1:], s, resp.NewWriter(&buf))
		if err := router.Execute(ctx); err != nil {
			t.Fatalf("%v execution: %v", args, err)
		}
		v, err := resp.NewReader(&buf).ReadValue()
		if err != nil {
			t.Fatalf("%v reply: %v", args, err)
		}
		return v
	}

	run("XADD", "s", "1-0", "f", "v")
	run("XGROUP", "CREATE", "s", "g", "0")

	t.Run("create_new_consumer_returns_one", func(t *testing.T) {
		got := run("XGROUP", "CREATECONSUMER", "s", "g", "c1")
		if got.Type != resp.TypeInteger || got.Int != 1 {
			t.Fatalf("CREATECONSUMER new replied %v, want integer 1", got)
		}
	})

	t.Run("create_existing_consumer_returns_zero", func(t *testing.T) {
		got := run("XGROUP", "CREATECONSUMER", "s", "g", "c1")
		if got.Type != resp.TypeInteger || got.Int != 0 {
			t.Fatalf("CREATECONSUMER existing replied %v, want integer 0", got)
		}
	})

	t.Run("missing_group_is_nogroup_error", func(t *testing.T) {
		got := run("XGROUP", "CREATECONSUMER", "s", "nosuchgroup", "c2")
		if got.Type != resp.TypeError {
			t.Fatalf("missing group replied %v, want an error", got)
		}
	})

	t.Run("missing_key_is_error", func(t *testing.T) {
		got := run("XGROUP", "CREATECONSUMER", "nosuchkey", "g", "c2")
		if got.Type != resp.TypeError {
			t.Fatalf("missing key replied %v, want an error", got)
		}
	})

	t.Run("consumer_visible_in_xinfo_control", func(t *testing.T) {
		run("XGROUP", "CREATECONSUMER", "s", "g", "c9")
		got := run("XINFO", "CONSUMERS", "s", "g")
		if got.Type != resp.TypeArray {
			t.Fatalf("XINFO CONSUMERS replied %v, want an array", got)
		}
		found := false
		for _, row := range got.Array {
			for i := 0; i+1 < len(row.Array); i += 2 {
				if row.Array[i].Type == resp.TypeBulkString && string(row.Array[i].Bulk) == "name" &&
					row.Array[i+1].Type == resp.TypeBulkString && string(row.Array[i+1].Bulk) == "c9" {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("created consumer c9 not visible in XINFO CONSUMERS: %v", got)
		}
	})

	t.Run("xreadgroup_auto_create_control", func(t *testing.T) {
		run("XREADGROUP", "GROUP", "g", "cauto", "STREAMS", "s", ">")
		got := run("XGROUP", "CREATECONSUMER", "s", "g", "cauto")
		if got.Type != resp.TypeInteger || got.Int != 0 {
			t.Fatalf("auto-created consumer replied %v, want integer 0", got)
		}
	})

	t.Run("delconsumer_still_works_control", func(t *testing.T) {
		got := run("XGROUP", "DELCONSUMER", "s", "g", "c9")
		if got.Type != resp.TypeInteger || got.Int != 0 {
			t.Fatalf("DELCONSUMER replied %v, want integer 0 (no pending)", got)
		}
	})

	t.Run("wrong_arity_still_error", func(t *testing.T) {
		got := run("XGROUP", "CREATECONSUMER", "s", "g")
		if got.Type != resp.TypeError {
			t.Fatalf("CREATECONSUMER with 2 args replied %v, want an error", got)
		}
	})
}
