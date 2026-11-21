package server

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// Redis 8.4's DIGEST, DELEX, MSETEX and SET's IFEQ, IFNE, IFDEQ and IFDNE,
// with the replies of Redis 8.10.3's t_string.c and db.c.
func TestRedis84StringCommands(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	const helloDigest = "9555e8555c62dcfd"

	expect("<null>", "DIGEST", "nokey")
	expect("OK", "SET", "s", "hello")
	expect(helloDigest, "DIGEST", "s")
	expect("1", "RPUSH", "list", "x")
	expect("ERR:WRONGTYPE Operation against a key holding the wrong kind of value", "DIGEST", "list")
	expect("ERR:ERR wrong number of arguments for 'digest' command", "DIGEST")

	// SET IFEQ / IFNE.
	expect("<null>", "SET", "s", "v2", "IFEQ", "nope")
	expect("hello", "GET", "s")
	expect("OK", "SET", "s", "v2", "IFEQ", "hello")
	expect("<null>", "SET", "s", "v3", "IFNE", "v2")
	expect("OK", "SET", "s", "v3", "IFNE", "other")
	expect("<null>", "SET", "missing", "v", "IFEQ", "x")
	expect("0", "EXISTS", "missing")
	expect("OK", "SET", "fresh", "v", "IFNE", "x")
	expect("v3", "SET", "s", "v4", "IFEQ", "zzz", "GET")
	expect("v3", "GET", "s")
	expect("v3", "SET", "s", "v4", "IFEQ", "v3", "GET")
	expect("ERR:WRONGTYPE Operation against a key holding the wrong kind of value", "SET", "list", "v", "IFEQ", "x")
	expect("ERR:ERR syntax error", "SET", "s", "v", "NX", "IFEQ", "x")
	expect("ERR:ERR syntax error", "SET", "s", "v", "IFEQ", "x", "IFNE", "y")
	expect("ERR:ERR syntax error", "SET", "s", "v", "IFEQ")

	// SET IFDEQ / IFDNE.
	expect("OK", "SET", "s", "hello")
	expect("ERR:ERR must be exactly 16 hexadecimal characters", "SET", "s", "v", "IFDEQ", "abc")
	expect("<null>", "SET", "missing", "v", "IFDEQ", "abc")
	expect("<null>", "SET", "s", "v", "IFDNE", strings.ToUpper(helloDigest))
	expect("OK", "SET", "s", "world", "IFDEQ", strings.ToUpper(helloDigest))
	expect("OK", "SET", "s", "again", "IFDNE", helloDigest)
	expect("OK", "SET", "s", "hello", "IFEQ", "again", "PX", "100000")
	if ttl, _ := strconv.Atoi(c.str("PTTL", "s")); ttl <= 0 || ttl > 100000 {
		t.Fatalf("PTTL after SET IFEQ PX = %d", ttl)
	}

	// DELEX.
	expect("ERR:ERR wrong number of arguments for 'delex' command", "DELEX")
	expect("ERR:ERR wrong number of arguments for 'delex' command", "DELEX", "s", "IFEQ")
	expect("ERR:ERR wrong number of arguments for 'delex' command", "DELEX", "s", "IFEQ", "a", "b")
	expect("0", "DELEX", "nokey", "IFEQ", "x")
	expect("ERR:ERR Key should be of string type if conditions are specified", "DELEX", "list", "IFEQ", "x")
	expect("ERR:ERR Invalid condition. Use IFEQ, IFNE, IFDEQ, or IFDNE", "DELEX", "s", "IFXX", "x")
	expect("ERR:ERR must be exactly 16 hexadecimal characters", "DELEX", "s", "IFDEQ", "x")
	expect("0", "DELEX", "s", "IFEQ", "nope")
	expect("0", "DELEX", "s", "IFNE", "hello")
	expect("0", "DELEX", "s", "IFDNE", helloDigest)
	expect("1", "DELEX", "s", "IFDEQ", helloDigest)
	expect("0", "EXISTS", "s")
	expect("OK", "SET", "s", "a")
	expect("1", "DELEX", "s", "IFNE", "b")
	expect("1", "DELEX", "list")
	expect("0", "DELEX", "list")

	// MSETEX.
	expect("ERR:ERR wrong number of arguments for 'msetex' command", "MSETEX", "1", "k")
	expect("ERR:ERR invalid numkeys value", "MSETEX", "0", "k", "v")
	expect("ERR:ERR invalid numkeys value", "MSETEX", "x", "k", "v")
	expect("ERR:ERR wrong number of key-value pairs", "MSETEX", "2", "k", "v")
	expect("ERR:ERR syntax error", "MSETEX", "1", "k", "v", "GET")
	expect("ERR:ERR syntax error", "MSETEX", "1", "k", "v", "NX", "XX")
	expect("ERR:ERR syntax error", "MSETEX", "1", "k", "v", "EX", "10", "KEEPTTL")
	expect("ERR:ERR invalid expire time in 'msetex' command", "MSETEX", "1", "k", "v", "EX", "0")
	expect("1", "MSETEX", "2", "a", "1", "b", "2", "EX", "100")
	expect("[1 2]", "MGET", "a", "b")
	for _, k := range []string{"a", "b"} {
		if ttl, _ := strconv.Atoi(c.str("TTL", k)); ttl <= 0 || ttl > 100 {
			t.Fatalf("TTL %s = %d", k, ttl)
		}
	}
	expect("0", "MSETEX", "2", "a", "x", "new", "y", "NX")
	expect("0", "EXISTS", "new")
	expect("0", "MSETEX", "2", "a", "x", "new", "y", "XX")
	expect("1", "MSETEX", "2", "a", "x", "b", "y", "XX", "KEEPTTL")
	if ttl, _ := strconv.Atoi(c.str("TTL", "a")); ttl <= 0 {
		t.Fatalf("KEEPTTL lost the TTL: %d", ttl)
	}
	expect("1", "MSETEX", "1", "a", "z")
	expect("-1", "TTL", "a")
	expect("1", "MSETEX", "2", "d", "1", "d", "2", "NX", "PX", "100000")
	expect("2", "GET", "d")
	expect("[a b]", "COMMAND", "GETKEYS", "MSETEX", "2", "a", "1", "b", "2", "EX", "10")
	expect("[s]", "COMMAND", "GETKEYS", "DELEX", "s", "IFEQ", "v")
	expect("1", "MSETEX", "1", "past", "v", "PXAT", strconv.FormatInt(time.Now().Add(-time.Second).UnixMilli(), 10))
	expect("0", "EXISTS", "past")
}

