package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestXInfoNoGroupErrors(t *testing.T) {
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

	t.Run("groups_missing_key_is_nogroup_error", func(t *testing.T) {
		got := run("XINFO", "GROUPS", "nosuchkey")
		if got.Type != resp.TypeError {
			t.Fatalf("XINFO GROUPS on missing key replied %v, want a NOGROUP error", got)
		}
	})

	t.Run("consumers_missing_key_is_nogroup_error", func(t *testing.T) {
		got := run("XINFO", "CONSUMERS", "nosuchkey", "g")
		if got.Type != resp.TypeError {
			t.Fatalf("XINFO CONSUMERS on missing key replied %v, want a NOGROUP error", got)
		}
	})

	t.Run("consumers_missing_group_is_nogroup_error", func(t *testing.T) {
		got := run("XINFO", "CONSUMERS", "s", "nosuchgroup")
		if got.Type != resp.TypeError {
			t.Fatalf("XINFO CONSUMERS on missing group replied %v, want a NOGROUP error", got)
		}
	})

	t.Run("groups_existing_key_no_groups_empty_control", func(t *testing.T) {
		run("XADD", "lonely", "1-0", "f", "v")
		got := run("XINFO", "GROUPS", "lonely")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("XINFO GROUPS with no groups delivered %v, want an empty array", got)
		}
	})

	t.Run("consumers_existing_group_no_consumers_control", func(t *testing.T) {
		got := run("XINFO", "CONSUMERS", "s", "g")
		if got.Type != resp.TypeArray {
			t.Fatalf("XINFO CONSUMERS replied %v, want an array", got)
		}
	})

	t.Run("stream_missing_key_error_control", func(t *testing.T) {
		got := run("XINFO", "STREAM", "nosuchkey")
		if got.Type != resp.TypeError {
			t.Fatalf("XINFO STREAM on missing key replied %v, want an error", got)
		}
	})
}
