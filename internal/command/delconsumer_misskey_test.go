package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestXGroupDelConsumerMissingKey(t *testing.T) {
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
	run("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", ">")

	t.Run("missing_stream_is_nogroup_error", func(t *testing.T) {
		got := run("XGROUP", "DELCONSUMER", "nosuchkey", "g", "c1")
		if got.Type != resp.TypeError {
			t.Fatalf("DELCONSUMER on missing key replied %v, want an error (NOGROUP family)", got)
		}
	})

	t.Run("setid_missing_stream_is_nogroup_error", func(t *testing.T) {
		got := run("XGROUP", "SETID", "nosuchkey", "g", "1-0")
		if got.Type != resp.TypeError {
			t.Fatalf("SETID on missing key replied %v, want an error (NOGROUP family)", got)
		}
	})

	t.Run("existing_consumer_returns_pending_control", func(t *testing.T) {
		got := run("XGROUP", "DELCONSUMER", "s", "g", "c1")
		if got.Type != resp.TypeInteger || got.Int != 1 {
			t.Fatalf("DELCONSUMER replied %v, want integer 1 (one pending)", got)
		}
	})

	t.Run("missing_group_is_nogroup_control", func(t *testing.T) {
		got := run("XGROUP", "DELCONSUMER", "s", "nosuchgroup", "c1")
		if got.Type != resp.TypeError {
			t.Fatalf("missing group replied %v, want an error", got)
		}
	})

	t.Run("createconsumer_missing_key_sibling_consistency", func(t *testing.T) {
		got := run("XGROUP", "CREATECONSUMER", "nosuchkey", "g", "c2")
		if got.Type != resp.TypeError {
			t.Fatalf("CREATECONSUMER on missing key replied %v, want an error (sibling)", got)
		}
	})

	t.Run("missing_consumer_zero_control", func(t *testing.T) {
		got := run("XGROUP", "DELCONSUMER", "s", "g", "ghost")
		if got.Type != resp.TypeInteger || got.Int != 0 {
			t.Fatalf("missing consumer replied %v, want integer 0", got)
		}
	})
}
