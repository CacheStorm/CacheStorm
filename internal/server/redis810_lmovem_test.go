package server

import (
	"testing"
	"time"
)

// Redis 8.10's LMOVEM and BLMOVEM, with the replies of its t_list.c.
func TestLMOVEM(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	reset := func() {
		c.do("DEL", "s", "d")
		c.do("RPUSH", "s", "a", "b", "c", "d", "e")
		c.do("RPUSH", "d", "x")
	}
	reset()
	expect("[a]", "LMOVEM", "s", "d", "LEFT", "RIGHT")
	expect("[x a]", "LRANGE", "d", "0", "-1")
	for _, tc := range []struct{ from, to, order, reply, dst, src string }{
		{"LEFT", "RIGHT", "BULK", "[a b c]", "[x a b c]", "[d e]"},
		{"LEFT", "RIGHT", "OBO", "[a b c]", "[x a b c]", "[d e]"},
		{"LEFT", "LEFT", "BULK", "[a b c]", "[a b c x]", "[d e]"},
		{"LEFT", "LEFT", "OBO", "[c b a]", "[c b a x]", "[d e]"},
		{"RIGHT", "RIGHT", "BULK", "[c d e]", "[x c d e]", "[a b]"},
		{"RIGHT", "RIGHT", "OBO", "[e d c]", "[x e d c]", "[a b]"},
		{"RIGHT", "LEFT", "BULK", "[c d e]", "[c d e x]", "[a b]"},
		{"RIGHT", "LEFT", "OBO", "[c d e]", "[c d e x]", "[a b]"},
	} {
		reset()
		expect(tc.reply, "LMOVEM", "s", "d", tc.from, tc.to, "COUNT", "3", tc.order)
		expect(tc.dst, "LRANGE", "d", "0", "-1")
		expect(tc.src, "LRANGE", "s", "0", "-1")
	}
	reset()
	expect("[a b c d e]", "LMOVEM", "s", "d", "LEFT", "RIGHT", "COUNT", "10", "BULK")
	expect("0", "EXISTS", "s")
	reset()
	expect("<nullarray>", "LMOVEM", "s", "d", "LEFT", "RIGHT", "EXACTLY", "6", "BULK")
	expect("[a b c d e]", "LRANGE", "s", "0", "-1")
	expect("[a b c d e]", "LMOVEM", "s", "new", "LEFT", "RIGHT", "EXACTLY", "5", "BULK")
	expect("[a b c d e]", "LRANGE", "new", "0", "-1")
	expect("<nullarray>", "LMOVEM", "missing", "d", "LEFT", "RIGHT")
	// The same key rotates in place.
	c.do("DEL", "r")
	c.do("RPUSH", "r", "1", "2", "3", "4")
	expect("[1 2]", "LMOVEM", "r", "r", "LEFT", "RIGHT", "COUNT", "2", "BULK")
	expect("[3 4 1 2]", "LRANGE", "r", "0", "-1")

	c.do("SET", "str", "v")
	reset()
	expect("ERR:WRONGTYPE Operation against a key holding the wrong kind of value", "LMOVEM", "str", "d", "LEFT", "RIGHT")
	expect("ERR:WRONGTYPE Operation against a key holding the wrong kind of value", "LMOVEM", "s", "str", "LEFT", "RIGHT")
	expect("[a b c d e]", "LRANGE", "s", "0", "-1")
	expect("ERR:ERR syntax error", "LMOVEM", "s", "d", "UP", "RIGHT")
	expect("ERR:ERR syntax error", "LMOVEM", "s", "d", "LEFT", "RIGHT", "COUNT", "2")
	expect("ERR:ERR syntax error", "LMOVEM", "s", "d", "LEFT", "RIGHT", "MANY", "2", "OBO")
	expect("ERR:ERR syntax error", "LMOVEM", "s", "d", "LEFT", "RIGHT", "COUNT", "2", "SOME")
	expect("ERR:ERR count should be greater than 0", "LMOVEM", "s", "d", "LEFT", "RIGHT", "COUNT", "0", "OBO")
	expect("ERR:ERR wrong number of arguments for 'lmovem' command", "LMOVEM", "s", "d", "LEFT")
	expect("ERR:ERR timeout is not a float or out of range", "BLMOVEM", "s", "d", "LEFT", "RIGHT", "x")

	// BLMOVEM EXACTLY waits until the source holds enough.
	c.do("DEL", "bs", "bd")
	c.do("RPUSH", "bs", "1")
	done := make(chan string, 1)
	go func() {
		b := dialRepl(t, s)
		done <- b.str("BLMOVEM", "bs", "bd", "LEFT", "RIGHT", "5", "EXACTLY", "3", "BULK")
	}()
	time.Sleep(100 * time.Millisecond)
	c.do("RPUSH", "bs", "2")
	select {
	case got := <-done:
		t.Fatalf("BLMOVEM returned %s with 2 elements", got)
	case <-time.After(100 * time.Millisecond):
	}
	c.do("RPUSH", "bs", "3")
	select {
	case got := <-done:
		if got != "[1 2 3]" {
			t.Fatalf("BLMOVEM = %s", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("BLMOVEM did not wake")
	}
	expect("[1 2 3]", "LRANGE", "bd", "0", "-1")
	expect("<nullarray>", "BLMOVEM", "empty", "bd", "LEFT", "RIGHT", "0.05")
	c.do("MULTI")
	c.do("BLMOVEM", "empty", "bd", "LEFT", "RIGHT", "0")
	expect("[<nullarray>]", "EXEC")
}

// The moves reach replicas as a pop and a push, which any Redis applies.
func TestLMOVEMReplicates(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	mc.do("RPUSH", "s", "a", "b", "c", "d")
	mc.do("RPUSH", "d", "x")
	mc.do("LMOVEM", "s", "d", "RIGHT", "LEFT", "COUNT", "3", "OBO")
	mc.do("LMOVEM", "d", "d", "LEFT", "RIGHT", "COUNT", "2", "BULK")
	waitSynced(t, replica, master)
	for _, k := range []string{"s", "d"} {
		if m, r := mc.str("LRANGE", k, "0", "-1"), rc.str("LRANGE", k, "0", "-1"); m != r {
			t.Fatalf("%s: master %s, replica %s", k, m, r)
		}
	}
}
