package server

import (
	"context"
	"strings"
	"testing"
)

// Redis 8.8's array type: sparse arrays of strings indexed up to 2^64-2.
func TestArrayCommands(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	expect("<null>", "ARGET", "a", "0")
	expect("0", "ARLEN", "a")
	expect("ERR:ERR invalid array index", "ARGET", "a", "-1")
	expect("ERR:ERR invalid array index", "ARGET", "a", "18446744073709551615")
	expect("ERR:ERR array index overflow", "ARSET", "a", "18446744073709551614", "x", "y")
	expect("3", "ARSET", "a", "0", "x", "y", "z")
	expect("1", "ARSET", "a", "2", "Y", "w")
	expect("array", "TYPE", "a")
	expect("sliced-array", "OBJECT", "ENCODING", "a")
	expect("4", "ARCOUNT", "a")
	expect("4", "ARLEN", "a")
	expect("[x <null> w <null>]", "ARMGET", "a", "0", "9", "3", "18446744073709551614")
	expect("2", "ARMSET", "a", "10", "ten", "0", "X", "1000000", "m")
	expect("1000001", "ARLEN", "a")
	expect("[w Y y]", "ARGETRANGE", "a", "3", "1")
	expect("ERR:ERR range exceeds maximum of 1000000 items", "ARGETRANGE", "a", "0", "1000000")
	expect("[[0 X] [1 y] [2 Y]]", "ARSCAN", "a", "0", "2")
	expect("[[1000000 m] [10 ten]]", "ARSCAN", "a", "18446744073709551614", "4", "LIMIT", "2")
	expect("ERR:ERR LIMIT must be positive", "ARSCAN", "a", "0", "1", "LIMIT", "0")
	expect("ERR:ERR wrong number of arguments for 'arscan' command", "ARSCAN", "a", "0", "1", "LIMIT")
	expect("2", "ARDEL", "a", "1000000", "10", "77")
	expect("0", "ARDEL", "a", "77")
	expect("2", "ARDELRANGE", "a", "3", "2")
	expect("[[0 X] [1 y]]", "ARSCAN", "a", "0", "100")
	expect("2", "ARDELRANGE", "a", "0", "18446744073709551614")
	expect("0", "EXISTS", "a")

	// Indexes are read as Redis' string2ull reads them.
	expect("1", "ARSET", "q", " +007", "v")
	expect("v", "ARGET", "q", "7")
	expect("1", "ARSET", "q", "-02", "top")
	expect("top", "ARGET", "q", "18446744073709551614")
	expect("ERR:ERR invalid array index", "ARGET", "q", "-01")

	// ARGREP
	c.do("ARSET", "g", "0", "apple", "Banana", "cherry", "date", "APPLE pie")
	expect("[0 4]", "ARGREP", "g", "-", "+", "MATCH", "apple", "NOCASE")
	expect("[[4 APPLE pie] [0 apple]]", "ARGREP", "g", "+", "-", "GLOB", "a*", "NOCASE", "WITHVALUES")
	expect("[1 2]", "ARGREP", "g", "0", "3", "RE", "an|err")
	expect("[2]", "ARGREP", "g", "-", "+", "RE", "^c", "AND", "MATCH", "rr")
	expect("[0]", "ARGREP", "g", "-", "+", "EXACT", "apple", "EXACT", "date", "LIMIT", "1")
	expect("ERR:ERR regular expression backreferences are not supported", "ARGREP", "g", "-", "+", "RE", `(a)\1`)
	expect("ERR:ERR invalid regular expression: Missing ']'", "ARGREP", "g", "-", "+", "RE", "[a")
	expect("ERR:ERR regular expression is empty", "ARGREP", "g", "-", "+", "RE", "")
	expect("ERR:ERR syntax error", "ARGREP", "g", "-", "+", "LIMIT", "1")

	// AROP
	c.do("ARSET", "n", "0", "1", "2.5", "x", "-4", "0.1", "7")
	expect("6.6", "AROP", "n", "0", "10", "SUM")
	expect("-4", "AROP", "n", "0", "10", "MIN")
	expect("7", "AROP", "n", "10", "0", "MAX")
	expect("0", "AROP", "n", "0", "10", "AND")
	expect("-1", "AROP", "n", "0", "10", "OR")
	expect("1", "AROP", "n", "0", "10", "MATCH", "x")
	expect("6", "AROP", "n", "0", "10", "USED")
	expect("<null>", "AROP", "missing", "0", "10", "SUM")
	expect("0", "AROP", "missing", "0", "10", "USED")
	expect("ERR:ERR unknown operation", "AROP", "n", "0", "1", "AVG")
	expect("ERR:ERR MATCH requires a value argument", "AROP", "n", "0", "1", "MATCH")

	// ARINSERT, ARNEXT, ARSEEK, ARLASTITEMS, ARRING
	expect("0", "ARNEXT", "r")
	expect("2", "ARINSERT", "r", "a", "b", "c")
	expect("3", "ARNEXT", "r")
	expect("[b c]", "ARLASTITEMS", "r", "2")
	expect("[c b a]", "ARLASTITEMS", "r", "10", "REV")
	expect("[]", "ARLASTITEMS", "r", "0", "bogus")
	expect("ERR:ERR invalid COUNT", "ARLASTITEMS", "r", "x")
	expect("1", "ARSEEK", "r", "10")
	expect("10", "ARINSERT", "r", "d")
	expect("0", "ARSEEK", "nokey", "1")
	expect("1", "ARSEEK", "r", "18446744073709551615")
	expect("<null>", "ARNEXT", "r")
	expect("ERR:ERR insert index overflow", "ARINSERT", "r", "e")
	expect("1", "ARRING", "ring", "3", "a", "b")
	expect("0", "ARRING", "ring", "3", "c", "d")
	expect("[b c d]", "ARLASTITEMS", "ring", "3")
	// Shrinking keeps the newest items, oldest first from index 0.
	expect("0", "ARRING", "ring", "2", "e")
	expect("[e d]", "ARGETRANGE", "ring", "0", "1")
	expect("ERR:ERR size must be positive", "ARRING", "ring", "0", "x")

	got := c.str("ARINFO", "r")
	if !strings.HasPrefix(got, "[count 4 len 11 next-insert-index 0 slices 1 directory-size ") || !strings.HasSuffix(got, " slice-size 4096]") {
		t.Fatalf("ARINFO = %s", got)
	}
	expect("ERR:ERR no such key", "ARINFO", "nokey")
	expect("ERR:ERR syntax error", "ARINFO", "r", "BOGUS")
	c.do("SET", "str", "x")
	expect("ERR:WRONGTYPE Operation against a key holding the wrong kind of value", "ARGET", "str", "0")
	expect("ERR:WRONGTYPE Operation against a key holding the wrong kind of value", "ARSET", "str", "0", "x")
}

