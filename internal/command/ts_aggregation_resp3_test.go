package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The expected replies were recorded from Redis 8.2: RESP3 shapes, the
// FILTER/GROUPBY order of TS.MRANGE, aggregations summed in RedisTimeSeries'
// order, TWA interpolation at bucket edges, and compaction recomputed after
// out-of-order writes and deletions.
func TestTimeSeriesAggregationsAndRESP3MatchRedisTimeSeries(t *testing.T) {
	router := NewRouter()
	RegisterTSCommands(router)
	st := store.NewStore()
	resp3 := false
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"flushall"}, ""},
		{[]string{"TS.CREATE", "t", "LABELS", "a", "b", "c", "d"}, "+OK\r\n"},
		{[]string{"TS.CREATE", "u", "LABELS", "a", "b"}, "+OK\r\n"},
		{[]string{"TS.MADD", "t", "1", "1.5", "u", "1", "2.25", "t", "12", "3"}, "*3\r\n:1\r\n:1\r\n:12\r\n"},
		{[]string{"TS.MRANGE", "-", "+", "GROUPBY", "a", "REDUCE", "max", "FILTER", "a=b"}, "-ERR TSDB: GROUPBY should always come after filter\r\n"},
		{[]string{"TS.MRANGE", "-", "+", "COUNT", "1"}, "-ERR TSDB: missing FILTER argument\r\n"},
		{[]string{"TS.MRANGE", "-", "+", "FILTER"}, "-ERR TSDB: missing labels for filter argument\r\n"},
		{[]string{"TS.MRANGE", "-", "+", "FILTER", "a=b", "COUNT", "1"}, "-ERR TSDB: failed parsing labels\r\n"},
		{[]string{"TS.MRANGE", "-", "+", "FILTER", "a=b", "GROUPBY", "a"}, "-ERR wrong number of arguments for 'ts.mrange' command\r\n"},
		{[]string{"TS.MRANGE", "-", "+", "SELECTED_LABELS", "c", "FILTER", "a=b", "GROUPBY", "a", "REDUCE", "max"}, "*1\r\n*3\r\n$3\r\na=b\r\n*1\r\n*2\r\n$1\r\nc\r\n$-1\r\n*2\r\n*2\r\n:1\r\n+2.25\r\n*2\r\n:12\r\n+3\r\n"},
		{[]string{"TS.MRANGE", "-", "+", "SELECTED_LABELS", "__reducer__", "FILTER", "a=b", "GROUPBY", "a", "REDUCE", "max"}, "*1\r\n*3\r\n$3\r\na=b\r\n*1\r\n*2\r\n$11\r\n__reducer__\r\n$3\r\nmax\r\n*2\r\n*2\r\n:1\r\n+2.25\r\n*2\r\n:12\r\n+3\r\n"},
		{[]string{"resp3"}, ""},
		{[]string{"TS.GET", "t"}, "*2\r\n:12\r\n,3\r\n"},
		{[]string{"TS.RANGE", "t", "-", "+"}, "*2\r\n*2\r\n:1\r\n,1.5\r\n*2\r\n:12\r\n,3\r\n"},
		{[]string{"TS.MGET", "WITHLABELS", "FILTER", "a=b"}, "%2\r\n$1\r\nt\r\n*2\r\n%2\r\n$1\r\na\r\n$1\r\nb\r\n$1\r\nc\r\n$1\r\nd\r\n*2\r\n:12\r\n,3\r\n$1\r\nu\r\n*2\r\n%1\r\n$1\r\na\r\n$1\r\nb\r\n*2\r\n:1\r\n,2.25\r\n"},
		{[]string{"TS.MGET", "SELECTED_LABELS", "c", "zz", "FILTER", "a=b"}, "%2\r\n$1\r\nt\r\n*2\r\n%2\r\n$1\r\nc\r\n$1\r\nd\r\n$2\r\nzz\r\n_\r\n*2\r\n:12\r\n,3\r\n$1\r\nu\r\n*2\r\n%2\r\n$1\r\nc\r\n_\r\n$2\r\nzz\r\n_\r\n*2\r\n:1\r\n,2.25\r\n"},
		{[]string{"TS.MRANGE", "-", "+", "AGGREGATION", "max", "10", "FILTER", "a=b"}, "%2\r\n$1\r\nt\r\n*3\r\n%0\r\n%1\r\n$11\r\naggregators\r\n*1\r\n$3\r\nmax\r\n*2\r\n*2\r\n:0\r\n,1.5\r\n*2\r\n:10\r\n,3\r\n$1\r\nu\r\n*3\r\n%0\r\n%1\r\n$11\r\naggregators\r\n*1\r\n$3\r\nmax\r\n*1\r\n*2\r\n:0\r\n,2.25\r\n"},
		{[]string{"TS.MRANGE", "-", "+", "WITHLABELS", "FILTER", "a=b", "GROUPBY", "a", "REDUCE", "max"}, "%1\r\n$3\r\na=b\r\n*4\r\n%1\r\n$1\r\na\r\n$1\r\nb\r\n%1\r\n$8\r\nreducers\r\n*1\r\n$3\r\nmax\r\n%1\r\n$7\r\nsources\r\n*2\r\n$1\r\nt\r\n$1\r\nu\r\n*2\r\n*2\r\n:1\r\n,2.25\r\n*2\r\n:12\r\n,3\r\n"},
		{[]string{"TS.MRANGE", "-", "+", "SELECTED_LABELS", "a", "FILTER", "a=b", "GROUPBY", "a", "REDUCE", "max"}, "%1\r\n$3\r\na=b\r\n*4\r\n%1\r\n$1\r\na\r\n$1\r\nb\r\n%1\r\n$8\r\nreducers\r\n*1\r\n$3\r\nmax\r\n%1\r\n$7\r\nsources\r\n*2\r\n$1\r\nt\r\n$1\r\nu\r\n*2\r\n*2\r\n:1\r\n,2.25\r\n*2\r\n:12\r\n,3\r\n"},
		{[]string{"TS.MGET", "FILTER", "a=zzz"}, "%0\r\n"},
		{[]string{"TS.QUERYINDEX", "a=b"}, "~2\r\n$1\r\nt\r\n$1\r\nu\r\n"},
		{[]string{"TS.CREATE", "n", "RETENTION", "100"}, "+OK\r\n"},
		{[]string{"TS.CREATERULE", "u", "n", "AGGREGATION", "sum", "10"}, "+OK\r\n"},
		{[]string{"TS.ALTER", "u", "IGNORE", "10", "0.5"}, "+OK\r\n"},
		{[]string{"resp2"}, ""},
		{[]string{"flushall"}, ""},
		{[]string{"TS.CREATE", "v"}, "+OK\r\n"},
		{[]string{"TS.MADD", "v", "1", "-344.43", "v", "2", "12.5", "v", "3", "100", "v", "9", "-7.25", "v", "10", "3", "v", "11", "0.1", "v", "15", "8", "v", "31", "-2", "v", "33", "1e3", "v", "40", "5"}, "*10\r\n:1\r\n:2\r\n:3\r\n:9\r\n:10\r\n:11\r\n:15\r\n:31\r\n:33\r\n:40\r\n"},
		{[]string{"TS.RANGE", "v", "-", "+", "AGGREGATION", "twa", "7"}, "*5\r\n*2\r\n:0\r\n+24.5475\r\n*2\r\n:7\r\n+4.266071428571428\r\n*2\r\n:14\r\n+6.251785714285715\r\n*2\r\n:28\r\n+387.21811224489795\r\n*2\r\n:35\r\n+360.3571428571429\r\n"},
		{[]string{"TS.REVRANGE", "v", "-", "+", "AGGREGATION", "twa", "7"}, "*5\r\n*2\r\n:35\r\n+360.3571428571429\r\n*2\r\n:28\r\n+387.21811224489795\r\n*2\r\n:14\r\n+6.251785714285715\r\n*2\r\n:7\r\n+4.266071428571428\r\n*2\r\n:0\r\n+24.5475\r\n"},
		{[]string{"TS.RANGE", "v", "4", "35", "AGGREGATION", "twa", "5", "EMPTY"}, "*8\r\n*2\r\n:0\r\n+73.1875\r\n*2\r\n:5\r\n+22.375\r\n*2\r\n:10\r\n+3.55\r\n*2\r\n:15\r\n+6.4375\r\n*2\r\n:20\r\n+3.3125\r\n*2\r\n:25\r\n+1.875E-1\r\n*2\r\n:30\r\n+542.4053571428572\r\n*2\r\n:35\r\n+715.7142857142858\r\n"},
		{[]string{"TS.RANGE", "v", "4", "35", "AGGREGATION", "twa", "5", "BUCKETTIMESTAMP", "end", "EMPTY"}, "*7\r\n*2\r\n:5\r\n+73.1875\r\n*2\r\n:10\r\n+22.375\r\n*2\r\n:15\r\n+3.55\r\n*2\r\n:20\r\n+6.4375\r\n*2\r\n:25\r\n+3.3125\r\n*2\r\n:30\r\n+1.875E-1\r\n*2\r\n:35\r\n+542.4053571428572\r\n"},
		{[]string{"TS.RANGE", "v", "-", "+", "AGGREGATION", "var.s", "7"}, "*5\r\n*2\r\n:0\r\n+55428.8833\r\n*2\r\n:7\r\n+27.91583333333333\r\n*2\r\n:14\r\n+0\r\n*2\r\n:28\r\n+502002\r\n*2\r\n:35\r\n+0\r\n"},
		{[]string{"TS.RANGE", "v", "-", "+", "AGGREGATION", "std.p", "7"}, "*5\r\n*2\r\n:0\r\n+192.23056173945562\r\n*2\r\n:7\r\n+4.313995312416965\r\n*2\r\n:14\r\n+0\r\n*2\r\n:28\r\n+501\r\n*2\r\n:35\r\n+0\r\n"},
		{[]string{"TS.REVRANGE", "v", "-", "+", "AGGREGATION", "avg", "7"}, "*5\r\n*2\r\n:35\r\n+5\r\n*2\r\n:28\r\n+499\r\n*2\r\n:14\r\n+8\r\n*2\r\n:7\r\n+-1.3833333333333335\r\n*2\r\n:0\r\n+-77.31\r\n"},
		{[]string{"TS.REVRANGE", "v", "-", "+", "AGGREGATION", "last", "7"}, "*5\r\n*2\r\n:35\r\n+5\r\n*2\r\n:28\r\n+-2\r\n*2\r\n:14\r\n+8\r\n*2\r\n:7\r\n+-7.25\r\n*2\r\n:0\r\n+-344.43\r\n"},
		{[]string{"TS.RANGE", "v", "-", "+", "AGGREGATION", "last", "5", "EMPTY"}, "*9\r\n*2\r\n:0\r\n+100\r\n*2\r\n:5\r\n+-7.25\r\n*2\r\n:10\r\n+1E-1\r\n*2\r\n:15\r\n+8\r\n*2\r\n:20\r\n+8\r\n*2\r\n:25\r\n+8\r\n*2\r\n:30\r\n+1000\r\n*2\r\n:35\r\n+1000\r\n*2\r\n:40\r\n+5\r\n"},
		{[]string{"TS.RANGE", "v", "-", "25", "ALIGN", "9", "AGGREGATION", "count", "10"}, "*2\r\n*2\r\n:0\r\n+3\r\n*2\r\n:9\r\n+4\r\n"},
		{[]string{"flushall"}, ""},
		{[]string{"TS.CREATE", "s", "DUPLICATE_POLICY", "LAST"}, "+OK\r\n"},
		{[]string{"TS.CREATE", "d"}, "+OK\r\n"},
		{[]string{"TS.CREATE", "e"}, "+OK\r\n"},
		{[]string{"TS.CREATE", "x"}, "+OK\r\n"},
		{[]string{"TS.CREATERULE", "s", "d", "AGGREGATION", "sum", "10"}, "+OK\r\n"},
		{[]string{"TS.CREATERULE", "s", "e", "AGGREGATION", "twa", "10"}, "+OK\r\n"},
		{[]string{"TS.CREATERULE", "d", "x", "AGGREGATION", "sum", "10"}, "-ERR TSDB: the source key already has a source rule\r\n"},
		{[]string{"TS.CREATERULE", "x", "s", "AGGREGATION", "sum", "10"}, "-ERR TSDB: the destination key already has a dst rule\r\n"},
		{[]string{"TS.MADD", "s", "1", "1", "s", "5", "2", "s", "12", "3", "s", "25", "4", "s", "31", "5"}, "*5\r\n:1\r\n:5\r\n:12\r\n:25\r\n:31\r\n"},
		{[]string{"TS.RANGE", "e", "-", "+"}, "*3\r\n*2\r\n:0\r\n+1.976190476190476\r\n*2\r\n:10\r\n+3.2175824175824177\r\n*2\r\n:20\r\n+4.112179487179486\r\n"},
		{[]string{"TS.ADD", "s", "3", "10"}, ":3\r\n"},
		{[]string{"TS.RANGE", "d", "-", "+"}, "*3\r\n*2\r\n:0\r\n+13\r\n*2\r\n:10\r\n+3\r\n*2\r\n:20\r\n+4\r\n"},
		{[]string{"TS.RANGE", "e", "-", "+"}, "*3\r\n*2\r\n:0\r\n+3.865079365079365\r\n*2\r\n:10\r\n+3.2175824175824177\r\n*2\r\n:20\r\n+4.112179487179486\r\n"},
		{[]string{"TS.DEL", "s", "10", "15"}, ":1\r\n"},
		{[]string{"TS.RANGE", "d", "-", "+"}, "*2\r\n*2\r\n:0\r\n+13\r\n*2\r\n:20\r\n+4\r\n"},
		{[]string{"TS.DEL", "s", "0", "100"}, ":5\r\n"},
		{[]string{"TS.ADD", "s", "32", "1"}, ":32\r\n"},
		{[]string{"TS.ADD", "s", "45", "1"}, ":45\r\n"},
		{[]string{"TS.RANGE", "d", "-", "+"}, "*1\r\n*2\r\n:30\r\n+1\r\n"},
	} {
		switch c.args[0] {
		case "flushall":
			st.Flush()
			continue
		case "resp2", "resp3":
			resp3 = c.args[0] == "resp3"
			continue
		}
		var out bytes.Buffer
		w := resp.NewWriter(&out)
		w.SetRESP3(resp3)
		ctx := NewContext(strings.ToUpper(c.args[0]), bytesArgs(c.args[1:]...), st, w)
		if err := router.Execute(ctx); err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		ctx.Writer.Flush()
		if out.String() != c.want {
			t.Errorf("%v = %q, want %q", c.args, out.String(), c.want)
		}
	}
}

