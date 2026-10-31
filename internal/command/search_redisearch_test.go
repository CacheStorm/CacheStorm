package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/search"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The expected replies were recorded from Redis 8.2, whose FT commands are
// RediSearch's. Results Redis orders by its internal document ids (ties,
// wildcard queries) and the exact scores are left out.
func TestSearchRepliesMatchRediSearch(t *testing.T) {
	router := NewRouter()
	router.SetSearchIndexes(search.NewIndexManager())
	RegisterSearchCommands(router)
	RegisterHashCommands(router)
	RegisterJSONCommands(router)
	RegisterKeyCommands(router)
	RegisterStringCommands(router)
	st := store.NewStore()
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"flushall"}, ""},
		{[]string{"ft.create", "idx", "ON", "HASH", "PREFIX", "1", "doc:", "SCHEMA", "title", "TEXT", "WEIGHT", "2", "body", "TEXT", "price", "NUMERIC", "SORTABLE", "tag", "TAG", "loc", "GEO"}, "+OK\r\n"},
		{[]string{"hset", "doc:1", "title", "hello world", "body", "the quick brown fox", "price", "10", "tag", "a,b", "loc", "13.36,38.11"}, ":5\r\n"},
		{[]string{"hset", "doc:2", "title", "goodbye world", "body", "lazy dogs jumping", "price", "20", "tag", "b", "loc", "15.08,37.50"}, ":5\r\n"},
		{[]string{"hset", "doc:3", "title", "hello there", "body", "worlds apart", "price", "30", "tag", "c,A"}, ":4\r\n"},
		{[]string{"hset", "other:1", "title", "hello"}, ":1\r\n"},
		{[]string{"ft.search", "idx", "hello goodbye|there", "NOCONTENT"}, "*1\r\n:0\r\n"},
		{[]string{"ft.search", "idx", "hello|goodbye world", "NOCONTENT"}, "*4\r\n:3\r\n$5\r\ndoc:2\r\n$5\r\ndoc:3\r\n$5\r\ndoc:1\r\n"},
		{[]string{"ft.search", "idx", "world", "NOCONTENT", "VERBATIM"}, "*3\r\n:2\r\n$5\r\ndoc:1\r\n$5\r\ndoc:2\r\n"},
		{[]string{"ft.search", "idx", "hel*", "NOCONTENT"}, "*3\r\n:2\r\n$5\r\ndoc:3\r\n$5\r\ndoc:1\r\n"},
		{[]string{"ft.search", "idx", "-hello", "NOCONTENT"}, "*2\r\n:1\r\n$5\r\ndoc:2\r\n"},
		{[]string{"ft.search", "idx", "\"hello world\"", "NOCONTENT"}, "*2\r\n:1\r\n$5\r\ndoc:1\r\n"},
		{[]string{"ft.search", "idx", "\"world hello\"", "NOCONTENT"}, "*1\r\n:0\r\n"},
		{[]string{"ft.search", "idx", "@title:world", "NOCONTENT"}, "*3\r\n:2\r\n$5\r\ndoc:1\r\n$5\r\ndoc:2\r\n"},
		{[]string{"ft.search", "idx", "@body:world", "NOCONTENT"}, "*2\r\n:1\r\n$5\r\ndoc:3\r\n"},
		{[]string{"ft.search", "idx", "@title:(hello|goodbye)", "NOCONTENT"}, "*4\r\n:3\r\n$5\r\ndoc:2\r\n$5\r\ndoc:3\r\n$5\r\ndoc:1\r\n"},
		{[]string{"ft.search", "idx", "@price:[15 +inf]", "NOCONTENT"}, "*3\r\n:2\r\n$5\r\ndoc:2\r\n$5\r\ndoc:3\r\n"},
		{[]string{"ft.search", "idx", "@price:[(10 30]", "NOCONTENT"}, "*3\r\n:2\r\n$5\r\ndoc:2\r\n$5\r\ndoc:3\r\n"},
		{[]string{"ft.search", "idx", "@price:[-inf (20]", "NOCONTENT"}, "*2\r\n:1\r\n$5\r\ndoc:1\r\n"},
		{[]string{"ft.search", "idx", "@tag:{b}", "NOCONTENT"}, "*3\r\n:2\r\n$5\r\ndoc:1\r\n$5\r\ndoc:2\r\n"},
		{[]string{"ft.search", "idx", "@loc:[13.36 38.11 10 km]", "NOCONTENT"}, "*2\r\n:1\r\n$5\r\ndoc:1\r\n"},
		{[]string{"ft.search", "idx", "the", "NOCONTENT"}, "*1\r\n:0\r\n"},
		{[]string{"ft.search", "idx", "*", "LIMIT", "0", "0"}, "*1\r\n:3\r\n"},
		{[]string{"ft.search", "idx", "hello", "RETURN", "1", "price"}, "*5\r\n:2\r\n$5\r\ndoc:3\r\n*2\r\n$5\r\nprice\r\n$2\r\n30\r\n$5\r\ndoc:1\r\n*2\r\n$5\r\nprice\r\n$2\r\n10\r\n"},
		{[]string{"ft.search", "idx", "hello", "RETURN", "2", "price", "title"}, "*5\r\n:2\r\n$5\r\ndoc:3\r\n*4\r\n$5\r\nprice\r\n$2\r\n30\r\n$5\r\ntitle\r\n$11\r\nhello there\r\n$5\r\ndoc:1\r\n*4\r\n$5\r\nprice\r\n$2\r\n10\r\n$5\r\ntitle\r\n$11\r\nhello world\r\n"},
		{[]string{"ft.search", "idx", "*", "SORTBY", "price", "DESC", "RETURN", "1", "title"}, "*7\r\n:3\r\n$5\r\ndoc:3\r\n*2\r\n$5\r\ntitle\r\n$11\r\nhello there\r\n$5\r\ndoc:2\r\n*2\r\n$5\r\ntitle\r\n$13\r\ngoodbye world\r\n$5\r\ndoc:1\r\n*2\r\n$5\r\ntitle\r\n$11\r\nhello world\r\n"},
		{[]string{"ft.search", "idx", "*", "SORTBY", "price", "ASC", "NOCONTENT", "WITHSORTKEYS"}, "*7\r\n:3\r\n$5\r\ndoc:1\r\n$3\r\n#10\r\n$5\r\ndoc:2\r\n$3\r\n#20\r\n$5\r\ndoc:3\r\n$3\r\n#30\r\n"},
		{[]string{"ft.search", "idx", "*", "FILTER", "price", "15", "25", "NOCONTENT"}, "*2\r\n:1\r\n$5\r\ndoc:2\r\n"},
		{[]string{"ft.search", "idx", "*", "GEOFILTER", "loc", "13.36", "38.11", "10", "km", "NOCONTENT"}, "*2\r\n:1\r\n$5\r\ndoc:1\r\n"},
		{[]string{"ft.search", "idx", "world", "INFIELDS", "1", "body", "NOCONTENT"}, "*2\r\n:1\r\n$5\r\ndoc:3\r\n"},
		{[]string{"ft.search", "idx", "@price:[$lo $hi]", "PARAMS", "4", "lo", "15", "hi", "25", "NOCONTENT", "DIALECT", "2"}, "*2\r\n:1\r\n$5\r\ndoc:2\r\n"},
		{[]string{"ft.search", "nope", "*"}, "-No such index nope\r\n"},
		{[]string{"ft.search", "idx", "@nofield:x"}, "*1\r\n:0\r\n"},
		{[]string{"ft.aggregate", "idx", "*", "GROUPBY", "1", "@tag", "REDUCE", "COUNT", "0", "AS", "n"}, "*4\r\n:3\r\n*4\r\n$3\r\ntag\r\n$3\r\na,b\r\n$1\r\nn\r\n$1\r\n1\r\n*4\r\n$3\r\ntag\r\n$1\r\nb\r\n$1\r\nn\r\n$1\r\n1\r\n*4\r\n$3\r\ntag\r\n$3\r\nc,A\r\n$1\r\nn\r\n$1\r\n1\r\n"},
		{[]string{"ft.aggregate", "idx", "*", "LOAD", "1", "@price", "GROUPBY", "0", "REDUCE", "SUM", "1", "@price", "AS", "total", "REDUCE", "AVG", "1", "@price", "AS", "avg"}, "*2\r\n:1\r\n*4\r\n$5\r\ntotal\r\n$2\r\n60\r\n$3\r\navg\r\n$2\r\n20\r\n"},
		{[]string{"ft.aggregate", "idx", "*", "LOAD", "2", "@title", "@price", "SORTBY", "2", "@price", "DESC", "LIMIT", "0", "2"}, "*3\r\n:3\r\n*4\r\n$5\r\ntitle\r\n$11\r\nhello there\r\n$5\r\nprice\r\n$2\r\n30\r\n*4\r\n$5\r\ntitle\r\n$13\r\ngoodbye world\r\n$5\r\nprice\r\n$2\r\n20\r\n"},
		{[]string{"ft.aggregate", "idx", "*", "LOAD", "1", "@price", "APPLY", "@price*2", "AS", "double", "FILTER", "@double>30"}, "*3\r\n:2\r\n*4\r\n$5\r\nprice\r\n$2\r\n20\r\n$6\r\ndouble\r\n$2\r\n40\r\n*4\r\n$5\r\nprice\r\n$2\r\n30\r\n$6\r\ndouble\r\n$2\r\n60\r\n"},
		{[]string{"hset", "doc:4", "title", "hello hello hello", "price", "5"}, ":2\r\n"},
		{[]string{"del", "doc:4"}, ":1\r\n"},
		{[]string{"ft.search", "idx", "hello", "NOCONTENT"}, "*3\r\n:2\r\n$5\r\ndoc:3\r\n$5\r\ndoc:1\r\n"},
		{[]string{"ft.tagvals", "idx", "tag"}, "*3\r\n$1\r\na\r\n$1\r\nb\r\n$1\r\nc\r\n"},
		{[]string{"ft.aliasadd", "al", "idx"}, "+OK\r\n"},
		{[]string{"ft.search", "al", "goodbye", "NOCONTENT"}, "*2\r\n:1\r\n$5\r\ndoc:2\r\n"},
		{[]string{"ft.aliasadd", "al", "idx"}, "-Alias already exists\r\n"},
		{[]string{"ft.aliasdel", "al"}, "+OK\r\n"},
		{[]string{"ft.aliasdel", "al"}, "-Alias does not exist\r\n"},
		{[]string{"ft.create", "idx", "SCHEMA", "t", "TEXT"}, "-Index already exists\r\n"},
		{[]string{"ft.create", "j", "ON", "JSON", "PREFIX", "1", "j:", "SCHEMA", "$.name", "AS", "name", "TEXT", "$.age", "AS", "age", "NUMERIC", "$.tags[*]", "AS", "tags", "TAG"}, "+OK\r\n"},
		{[]string{"json.set", "j:1", "$", "{\"name\":\"alice\",\"age\":30,\"tags\":[\"x\",\"y\"]}"}, "+OK\r\n"},
		{[]string{"json.set", "j:2", "$", "{\"name\":\"bob\",\"age\":25,\"tags\":[\"y\"]}"}, "+OK\r\n"},
		{[]string{"ft.search", "j", "@age:[26 40]"}, "*3\r\n:1\r\n$3\r\nj:1\r\n*2\r\n$1\r\n$\r\n$42\r\n{\"name\":\"alice\",\"age\":30,\"tags\":[\"x\",\"y\"]}\r\n"},
		{[]string{"ft.search", "j", "@tags:{y}", "NOCONTENT"}, "*3\r\n:2\r\n$3\r\nj:1\r\n$3\r\nj:2\r\n"},
		{[]string{"ft.search", "j", "bob", "RETURN", "1", "name"}, "*3\r\n:1\r\n$3\r\nj:2\r\n*2\r\n$4\r\nname\r\n$3\r\nbob\r\n"},
		{[]string{"ft.search", "j", "*", "RETURN", "1", "$.age"}, "*5\r\n:2\r\n$3\r\nj:1\r\n*2\r\n$5\r\n$.age\r\n$2\r\n30\r\n$3\r\nj:2\r\n*2\r\n$5\r\n$.age\r\n$2\r\n25\r\n"},
		{[]string{"ft.dropindex", "j"}, "+OK\r\n"},
		{[]string{"ft.dropindex", "j"}, "-j: no such index\r\n"},
		{[]string{"ft.dropindex", "idx", "DD"}, "+OK\r\n"},
		{[]string{"exists", "doc:1"}, ":0\r\n"},
		{[]string{"exists", "other:1"}, ":1\r\n"},
	} {
		if c.args[0] == "flushall" {
			st.Flush()
			continue
		}
		var out bytes.Buffer
		ctx := NewContext(strings.ToUpper(c.args[0]), bytesArgs(c.args[1:]...), st, resp.NewWriter(&out))
		if err := router.Execute(ctx); err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		ctx.Writer.Flush()
		if out.String() != c.want {
			t.Errorf("%v = %q, want %q", c.args, out.String(), c.want)
		}
	}
}
