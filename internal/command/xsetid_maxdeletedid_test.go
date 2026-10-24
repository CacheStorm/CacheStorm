package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestXSetIdMaxDeletedIDFloor(t *testing.T) {
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

	t.Run("maxdeletedid_above_the_id_rejected", func(t *testing.T) {
		got := run("XSETID", "s", "1-0", "MAXDELETEDID", "7-0")
		if got.Type != resp.TypeError || got.Err != "ERR The ID specified in XSETID is smaller than the provided max_deleted_entry_id" {
			t.Fatalf("XSETID 1-0 MAXDELETEDID 7-0 replied %v, want the Redis error", got)
		}
	})

	t.Run("xsetid_with_maxdeletedid_accepted", func(t *testing.T) {
		got := run("XSETID", "s", "7-0", "MAXDELETEDID", "5-0")
		if got.Type != resp.TypeSimpleString {
			t.Fatalf("XSETID with MAXDELETEDID replied %v, want OK", got)
		}
	})

	t.Run("xadd_at_the_new_top_rejected", func(t *testing.T) {
		got := run("XADD", "s", "7-0", "f", "v")
		if got.Type != resp.TypeError {
			t.Fatalf("XADD 7-0 at the 7-0 top succeeded with %v, want an error", got)
		}
	})

	t.Run("xadd_above_the_top_succeeds", func(t *testing.T) {
		got := run("XADD", "s", "8-0", "f", "v")
		if got.Type != resp.TypeBulkString {
			t.Fatalf("XADD 8-0 above the top replied %v, want the id", got)
		}
	})

	t.Run("xsetid_below_max_deleted_rejected", func(t *testing.T) {
		run("XADD", "d", "5-0", "f", "v")
		run("XDEL", "d", "5-0")
		got := run("XSETID", "d", "4-0")
		if got.Type != resp.TypeError || got.Err != "ERR The ID specified in XSETID is smaller than current max_deleted_entry_id" {
			t.Fatalf("XSETID below the max deleted id replied %v, want the Redis error", got)
		}
	})

	t.Run("without_maxdeletedid_no_floor_control", func(t *testing.T) {
		run("XADD", "t", "1-0", "f", "v")
		run("XSETID", "t", "2-0")
		got := run("XADD", "t", "9-0", "f", "v")
		if got.Type != resp.TypeBulkString {
			t.Fatalf("no-floor stream rejected 9-0 with %v, want success", got)
		}
	})

	t.Run("below_lastid_still_rejected_control", func(t *testing.T) {
		got := run("XADD", "s", "1-0", "f", "v")
		if got.Type != resp.TypeError {
			t.Fatalf("XADD 1-0 at LastID succeeded with %v, want an error", got)
		}
	})
}
