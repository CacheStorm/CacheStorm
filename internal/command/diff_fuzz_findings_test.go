package command

import (
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestDiffFuzzFindings(t *testing.T) {
	r := newMemAcctRouter()
	RegisterDebugCommands(r)
	RegisterHyperLogLogCommands(r)
	s := store.NewStore()
	run := func(args ...string) string { return memAcctRun(r, s, nil, args...) }
	run("RPUSH", "l", "-3", "3.5", "2")
	run("ZADD", "z", "1", "b", "2", "a", "3", "c")
	run("PFADD", "hll", "a")
	run("APPEND", "hll", "10")
	run("HSET", "h", "f", "v")
	run("SET", "small", "42")
	run("SET", "big", "10000")
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"SORT", "l", "BY", "nosort", "DESC"}, "*3\r\n$1\r\n2\r\n$3\r\n3.5\r\n$2\r\n-3\r\n"},
		{[]string{"SORT", "l", "BY", "nosort"}, "*3\r\n$2\r\n-3\r\n$3\r\n3.5\r\n$1\r\n2\r\n"},
		{[]string{"SORT", "z", "BY", "nosort", "DESC", "LIMIT", "0", "2"}, "*2\r\n$1\r\nc\r\n$1\r\na\r\n"},
		{[]string{"PFMERGE", "hll", "h"}, "-INVALIDOBJ Corrupted HLL object detected\r\n"},
		{[]string{"PFMERGE", "h", "hll"}, "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"},
		{[]string{"OBJECT", "REFCOUNT", "small"}, ":2147483647\r\n"},
		{[]string{"OBJECT", "REFCOUNT", "big"}, ":1\r\n"},
		{[]string{"OBJECT", "REFCOUNT", "l"}, ":1\r\n"},
	} {
		if got := run(c.args...); got != c.want {
			t.Fatalf("%v: got %q, want %q", c.args, got, c.want)
		}
	}
}

func TestSortConvertsSortedSetsAndParsesBeforeTheTypeCheck(t *testing.T) {
	r := newMemAcctRouter()
	RegisterDebugCommands(r)
	s := store.NewStore()
	run := func(args ...string) string { return memAcctRun(r, s, nil, args...) }
	run("ZADD", "z", "1.5", "xyz", "2", "-3")
	run("SET", "s", "x")
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"SORT", "s", "FOO"}, "-ERR syntax error\r\n"},
		{[]string{"SORT", "z", "FOO"}, "-ERR syntax error\r\n"},
		{[]string{"OBJECT", "ENCODING", "z"}, "$8\r\nlistpack\r\n"},
		{[]string{"SORT", "s"}, "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"},
		{[]string{"SORT_RO", "z", "ALPHA"}, "*2\r\n$2\r\n-3\r\n$3\r\nxyz\r\n"},
		{[]string{"OBJECT", "ENCODING", "z"}, "$8\r\nskiplist\r\n"},
	} {
		if got := run(c.args...); got != c.want {
			t.Fatalf("%v: got %q, want %q", c.args, got, c.want)
		}
	}
	// A skiplist's ZREMRANGEBYLEX skips the overlap check, so with mixed
	// scores what it removes depends on the nodes' random levels, as in Redis.
	seen := map[string]bool{}
	for range 200 {
		run("DEL", "z")
		run("ZADD", "z", "1.5", "xyz", "2", "-3")
		run("SORT", "z", "ALPHA")
		seen[run("ZREMRANGEBYLEX", "z", "(b", "(z")] = true
	}
	if len(seen) != 2 || !seen[":0\r\n"] || !seen[":2\r\n"] {
		t.Fatalf("ZREMRANGEBYLEX on a skiplist removed %v, want both :0 and :2", seen)
	}
}

func TestPFADDOnCorruptSparseHLLMatchesRedis(t *testing.T) {
	r := newMemAcctRouter()
	RegisterHyperLogLogCommands(r)
	s := store.NewStore()
	run := func(args ...string) string { return memAcctRun(r, s, nil, args...) }
	bulk := func(v string) string { return "$" + strconv.Itoa(len(v)) + "\r\n" + v + "\r\n" }
	const afterAppend = "HYLL\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x80\\{\x80Dv\x80P\xb1\x84I\x8c\x80Bm\x80BZzz"
	const afterSetRange = "HYLL\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x80\\{\x80D\\Yy\x88E\xa0\x84Y\x1af\x84I\x8c\x80Bm\x80BZzz"
	run("PFADD", "h", "a", "b")
	run("APPEND", "h", "zz")
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"PFADD", "h", "c", "d", "e"}, ":1\r\n"},
		{[]string{"GET", "h"}, bulk(afterAppend)},
		{[]string{"PFCOUNT", "h"}, "-INVALIDOBJ Corrupted HLL object detected\r\n"},
		{[]string{"SETRANGE", "h", "20", `\x7f`}, ":35\r\n"},
		{[]string{"PFADD", "h", "x", "y"}, ":1\r\n"},
		{[]string{"GET", "h"}, bulk(afterSetRange)},
	} {
		if got := run(c.args...); got != c.want {
			t.Fatalf("%v: got %q, want %q", c.args, got, c.want)
		}
	}
}

