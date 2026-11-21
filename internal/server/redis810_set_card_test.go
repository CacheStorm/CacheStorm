package server

import (
	"strconv"
	"testing"
)

// Redis 8.10's SUNIONCARD and SDIFFCARD, with the replies of its t_set.c.
func TestSetUnionDiffCard(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	c.do("SADD", "a", "1", "2", "3", "x")
	c.do("SADD", "b", "3", "4", "x", "y")
	c.do("SADD", "c", "5")
	c.do("SET", "str", "v")

	expect("7", "SUNIONCARD", "3", "a", "b", "c")
	expect("7", "SUNIONCARD", "4", "a", "b", "c", "missing")
	expect("3", "SUNIONCARD", "3", "a", "b", "c", "LIMIT", "3")
	expect("7", "SUNIONCARD", "3", "a", "b", "c", "LIMIT", "0")
	expect("7", "SUNIONCARD", "3", "a", "b", "c", "APPROX")
	expect("2", "SUNIONCARD", "3", "a", "b", "c", "APPROX", "LIMIT", "2")
	expect("0", "SUNIONCARD", "1", "missing")
	expect("2", "SDIFFCARD", "2", "a", "b")
	expect("1", "SDIFFCARD", "2", "a", "b", "LIMIT", "1")
	expect("4", "SDIFFCARD", "2", "a", "missing")
	expect("0", "SDIFFCARD", "2", "missing", "a")
	expect("0", "SDIFFCARD", "2", "a", "a")
	expect("4", "SDIFFCARD", "1", "a")

	expect("ERR:ERR numkeys should be greater than 0", "SUNIONCARD", "0", "a")
	expect("ERR:ERR numkeys should be greater than 0", "SDIFFCARD", "x", "a")
	expect("ERR:ERR Number of keys can't be greater than number of args", "SUNIONCARD", "3", "a", "b")
	expect("ERR:ERR LIMIT can't be negative", "SUNIONCARD", "1", "a", "LIMIT", "-1")
	expect("ERR:ERR LIMIT can't be negative", "SDIFFCARD", "1", "a", "LIMIT", "x")
	expect("ERR:ERR syntax error", "SDIFFCARD", "1", "a", "APPROX")
	expect("ERR:ERR syntax error", "SUNIONCARD", "1", "a", "LIMIT")
	expect("ERR:WRONGTYPE Operation against a key holding the wrong kind of value", "SUNIONCARD", "2", "a", "str")
	expect("ERR:WRONGTYPE Operation against a key holding the wrong kind of value", "SDIFFCARD", "2", "missing", "str")
	expect("ERR:ERR wrong number of arguments for 'sunioncard' command", "SUNIONCARD", "1")

	// APPROX is the estimate PFCOUNT gives for the union's members.
	args := []string{"SADD", "big1"}
	pf := []string{"PFADD", "hll"}
	for i := 0; i < 3000; i++ {
		m := "m" + strconv.Itoa(i)
		args = append(args, m)
		pf = append(pf, m)
	}
	c.do(args...)
	c.do("SADD", "big2", "m1", "m2", "extra")
	pf = append(pf, "extra")
	c.do(pf...)
	expect(c.str("PFCOUNT", "hll"), "SUNIONCARD", "2", "big1", "big2", "APPROX")
	expect("3001", "SUNIONCARD", "2", "big1", "big2")
	expect("[big1 big2]", "COMMAND", "GETKEYS", "SUNIONCARD", "2", "big1", "big2", "APPROX")
}
