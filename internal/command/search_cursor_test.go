package command

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/search"
	"github.com/cachestorm/cachestorm/internal/store"
)

type cursorHarness struct {
	t      *testing.T
	router *Router
	st     *store.Store
	resp3  bool
	cid    string
}

var cursorIDReply = regexp.MustCompile(`:(\d+)\r\n$`)

// do runs a command; "$C" stands for the last cursor id a reply gave, and
// a reply's own nonzero cursor id reads as <cid>.
func (h *cursorHarness) do(args ...string) string {
	h.t.Helper()
	for i, a := range args {
		if a == "$C" {
			args[i] = h.cid
		}
	}
	var out bytes.Buffer
	w := resp.NewWriter(&out)
	w.SetRESP3(h.resp3)
	ctx := NewContext(strings.ToUpper(args[0]), bytesArgs(args[1:]...), h.st, w)
	if err := h.router.Execute(ctx); err != nil {
		h.t.Fatalf("%v: %v", args, err)
	}
	ctx.Writer.Flush()
	got := out.String()
	upper := strings.ToUpper(strings.Join(args, " "))
	if m := cursorIDReply.FindStringSubmatch(got); m != nil && !strings.HasPrefix(got, "-") &&
		(strings.Contains(upper, "WITHCURSOR") || strings.HasPrefix(upper, "FT.CURSOR READ")) && m[1] != "0" {
		h.cid = m[1]
		got = got[:len(got)-len(m[0])] + ":<cid>\r\n"
	}
	return got
}

func (h *cursorHarness) expect(want string, args ...string) {
	h.t.Helper()
	if got := h.do(args...); got != want {
		h.t.Errorf("%v:\n got %q\nwant %q", args, got, want)
	}
}

func newCursorHarness(t *testing.T) *cursorHarness {
	router := NewRouter()
	RegisterSearchCommands(router)
	RegisterHashCommands(router)
	RegisterKeyCommands(router)
	RegisterStringCommands(router)
	search.GetIndexManager().Reset()
	t.Cleanup(search.GetIndexManager().Reset)
	h := &cursorHarness{t: t, router: router, st: store.NewStore()}
	h.expect("+OK\r\n", "FT.CREATE", "i", "ON", "HASH", "PREFIX", "1", "d:", "SCHEMA", "t", "TEXT", "n", "NUMERIC", "SORTABLE", "g", "TAG")
	for _, d := range [][]string{{"1", "hello world", "a"}, {"2", "hello", "b"}, {"3", "world", "a"}, {"4", "hello there", "c"}, {"5", "foo", "b"}} {
		h.do("HSET", "d:"+d[0], "t", d[1], "n", d[0], "g", d[2])
	}
	return h
}

func nRow(v string) string { return "*2\r\n$1\r\nn\r\n$1\r\n" + v + "\r\n" }

// The replies are Redis 8.2's.
func TestFTCursorReadsChunks(t *testing.T) {
	h := newCursorHarness(t)
	h.expect("*2\r\n*3\r\n:1\r\n"+nRow("1")+nRow("2")+":<cid>\r\n", "FT.AGGREGATE", "i", "*", "LOAD", "1", "@n", "WITHCURSOR", "COUNT", "2")
	h.expect("*2\r\n*3\r\n:1\r\n"+nRow("3")+nRow("4")+":<cid>\r\n", "FT.CURSOR", "READ", "i", "$C")
	h.expect("*2\r\n*2\r\n:1\r\n"+nRow("5")+":0\r\n", "FT.CURSOR", "READ", "i", "$C")
	h.expect("-Cursor not found, id: "+h.cid+"\r\n", "FT.CURSOR", "READ", "i", "$C")

	// COUNT on a read becomes the cursor's chunk size.
	h.do("FT.AGGREGATE", "i", "*", "LOAD", "1", "@n", "WITHCURSOR", "COUNT", "2")
	h.expect("*2\r\n*2\r\n:1\r\n"+nRow("3")+":<cid>\r\n", "FT.CURSOR", "READ", "i", "$C", "COUNT", "1")
	h.expect("*2\r\n*2\r\n:1\r\n"+nRow("4")+":<cid>\r\n", "FT.CURSOR", "READ", "i", "$C")
	h.expect("+OK\r\n", "FT.CURSOR", "DEL", "i", "$C")
	h.expect("-Cursor does not exist\r\n", "FT.CURSOR", "DEL", "i", "$C")

	// A sorted or grouped reply sends its total once.
	h.expect("*2\r\n*3\r\n:3\r\n"+nRow("4")+nRow("2")+":<cid>\r\n", "FT.AGGREGATE", "i", "hello", "LOAD", "1", "@n", "SORTBY", "2", "@n", "DESC", "WITHCURSOR", "COUNT", "2")
	h.expect("*2\r\n*2\r\n:0\r\n"+nRow("1")+":0\r\n", "FT.CURSOR", "READ", "i", "$C")

	// A LIMIT that ends with a full chunk leaves an empty one.
	h.expect("*2\r\n*3\r\n:1\r\n"+nRow("1")+nRow("2")+":<cid>\r\n", "FT.AGGREGATE", "i", "*", "LOAD", "1", "@n", "LIMIT", "0", "2", "WITHCURSOR", "COUNT", "2")
	h.expect("*2\r\n*1\r\n:0\r\n:0\r\n", "FT.CURSOR", "READ", "i", "$C")
	h.expect("*2\r\n*1\r\n:5\r\n:0\r\n", "FT.AGGREGATE", "i", "*", "LOAD", "1", "@n", "LIMIT", "9", "2", "WITHCURSOR", "COUNT", "2")
	h.expect("*2\r\n*1\r\n:0\r\n:0\r\n", "FT.AGGREGATE", "i", "nomatch", "WITHCURSOR")

	// Documents written or deleted between reads are skipped.
	h.do("FT.AGGREGATE", "i", "*", "LOAD", "1", "@n", "WITHCURSOR", "COUNT", "2")
	h.do("HSET", "d:3", "n", "33")
	h.do("DEL", "d:4")
	h.expect("*2\r\n*2\r\n:1\r\n"+nRow("5")+":0\r\n", "FT.CURSOR", "READ", "i", "$C")

	h.resp3 = true
	h.expect("*2\r\n%5\r\n+attributes\r\n*0\r\n+format\r\n+STRING\r\n+results\r\n*0\r\n+total_results\r\n:0\r\n+warning\r\n*0\r\n:0\r\n",
		"FT.AGGREGATE", "i", "nomatch", "WITHCURSOR")
}

