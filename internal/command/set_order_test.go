package command

import (
	"strings"
	"testing"
)

// TestSetOrderMatchesRedis pins the member order and encoding Redis 7.4
// gives compact sets: an intset sorted by value, a listpack in insertion
// order, and results of set operations built by adding members to an intset.
func TestSetOrderMatchesRedis(t *testing.T) {
	h := newLuaHarness(t)
	RegisterSetCommands(h.router)
	RegisterDebugCommands(h.router)
	list := func(args ...string) string {
		t.Helper()
		var out []string
		for _, line := range strings.Split(h.do(args...), "\r\n") {
			if line != "" && line[0] != '*' && line[0] != '~' && line[0] != '$' {
				out = append(out, line)
			}
		}
		return strings.Join(out, " ")
	}
	h.do("SADD", "s", "c", "a", "b", "10")
	h.do("SADD", "s2", "3", "1", "2")
	h.do("SADD", "s4", "9", "8", "9", "7")
	h.do("SADD", "s5", "x", "3", "1", "q")
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"SMEMBERS", "s"}, "c a b 10"},
		{[]string{"SMEMBERS", "s2"}, "1 2 3"},
		{[]string{"OBJECT", "ENCODING", "s2"}, "intset"},
		{[]string{"SMEMBERS", "s4"}, "7 8 9"},
		{[]string{"SUNION", "s", "s2", "s4"}, "c a b 10 1 2 3 7 8 9"},
		{[]string{"SUNION", "s4", "s2"}, "1 2 3 7 8 9"},
		{[]string{"SINTER", "s5", "s2"}, "1 3"},
		{[]string{"SDIFF", "s5", "s4"}, "x 3 1 q"},
		{[]string{"SRANDMEMBER", "s", "10"}, "c a b 10"},
		{[]string{"SSCAN", "s", "0", "COUNT", "1"}, "0 c a b 10"},
	} {
		if got := list(c.args...); got != c.want {
			t.Errorf("%v: %q, want %q", c.args, got, c.want)
		}
	}
	h.do("SADD", "s2", "x")
	if got := list("SMEMBERS", "s2"); got != "1 2 3 x" {
		t.Errorf("intset turned listpack: %q", got)
	}
	h.do("SREM", "s2", "1")
	h.do("SADD", "s2", "1")
	if got := list("SMEMBERS", "s2"); got != "2 3 x 1" {
		t.Errorf("listpack re-add: %q", got)
	}
	h.do("SINTERSTORE", "d", "s5", "s2")
	if got := list("SMEMBERS", "d"); got != "x 3 1" {
		t.Errorf("SINTERSTORE: %q", got)
	}
	h.do("SADD", "big", strings.Repeat("x", 65))
	if got := list("OBJECT", "ENCODING", "big"); got != "hashtable" {
		t.Errorf("long member: %q", got)
	}
}
