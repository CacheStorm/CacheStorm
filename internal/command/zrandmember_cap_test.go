package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestZRandMemberCapacityBounds(t *testing.T) {
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
	seed := func() {
		s.Delete("z")
		run("ZADD", "z", "1", "a", "2", "b", "3", "c", "4", "d", "5", "e")
	}

	seed()
	t.Run("overflow_count_must_not_crash", func(t *testing.T) {
		got := run("ZRANDMEMBER", "z", "-5000000000000000000")
		if got.Type != resp.TypeArray {
			t.Fatalf("overflow count replied %v, want an array", got)
		}
		if len(got.Array) != 1000000 {
			t.Fatalf("overflow count delivered %d elements, want the 1000000 hardening cap", len(got.Array))
		}
	})

	t.Run("negative_count_cycles_with_repeats", func(t *testing.T) {
		got := run("ZRANDMEMBER", "z", "-7")
		if got.Type != resp.TypeArray || len(got.Array) != 7 {
			t.Fatalf("count -7 delivered %v, want 7 elements", got)
		}
	})

	t.Run("positive_count_unique", func(t *testing.T) {
		got := run("ZRANDMEMBER", "z", "3")
		if got.Type != resp.TypeArray || len(got.Array) != 3 {
			t.Fatalf("count 3 delivered %v, want 3 elements", got)
		}
	})

	t.Run("zero_count_empty_array", func(t *testing.T) {
		got := run("ZRANDMEMBER", "z", "0")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("count 0 delivered %v, want an empty array", got)
		}
	})

	t.Run("withscores_pairs", func(t *testing.T) {
		got := run("ZRANDMEMBER", "z", "-4", "WITHSCORES")
		if got.Type != resp.TypeArray || len(got.Array) != 8 {
			t.Fatalf("WITHSCORES -4 delivered %d elements, want 8 (pairs)", len(got.Array))
		}
	})

	t.Run("missing_key_negative_null", func(t *testing.T) {
		got := run("ZRANDMEMBER", "nosuch", "-3")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("missing key with count replied %v, want an empty array (Redis count-form shape)", got)
		}
	})

	t.Run("missing_key_zero_empty", func(t *testing.T) {
		got := run("ZRANDMEMBER", "nosuch", "0")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("missing key count 0 replied %v, want an empty array", got)
		}
	})

	t.Run("no_count_single_member", func(t *testing.T) {
		got := run("ZRANDMEMBER", "z")
		if got.Type != resp.TypeBulkString || got.IsNull || len(got.Bulk) == 0 {
			t.Fatalf("no-count ZRANDMEMBER replied %v, want a bare member bulk (Redis shape)", got)
		}
	})

	t.Run("empty_set_negative", func(t *testing.T) {
		run("ZADD", "empty", "1", "x")
		run("ZREM", "empty", "x")
		got := run("ZRANDMEMBER", "empty", "-3")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("empty set with count replied %v, want an empty array (Redis count-form shape)", got)
		}
	})

	t.Run("minint64_count_out_of_range", func(t *testing.T) {
		// Redis accepts counts in [-LONG_MAX, LONG_MAX] only.
		got := run("ZRANDMEMBER", "z", "-9223372036854775808")
		if got.Type != resp.TypeError || got.Err != "ERR value is out of range, value must between -9223372036854775807 and 9223372036854775807" {
			t.Fatalf("MinInt64 count replied %v, want the out-of-range error", got)
		}
	})
}