func TestFTCursorErrors(t *testing.T) {
	h := newCursorHarness(t)
	for _, c := range []struct {
		want string
		args []string
	}{
		{"-Bad arguments for COUNT: Value is outside acceptable bounds\r\n", []string{"FT.AGGREGATE", "i", "*", "WITHCURSOR", "COUNT", "0"}},
		{"-Bad arguments for COUNT: Could not convert argument to expected type\r\n", []string{"FT.AGGREGATE", "i", "*", "WITHCURSOR", "COUNT", "x"}},
		{"-Bad arguments for MAXIDLE: Value is outside acceptable bounds\r\n", []string{"FT.AGGREGATE", "i", "*", "WITHCURSOR", "MAXIDLE", "0"}},
		{"-Bad arguments for COUNT: Expected an argument, but none provided\r\n", []string{"FT.AGGREGATE", "i", "*", "WITHCURSOR", "COUNT"}},
		{"-FT.AGGREGATE + WITHCOUNT is not available when `ENABLE_UNSTABLE_FEATURES` is off\r\n", []string{"FT.AGGREGATE", "i", "*", "WITHCOUNT"}},
		{"-ERR wrong number of arguments for 'FT.CURSOR' command\r\n", []string{"FT.CURSOR", "GC", "i"}},
		{"-nosuch: no such index\r\n", []string{"FT.CURSOR", "READ", "nosuch", "abc"}},
		{"-Bad cursor ID\r\n", []string{"FT.CURSOR", "READ", "i", "abc"}},
		{"-Cursor not found, id: 12345\r\n", []string{"FT.CURSOR", "READ", "i", "12345"}},
		{"-Cursor not found, id: 1\r\n", []string{"FT.CURSOR", "READ", "i", "4294967297"}},
		{"-Unknown argument `FOO`\r\n", []string{"FT.CURSOR", "READ", "i", "1", "FOO", "1"}},
		{"-Bad value for COUNT: `x`\r\n", []string{"FT.CURSOR", "READ", "i", "1", "COUNT", "x"}},
		{"-Unknown subcommand\r\n", []string{"FT.CURSOR", "FOO", "i", "1"}},
	} {
		h.expect(c.want, c.args...)
	}

	h.do("FT.AGGREGATE", "i", "*", "WITHCURSOR", "COUNT", "1")
	h.expect("-cursor request is not profile, id: "+h.cid+"\r\n", "FT.CURSOR", "PROFILE", "i", "$C")
	h.expect(":-1\r\n", "FT.CURSOR", "GC", "i", "0")
	h.expect("+OK\r\n", "FT.DROPINDEX", "i")
	h.expect("+OK\r\n", "FT.CREATE", "i", "ON", "HASH", "PREFIX", "1", "d:", "SCHEMA", "t", "TEXT")
	h.expect("-The index was dropped while the cursor was idle\r\n", "FT.CURSOR", "READ", "i", "$C")
	h.expect("-Cursor not found, id: "+h.cid+"\r\n", "FT.CURSOR", "READ", "i", "$C")

	for range search.IndexCursorLimit {
		h.do("FT.AGGREGATE", "i", "*", "WITHCURSOR", "COUNT", "1")
	}
	h.expect("-INDEX_CURSOR_LIMIT of 128 has been reached for an index\r\n", "FT.AGGREGATE", "i", "*", "WITHCURSOR", "COUNT", "1")
	h.expect("+OK\r\n", "FT.CURSOR", "DEL", "i", "$C")
	h.do("FT.AGGREGATE", "i", "*", "WITHCURSOR", "COUNT", "1")
	h.expect("+OK\r\n", "FT.CURSOR", "DEL", "i", "$C")
}

