package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func helloRun(r *Router, ctx *Context, args ...string) string {
	var out bytes.Buffer
	argv := make([][]byte, len(args)-1)
	for i, a := range args[1:] {
		argv[i] = []byte(a)
	}
	ctx.Command, ctx.Args, ctx.Writer = args[0], argv, resp.NewWriter(&out)
	if err := r.Execute(ctx); err != nil {
		ctx.Writer.WriteError(err.Error())
	}
	return out.String()
}

func TestHELLO(t *testing.T) {
	r := NewRouter()
	RegisterServerCommands(r)
	s := store.NewStore()

	ctx := NewContext("", nil, s, nil)
	if got := helloRun(r, ctx, "HELLO", "3"); !strings.HasPrefix(got, "-NOPROTO") {
		t.Fatalf("HELLO 3: %q", got)
	}
	got := helloRun(r, ctx, "HELLO", "2", "SETNAME", "worker-1")
	for _, want := range []string{"$6\r\nserver\r\n", "$5\r\nproto\r\n:2\r\n", "$4\r\nmode\r\n$10\r\nstandalone\r\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("HELLO 2 reply %q lacks %q", got, want)
		}
	}
	if ctx.ClientName != "worker-1" {
		t.Fatalf("SETNAME not applied: %q", ctx.ClientName)
	}
	if got := helloRun(r, ctx, "HELLO", "2", "SETNAME", "has space"); !strings.HasPrefix(got, "-ERR Client names") {
		t.Fatalf("bad name: %q", got)
	}

	r.SetRequirePass("pw")
	defer r.SetRequirePass("")
	locked := NewContext("", nil, s, nil)
	if got := helloRun(r, locked, "HELLO", "2"); !strings.HasPrefix(got, "-NOAUTH") {
		t.Fatalf("HELLO before auth: %q", got)
	}
	if got := helloRun(r, locked, "HELLO", "2", "AUTH", "default", "wrong"); !strings.HasPrefix(got, "-WRONGPASS") {
		t.Fatalf("wrong password: %q", got)
	}
	if got := helloRun(r, locked, "HELLO", "2", "AUTH", "default", "pw"); !strings.HasPrefix(got, "*14\r\n") || !locked.IsAuthenticated() {
		t.Fatalf("HELLO AUTH: %q authenticated=%v", got, locked.IsAuthenticated())
	}
}

func TestPINGArityAndClientNames(t *testing.T) {
	r := NewRouter()
	RegisterServerCommands(r)
	RegisterClientCommands(r)
	ctx := NewContext("", nil, store.NewStore(), nil)
	if got := helloRun(r, ctx, "PING", "a", "b"); !strings.HasPrefix(got, "-ERR wrong number of arguments for 'ping'") {
		t.Fatalf("PING a b: %q", got)
	}
	if got := helloRun(r, ctx, "CLIENT", "SETNAME", "a b"); !strings.HasPrefix(got, "-ERR Client names") {
		t.Fatalf("CLIENT SETNAME with a space: %q", got)
	}
	if got := helloRun(r, ctx, "CLIENT", "SETNAME", ""); got != "+OK\r\n" {
		t.Fatalf("clearing the name: %q", got)
	}
}
