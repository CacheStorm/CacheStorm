package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func execDispatchRouter() *Router {
	r := NewRouter()
	RegisterTransactionCommands(r)
	RegisterStringCommands(r)
	RegisterHashCommands(r)
	RegisterListCommands(r)
	RegisterSortedSetCommands(r)
	return r
}

func execDispatchRunner(r *Router, s *store.Store) func(name string, args ...string) string {
	tx := NewTransaction()
	return func(name string, args ...string) string {
		var out bytes.Buffer
		argv := make([][]byte, len(args))
		for i, a := range args {
			argv[i] = []byte(a)
		}
		ctx := NewContext(name, argv, s, resp.NewWriter(&out))
		ctx.Transaction = tx
		if err := r.Execute(ctx); err != nil {
			ctx.Writer.WriteError(err.Error())
		}
		return out.String()
	}
}

func TestExecRunsRegisteredHandlers(t *testing.T) {
	run := execDispatchRunner(execDispatchRouter(), store.NewStore())

	run("HSET", "h", "field", "value")
	run("RPUSH", "l", "a", "b")
	run("MULTI")
	run("HGETALL", "h")
	run("LRANGE", "l", "0", "-1")
	run("ZADD", "z", "1", "m")
	got := run("EXEC")
	want := "*3\r\n*2\r\n$5\r\nfield\r\n$5\r\nvalue\r\n*2\r\n$1\r\na\r\n$1\r\nb\r\n:1\r\n"
	if got != want {
		t.Fatalf("EXEC reply %q, want %q", got, want)
	}
}

func TestExecAbortsAfterUnknownCommand(t *testing.T) {
	s := store.NewStore()
	run := execDispatchRunner(execDispatchRouter(), s)

	run("MULTI")
	run("SET", "k", "v")
	if got := run("NOSUCHCOMMAND"); got[0] != '-' {
		t.Fatalf("unknown command inside MULTI replied %q, want an error", got)
	}
	if got := run("EXEC"); got != "-"+ErrExecAbort.Error()+"\r\n" {
		t.Fatalf("EXEC reply %q, want EXECABORT", got)
	}
	if s.Exists("k") {
		t.Fatal("an aborted transaction applied its commands")
	}
	run("MULTI")
	run("SET", "k", "v")
	if got := run("EXEC"); got != "*1\r\n+OK\r\n" {
		t.Fatalf("the next transaction is still aborted: %q", got)
	}
}

func TestExecDoesNotBlock(t *testing.T) {
	run := execDispatchRunner(execDispatchRouter(), store.NewStore())

	run("MULTI")
	run("BLPOP", "empty", "5")
	run("BZPOPMIN", "emptyz", "5")
	done := make(chan string, 1)
	go func() { done <- run("EXEC") }()
	select {
	case got := <-done:
		if got != "*2\r\n*-1\r\n*-1\r\n" && got != "*2\r\n_\r\n_\r\n" && got != "*2\r\n$-1\r\n$-1\r\n" {
			t.Fatalf("EXEC reply %q, want two null replies", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a blocking command inside EXEC blocked")
	}
}

func TestExecReportsHandlerErrorsInPlace(t *testing.T) {
	run := execDispatchRunner(execDispatchRouter(), store.NewStore())

	run("SET", "s", "notanumber")
	run("MULTI")
	run("INCR", "s")
	run("SET", "t", "1")
	got := run("EXEC")
	if len(got) < 4 || got[:4] != "*2\r\n" || !bytes.Contains([]byte(got), []byte("+OK\r\n")) || !bytes.Contains([]byte(got), []byte("-ERR")) {
		t.Fatalf("EXEC reply %q, want an error for INCR and OK for SET", got)
	}
}
