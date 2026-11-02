package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// The replies are Redis 8.2's.
func TestFTConfig(t *testing.T) {
	h := newCursorHarness(t)
	t.Cleanup(func() {
		h.do("FT.CONFIG", "SET", "MINPREFIX", "2")
		h.do("FT.CONFIG", "SET", "MAXSEARCHRESULTS", "1000000")
		h.do("FT.CONFIG", "SET", "DEFAULT_DIALECT", "1")
	})
	for _, c := range []struct {
		want string
		args []string
	}{
		{"*1\r\n*2\r\n+MINPREFIX\r\n$1\r\n2\r\n", []string{"GET", "minprefix"}},
		{"-Could not convert argument to expected type\r\n", []string{"SET", "MINPREFIX", "x"}},
		{"-Value is outside acceptable bounds\r\n", []string{"SET", "MINPREFIX", "0"}},
		{"+OK\r\n", []string{"SET", "MINPREFIX", "3"}},
		{"*1\r\n*2\r\n+MINPREFIX\r\n$1\r\n3\r\n", []string{"GET", "MINPREFIX"}},
		{"-Not modifiable at runtime\r\n", []string{"SET", "EXTLOAD", "x"}},
		{"-Invalid option\r\n", []string{"SET", "NOSUCH", "1"}},
		{"-Expected an argument, but none provided\r\n", []string{"SET", "TIMEOUT"}},
		{"+EXCESSARGS\r\n", []string{"SET", "TIMEOUT", "500", "2"}},
		{"+OK\r\n", []string{"SET", "MAXSEARCHRESULTS", "-1"}},
		{"*1\r\n*2\r\n+MAXSEARCHRESULTS\r\n$9\r\nunlimited\r\n", []string{"GET", "MAXSEARCHRESULTS"}},
		{"+OK\r\n", []string{"SET", "MAXSEARCHRESULTS", "1000000"}},
		{"-Default dialect version cannot be higher than 4\r\n", []string{"SET", "DEFAULT_DIALECT", "5"}},
		{"-Minimum stem length cannot be lower than 2\r\n", []string{"SET", "MINSTEMLEN", "1"}},
		{"*1\r\n*5\r\n+TIMEOUT\r\n+Description\r\n+Query (search) timeout\r\n+Value\r\n$3\r\n500\r\n", []string{"HELP", "TIMEOUT"}},
		{"*0\r\n", []string{"GET", "NOSUCH"}},
		{"+No such configuration action\r\n", []string{"FOO", "BAR"}},
		{"-ERR wrong number of arguments for 'FT.CONFIG' command\r\n", []string{"GET"}},
	} {
		h.expect(c.want, append([]string{"FT.CONFIG"}, c.args...)...)
	}
	h.expect("-LIMIT exceeds maximum of 1000000\r\n", "FT.SEARCH", "i", "*", "LIMIT", "0", "2000000")
	h.expect("-OFFSET exceeds maximum of 1000000\r\n", "FT.SEARCH", "i", "*", "LIMIT", "1000001", "1")

	// The default dialect decides how a query parses.
	h.expect("$66\r\nINTERSECT {\n  UNION {\n    w\n    +w(expanded)\n  }\n  PREFIX{hel*}\n}\n\r\n", "FT.EXPLAIN", "i", "w'hel*'")
	h.expect("+OK\r\n", "FT.CONFIG", "SET", "DEFAULT_DIALECT", "2")
	h.expect("$15\r\nWILDCARD{hel*}\n\r\n", "FT.EXPLAIN", "i", "w'hel*'")
}

