package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestXGroupDelConsumerPurgesPEL(t *testing.T) {
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
	run("XADD", "s", "2-0", "f", "v")
	run("XGROUP", "CREATE", "s", "g", "0")
	run("XREADGROUP", "GROUP", "g", "c1", "COUNT", "10", "STREAMS", "s", ">")

	t.Run("delconsumer_returns_pending_count_control", func(t *testing.T) {
		got := run("XGROUP", "DELCONSUMER", "s", "g", "c1")
		if got.Type != resp.TypeInteger || got.Int != 2 {
			t.Fatalf("DELCONSUMER replied %v, want integer 2", got)
		}
	})

	t.Run("pel_purged_after_delete", func(t *testing.T) {
		got := run("XPENDING", "s", "g", "-", "+", "10")
		if got.Type != resp.TypeArray {
			t.Fatalf("XPENDING after DELCONSUMER replied %v, want an array", got)
		}
		if len(got.Array) != 0 {
			t.Fatalf("XPENDING after DELCONSUMER shows %d ghost entries, want 0 (purged from PEL)", len(got.Array))
		}
	})

	t.Run("autoclaim_finds_nothing_after_delete", func(t *testing.T) {
		got := run("XAUTOCLAIM", "s", "g", "c2", "0", "10")
		if got.Type != resp.TypeArray || len(got.Array) < 2 {
			t.Fatalf("XAUTOCLAIM replied %v, want the [cursor, claimed, deleted] shape", got)
		}
		if claimed := got.Array[1]; claimed.Type != resp.TypeArray || len(claimed.Array) != 0 {
			t.Fatalf("XAUTOCLAIM claimed %v after DELCONSUMER, want nothing to claim", claimed)
		}
	})

	t.Run("delconsumer_missing_consumer_zero_control", func(t *testing.T) {
		got := run("XGROUP", "DELCONSUMER", "s", "g", "ghost")
		if got.Type != resp.TypeInteger || got.Int != 0 {
			t.Fatalf("DELCONSUMER of missing consumer replied %v, want integer 0", got)
		}
	})

	t.Run("new_consumer_sees_fresh_deliveries_control", func(t *testing.T) {
		run("XADD", "s", "3-0", "f", "v")
		got := run("XREADGROUP", "GROUP", "g", "c2", "STREAMS", "s", ">")
		if got.Type != resp.TypeArray {
			t.Fatalf("XREADGROUP for new consumer replied %v, want delivery", got)
		}
		if got.Array[0].Type != resp.TypeArray || len(got.Array[0].Array) < 2 {
			t.Fatalf("XREADGROUP reply shape unexpected: %v", got)
		}
	})
}