// Arrays reach replicas and survive DUMP and RESTORE and an AOF rewrite and
// restart, with their insert cursor.
func TestArrayPersists(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	values := []string{"1", "2.5", "0.1", "short", "a much longer string value", "-17", "1.0", ""}
	args := append([]string{"ARINSERT", "a"}, values...)
	mc.do(args...)
	mc.do("ARSET", "a", "18446744073709551614", "top")
	mc.do("ARMSET", "a", "5000", "x", "70000", "y")
	mc.do("ARSEEK", "a", "100")
	for i := 0; i < 70; i++ {
		mc.do("ARSET", "big", string(rune('0'+i%10))+"0", "v")
	}
	mc.do("ARRING", "ring", "4", "a", "b", "c", "d", "e")
	waitSynced(t, replica, master)
	scan := func(c *replClient, key string) string {
		return c.str("ARSCAN", key, "0", "18446744073709551614") + " " + c.str("ARNEXT", key)
	}
	want := scan(mc, "a")
	if got := scan(rc, "a"); got != want {
		t.Fatalf("replica = %s, master %s", got, want)
	}
	dump := mc.do("DUMP", "a")
	mc.do("RESTORE", "copy", "0", string(dump.Bulk))
	if got := scan(mc, "copy"); got != want {
		t.Fatalf("restored = %s, want %s", got, want)
	}
	mc.do("BGREWRITEAOF")
	waitAOFRewrites(t, master, 1)
	ringWant := mc.str("ARLASTITEMS", "ring", "4")
	stopReplServer(master)
	restarted, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	c := dialRepl(t, restarted)
	for _, key := range []string{"a", "copy"} {
		if got := scan(c, key); got != want {
			t.Fatalf("%s after restart = %s, want %s", key, got, want)
		}
	}
	if got := c.str("ARLASTITEMS", "ring", "4"); got != ringWant {
		t.Fatalf("ring after restart = %s, want %s", got, ringWant)
	}
}
