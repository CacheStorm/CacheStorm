package server

import (
	"strconv"
	"strings"
	"testing"
)

// Clients look for Redis 8's bundled modules before using their commands.
func TestBundledModulesReported(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	want := "[[name search ver 80213 path  args []] [name vectorset ver 1 path  args []] [name timeseries ver 80214 path  args []] [name bf ver 80216 path  args []] [name ReJSON ver 80209 path  args []]]"
	if got := c.str("MODULE", "LIST"); got != want {
		t.Fatalf("MODULE LIST:\n%s\nwant\n%s", got, want)
	}
	hello := c.do("HELLO", "2")
	if got := str(hello.Array[len(hello.Array)-1]); got != want {
		t.Fatalf("HELLO modules: %s", got)
	}
	info := str(c.do("INFO", "modules"))
	for _, line := range []string{
		"# Modules\r\nmodule:name=search,ver=80213,api=1,filters=0,usedby=[],using=[ReJSON],options=[handle-io-errors]\r\n",
		"module:name=ReJSON,ver=80209,api=1,filters=0,usedby=[search],using=[],options=[handle-io-errors]\r\n",
		"# search_version\r\nsearch_version:8.2.13\r\n",
		"# search_indexes\r\nsearch_number_of_indexes:0\r\n",
		"# search_runtime_configurations\r\nsearch_extension_load:\r\nsearch_friso_ini:\r\nsearch_default_scorer:BM25STD\r\nsearch_enableGC:ON\r\n",
		"search_max_aggregate_results:2147483648\r\n",
		"search_info_on_zero_indexes:OFF",
	} {
		if !strings.Contains(info, line) {
			t.Fatalf("INFO modules lacks %q:\n%s", line, info)
		}
	}
	if strings.Contains(info, "search_memory") {
		t.Fatal("INFO search shows metrics with no index")
	}
	if all := str(c.do("INFO", "all")); strings.Contains(all, "search_version") || !strings.Contains(all, "module:name=bf,") {
		t.Fatal("INFO all must list the modules but not their sections")
	}

	fields := func() map[string]string {
		out := map[string]string{}
		for _, l := range strings.Split(str(c.do("INFO", "search")), "\r\n") {
			if k, v, ok := strings.Cut(l, ":"); ok {
				out[k] = v
			}
		}
		return out
	}
	c.str("FT.CREATE", "idx", "ON", "HASH", "PREFIX", "1", "d:", "SCHEMA", "t", "TEXT", "SORTABLE", "n", "NUMERIC", "g", "TAG", "CASESENSITIVE",
		"v", "VECTOR", "HNSW", "6", "TYPE", "FLOAT32", "DIM", "2", "DISTANCE_METRIC", "L2")
	// With an index the counters are shown; they are the process's.
	before := fields()
	c.str("HSET", "d:1", "t", "hello", "n", "5", "g", "a")
	c.str("HSET", "d:2", "t", "world", "n", "7")
	c.do("FT.SEARCH", "idx", "hello")
	c.do("FT.AGGREGATE", "idx", "*", "DIALECT", "3")
	c.do("FT.SEARCH", "idx", "(")
	c.do("FT.SEARCH", "idx", "*", "LIMIT", "x")
	c.do("FT.SEARCH", "nosuch", "*")
	after := fields()
	for name, want := range map[string]string{
		"search_number_of_indexes":         "1",
		"search_total_num_docs_in_indexes": "2",
		"search_fields_text":               "Text=1,Sortable=1,IndexErrors=0",
		"search_fields_numeric":            "Numeric=1,IndexErrors=0",
		"search_fields_tag":                "Tag=1,CaseSensitive=1,IndexErrors=0",
		"search_fields_vector":             "Vector=1,HNSW=1,IndexErrors=0",
		"search_dialect_1":                 "1",
		"search_dialect_3":                 "1",
	} {
		if after[name] != want {
			t.Fatalf("%s: %q, want %q", name, after[name], want)
		}
	}
	for name, delta := range map[string]int{
		"search_total_indexing_ops_text_fields":     2,
		"search_total_indexing_ops_tag_fields":      1,
		"search_total_indexing_ops_numeric_fields":  2,
		"search_total_queries_processed":            2,
		"search_total_query_commands":               2,
		"search_coord_total_query_errors_syntax":    1,
		"search_coord_total_query_errors_arguments": 1,
	} {
		b, _ := strconv.Atoi(before[name])
		a, err := strconv.Atoi(after[name])
		if err != nil || a-b != delta {
			t.Fatalf("%s went from %q to %q, want +%d", name, before[name], after[name], delta)
		}
	}
	if _, ok := after["module"]; ok {
		t.Fatal("INFO search must not list the modules")
	}
}