func TestFTAdminVariants(t *testing.T) {
	h := newCursorHarness(t)
	for _, c := range []struct {
		want string
		args []string
	}{
		{"*5\r\n+UNION {\r\n+  hello\r\n+  +hello(expanded)\r\n+}\r\n+\r\n", []string{"FT.EXPLAINCLI", "i", "hello"}},
		{"-nosuch: no such index\r\n", []string{"FT.EXPLAINCLI", "nosuch", "hello"}},
		{"*2\r\n*6\r\n$1\r\nt\r\n$5\r\nhello\r\n$1\r\nn\r\n$1\r\n2\r\n$1\r\ng\r\n$1\r\nb\r\n$-1\r\n", []string{"FT.MGET", "i", "d:2", "d:9"}},
		{"-ERR wrong number of arguments for 'FT.MGET' command\r\n", []string{"FT.MGET", "i"}},
		{"+OK\r\n", []string{"FT._CREATEIFNX", "i", "ON", "HASH", "SCHEMA", "t", "TEXT"}},
		{"+OK\r\n", []string{"FT._CREATEIFNX", "j", "ON", "HASH", "PREFIX", "1", "e:", "SCHEMA", "t", "TEXT"}},
		{"+OK\r\n", []string{"FT._ALIASADDIFNX", "al", "i"}},
		{"+OK\r\n", []string{"FT._ALIASADDIFNX", "al", "i"}},
		{"-Alias already exists\r\n", []string{"FT._ALIASADDIFNX", "al", "j"}},
		{"+OK\r\n", []string{"FT._ALIASDELIFX", "al"}},
		{"+OK\r\n", []string{"FT._ALIASDELIFX", "al"}},
		{"+OK\r\n", []string{"FT._ALTERIFNX", "i", "SCHEMA", "ADD", "x", "TEXT"}},
		{"+OK\r\n", []string{"FT._ALTERIFNX", "i", "SCHEMA", "ADD", "x", "TEXT"}},
		{"+OK\r\n", []string{"FT._DROPINDEXIFX", "j"}},
		{"+OK\r\n", []string{"FT._DROPINDEXIFX", "j"}},
		{"+OK\r\n", []string{"FT.DROP", "i", "KEEPDOCS"}},
		{":1\r\n", []string{"EXISTS", "d:1"}},
		{"+OK\r\n", []string{"FT.CREATE", "i", "ON", "HASH", "PREFIX", "1", "d:", "SCHEMA", "t", "TEXT"}},
		{"+OK\r\n", []string{"FT.DROP", "i"}},
		{":0\r\n", []string{"EXISTS", "d:1"}},
		{"-nosuch: no such index\r\n", []string{"FT.DROP", "nosuch"}},
	} {
		h.expect(c.want, c.args...)
	}
}

// Module commands are described, and grouped in ACL categories, as Redis
// 8.2 does with its bundled modules.
func TestModuleCommandsFollowRedis8(t *testing.T) {
	r := newMemAcctRouter()
	RegisterServerCommands(r)
	RegisterSearchCommands(r)
	RegisterJSONCommands(r)
	RegisterStringCommands(r)
	s := store.NewStore()
	run := func(args ...string) string { return memAcctRun(r, s, nil, args...) }
	defer run("ACL", "DELUSER", "modtest")
	for _, c := range []struct {
		want string
		args []string
	}{
		{"*1\r\n*10\r\n$9\r\nFT.SEARCH\r\n:-1\r\n*2\r\n+readonly\r\n+module\r\n:0\r\n:0\r\n:0\r\n*2\r\n+@read\r\n+@search\r\n*0\r\n*0\r\n*0\r\n", []string{"COMMAND", "INFO", "ft.search"}},
		{"*2\r\n$9\r\nFT.SEARCH\r\n*4\r\n$5\r\ngroup\r\n$6\r\nmodule\r\n$6\r\nmodule\r\n$6\r\nsearch\r\n", []string{"COMMAND", "DOCS", "FT.SEARCH"}},
		{"*2\r\n$9\r\nFT.SEARCH\r\n$13\r\nFT.SPELLCHECK\r\n", []string{"COMMAND", "LIST", "FILTERBY", "PATTERN", "ft.s[ep]*"}},
		{"+OK\r\n", []string{"ACL", "SETUSER", "modtest", "on", "nopass", "+@search", "+@read", "~*"}},
		{"+OK\r\n", []string{"ACL", "DRYRUN", "modtest", "FT.SEARCH", "idx", "hello"}},
		{"+OK\r\n", []string{"ACL", "DRYRUN", "modtest", "JSON.GET", "doc"}},
		{"$56\r\nUser modtest has no permissions to run the 'set' command\r\n", []string{"ACL", "DRYRUN", "modtest", "SET", "k", "v"}},
	} {
		if got := run(c.args...); got != c.want {
			t.Errorf("%v = %q, want %q", c.args, got, c.want)
		}
	}
}
