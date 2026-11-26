package server

import (
	"strings"
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

// Every RediSearch scorer gives Redis 8.2's scores: single terms, unions,
// intersections (with the slop TFIDF and BM25 divide by), negation and
// prefixes, field restrictions as the default dialect 2 reads them.
func TestSearchScorersMatchRedis82X(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	c.do("FT.CREATE", "i1", "ON", "HASH", "PREFIX", "1", "product:", "SCHEMA", "title", "TEXT", "description", "TEXT")
	c.do("HSET", "product:1", "title", "New Gaming Laptop", "description", "this is not a desktop")
	c.do("HSET", "product:2", "title", "Super Old Not Gaming Laptop", "description", "this laptop is not a new laptop but it is a laptop")
	c.do("HSET", "product:3", "title", "Office PC", "description", "office desktop pc")
	c.do("HSET", "product:4", "title", "Gaming chair", "description", "a chair for gaming, not a laptop or office desktop")
	for _, tc := range []struct{ scorer, query, want string }{
		{"BM25STD", "laptop", "3 product:2 0.5706799546335213 product:1 0.4129920691342834 product:4 0.33390848009048474"},
		{"BM25STD", "gaming laptop", "3 product:4 1.2708457141914737 product:1 1.23897620740285 product:2 1.1984278965667527"},
		{"BM25STD", "gaming|desktop", "4 product:4 1.2708457141914737 product:1 1.23897620740285 product:2 0.6277479419332315 product:3 0.3827731366606566"},
		{"BM25STD", "@title:gaming laptop", "3 product:4 1.2708457141914737 product:1 1.23897620740285 product:2 1.1984278965667527"},
		{"BM25STD", "office desktop", "2 product:4 1.6317159656001046 product:3 1.382723175626643"},
		{"BM25STD", "laptop desktop", "2 product:1 0.8259841382685668 product:4 0.6678169601809695"},
		{"BM25STD", "-laptop", "1 product:3 0"},
		{"BM25STD", "gam*", "3 product:4 0.46846861705049453 product:1 0.4129920691342834 product:2 0.31387397096661573"},
		{"BM25", "laptop", "3 product:2 0.4878048681205829 product:1 0.19230768613561391 product:4 0.19230768613561391"},
		{"BM25", "gaming laptop", "3 product:2 0.8724202403918107 product:4 0.8374689590915971 product:1 0.5769230584068418"},
		{"BM25", "gaming|desktop", "4 product:2 0.38461537227122783 product:1 0.2884615292034209 product:4 0.27915631969719906 product:3 0.19230768613561391"},
		{"BM25", "@title:gaming laptop", "3 product:2 0.8724202403918107 product:4 0.8374689590915971 product:1 0.5769230584068418"},
		{"BM25", "office desktop", "2 product:4 0.5769230584068418 product:3 0.5148883226136055"},
		{"BM25", "laptop desktop", "2 product:1 0.38461537227122783 product:4 0.19230768613561391"},
		{"BM25", "-laptop", "1 product:3 0"},
		{"BM25", "gam*", "3 product:4 0.3225806364779916 product:1 0.19230768613561391 product:2 0.19230768613561391"},
		{"TFIDF", "laptop", "3 product:1 1 product:2 1 product:4 0.5"},
		{"TFIDF", "gaming laptop", "3 product:1 3 product:4 2.5 product:2 1.5"},
		{"TFIDF", "gaming|desktop", "4 product:1 1.5 product:4 0.8333333333333334 product:2 0.5 product:3 0.5"},
		{"TFIDF", "@title:gaming laptop", "3 product:1 3 product:4 2.5 product:2 1.5"},
		{"TFIDF", "office desktop", "2 product:3 1.5 product:4 1.5"},
		{"TFIDF", "laptop desktop", "2 product:1 2 product:4 0.5"},
		{"TFIDF", "-laptop", "1 product:3 0"},
		{"TFIDF", "gam*", "3 product:1 1 product:4 1 product:2 0.25"},
		{"TFIDF.DOCNORM", "laptop", "3 product:2 0.5 product:1 0.25 product:4 0.14285714285714285"},
		{"TFIDF.DOCNORM", "gaming laptop", "3 product:1 0.75 product:2 0.75 product:4 0.7142857142857143"},
		{"TFIDF.DOCNORM", "gaming|desktop", "4 product:1 0.375 product:2 0.25 product:4 0.2380952380952381 product:3 0.2"},
		{"TFIDF.DOCNORM", "@title:gaming laptop", "3 product:1 0.75 product:2 0.75 product:4 0.7142857142857143"},
		{"TFIDF.DOCNORM", "office desktop", "2 product:3 0.6 product:4 0.42857142857142855"},
		{"TFIDF.DOCNORM", "laptop desktop", "2 product:1 0.5 product:4 0.14285714285714285"},
		{"TFIDF.DOCNORM", "-laptop", "1 product:3 0"},
		{"TFIDF.DOCNORM", "gam*", "3 product:4 0.2857142857142857 product:1 0.25 product:2 0.125"},
		{"DISMAX", "laptop", "3 product:2 4 product:1 1 product:4 1"},
		{"DISMAX", "gaming laptop", "3 product:2 5 product:4 3 product:1 2"},
		{"DISMAX", "gaming|desktop", "4 product:4 2 product:1 1 product:2 1 product:3 1"},
		{"DISMAX", "@title:gaming laptop", "3 product:2 5 product:4 3 product:1 2"},
		{"DISMAX", "office desktop", "2 product:3 3 product:4 2"},
		{"DISMAX", "laptop desktop", "2 product:1 2 product:4 2"},
		{"DISMAX", "-laptop", "1 product:3 0"},
		{"DISMAX", "gam*", "3 product:4 2 product:1 1 product:2 1"},
		{"DOCSCORE", "laptop", "3 product:1 1 product:2 1 product:4 1"},
		{"DOCSCORE", "gaming laptop", "3 product:1 1 product:2 1 product:4 1"},
		{"DOCSCORE", "gaming|desktop", "4 product:1 1 product:2 1 product:3 1 product:4 1"},
		{"DOCSCORE", "@title:gaming laptop", "3 product:1 1 product:2 1 product:4 1"},
		{"DOCSCORE", "office desktop", "2 product:3 1 product:4 1"},
		{"DOCSCORE", "laptop desktop", "2 product:1 1 product:4 1"},
		{"DOCSCORE", "-laptop", "1 product:3 1"},
		{"DOCSCORE", "gam*", "3 product:1 1 product:2 1 product:4 1"},
		{"BM25STD.NORM", "laptop", "3 product:2 1 product:1 0.7236842047474722 product:4 0.585106375963238"},
		{"BM25STD.NORM", "gaming laptop", "3 product:4 1 product:1 0.9749225996257938 product:2 0.9430160429263484"},
		{"BM25STD.NORM", "gaming|desktop", "4 product:4 1 product:1 0.9749225996257938 product:2 0.49396078133104593 product:3 0.3011955994234762"},
		{"BM25STD.NORM", "@title:gaming laptop", "3 product:4 1 product:1 0.9749225996257938 product:2 0.9430160429263484"},
		{"BM25STD.NORM", "office desktop", "2 product:4 1 product:3 0.8474043306416456"},
		{"BM25STD.NORM", "laptop desktop", "2 product:1 1 product:4 0.8085106350599561"},
		{"BM25STD.NORM", "-laptop", "1 product:3 0"},
		{"BM25STD.NORM", "gam*", "3 product:4 1 product:1 0.8815789448917738 product:2 0.6699999947547914"},
		{"BM25STD.TANH", "laptop", "3 product:2 0.14170980280265388 product:1 0.10288269507123911 product:4 0.08328375750086874"},
		{"BM25STD.TANH", "gaming laptop", "3 product:4 0.30743611920373837 product:1 0.3002042336039616 product:2 0.29095289879538194"},
		{"BM25STD.TANH", "gaming|desktop", "4 product:4 0.30743611920373837 product:1 0.3002042336039616 product:2 0.15566114151432747 product:3 0.09540225579283315"},
		{"BM25STD.TANH", "@title:gaming laptop", "3 product:4 0.30743611920373837 product:1 0.3002042336039616 product:2 0.29095289879538194"},
		{"BM25STD.TANH", "office desktop", "2 product:4 0.38671279627932664 product:3 0.33253950052428927"},
		{"BM25STD.TANH", "laptop desktop", "2 product:1 0.20361020685918874 product:4 0.1654201304957329"},
		{"BM25STD.TANH", "-laptop", "1 product:3 0"},
		{"BM25STD.TANH", "gam*", "3 product:4 0.1165845996396351 product:1 0.10288269507123911 product:2 0.07830783694871975"},
		{"HAMMING", "laptop", "3 product:1 0 product:2 0 product:4 0"},
		{"HAMMING", "gaming laptop", "3 product:1 0 product:2 0 product:4 0"},
		{"HAMMING", "gaming|desktop", "4 product:1 0 product:2 0 product:3 0 product:4 0"},
		{"HAMMING", "@title:gaming laptop", "3 product:1 0 product:2 0 product:4 0"},
		{"HAMMING", "office desktop", "2 product:3 0 product:4 0"},
		{"HAMMING", "laptop desktop", "2 product:1 0 product:4 0"},
		{"HAMMING", "-laptop", "1 product:3 0"},
		{"HAMMING", "gam*", "3 product:1 0 product:2 0 product:4 0"},
	} {
		v := c.do("FT.SEARCH", "i1", tc.query, "WITHSCORES", "NOCONTENT", "SCORER", tc.scorer)
		var got []string
		for _, e := range v.Array {
			got = append(got, str(e))
		}
		if g := strings.Join(got, " "); g != tc.want {
			t.Errorf("%s [%s]\n got %s\nwant %s", tc.scorer, tc.query, g, tc.want)
		}
	}
	if got := c.do("FT.SEARCH", "i1", "laptop", "SCORER", "NOPE"); got.Err != "No such scorer NOPE" {
		t.Errorf("unknown scorer = %s", str(got))
	}
}

// FT.CREATE SKIPINITIALSCAN leaves out the keys that exist, a failed write
// does not bring one in, and FT.INFO lists the fields' flags in
// RediSearch's order.
func TestSearchIndexOptionsMatchRedis82X(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	c.do("HSET", "pre", "foo", "bar")
	c.do("FT.CREATE", "i2", "SKIPINITIALSCAN", "SCHEMA", "foo", "TEXT")
	if got := c.str("FT.SEARCH", "i2", "@foo:bar", "NOCONTENT"); got != "[0]" {
		t.Fatalf("SKIPINITIALSCAN index has %s", got)
	}
	c.do("HSET", "pre", "foo", "bar", "odd")
	c.do("HSET", "post", "foo", "bar")
	if got := c.str("FT.SEARCH", "i2", "@foo:bar", "NOCONTENT"); got != "[1 post]" {
		t.Fatalf("after a failed and a new write: %s", got)
	}
	c.do("FT.CREATE", "i6", "SCHEMA", "a", "TAG", "SORTABLE", "b", "TAG", "CASESENSITIVE", "SORTABLE", "c", "GEO", "SORTABLE",
		"d", "TEXT", "SORTABLE", "UNF", "NOSTEM", "WITHSUFFIXTRIE", "INDEXEMPTY", "INDEXMISSING", "n", "NUMERIC", "SORTABLE")
	info := c.str("FT.INFO", "i6")
	for _, want := range []string{
		"[identifier a attribute a type TAG SEPARATOR , SORTABLE]",
		"[identifier b attribute b type TAG SEPARATOR , CASESENSITIVE SORTABLE UNF]",
		"[identifier c attribute c type GEO SORTABLE]",
		"[identifier d attribute d type TEXT WEIGHT 1 SORTABLE UNF NOSTEM WITHSUFFIXTRIE INDEXEMPTY INDEXMISSING]",
		"[identifier n attribute n type NUMERIC SORTABLE UNF]",
	} {
		if !strings.Contains(info, want) {
			t.Errorf("FT.INFO lacks %s:\n%s", want, info)
		}
	}
	if got := c.str("FT.CONFIG", "GET", "DEFAULT_DIALECT"); got != "[[DEFAULT_DIALECT 2]]" {
		t.Errorf("default dialect %s, Redis 8's is 2", got)
	}
}
