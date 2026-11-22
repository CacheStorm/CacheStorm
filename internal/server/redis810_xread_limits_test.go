package server

import (
	"strings"
	"testing"
)

// Redis 8.10's MAXCOUNT caps the entries XREAD and XREADGROUP return over
// all streams, and MAXSIZE stops before an entry once the reply holds that
// many bytes, one entry always going out.
func TestXReadMaxCountMaxSize(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	expect("ERR:ERR MAXCOUNT must be a positive integer", "XREAD", "MAXCOUNT", "0", "STREAMS", "s", "0")
	expect("ERR:ERR MAXSIZE must be a positive integer", "XREAD", "MAXSIZE", "-1", "STREAMS", "s", "0")
	expect("ERR:ERR value is not an integer or out of range", "XREAD", "MAXSIZE", "x", "STREAMS", "s", "0")
	expect("ERR:ERR MAXCOUNT must be greater than or equal to COUNT", "XREAD", "COUNT", "5", "MAXCOUNT", "2", "STREAMS", "s", "0")
	for _, k := range []string{"s", "t"} {
		for _, id := range []string{"1-0", "2-0", "3-0"} {
			c.do("XADD", k, id, "f", "v")
		}
	}
	entry := func(id string) string { return "[" + id + " [f v]]" }
	expect("[[s ["+entry("1-0")+" "+entry("2-0")+"]]]", "XREAD", "MAXCOUNT", "2", "STREAMS", "s", "t", "0", "0")
	expect("[[s ["+entry("1-0")+"]] [t ["+entry("1-0")+"]]]", "XREAD", "COUNT", "1", "MAXCOUNT", "2", "STREAMS", "s", "t", "0", "0")
	// Each entry takes 31 bytes, a stream's key and pair header 11, and
	// its entry count 4 once the stream is done.
	expect("[[s ["+entry("1-0")+"]]]", "XREAD", "MAXSIZE", "1", "STREAMS", "s", "0")
	expect("[[s ["+entry("1-0")+"]]]", "XREAD", "MAXSIZE", "42", "STREAMS", "s", "0")
	expect("[[s ["+entry("1-0")+" "+entry("2-0")+"]]]", "XREAD", "MAXSIZE", "43", "STREAMS", "s", "0")
	expect("[[s ["+entry("1-0")+"]]]", "XREAD", "COUNT", "1", "MAXSIZE", "46", "STREAMS", "s", "t", "0", "0")
	// Under the limit when t starts, but over it once t's key is written:
	// t comes back empty, as in Redis.
	expect("[[s ["+entry("1-0")+"]] [t []]]", "XREAD", "COUNT", "1", "MAXSIZE", "47", "STREAMS", "s", "t", "0", "0")

	// An entry MAXSIZE holds back is neither delivered nor pending.
	c.do("XGROUP", "CREATE", "s", "g", "0")
	expect("[[s ["+entry("1-0")+"]]]", "XREADGROUP", "GROUP", "g", "c", "MAXSIZE", "1", "STREAMS", "s", ">")
	expect("[1 1-0 1-0 [[c 1]]]", "XPENDING", "s", "g")
	if got := groupField(t, c, "last-delivered-id"); got != "1-0" {
		t.Fatalf("last-delivered-id = %s", got)
	}
	expect("[[s ["+entry("2-0")+" "+entry("3-0")+"]]]", "XREADGROUP", "GROUP", "g", "c", "MAXCOUNT", "5", "STREAMS", "s", ">")
	expect("[[s ["+entry("1-0")+"]]]", "XREADGROUP", "GROUP", "g", "c", "MAXSIZE", "1", "STREAMS", "s", "0")
	// CLAIM entries count too: idle and delivery count are part of each.
	got := c.str("XREADGROUP", "GROUP", "g", "d", "CLAIM", "0", "MAXCOUNT", "2", "STREAMS", "s", ">")
	// 1-0 was delivered last, by the history read.
	if !strings.HasPrefix(got, "[[s [[2-0 [f v] ") || !strings.Contains(got, " [3-0 [f v] ") || strings.Count(got, "[f v]") != 2 {
		t.Fatalf("XREADGROUP CLAIM MAXCOUNT = %s", got)
	}
	expect("[3 1-0 3-0 [[c 1] [d 2]]]", "XPENDING", "s", "g")

	c.do("HELLO", "3")
	// RESP3 has no pair header: the key alone takes 7 bytes.
	for size, want := range map[string]int{"38": 1, "39": 2} {
		v := c.do("XREAD", "MAXSIZE", size, "STREAMS", "s", "0")
		if len(v.Array) != 2 || len(v.Array[1].Array) != want {
			t.Fatalf("RESP3 XREAD MAXSIZE %s = %+v", size, v)
		}
	}
}