func TestBITFIELDNotifiesLikeRedis(t *testing.T) {
	r := newMemAcctRouter()
	RegisterStringCommands(r)
	RegisterBitmapCommands(r)
	s := store.NewStore()
	sub := store.NewSubscriber(1)
	s.GetPubSub().PSubscribe(sub, "*")
	defer s.GetPubSub().RemoveSubscriber(sub)
	s.SetNotifyFlags(store.NotifyKeyevent | store.NotifyString)
	run := func(args ...string) string { return memAcctRun(r, s, nil, args...) }
	run("SET", "k", "a")
	_ = hgetdelEvents(sub)
	for _, c := range []struct {
		args   []string
		events int
	}{
		{[]string{"BITFIELD", "k", "SET", "u8", "0", "97"}, 0},
		{[]string{"BITFIELD", "k", "INCRBY", "u8", "0", "0"}, 0},
		{[]string{"BITFIELD", "k", "OVERFLOW", "FAIL", "INCRBY", "u2", "100", "5"}, 0},
		{[]string{"BITFIELD", "k", "INCRBY", "u4", "200", "0"}, 1},
		{[]string{"BITFIELD", "k", "SET", "u8", "0", "98"}, 1},
		{[]string{"BITFIELD", "new", "OVERFLOW", "FAIL", "INCRBY", "u2", "0", "5"}, 0},
	} {
		run(c.args...)
		if got := hgetdelEvents(sub); len(got) != c.events {
			t.Fatalf("%v: events %q, want %d setbit", c.args, got, c.events)
		}
	}
}

func TestSPOPKeepingFewMembersRebuildsTheSet(t *testing.T) {
	r := newMemAcctRouter()
	RegisterDebugCommands(r)
	s := store.NewStore()
	run := func(args ...string) string { return memAcctRun(r, s, nil, args...) }
	sadd := func(key string, n int, prefix string) {
		args := []string{"SADD", key}
		for i := range n {
			args = append(args, prefix+strconv.Itoa(i))
		}
		run(args...)
	}
	sadd("ht", 600, "")
	sadd("is", 12, "")
	sadd("lp", 12, "m")
	for _, c := range []struct {
		key   string
		count string
		enc   string
	}{{"ht", "590", "listpack"}, {"is", "10", "intset"}, {"lp", "10", "listpack"}} {
		run("SPOP", c.key, c.count)
		if got := run("OBJECT", "ENCODING", c.key); got != "$"+strconv.Itoa(len(c.enc))+"\r\n"+c.enc+"\r\n" {
			t.Fatalf("%s after SPOP: encoding %q, want %s", c.key, got, c.enc)
		}
	}
	entry, _ := s.Get("lp")
	set := entry.Value.(*store.SetValue)
	var order []string
	set.Range(func(m string) bool { order = append(order, m); return true })
	if len(order) != 2 {
		t.Fatalf("listpack kept %v, want two members", order)
	}
	first, _ := strconv.Atoi(order[0][1:])
	second, _ := strconv.Atoi(order[1][1:])
	if first >= second {
		t.Fatalf("listpack kept %v, want insertion order", order)
	}
}

func TestACLCommandRulesAndDefaultUserFollowRedis(t *testing.T) {
	r := newMemAcctRouter()
	RegisterServerCommands(r)
	s := store.NewStore()
	run := func(args ...string) string { return memAcctRun(r, s, nil, args...) }
	defer run("ACL", "DELUSER", "acltest")
	for rule, why := range map[string]string{
		"-get|foo":       "Unknown command or category name in ACL",
		"+config|get|x":  "Allowing first-arg of a subcommand is not supported",
		"+config|":       "Syntax error",
		"+config|nosuch": "Unknown command or category name in ACL",
		"-nosuch":        "Unknown command or category name in ACL",
		"+GET":           "",
		"+select|0":      "",
		"-CONFIG|GET":    "",
	} {
		want := "+OK\r\n"
		if why != "" {
			want = "-ERR Error in ACL SETUSER modifier '" + rule + "': " + why + "\r\n"
		}
		if got := run("ACL", "SETUSER", "acltest", rule); got != want {
			t.Fatalf("ACL SETUSER %s = %q, want %q", rule, got, want)
		}
	}
	if got := run("ACL", "DELUSER", "acltest", "default"); got != "-ERR The 'default' user cannot be removed\r\n" {
		t.Fatalf("ACL DELUSER default = %q", got)
	}
	if got := run("ACL", "CAT"); !strings.HasPrefix(got, "*21\r\n$8\r\nkeyspace\r\n$4\r\nread\r\n") {
		t.Fatalf("ACL CAT = %q, want Redis' category order", got)
	}
}

func TestXADDInATransactionSharesItsMillisecond(t *testing.T) {
	r := newMemAcctRouter()
	RegisterStreamCommands(r)
	RegisterTransactionCommands(r)
	RegisterDebugCommands(r)
	s := store.NewStore()
	tx := NewTransaction()
	memAcctRun(r, s, tx, "MULTI")
	memAcctRun(r, s, tx, "XADD", "s", "*", "a", "1")
	memAcctRun(r, s, tx, "DEBUG", "SLEEP", "0.005")
	memAcctRun(r, s, tx, "XADD", "s", "*", "a", "2")
	reply := memAcctRun(r, s, tx, "EXEC")
	ids := strings.Split(reply, "\r\n")
	if len(ids) < 6 {
		t.Fatalf("EXEC = %q", reply)
	}
	first, second := ids[2], ids[5]
	if ms := strings.TrimSuffix(first, "-0"); !strings.HasSuffix(first, "-0") || second != ms+"-1" {
		t.Fatalf("XADD * in one EXEC gave %s and %s, want one millisecond", first, second)
	}
}
