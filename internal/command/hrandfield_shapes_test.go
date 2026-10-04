package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestHRandFieldShapes(t *testing.T) {
	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterHashCommands(router)
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

	run("HSET", "h", "f1", "v1", "f2", "v2", "f3", "v3", "f4", "v4")

	t.Run("count_zero_on_existing_is_empty_array", func(t *testing.T) {
		got := run("HRANDFIELD", "h", "0")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("count 0 on existing hash replied %v, want an empty array", got)
		}
	})

	t.Run("missing_key_with_count_is_empty_array", func(t *testing.T) {
		got := run("HRANDFIELD", "nosuch", "3")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("missing key with count replied %v, want an empty array", got)
		}
	})

	t.Run("empty_hash_with_count_is_empty_array", func(t *testing.T) {
		run("HSET", "empty", "x", "y")
		run("HDEL", "empty", "x")
		got := run("HRANDFIELD", "empty", "3")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("empty hash with count replied %v, want an empty array", got)
		}
	})

	t.Run("no_count_missing_key_null_control", func(t *testing.T) {
		got := run("HRANDFIELD", "nosuch")
		if !got.IsNull {
			t.Fatalf("no-count missing key replied %v, want null", got)
		}
	})

	t.Run("no_count_existing_single_bulk_control", func(t *testing.T) {
		got := run("HRANDFIELD", "h")
		if got.Type != resp.TypeBulkString || got.IsNull {
			t.Fatalf("no-count existing replied %v, want a field bulk", got)
		}
	})

	t.Run("negative_count_cycles_control", func(t *testing.T) {
		got := run("HRANDFIELD", "h", "-2", "WITHVALUES")
		if got.Type != resp.TypeArray || len(got.Array) != 4 {
			t.Fatalf("count -2 WITHVALUES delivered %v, want 4 elements (pairs)", got)
		}
	})
}