// MSETEX reaches replicas as one SET per pair with an absolute expiry, and
// SET IF* and DELEX as plain SET and DEL, so a replica applies them without
// knowing the Redis 8.4 commands.
func TestRedis84StringCommandsReplicate(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)

	mc.do("MSETEX", "2", "a", "1", "b", "2", "EX", "1000")
	mc.do("SET", "c", "old")
	mc.do("SET", "c", "new", "IFEQ", "old")
	mc.do("SET", "gone", "x")
	mc.do("DELEX", "gone", "IFEQ", "x")
	waitSynced(t, replica, master)
	if got := rc.str("MGET", "a", "b", "c", "gone"); got != "[1 2 new <null>]" {
		t.Fatalf("replica MGET = %s", got)
	}
	for _, k := range []string{"a", "b"} {
		if ttl, _ := strconv.Atoi(rc.str("TTL", k)); ttl <= 0 || ttl > 1000 {
			t.Fatalf("replica TTL %s = %d", k, ttl)
		}
	}
}

// A user confined to some keys cannot reach others through the keys
// MSETEX, LMOVEM or SUNIONCARD name after their first argument.
func TestNewCommandsACLKeys(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	if got := c.str("ACL", "SETUSER", "u", "on", "nopass", "~a*", "+@all"); got != "OK" {
		t.Fatalf("ACL SETUSER: %s", got)
	}
	if got := c.str("AUTH", "u", "x"); got != "OK" {
		t.Fatalf("AUTH: %s", got)
	}
	for _, args := range [][]string{
		{"MSETEX", "2", "a1", "v", "b1", "v", "EX", "10"},
		{"LMOVEM", "a1", "b1", "LEFT", "RIGHT"},
		{"SUNIONCARD", "2", "a1", "b1"},
		{"SDIFFCARD", "2", "a1", "b1"},
	} {
		if got := c.str(args...); !strings.HasPrefix(got, "ERR:NOPERM") {
			t.Fatalf("%v = %s, want NOPERM", args, got)
		}
	}
	if got := c.str("MSETEX", "2", "a1", "v", "a2", "v"); got != "1" {
		t.Fatalf("MSETEX on permitted keys: %s", got)
	}
}