func TestFTSearchCursor(t *testing.T) {
	h := newCursorHarness(t)
	doc := func(k, t, n, g string) string {
		return "$3\r\n" + k + "\r\n*6\r\n$1\r\nt\r\n$" + strconv.Itoa(len(t)) + "\r\n" + t + "\r\n$1\r\nn\r\n$1\r\n" + n + "\r\n$1\r\ng\r\n$1\r\n" + g + "\r\n"
	}
	h.expect("*2\r\n*3\r\n:5\r\n"+doc("d:2", "hello", "2", "b")+":<cid>\r\n", "FT.SEARCH", "i", "*", "WITHCURSOR", "COUNT", "1")
	h.expect("*2\r\n*5\r\n:0\r\n"+doc("d:3", "world", "3", "a")+doc("d:4", "hello there", "4", "c")+":<cid>\r\n", "FT.CURSOR", "READ", "i", "$C", "COUNT", "2")
	h.expect("*2\r\n*1\r\n:5\r\n:0\r\n", "FT.SEARCH", "i", "*", "WITHCURSOR", "LIMIT", "0", "0")
	h.expect("*2\r\n*4\r\n:5\r\n$3\r\nd:2\r\n$3\r\nd:3\r\n$3\r\nd:4\r\n:<cid>\r\n", "FT.SEARCH", "i", "*", "NOCONTENT", "LIMIT", "0", "3", "WITHCURSOR", "COUNT", "3")
	h.expect("*2\r\n*1\r\n:0\r\n:0\r\n", "FT.CURSOR", "READ", "i", "$C")
}

// Steps read only what earlier steps loaded or made, SORTABLE fields, or
// (SORTBY, GROUPBY and reducers) fields of the schema; replies are Redis
// 8.2's.
func TestFTAggregateResolvesProperties(t *testing.T) {
	h := newCursorHarness(t)
	for _, c := range []struct {
		want string
		args []string
	}{
		{"-Property `zz` not loaded nor in pipeline\r\n", []string{"APPLY", "exists(@zz)", "AS", "x"}},
		{"-Property `t` not loaded nor in pipeline\r\n", []string{"APPLY", "@t", "AS", "x"}},
		{"-Property `t` not loaded nor in pipeline\r\n", []string{"LOAD", "3", "@t", "AS", "tt", "APPLY", "@t", "AS", "x"}},
		{"-Property `n` not loaded nor in pipeline\r\n", []string{"GROUPBY", "1", "@g", "APPLY", "@n", "AS", "x"}},
		{"-No such property `zz`\r\n", []string{"GROUPBY", "1", "@zz"}},
		{"-Property `g` specified more than once\r\n", []string{"GROUPBY", "2", "@g", "@g"}},
		{"-Property is not present in document or pipeline: `zz`\r\n", []string{"GROUPBY", "1", "@g", "REDUCE", "SUM", "1", "@zz", "AS", "s"}},
		{"-Property is not present in document or pipeline: `zz`\r\n", []string{"GROUPBY", "1", "@g", "REDUCE", "FIRST_VALUE", "3", "@n", "BY", "@zz"}},
		{"-Property `n` not loaded nor in schema\r\n", []string{"GROUPBY", "1", "@g", "SORTBY", "1", "@n"}},
		{"-Property `zz` not loaded nor in schema\r\n", []string{"SORTBY", "1", "@zz"}},
		{"-Property `__key` not loaded nor in pipeline\r\n", []string{"APPLY", "@__key", "AS", "x"}},
		{"-Property `__score` not loaded nor in pipeline\r\n", []string{"APPLY", "@__score", "AS", "x"}},
		{"*2\r\n:1\r\n*4\r\n$1\r\nn\r\n$1\r\n1\r\n$1\r\nx\r\n$1\r\n1\r\n", []string{"FILTER", "@n==1", "APPLY", "@n", "AS", "x"}},
		{"*2\r\n:1\r\n*4\r\n$2\r\ntt\r\n$11\r\nhello world\r\n$1\r\nx\r\n$1\r\n1\r\n", []string{"LOAD", "3", "@t", "AS", "tt", "APPLY", "1", "AS", "x", "LIMIT", "0", "1"}},
		{"*2\r\n:1\r\n*4\r\n$5\r\n__key\r\n$3\r\nd:1\r\n$1\r\nx\r\n$3\r\nd:1\r\n", []string{"LOAD", "1", "@__key", "APPLY", "@__key", "AS", "x", "LIMIT", "0", "1"}},
		{"*2\r\n:5\r\n*2\r\n$1\r\nt\r\n$3\r\nfoo\r\n", []string{"SORTBY", "1", "@t", "LIMIT", "0", "1"}},
	} {
		h.expect(c.want, append([]string{"FT.AGGREGATE", "i", "*"}, c.args...)...)
	}
}
