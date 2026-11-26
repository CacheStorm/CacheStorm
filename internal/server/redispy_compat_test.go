package server

import (
	"strings"
	"testing"
	"time"
)

// Replies redis-py's own test suite checks, as Redis gives them.
func TestRedisPySuiteRepliesX(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Errorf("%v = %q, want %q", args, got, want)
		}
	}

	// A legacy path matching several arrays pops from each and answers
	// with the last array's element; non-arrays are skipped.
	c.do("JSON.SET", "doc1", "$", `{"a":["foo"],"nested1":{"a":["hello",null,"world"]},"nested2":{"a":31}}`)
	expect("null", "JSON.ARRPOP", "doc1", "..a", "1")
	expect(`[{"a":[],"nested1":{"a":["hello","world"]},"nested2":{"a":31}}]`, "JSON.GET", "doc1", "$")
	expect(`"world"`, "JSON.ARRPOP", "doc1", "..a")
	c.do("JSON.SET", "d", "$", `{"x":{"a":5},"y":{"a":6}}`)
	expect("ERR:ERR Path does not exist or not an array", "JSON.ARRPOP", "d", "..a")

	// LOLWUT names the Redis version, as Redis 7 does.
	if got := c.str("LOLWUT"); !strings.HasPrefix(got, "Redis ver. ") {
		t.Errorf("LOLWUT = %q", got)
	}

	// ACL LOG records the client's CLIENT INFO line.
	c.str("ACL", "SETUSER", "tl", "on", ">pw", "~*", "-get")
	u := dialRepl(t, s)
	u.str("AUTH", "tl", "pw")
	u.do("GET", "x")
	log := c.str("ACL", "LOG", "1")
	for _, field := range []string{" laddr=", " obl=0 ", " tot-mem=", " user=tl ", " cmd=get "} {
		if !strings.Contains(log, field) {
			t.Errorf("ACL LOG client-info lacks %q: %s", field, log)
		}
	}

	// MONITOR shows the commands a script runs, from "lua".
	m := dialRepl(t, s)
	m.str("MONITOR")
	c.do("EVAL", `return redis.call("GET", "foo")`, "0")
	deadline := time.Now().Add(3 * time.Second)
	seen := ""
	for time.Now().Before(deadline) && !strings.Contains(seen, "[0 lua] \"GET\" \"foo\"") {
		if v, ok := m.next(time.Second); ok {
			seen += str(v) + "\n"
		}
	}
	if !strings.Contains(seen, "[0 lua] \"GET\" \"foo\"") {
		t.Errorf("MONITOR did not show the script's GET:\n%s", seen)
	}

	// FT.INFO has RediSearch's statistics, and counts the index's uses.
	c.do("FT.CREATE", "idx", "SCHEMA", "txt", "TEXT", "v", "VECTOR", "FLAT", "6", "TYPE", "FLOAT32", "DIM", "2", "DISTANCE_METRIC", "L2")
	c.do("HSET", "doc", "txt", "hello world")
	info := c.str("FT.INFO", "idx")
	for _, k := range []string{"inverted_sz_mb", "offset_vectors_sz_mb", "doc_table_size_mb", "key_table_size_mb", "records_per_doc_avg",
		"bytes_per_record_avg", "offsets_per_term_avg", "offset_bits_per_record_avg", "gc_stats", "dialect_stats", "Index Errors", "field statistics", "marked_deleted"} {
		if !strings.Contains(info, k) {
			t.Errorf("FT.INFO lacks %s", k)
		}
	}
	c.do("FT.SEARCH", "idx", "hello")
	if after := c.str("FT.INFO", "idx"); !strings.Contains(after, "number_of_uses 3") || !strings.Contains(after, "dialect_2 1") {
		t.Errorf("FT.INFO after a search: %s", after)
	}
}
