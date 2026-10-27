package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func introspectionRun(t *testing.T, resp3 bool, args ...string) string {
	t.Helper()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterClientCommands(router)
	RegisterConfigCommands(router)
	RegisterListCommands(router)
	RegisterStringCommands(router)
	RegisterHashCommands(router)
	router.Register(&CommandDef{Name: "CS.EXT", Handler: cmdPING, Arity: -2, Flags: []string{"write", "fast"}})
	parsed := make([][]byte, len(args)-1)
	for i, a := range args[1:] {
		parsed[i] = []byte(a)
	}
	ctx := newTestContext(strings.ToUpper(args[0]), parsed, store.NewStore())
	buf := &bytes.Buffer{}
	ctx.Writer = resp.NewWriter(buf)
	ctx.Writer.SetRESP3(resp3)
	_ = router.Execute(ctx)
	return buf.String()
}

func TestCommandIntrospectionReplies(t *testing.T) {
	for _, tc := range []struct {
		resp3 bool
		args  []string
		want  string
	}{
		{false, []string{"COMMAND", "GETKEYSANDFLAGS", "LMOVE", "a", "b", "LEFT", "RIGHT"},
			"*2\r\n*2\r\n$1\r\na\r\n*3\r\n+RW\r\n+access\r\n+delete\r\n*2\r\n$1\r\nb\r\n*2\r\n+RW\r\n+insert\r\n"},
		{true, []string{"COMMAND", "GETKEYSANDFLAGS", "LMOVE", "a", "b", "LEFT", "RIGHT"},
			"*2\r\n*2\r\n$1\r\na\r\n~3\r\n+RW\r\n+access\r\n+delete\r\n*2\r\n$1\r\nb\r\n~2\r\n+RW\r\n+insert\r\n"},
		{false, []string{"COMMAND", "GETKEYS", "LMPOP", "2", "x", "y", "LEFT"}, "*2\r\n$1\r\nx\r\n$1\r\ny\r\n"},
		{false, []string{"COMMAND", "GETKEYS", "PING"}, "-ERR The command has no key arguments\r\n"},
		{false, []string{"COMMAND", "GETKEYS", "NOSUCH"}, "-ERR Invalid command specified\r\n"},
		{false, []string{"COMMAND", "GETKEYS", "GET"}, "-ERR Invalid number of arguments specified for command\r\n"},
		{false, []string{"COMMAND", "GETKEYSANDFLAGS", "SET", "k", "v", "GET"}, "*1\r\n*2\r\n$1\r\nk\r\n*3\r\n+RW\r\n+access\r\n+update\r\n"},
		{false, []string{"COMMAND", "GETKEYS", "CS.EXT", "k"}, "*1\r\n$1\r\nk\r\n"},
		{false, []string{"COMMAND", "INFO", "client|kill", "nosuch"},
			"*2\r\n*10\r\n$11\r\nclient|kill\r\n:-3\r\n*4\r\n+admin\r\n+noscript\r\n+loading\r\n+stale\r\n:0\r\n:0\r\n:0\r\n*4\r\n+@admin\r\n+@slow\r\n+@dangerous\r\n+@connection\r\n*0\r\n*0\r\n*0\r\n$-1\r\n"},
		{false, []string{"COMMAND", "INFO", "cs.ext"},
			"*1\r\n*10\r\n$6\r\ncs.ext\r\n:-2\r\n*2\r\n+write\r\n+fast\r\n:1\r\n:1\r\n:1\r\n*2\r\n+@write\r\n+@fast\r\n*0\r\n*0\r\n*0\r\n"},
		{true, []string{"COMMAND", "DOCS", "cs.ext", "nosuch"},
			"%1\r\n$6\r\ncs.ext\r\n%4\r\n$7\r\nsummary\r\n$29\r\nCacheStorm extension command.\r\n$5\r\nsince\r\n$5\r\n0.1.0\r\n$5\r\ngroup\r\n$6\r\nmodule\r\n$6\r\nmodule\r\n$10\r\ncachestorm\r\n"},
		{false, []string{"COMMAND", "BOGUS"}, "-ERR unknown subcommand 'BOGUS'. Try COMMAND HELP.\r\n"},
		{false, []string{"CLIENT", "NO-EVICT", "on"}, "+OK\r\n"},
		{false, []string{"CLIENT", "NO-EVICT", "maybe"}, "-ERR syntax error\r\n"},
		{false, []string{"CONFIG", "SET", "nosuchparam", "1"}, "-ERR Unknown option or number of arguments for CONFIG SET - 'nosuchparam'\r\n"},
		{false, []string{"CONFIG", "SET", "maxmemory-samples", "x"},
			"-ERR CONFIG SET failed (possibly related to argument 'maxmemory-samples') - argument couldn't be parsed into an integer\r\n"},
	} {
		if got := introspectionRun(t, tc.resp3, tc.args...); got != tc.want {
			t.Errorf("%v (resp3=%v):\n got %q\nwant %q", tc.args, tc.resp3, got, tc.want)
		}
	}
	if got := introspectionRun(t, false, "COMMAND", "DOCS", "get"); !strings.HasPrefix(got, "*2\r\n$3\r\nget\r\n*10\r\n$7\r\nsummary\r\n$34\r\nReturns the string value of a key.") {
		t.Errorf("COMMAND DOCS get: %q", got)
	}
}

func TestCommandMetaCoversRedisTable(t *testing.T) {
	for name := range redisCommandSpecs {
		m := redisCommandMeta(name)
		if m == nil || m.docs == nil {
			t.Fatalf("%s has no generated COMMAND INFO/DOCS entry", name)
		}
		if m.arity != redisCommandSpecs[name].Arity {
			t.Fatalf("%s arity %d, spec %d", name, m.arity, redisCommandSpecs[name].Arity)
		}
	}
	for sub := range redisSubcommandCategories {
		if redisCommandMeta(sub) == nil {
			t.Fatalf("subcommand %s missing", sub)
		}
	}
}
