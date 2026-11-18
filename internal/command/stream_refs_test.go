package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// The replies Redis 8.2 gives for XDELEX, XACKDEL and the reference options
// of XADD and XTRIM, step by step.
func TestStreamReferenceStrategies(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterStreamCommands(r)
	RegisterStringCommands(r)
	do := func(want string, cmd string, args ...string) {
		t.Helper()
		if got := runCmd(t, s, r, cmd, args...); got != want {
			t.Errorf("%s %v\n got %q\nwant %q", cmd, args, got, want)
		}
	}
	fields := func(cmd string, args ...string) []string {
		t.Helper()
		return strings.Fields(runCmd(t, s, r, cmd, args...))
	}
	for i := 1; i <= 5; i++ {
		runCmd(t, s, r, "XADD", "s", fmt.Sprintf("%d-0", i), "f", "v")
	}
	runCmd(t, s, r, "XGROUP", "CREATE", "s", "g1", "0")
	runCmd(t, s, r, "XGROUP", "CREATE", "s", "g2", "0")
	runCmd(t, s, r, "XREADGROUP", "GROUP", "g1", "c1", "COUNT", "3", "STREAMS", "s", ">")
	runCmd(t, s, r, "XREADGROUP", "GROUP", "g2", "c2", "COUNT", "1", "STREAMS", "s", ">")
	runCmd(t, s, r, "XACK", "s", "g1", "1-0")
	runCmd(t, s, r, "XACK", "s", "g2", "1-0")

	// 1-0 is acknowledged by both groups; 2-0 is pending in g1; 4-0 and 9-0
	// are past g1's last delivered ID.
	do(ints(1, 2, 2, 2), "XDELEX", "s", "ACKED", "IDS", "4", "1-0", "2-0", "4-0", "9-0")
	do(ints(1), "XDELEX", "s", "IDS", "1", "2-0")
	if p := fields("XPENDING", "s", "g1"); p[1] != ":2" {
		t.Errorf("KEEPREF must leave 2-0 pending: %v", p)
	}
	do(ints(1), "XDELEX", "s", "DELREF", "IDS", "1", "3-0")
	if p := fields("XPENDING", "s", "g1"); p[1] != ":1" {
		t.Errorf("DELREF must drop 3-0 from g1: %v", p)
	}
	// 2-0 is acknowledged though the entry is gone; 5-0 is not pending.
	do(ints(1, -1), "XACKDEL", "s", "g1", "IDS", "2", "2-0", "5-0")
	runCmd(t, s, r, "XREADGROUP", "GROUP", "g1", "c1", "STREAMS", "s", ">")
	runCmd(t, s, r, "XREADGROUP", "GROUP", "g2", "c2", "STREAMS", "s", ">")
	do(ints(2), "XACKDEL", "s", "g1", "ACKED", "IDS", "1", "4-0")
	do(ints(1), "XACKDEL", "s", "g2", "ACKED", "IDS", "1", "4-0")
	do(ints(1), "XACKDEL", "s", "g1", "IDS", "1", "5-0")
	do(ints(1), "XACKDEL", "s", "g2", "DELREF", "IDS", "1", "5-0")
	do("*0\r\n", "XRANGE", "s", "-", "+")

	do(ints(-1), "XACKDEL", "s", "nogroup", "IDS", "1", "bad")
	do(ints(-1, -1), "XDELEX", "missing", "IDS", "2", "bad", "1-0")
	do(ints(-1, -1), "XDELEX", "s", "IDS", "2", "1-0", "1-0")
	do(ints(-1), "XDELEX", "s", "IDS", "1", "1-0", "ACKED")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"IDS", "2", "1-0"}, "ERR The `numids` parameter must match the number of arguments"},
		{[]string{"IDS", "-1", "1-0"}, "ERR Number of IDs must be a positive integer"},
		{[]string{"IDS", "abc", "1-0"}, "ERR Number of IDs must be a positive integer"},
		{[]string{"IDS", "1", "1-0", "2-0"}, "ERR syntax error"},
		{[]string{"KEEPREF", "ACKED", "IDS", "1", "1-0"}, "ERR syntax error"},
		{[]string{"IDS", "1", "bad"}, "ERR Invalid stream ID specified as stream command argument"},
		{[]string{"ACKED", "FOO", "BAR"}, "ERR syntax error"},
	} {
		do(errReply(tc.want), "XDELEX", append([]string{"s"}, tc.args...)...)
	}
	runCmd(t, s, r, "SET", "str", "x")
	do(errReply("WRONGTYPE Operation against a key holding the wrong kind of value"), "XDELEX", "str", "FOO", "IDS", "1", "1-0")
	do(errReply("WRONGTYPE Operation against a key holding the wrong kind of value"), "XACKDEL", "str", "g", "IDS", "1", "1-0")
}

// XTRIM and XADD skip the entries ACKED keeps and clean up only the
// references of the entries DELREF trims, as Redis 8.2 does.
func TestStreamTrimReferenceStrategies(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterStreamCommands(r)
	do := func(want string, cmd string, args ...string) {
		t.Helper()
		if got := runCmd(t, s, r, cmd, args...); got != want {
			t.Errorf("%s %v\n got %q\nwant %q", cmd, args, got, want)
		}
	}
	for i := 1; i <= 5; i++ {
		runCmd(t, s, r, "XADD", "s", fmt.Sprintf("%d-0", i), "f", "v")
	}
	runCmd(t, s, r, "XGROUP", "CREATE", "s", "g1", "0")
	runCmd(t, s, r, "XREADGROUP", "GROUP", "g1", "c", "COUNT", "2", "STREAMS", "s", ">")
	runCmd(t, s, r, "XACK", "s", "g1", "2-0")
	// 1-0 is pending and 3-0 onwards unread: only 2-0 goes.
	do(":1\r\n", "XTRIM", "s", "MAXLEN", "2", "ACKED")
	do(":2\r\n", "XTRIM", "s", "MAXLEN", "2", "KEEPREF")
	do(":1\r\n", "XTRIM", "s", "MAXLEN", "1", "DELREF")
	if p := strings.Fields(runCmd(t, s, r, "XPENDING", "s", "g1")); p[1] != ":1" || p[3] != "1-0" {
		t.Errorf("DELREF must leave 1-0, deleted earlier, pending: %v", p)
	}
	do("$3\r\n6-0\r\n", "XADD", "s", "DELREF", "MAXLEN", "1", "6-0", "f", "v")
	do(errReply("ERR Invalid stream ID specified as stream command argument"), "XADD", "s", "ACKED", "KEEPREF", "7-0", "f", "v")
	do("$3\r\n7-0\r\n", "XADD", "s", "NOMKSTREAM", "ACKED", "MAXLEN", "=", "1", "7-0", "f", "v")
	do(errReply("ERR syntax error"), "XTRIM", "s", "MAXLEN", "1", "ACKED", "ACKED")
	do(":0\r\n", "XTRIM", "s", "ACKED", "MAXLEN", "~", "1")
	do(":2\r\n", "XLEN", "s")
}
