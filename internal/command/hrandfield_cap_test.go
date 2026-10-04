package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestHRandFieldCapacityBounds(t *testing.T) {
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
	seed := func() {
		s.Delete("h")
		run("HSET", "h", "f1", "v1", "f2", "v2", "f3", "v3", "f4", "v4")
	}

	seed()
	t.Run("overflow_count_must_not_crash", func(t *testing.T) {
		got := run("HRANDFIELD", "h", "-5000000000000000000")
		if got.Type != resp.TypeArray {
			t.Fatalf("overflow count replied %v, want an array", got)
		}
		if len(got.Array) != 1000000 {
			t.Fatalf("overflow count delivered %d elements, want the 1000000 hardening cap", len(got.Array))
		}
	})

	t.Run("negative_count_cycles_with_repeats", func(t *testing.T) {
		got := run("HRANDFIELD", "h", "-7")
		if got.Type != resp.TypeArray || len(got.Array) != 7 {
			t.Fatalf("count -7 delivered %v, want 7 elements", got)
		}
	})

	t.Run("positive_count_clamps_to_size", func(t *testing.T) {
		got := run("HRANDFIELD", "h", "10")
		if got.Type != resp.TypeArray || len(got.Array) != 4 {
			t.Fatalf("count 10 delivered %d elements, want 4 (unique clamp)", len(got.Array))
		}
	})

	t.Run("zero_count_null_control", func(t *testing.T) {
		got := run("HRANDFIELD", "h", "0")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("count 0 replied %v, want an empty array (Redis count-form shape)", got)
		}
	})

	t.Run("withvalues_pairs", func(t *testing.T) {
		got := run("HRANDFIELD", "h", "-3", "WITHVALUES")
		if got.Type != resp.TypeArray || len(got.Array) != 6 {
			t.Fatalf("WITHVALUES -3 delivered %d elements, want 6 (pairs)", len(got.Array))
		}
	})

	t.Run("missing_key_null", func(t *testing.T) {
		got := run("HRANDFIELD", "nosuch", "-3")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("missing key with count replied %v, want an empty array (Redis count-form shape)", got)
		}
	})

	t.Run("single_no_count_bulk", func(t *testing.T) {
		got := run("HRANDFIELD", "h")
		if got.Type != resp.TypeBulkString || got.IsNull {
			t.Fatalf("no-count replied %v, want a field bulk", got)
		}
	})

	t.Run("empty_hash_negative_null", func(t *testing.T) {
		run("HSET", "empty", "x", "y")
		run("HDEL", "empty", "x")
		got := run("HRANDFIELD", "empty", "-3")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("empty hash replied %v, want an empty array (Redis count-form shape)", got)
		}
	})

	t.Run("malformed_count_error", func(t *testing.T) {
		got := run("HRANDFIELD", "h", "abc")
		if got.Type != resp.TypeError {
			t.Fatalf("malformed count replied %v, want an error", got)
		}
	})
}
