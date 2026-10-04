package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestXAckNoGroupReturnsZero(t *testing.T) {
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

	t.Run("missing_group_on_existing_stream_returns_zero", func(t *testing.T) {
		got := run("XACK", "s", "nosuchgroup", "1-0")
		if got.Type != resp.TypeInteger || got.Int != 0 {
			t.Fatalf("XACK on missing group replied %v, want integer 0", got)
		}
	})

	t.Run("missing_stream_returns_zero", func(t *testing.T) {
		got := run("XACK", "nosuch", "g", "1-0")
		if got.Type != resp.TypeInteger || got.Int != 0 {
			t.Fatalf("XACK on missing stream replied %v, want integer 0", got)
		}
	})

	t.Run("valid_group_without_pending_returns_zero", func(t *testing.T) {
		run("XGROUP", "CREATE", "s", "g", "0")
		got := run("XACK", "s", "g", "1-0")
		if got.Type != resp.TypeInteger || got.Int != 0 {
			t.Fatalf("XACK with nothing pending replied %v, want integer 0", got)
		}
	})

	t.Run("real_ack_returns_one", func(t *testing.T) {
		run("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", ">")
		got := run("XACK", "s", "g", "1-0")
		if got.Type != resp.TypeInteger || got.Int != 1 {
			t.Fatalf("real XACK replied %v, want integer 1", got)
		}
	})

	t.Run("wrong_arity_still_error", func(t *testing.T) {
		got := run("XACK", "s", "g")
		if got.Type != resp.TypeError {
			t.Fatalf("XACK with 2 args replied %v, want an error", got)
		}
	})
}
