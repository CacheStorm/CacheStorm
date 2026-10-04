package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestZRandMemberNoCountShape(t *testing.T) {
	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterSortedSetCommands(router)
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

	run("ZADD", "z", "1", "a", "2", "b", "3", "c")

	t.Run("no_count_returns_bare_bulk_member", func(t *testing.T) {
		got := run("ZRANDMEMBER", "z")
		if got.Type != resp.TypeBulkString || got.IsNull || len(got.Bulk) == 0 {
			t.Fatalf("no-count ZRANDMEMBER replied %v, want a bare member bulk", got)
		}
	})

	t.Run("count_on_missing_key_is_empty_array", func(t *testing.T) {
		got := run("ZRANDMEMBER", "nosuch", "COUNT", "2")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("COUNT on missing key replied %v, want an empty array", got)
		}
	})

	t.Run("no_count_missing_key_null_control", func(t *testing.T) {
		got := run("ZRANDMEMBER", "nosuch")
		if !got.IsNull {
			t.Fatalf("no-count missing key replied %v, want null", got)
		}
	})

	t.Run("count_zero_empty_array_control", func(t *testing.T) {
		got := run("ZRANDMEMBER", "z", "COUNT", "0")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("count 0 replied %v, want an empty array", got)
		}
	})

	t.Run("negative_count_cycles_control", func(t *testing.T) {
		got := run("ZRANDMEMBER", "z", "COUNT", "-2")
		if got.Type != resp.TypeArray || len(got.Array) != 2 {
			t.Fatalf("count -2 delivered %v, want 2 elements", got)
		}
	})
}