func tsRun(t *testing.T, router *Router, st *store.Store, resp3 bool, args ...string) *resp.Value {
	t.Helper()
	var out bytes.Buffer
	w := resp.NewWriter(&out)
	w.SetRESP3(resp3)
	ctx := NewContext(strings.ToUpper(args[0]), bytesArgs(args[1:]...), st, w)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	ctx.Writer.Flush()
	v, err := resp.NewReader(&out).ReadValue()
	if err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	return v
}

func TestTSInfoIsAMapForRESP3(t *testing.T) {
	router := NewRouter()
	RegisterTSCommands(router)
	st := store.NewStore()
	tsRun(t, router, st, false, "TS.CREATE", "s", "LABELS", "a", "b")
	tsRun(t, router, st, false, "TS.CREATE", "d")
	tsRun(t, router, st, false, "TS.CREATERULE", "s", "d", "AGGREGATION", "avg", "10")
	v := tsRun(t, router, st, true, "TS.INFO", "s")
	if v.Type != resp.TypeMap {
		t.Fatalf("TS.INFO = %v, want a map", v.Type)
	}
	if labels := v.Field("labels"); labels == nil || labels.Type != resp.TypeMap || labels.Field("a").String() != "b" {
		t.Fatalf("labels = %+v", labels)
	}
	if rules := v.Field("rules"); rules == nil || rules.Type != resp.TypeMap || rules.Field("d") == nil {
		t.Fatalf("rules = %+v", rules)
	}
	if diff := v.Field("ignoreMaxValDiff"); diff == nil || diff.Type != resp.TypeDouble {
		t.Fatalf("ignoreMaxValDiff = %+v", diff)
	}
	if v := tsRun(t, router, st, false, "TS.INFO", "s"); v.Type != resp.TypeArray || len(v.Array) != 28 {
		t.Fatalf("RESP2 TS.INFO = %v with %d items", v.Type, len(v.Array))
	}
}

// RedisTimeSeries fills TWA buckets up to the end of an open range and runs
// out of memory; the fill stops at the series' last sample instead.
func TestTSTWAEmptyOnOpenRangeIsBounded(t *testing.T) {
	router := NewRouter()
	RegisterTSCommands(router)
	st := store.NewStore()
	for i, v := range []string{"1", "50", "2", "3", "60"} {
		tsRun(t, router, st, false, "TS.ADD", "s", string(rune('1'+i))+"0", v)
	}
	v := tsRun(t, router, st, false, "TS.RANGE", "s", "-", "+", "FILTER_BY_VALUE", "0", "10", "AGGREGATION", "twa", "1", "EMPTY")
	if v.Type != resp.TypeArray || len(v.Array) == 0 || len(v.Array) > 60 {
		t.Fatalf("TWA EMPTY returned %d buckets", len(v.Array))
	}
}
