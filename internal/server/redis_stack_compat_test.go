package server

import (
	"testing"
)

// Replies of the Redis Stack commands that go-redis v9.7.3's test suite
// checks, as Redis 8.2 gives them.
func TestRedisStackRepliesMatchRedis82X(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Errorf("%v = %s, want %s", args, got, want)
		}
	}

	// LATEST adds a compaction's open bucket.
	c.do("TS.CREATE", "s")
	c.do("TS.CREATE", "d", "LABELS", "x", "1")
	c.do("TS.CREATERULE", "s", "d", "AGGREGATION", "sum", "10")
	for _, sample := range [][2]string{{"1", "1"}, {"2", "3"}, {"11", "7"}, {"13", "1"}} {
		c.do("TS.ADD", "s", sample[0], sample[1])
	}
	expect("[0 4]", "TS.GET", "d")
	expect("[10 8]", "TS.GET", "d", "LATEST")
	expect("[13 1]", "TS.GET", "s", "LATEST")
	expect("[[0 4] [10 8]]", "TS.RANGE", "d", "-", "+", "LATEST")
	expect("[[10 8] [0 4]]", "TS.REVRANGE", "d", "-", "+", "LATEST")
	expect("[[0 4]]", "TS.REVRANGE", "d", "0", "9", "LATEST")
	expect("[[d [] [10 8]]]", "TS.MGET", "LATEST", "FILTER", "x=1")
	expect("ERR:ERR TSDB: wrong 3rd argument", "TS.GET", "d", "foo")

	// A bracket after a dot is the bracket alone.
	c.do("JSON.SET", "j", "$", `[{"nested":[{"ciao":[1]},{"ciaoc":[2]},{"ciao":[3]}]}]`)
	expect(`[[1],[3]]`, "JSON.GET", "j", `$.[0]["nested"]..ciao`)
	expect("2", "JSON.DEL", "j", `$.[0]["nested"]..ciao`)

	// A reducer's generated name joins its arguments with commas.
	c.do("FT.CREATE", "ia", "SCHEMA", "parent", "TEXT", "n", "NUMERIC")
	for i, n := range []string{"10", "3", "8"} {
		c.do("HSET", "doc"+string(rune('a'+i)), "parent", "redis", "n", n)
	}
	expect("[1 [parent redis __generated_aliasquantilen,0.5 8]]", "FT.AGGREGATE", "ia", "redis", "GROUPBY", "1", "@parent", "REDUCE", "QUANTILE", "2", "@n", "0.5")
}
