package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestJSONRepliesMatchRedisJSONOnRESP2AndRESP3(t *testing.T) {
	router := NewRouter()
	RegisterJSONCommands(router)
	st := store.NewStore()
	run := func(resp3 bool, args ...string) string {
		var out bytes.Buffer
		w := resp.NewWriter(&out)
		w.SetRESP3(resp3)
		ctx := NewContext(strings.ToUpper(args[0]), bytesArgs(args[1:]...), st, w)
		if err := router.Execute(ctx); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		ctx.Writer.Flush()
		return out.String()
	}
	run(false, "JSON.SET", "j", "$", `{"a":1,"b":"s","c":{"a":0.1},"u":12345678901234567890,"f":1e300}`)
	cases := []struct {
		resp3 bool
		args  []string
		want  string
	}{
		{true, []string{"JSON.TYPE", "nokey"}, "*1\r\n_\r\n"},
		{true, []string{"JSON.TYPE", "nokey", "$"}, "*1\r\n_\r\n"},
		{true, []string{"JSON.TYPE", "j", "nope"}, "*1\r\n_\r\n"},
		{true, []string{"JSON.TYPE", "j", "$.nope"}, "*1\r\n*0\r\n"},
		{true, []string{"JSON.TYPE", "j", "$..a"}, "*1\r\n*2\r\n$7\r\ninteger\r\n$6\r\nnumber\r\n"},
		{true, []string{"JSON.TYPE", "j", ".c"}, "*1\r\n$6\r\nobject\r\n"},
		{false, []string{"JSON.TYPE", "nokey"}, "$-1\r\n"},
		{false, []string{"JSON.TYPE", "j", "$..a"}, "*2\r\n$7\r\ninteger\r\n$6\r\nnumber\r\n"},
		{true, []string{"JSON.NUMINCRBY", "j", "$..a", "1"}, "*2\r\n:2\r\n,1.1\r\n"},
		{true, []string{"JSON.NUMINCRBY", "j", "$.*", "1"}, "*5\r\n:3\r\n_\r\n_\r\n,12345678901234567000\r\n,1e+300\r\n"},
		{true, []string{"JSON.NUMINCRBY", "j", "b", "1"}, "*1\r\n_\r\n"},
		{true, []string{"JSON.NUMINCRBY", "j", "$.nope", "1"}, "*0\r\n"},
		{true, []string{"JSON.NUMINCRBY", "j", "nope", "1"}, "*0\r\n"},
		{true, []string{"JSON.NUMINCRBY", "j", "..a", "1"}, "*2\r\n:4\r\n,2.1\r\n"},
		{true, []string{"JSON.NUMMULTBY", "j", "f", "10"}, "*1\r\n,1e+301\r\n"},
		{true, []string{"JSON.NUMINCRBY", "j", "a", "0.5"}, "*1\r\n,4.5\r\n"},
		{true, []string{"JSON.NUMINCRBY", "j", "a", "1e20"}, "*1\r\n,1e+20\r\n"},
		{false, []string{"JSON.NUMINCRBY", "j", "b", "1"}, "-ERR Path does not exist or does not contains a number\r\n"},
		{true, []string{"JSON.RESP", "j"}, "*11\r\n+{\r\n$1\r\na\r\n,1e+20\r\n$1\r\nb\r\n$1\r\ns\r\n$1\r\nc\r\n*3\r\n+{\r\n$1\r\na\r\n,2.1\r\n$1\r\nu\r\n,12345678901234567000\r\n$1\r\nf\r\n,1e+301\r\n"},
		{false, []string{"JSON.RESP", "j"}, "*11\r\n+{\r\n$1\r\na\r\n$5\r\n1e+20\r\n$1\r\nb\r\n$1\r\ns\r\n$1\r\nc\r\n*3\r\n+{\r\n$1\r\na\r\n$3\r\n2.1\r\n$1\r\nu\r\n$20\r\n12345678901234567000\r\n$1\r\nf\r\n$6\r\n1e+301\r\n"},
		{false, []string{"JSON.NUMINCRBY", "j", "$..a", "1"}, "$10\r\n[1e20,3.1]\r\n"},
	}
	for _, c := range cases {
		if got := run(c.resp3, c.args...); got != c.want {
			t.Errorf("%v (resp3=%v) = %q, want %q", c.args, c.resp3, got, c.want)
		}
	}
}

func TestJSONDelDropsADocumentItEmpties(t *testing.T) {
	router := NewRouter()
	RegisterJSONCommands(router)
	RegisterStringCommands(router)
	RegisterKeyCommands(router)
	st := store.NewStore()
	run := func(args ...string) *resp.Value { return tsRun(t, router, st, false, args...) }
	for _, c := range []struct {
		doc, path string
		deleted   int64
		exists    bool
	}{
		{`{"q":1}`, "$.q", 1, false},
		{`{"q":1,"r":2}`, "$.q", 1, true},
		{`[1,2]`, "$.*", 2, false},
		{`{"x":{"q":1}}`, "$.x.q", 1, true},
		{`{}`, "$.x", 0, true},
		{`{"q":1}`, "q", 1, false},
	} {
		run("JSON.SET", "d", "$", c.doc)
		if v := run("JSON.DEL", "d", c.path); v.Int != c.deleted {
			t.Fatalf("JSON.DEL %s %s = %d, want %d", c.doc, c.path, v.Int, c.deleted)
		}
		if v := run("EXISTS", "d"); (v.Int == 1) != c.exists {
			t.Fatalf("after JSON.DEL %s %s EXISTS = %d", c.doc, c.path, v.Int)
		}
		run("DEL", "d")
	}
}
