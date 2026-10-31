package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The expected replies were recorded from Redis 8.2, whose time series
// commands are RedisTimeSeries'. Replies that depend on the clock or on
// memory estimates are left out.
func TestTimeSeriesRepliesMatchRedisTimeSeries(t *testing.T) {
	router := NewRouter()
	RegisterTSCommands(router)
	RegisterStringCommands(router)
	RegisterKeyCommands(router)
	st := store.NewStore()
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"flushall"}, ""},
		{[]string{"ts.create", "t", "RETENTION", "0", "LABELS", "sensor", "temp", "room", "a"}, "+OK\r\n"},
		{[]string{"ts.create", "t"}, "-ERR TSDB: key already exists\r\n"},
		{[]string{"ts.create", "t2", "RETENTION", "-1"}, "-TSDB: Couldn't parse RETENTION\r\n"},
		{[]string{"ts.create", "t2", "DUPLICATE_POLICY", "foo"}, "-ERR TSDB: Unknown DUPLICATE_POLICY\r\n"},
		{[]string{"ts.create", "t2", "LABELS", "a"}, "+OK\r\n"},
		{[]string{"ts.create", "t2", "ENCODING", "UNCOMPRESSED", "CHUNK_SIZE", "128", "DUPLICATE_POLICY", "LAST"}, "-ERR TSDB: key already exists\r\n"},
		{[]string{"ts.add", "t", "1000", "1.5"}, ":1000\r\n"},
		{[]string{"ts.add", "t", "2000", "2"}, ":2000\r\n"},
		{[]string{"ts.add", "t", "2000", "3"}, "-ERR TSDB: Error at upsert, update is not supported when DUPLICATE_POLICY is set to BLOCK mode\r\n"},
		{[]string{"ts.add", "t2", "2000", "1"}, ":2000\r\n"},
		{[]string{"ts.add", "t2", "2000", "5"}, "-ERR TSDB: Error at upsert, update is not supported when DUPLICATE_POLICY is set to BLOCK mode\r\n"},
		{[]string{"ts.add", "t", "1500", "9"}, ":1500\r\n"},
		{[]string{"ts.add", "t", "500", "1", "ON_DUPLICATE", "SUM"}, ":500\r\n"},
		{[]string{"ts.add", "t", "1", "x"}, "-ERR TSDB: invalid value\r\n"},
		{[]string{"ts.add", "t", "-1", "1"}, "-ERR TSDB: invalid timestamp, must be a nonnegative integer\r\n"},
		{[]string{"ts.get", "t"}, "*2\r\n:2000\r\n+2\r\n"},
		{[]string{"ts.get", "nokey"}, "-ERR TSDB: the key does not exist\r\n"},
		{[]string{"ts.range", "t", "-", "+"}, "*4\r\n*2\r\n:500\r\n+1\r\n*2\r\n:1000\r\n+1.5\r\n*2\r\n:1500\r\n+9\r\n*2\r\n:2000\r\n+2\r\n"},
		{[]string{"ts.range", "t", "0", "1600"}, "*3\r\n*2\r\n:500\r\n+1\r\n*2\r\n:1000\r\n+1.5\r\n*2\r\n:1500\r\n+9\r\n"},
		{[]string{"ts.range", "t", "-", "+", "COUNT", "2"}, "*2\r\n*2\r\n:500\r\n+1\r\n*2\r\n:1000\r\n+1.5\r\n"},
		{[]string{"ts.revrange", "t", "-", "+", "COUNT", "2"}, "*2\r\n*2\r\n:2000\r\n+2\r\n*2\r\n:1500\r\n+9\r\n"},
		{[]string{"ts.range", "t", "-", "+", "AGGREGATION", "avg", "1000"}, "*3\r\n*2\r\n:0\r\n+1\r\n*2\r\n:1000\r\n+5.25\r\n*2\r\n:2000\r\n+2\r\n"},
		{[]string{"ts.range", "t", "-", "+", "AGGREGATION", "sum", "1000"}, "*3\r\n*2\r\n:0\r\n+1\r\n*2\r\n:1000\r\n+10.5\r\n*2\r\n:2000\r\n+2\r\n"},
		{[]string{"ts.range", "t", "-", "+", "AGGREGATION", "count", "1000"}, "*3\r\n*2\r\n:0\r\n+1\r\n*2\r\n:1000\r\n+2\r\n*2\r\n:2000\r\n+1\r\n"},
		{[]string{"ts.range", "t", "-", "+", "AGGREGATION", "max", "1000"}, "*3\r\n*2\r\n:0\r\n+1\r\n*2\r\n:1000\r\n+9\r\n*2\r\n:2000\r\n+2\r\n"},
		{[]string{"ts.range", "t", "-", "+", "AGGREGATION", "first", "1000"}, "*3\r\n*2\r\n:0\r\n+1\r\n*2\r\n:1000\r\n+1.5\r\n*2\r\n:2000\r\n+2\r\n"},
		{[]string{"ts.range", "t", "-", "+", "AGGREGATION", "range", "1000"}, "*3\r\n*2\r\n:0\r\n+0\r\n*2\r\n:1000\r\n+7.5\r\n*2\r\n:2000\r\n+0\r\n"},
		{[]string{"ts.range", "t", "-", "+", "AGGREGATION", "foo", "1000"}, "-ERR TSDB: Unknown aggregation type\r\n"},
		{[]string{"ts.range", "t", "-", "+", "AGGREGATION", "avg", "0"}, "-ERR TSDB: bucketDuration must be greater than zero\r\n"},
		{[]string{"ts.range", "t", "-", "+", "FILTER_BY_VALUE", "1", "2"}, "*3\r\n*2\r\n:500\r\n+1\r\n*2\r\n:1000\r\n+1.5\r\n*2\r\n:2000\r\n+2\r\n"},
		{[]string{"ts.range", "t", "-", "+", "FILTER_BY_TS", "1000", "2000"}, "*2\r\n*2\r\n:1000\r\n+1.5\r\n*2\r\n:2000\r\n+2\r\n"},
		{[]string{"ts.range", "nokey", "-", "+"}, "-ERR TSDB: the key does not exist\r\n"},
		{[]string{"ts.range", "t", "x", "+"}, "-ERR TSDB: wrong fromTimestamp\r\n"},
		{[]string{"ts.madd", "t", "3000", "3", "t2", "3000", "4", "nokey", "1", "1"}, "*3\r\n:3000\r\n:3000\r\n-ERR TSDB: the key is not a TSDB key\r\n"},
		{[]string{"ts.madd", "t", "1"}, "-ERR wrong number of arguments for 'ts.madd' command\r\n"},
		{[]string{"ts.del", "t", "0", "1000"}, ":2\r\n"},
		{[]string{"ts.del", "nokey", "0", "1"}, "-ERR TSDB: the key does not exist\r\n"},
		{[]string{"ts.range", "t", "-", "+"}, "*3\r\n*2\r\n:1500\r\n+9\r\n*2\r\n:2000\r\n+2\r\n*2\r\n:3000\r\n+3\r\n"},
		{[]string{"ts.alter", "t", "RETENTION", "100000", "LABELS", "sensor", "hum"}, "+OK\r\n"},
		{[]string{"ts.alter", "nokey", "RETENTION", "1"}, "-ERR TSDB: the key does not exist\r\n"},
		{[]string{"ts.queryindex", "sensor=hum"}, "*1\r\n$1\r\nt\r\n"},
		{[]string{"ts.queryindex", "sensor!=hum"}, "-ERR TSDB: please provide at least one matcher\r\n"},
		{[]string{"ts.queryindex", "room="}, "-ERR TSDB: please provide at least one matcher\r\n"},
		{[]string{"ts.mget", "FILTER", "sensor=hum"}, "*1\r\n*3\r\n$1\r\nt\r\n*0\r\n*2\r\n:3000\r\n+3\r\n"},
		{[]string{"ts.mget", "WITHLABELS", "FILTER", "sensor=hum"}, "*1\r\n*3\r\n$1\r\nt\r\n*1\r\n*2\r\n$6\r\nsensor\r\n$3\r\nhum\r\n*2\r\n:3000\r\n+3\r\n"},
		{[]string{"ts.mrange", "-", "+", "FILTER", "sensor=hum"}, "*1\r\n*3\r\n$1\r\nt\r\n*0\r\n*3\r\n*2\r\n:1500\r\n+9\r\n*2\r\n:2000\r\n+2\r\n*2\r\n:3000\r\n+3\r\n"},
		{[]string{"ts.mrevrange", "-", "+", "WITHLABELS", "FILTER", "sensor=hum"}, "*1\r\n*3\r\n$1\r\nt\r\n*1\r\n*2\r\n$6\r\nsensor\r\n$3\r\nhum\r\n*3\r\n*2\r\n:3000\r\n+3\r\n*2\r\n:2000\r\n+2\r\n*2\r\n:1500\r\n+9\r\n"},
		{[]string{"ts.createrule", "t", "t2", "AGGREGATION", "avg", "1000"}, "+OK\r\n"},
		{[]string{"ts.deleterule", "t", "t2"}, "+OK\r\n"},
		{[]string{"type", "t"}, "+TSDB-TYPE\r\n"},
		{[]string{"set", "s", "v"}, "+OK\r\n"},
		{[]string{"ts.add", "s", "1", "1"}, "-ERR TSDB: the key is not a TSDB key\r\n"},
		{[]string{"ts.get", "s"}, "-ERR WRONGTYPE Operation against a key holding the wrong kind of value\r\n"},
		{[]string{"ts.range", "s", "-", "+"}, "-ERR WRONGTYPE Operation against a key holding the wrong kind of value\r\n"},
		{[]string{"flushall"}, ""},
		{[]string{"ts.create", "a", "CHUNK_SIZE", "10"}, "-ERR TSDB: CHUNK_SIZE value must be a multiple of 8 in the range [48 .. 1048576]\r\n"},
		{[]string{"ts.create", "a", "CHUNK_SIZE", "x"}, "-ERR TSDB: Couldn't parse CHUNK_SIZE\r\n"},
		{[]string{"ts.create", "a", "ENCODING", "foo"}, "-ERR TSDB: unknown ENCODING parameter\r\n"},
		{[]string{"ts.create", "a", "IGNORE", "x", "1"}, "-ERR TSDB: Couldn't parse IGNORE\r\n"},
		{[]string{"ts.create", "a", "IGNORE", "1"}, "-ERR TSDB: Couldn't parse IGNORE\r\n"},
		{[]string{"ts.create", "a", "foo"}, "+OK\r\n"},
		{[]string{"ts.create", "a", "RETENTION"}, "-ERR TSDB: Couldn't parse RETENTION\r\n"},
		{[]string{"ts.create", "a", "LABELS"}, "-ERR TSDB: key already exists\r\n"},
		{[]string{"ts.create", "a", "LABELS", "k", "v", "k2"}, "-ERR TSDB: key already exists\r\n"},
		{[]string{"ts.create", "r", "RETENTION", "1000"}, "+OK\r\n"},
		{[]string{"ts.add", "r", "5000", "1"}, ":5000\r\n"},
		{[]string{"ts.add", "r", "3000", "1"}, "-ERR TSDB: Timestamp is older than retention\r\n"},
		{[]string{"ts.add", "r", "4000", "1"}, ":4000\r\n"},
		{[]string{"ts.add", "r", "6500", "1"}, ":6500\r\n"},
		{[]string{"ts.range", "r", "-", "+"}, "*1\r\n*2\r\n:6500\r\n+1\r\n"},
		{[]string{"ts.create", "p", "DUPLICATE_POLICY", "SUM"}, "+OK\r\n"},
		{[]string{"ts.add", "p", "1", "1"}, ":1\r\n"},
		{[]string{"ts.add", "p", "1", "2"}, ":1\r\n"},
		{[]string{"ts.add", "p", "1", "5", "ON_DUPLICATE", "MIN"}, ":1\r\n"},
		{[]string{"ts.add", "p", "1", "5", "ON_DUPLICATE", "foo"}, "-ERR TSDB: Unknown DUPLICATE_POLICY\r\n"},
		{[]string{"ts.add", "p", "0", "7"}, ":0\r\n"},
		{[]string{"ts.range", "p", "-", "+"}, "*2\r\n*2\r\n:0\r\n+7\r\n*2\r\n:1\r\n+3\r\n"},
		{[]string{"ts.add", "new", "1", "1", "RETENTION", "5", "LABELS", "x", "y"}, ":1\r\n"},
		{[]string{"ts.create", "ig", "IGNORE", "10", "0.5", "DUPLICATE_POLICY", "LAST"}, "+OK\r\n"},
		{[]string{"ts.add", "ig", "100", "1"}, ":100\r\n"},
		{[]string{"ts.add", "ig", "105", "1.2"}, ":100\r\n"},
		{[]string{"ts.add", "ig", "120", "1.2"}, ":120\r\n"},
		{[]string{"ts.range", "ig", "-", "+"}, "*2\r\n*2\r\n:100\r\n+1\r\n*2\r\n:120\r\n+1.2\r\n"},
		{[]string{"ts.create", "s1", "LABELS", "type", "A", "k", "1"}, "+OK\r\n"},
		{[]string{"ts.create", "s2", "LABELS", "type", "B", "k", "2"}, "+OK\r\n"},
		{[]string{"ts.add", "s1", "1000", "1"}, ":1000\r\n"},
		{[]string{"ts.add", "s1", "2000", "3"}, ":2000\r\n"},
		{[]string{"ts.add", "s2", "1500", "10"}, ":1500\r\n"},
		{[]string{"ts.mget", "FILTER", "k=(1,2)"}, "*2\r\n*3\r\n$2\r\ns1\r\n*0\r\n*2\r\n:2000\r\n+3\r\n*3\r\n$2\r\ns2\r\n*0\r\n*2\r\n:1500\r\n+10\r\n"},
		{[]string{"ts.mget", "SELECTED_LABELS", "type", "FILTER", "k=(1,2)"}, "*2\r\n*3\r\n$2\r\ns1\r\n*1\r\n*2\r\n$4\r\ntype\r\n$1\r\nA\r\n*2\r\n:2000\r\n+3\r\n*3\r\n$2\r\ns2\r\n*1\r\n*2\r\n$4\r\ntype\r\n$1\r\nB\r\n*2\r\n:1500\r\n+10\r\n"},
		{[]string{"ts.mget", "FILTER"}, "-ERR wrong number of arguments for 'ts.mget' command\r\n"},
		{[]string{"ts.mget", "FILTER", "k!=1"}, "-ERR TSDB: please provide at least one matcher\r\n"},
		{[]string{"ts.mget", "WITHLABELS"}, "-ERR wrong number of arguments for 'ts.mget' command\r\n"},
		{[]string{"ts.mrange", "-", "+", "WITHLABELS", "FILTER", "k=(1,2)"}, "*2\r\n*3\r\n$2\r\ns1\r\n*2\r\n*2\r\n$4\r\ntype\r\n$1\r\nA\r\n*2\r\n$1\r\nk\r\n$1\r\n1\r\n*2\r\n*2\r\n:1000\r\n+1\r\n*2\r\n:2000\r\n+3\r\n*3\r\n$2\r\ns2\r\n*2\r\n*2\r\n$4\r\ntype\r\n$1\r\nB\r\n*2\r\n$1\r\nk\r\n$1\r\n2\r\n*1\r\n*2\r\n:1500\r\n+10\r\n"},
		{[]string{"ts.mrange", "-", "+", "COUNT", "1", "FILTER", "k=(1,2)"}, "*2\r\n*3\r\n$2\r\ns1\r\n*0\r\n*1\r\n*2\r\n:1000\r\n+1\r\n*3\r\n$2\r\ns2\r\n*0\r\n*1\r\n*2\r\n:1500\r\n+10\r\n"},
		{[]string{"ts.mrange", "-", "+", "AGGREGATION", "sum", "10000", "FILTER", "k=(1,2)"}, "*2\r\n*3\r\n$2\r\ns1\r\n*0\r\n*1\r\n*2\r\n:0\r\n+4\r\n*3\r\n$2\r\ns2\r\n*0\r\n*1\r\n*2\r\n:0\r\n+10\r\n"},
		{[]string{"ts.mrange", "-", "+", "FILTER", "k=(1,2)", "GROUPBY", "k", "REDUCE", "sum"}, "*2\r\n*3\r\n$3\r\nk=1\r\n*0\r\n*2\r\n*2\r\n:1000\r\n+1\r\n*2\r\n:2000\r\n+3\r\n*3\r\n$3\r\nk=2\r\n*0\r\n*1\r\n*2\r\n:1500\r\n+10\r\n"},
		{[]string{"ts.mrange", "-", "+", "WITHLABELS", "FILTER", "k=(1,2)", "GROUPBY", "k", "REDUCE", "max"}, "*2\r\n*3\r\n$3\r\nk=1\r\n*3\r\n*2\r\n$1\r\nk\r\n$1\r\n1\r\n*2\r\n$11\r\n__reducer__\r\n$3\r\nmax\r\n*2\r\n$10\r\n__source__\r\n$2\r\ns1\r\n*2\r\n*2\r\n:1000\r\n+1\r\n*2\r\n:2000\r\n+3\r\n*3\r\n$3\r\nk=2\r\n*3\r\n*2\r\n$1\r\nk\r\n$1\r\n2\r\n*2\r\n$11\r\n__reducer__\r\n$3\r\nmax\r\n*2\r\n$10\r\n__source__\r\n$2\r\ns2\r\n*1\r\n*2\r\n:1500\r\n+10\r\n"},
		{[]string{"ts.mrange", "-", "+", "FILTER", "k=(1,2)", "GROUPBY", "foo", "REDUCE", "sum"}, "*0\r\n"},
		{[]string{"ts.mrevrange", "-", "+", "SELECTED_LABELS", "type", "FILTER", "k=(1,2)"}, "*2\r\n*3\r\n$2\r\ns1\r\n*1\r\n*2\r\n$4\r\ntype\r\n$1\r\nA\r\n*2\r\n*2\r\n:2000\r\n+3\r\n*2\r\n:1000\r\n+1\r\n*3\r\n$2\r\ns2\r\n*1\r\n*2\r\n$4\r\ntype\r\n$1\r\nB\r\n*1\r\n*2\r\n:1500\r\n+10\r\n"},
		{[]string{"ts.create", "src"}, "+OK\r\n"},
		{[]string{"ts.create", "dst"}, "+OK\r\n"},
		{[]string{"ts.createrule", "src", "dst", "AGGREGATION", "sum", "10"}, "+OK\r\n"},
		{[]string{"ts.createrule", "src", "dst", "AGGREGATION", "sum", "10"}, "-ERR TSDB: the destination key already has a src rule\r\n"},
		{[]string{"ts.createrule", "src", "nokey", "AGGREGATION", "sum", "10"}, "-ERR TSDB: the key does not exist\r\n"},
		{[]string{"ts.createrule", "src", "src", "AGGREGATION", "sum", "10"}, "-ERR TSDB: the source key and destination key should be different\r\n"},
		{[]string{"ts.createrule", "src", "dst2", "AGGREGATION", "foo", "10"}, "-ERR TSDB: Unknown aggregation type\r\n"},
		{[]string{"ts.add", "src", "1", "1"}, ":1\r\n"},
		{[]string{"ts.add", "src", "5", "2"}, ":5\r\n"},
		{[]string{"ts.add", "src", "12", "3"}, ":12\r\n"},
		{[]string{"ts.add", "src", "25", "4"}, ":25\r\n"},
		{[]string{"ts.range", "dst", "-", "+"}, "*2\r\n*2\r\n:0\r\n+3\r\n*2\r\n:10\r\n+3\r\n"},
		{[]string{"ts.add", "dst", "100", "1"}, ":100\r\n"},
		{[]string{"ts.deleterule", "src", "dst"}, "+OK\r\n"},
		{[]string{"ts.deleterule", "src", "dst"}, "-ERR TSDB: compaction rule does not exist\r\n"},
		{[]string{"ts.range", "s1", "-", "+", "AGGREGATION", "avg", "1000", "BUCKETTIMESTAMP", "mid"}, "*2\r\n*2\r\n:1500\r\n+1\r\n*2\r\n:2500\r\n+3\r\n"},
		{[]string{"ts.range", "s1", "-", "+", "AGGREGATION", "avg", "1000", "EMPTY"}, "*2\r\n*2\r\n:1000\r\n+1\r\n*2\r\n:2000\r\n+3\r\n"},
		{[]string{"ts.range", "s1", "0", "5000", "ALIGN", "start", "AGGREGATION", "count", "700"}, "*2\r\n*2\r\n:700\r\n+1\r\n*2\r\n:1400\r\n+1\r\n"},
		{[]string{"ts.range", "s1", "-", "+", "ALIGN", "end", "AGGREGATION", "count", "700"}, "-ERR TSDB: end alignment can only be used with explicit end timestamp\r\n"},
		{[]string{"ts.range", "s1", "-", "+", "AGGREGATION", "std.p", "10000"}, "*1\r\n*2\r\n:0\r\n+1\r\n"},
		{[]string{"ts.range", "s1", "-", "+", "AGGREGATION", "var.s", "10000"}, "*1\r\n*2\r\n:0\r\n+2\r\n"},
		{[]string{"ts.range", "s1", "-", "+", "AGGREGATION", "twa", "10000"}, "*1\r\n*2\r\n:0\r\n+2\r\n"},
		{[]string{"ts.range", "s1", "-", "+", "LATEST"}, "*2\r\n*2\r\n:1000\r\n+1\r\n*2\r\n:2000\r\n+3\r\n"},
		{[]string{"ts.range", "s1", "-", "+", "COUNT", "0"}, "-ERR TSDB: Invalid COUNT value\r\n"},
		{[]string{"ts.range", "s1", "-", "+", "FILTER_BY_TS"}, "-ERR TSDB: FILTER_BY_TS one or more arguments are missing\r\n"},
		{[]string{"ts.range", "s1", "-", "+", "FILTER_BY_VALUE", "x", "1"}, "-ERR TSDB: Couldn't parse MIN\r\n"},
		{[]string{"ts.get", "s1", "LATEST"}, "*2\r\n:2000\r\n+3\r\n"},
		{[]string{"ts.create", "e"}, "+OK\r\n"},
		{[]string{"ts.get", "e"}, "*0\r\n"},
		{[]string{"ts.range", "e", "-", "+"}, "*0\r\n"},
		{[]string{"ts.add", "f", "1", "1.0"}, ":1\r\n"},
		{[]string{"ts.add", "f", "2", "0.1"}, ":2\r\n"},
		{[]string{"ts.add", "f", "3", "1e20"}, ":3\r\n"},
		{[]string{"ts.add", "f", "4", "-3.5e-7"}, ":4\r\n"},
		{[]string{"ts.add", "f", "5", "inf"}, "-ERR TSDB: invalid value\r\n"},
		{[]string{"ts.add", "f", "6", "nan"}, "-ERR TSDB: invalid value\r\n"},
		{[]string{"ts.range", "f", "-", "+"}, "*4\r\n*2\r\n:1\r\n+1\r\n*2\r\n:2\r\n+1E-1\r\n*2\r\n:3\r\n+1E20\r\n*2\r\n:4\r\n+-3.5E-7\r\n"},
		{[]string{"ts.alter", "s1", "DUPLICATE_POLICY", "MAX", "IGNORE", "5", "1"}, "+OK\r\n"},
		{[]string{"ts.alter", "s1", "LABELS"}, "+OK\r\n"},
		{[]string{"flushall"}, ""},
		{[]string{"ts.incrby", "x", "5", "TIMESTAMP", "1000"}, ":1000\r\n"},
		{[]string{"ts.incrby", "x", "7", "TIMESTAMP", "1000"}, ":1000\r\n"},
		{[]string{"ts.incrby", "x", "1", "TIMESTAMP", "500"}, "-TSDB: timestamp must be equal to or higher than the maximum existing timestamp\r\n"},
		{[]string{"ts.decrby", "x", "2", "TIMESTAMP", "2000"}, ":2000\r\n"},
		{[]string{"ts.get", "x"}, "*2\r\n:2000\r\n+10\r\n"},
		{[]string{"ts.incrby", "x", "abc"}, "-ERR TSDB: invalid increase/decrease value\r\n"},
		{[]string{"ts.decrby", "x", "1.5", "TIMESTAMP", "2000"}, ":2000\r\n"},
		{[]string{"ts.range", "x", "-", "+"}, "*2\r\n*2\r\n:1000\r\n+12\r\n*2\r\n:2000\r\n+8.5\r\n"},
		{[]string{"ts.incrby", "v", "1", "TIMESTAMP", "0"}, ":0\r\n"},
		{[]string{"ts.add", "v", "5", "0.5"}, ":5\r\n"},
		{[]string{"ts.get", "v"}, "*2\r\n:5\r\n+5E-1\r\n"},
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
