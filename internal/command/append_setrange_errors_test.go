package command_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAppendSetRangePropagateErrors(t *testing.T) {
	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterStringCommands(router)
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
	bigKey := strings.Repeat("k", 65537)

	t.Run("append_oversized_key_replies_error", func(t *testing.T) {
		got := run("APPEND", bigKey, "v")
		if got.Type != resp.TypeError {
			t.Fatalf("APPEND on oversized key replied %v, want an error (Store.Set must not fail silently)", got)
		}
	})

	t.Run("setrange_oversized_key_replies_error", func(t *testing.T) {
		got := run("SETRANGE", bigKey, "0", "hello")
		if got.Type != resp.TypeError {
			t.Fatalf("SETRANGE on oversized key replied %v, want an error", got)
		}
	})

	t.Run("append_normal_control", func(t *testing.T) {
		got := run("APPEND", "ok", "hello")
		if got.Type != resp.TypeInteger || got.Int != 5 {
			t.Fatalf("normal APPEND replied %v, want integer 5", got)
		}
	})

	t.Run("setrange_normal_control", func(t *testing.T) {
		got := run("SETRANGE", "ok2", "0", "hello")
		if got.Type != resp.TypeInteger || got.Int != 5 {
			t.Fatalf("normal SETRANGE replied %v, want integer 5", got)
		}
	})
}
