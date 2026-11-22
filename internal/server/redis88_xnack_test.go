package server

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func xnackFixture(c *replClient) {
	for _, id := range []string{"1-0", "2-0", "3-0", "4-0"} {
		c.do("XADD", "s", id, "f", "v"+id[:1])
	}
	c.do("XGROUP", "CREATE", "s", "g", "0")
	c.do("XREADGROUP", "GROUP", "g", "c1", "COUNT", "3", "STREAMS", "s", ">")
	c.do("XNACK", "s", "g", "SILENT", "IDS", "1", "2-0")
	c.do("XNACK", "s", "g", "FATAL", "IDS", "2", "1-0", "9-0")
	c.do("XNACK", "s", "g", "FAIL", "RETRYCOUNT", "5", "FORCE", "IDS", "2", "4-0", "9-0")
}

const xnackPending = "[[1-0  -1 9223372036854775807] [2-0  -1 0] [3-0 c1 _ 1] [4-0  -1 5]]"

// Redis 8.8's XNACK releases pending entries: they lose their consumer and
// go to the front of the PEL's time order, for XREADGROUP CLAIM to take
// first.
func TestXNack(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	expect("ERR:NOGROUP No such key 's' or consumer group 'g'", "XNACK", "s", "g", "FAIL", "IDS", "1", "1-0")
	for _, id := range []string{"1-0", "2-0", "3-0", "4-0"} {
		c.do("XADD", "s", id, "f", "v"+id[:1])
	}
	c.do("XGROUP", "CREATE", "s", "g", "0")
	c.do("XREADGROUP", "GROUP", "g", "c1", "COUNT", "3", "STREAMS", "s", ">")
	expect("ERR:NOGROUP No such key 's' or consumer group 'x'", "XNACK", "s", "x", "FAIL", "IDS", "1", "1-0")
	expect("ERR:ERR mode must be SILENT, FAIL, or FATAL", "XNACK", "s", "g", "BAD", "IDS", "1", "1-0")
	expect("ERR:ERR numids must be a positive integer", "XNACK", "s", "g", "FAIL", "IDS", "0", "1-0")
	expect("ERR:ERR numids must be a positive integer", "XNACK", "s", "g", "FAIL", "IDS", "x", "1-0")
	expect("ERR:ERR number of IDs doesn't match numids", "XNACK", "s", "g", "FAIL", "IDS", "2", "1-0")
	expect("ERR:ERR Invalid RETRYCOUNT value, must be >= 0", "XNACK", "s", "g", "FAIL", "RETRYCOUNT", "-1", "IDS", "1", "1-0")
	expect("ERR:ERR value is not an integer or out of range", "XNACK", "s", "g", "FAIL", "RETRYCOUNT", "x", "IDS", "1", "1-0")
	expect("ERR:ERR Unrecognized XNACK option 'BOGUS'", "XNACK", "s", "g", "FAIL", "IDS", "1", "1-0", "BOGUS")
	expect("ERR:ERR syntax error, expected IDS keyword", "XNACK", "s", "g", "FAIL", "FORCE", "FORCE", "FORCE")
	expect("ERR:ERR Invalid stream ID specified as stream command argument", "XNACK", "s", "g", "FAIL", "IDS", "1", "bad")
	expect("ERR:ERR wrong number of arguments for 'xnack' command", "XNACK", "s", "g", "FAIL", "IDS", "1")

	expect("1", "XNACK", "s", "g", "SILENT", "IDS", "1", "2-0")
	expect("1", "XNACK", "s", "g", "FATAL", "IDS", "2", "1-0", "9-0")
	expect("1", "XNACK", "s", "g", "FAIL", "RETRYCOUNT", "5", "FORCE", "IDS", "2", "4-0", "9-0")

	expect("[4 1-0 4-0 [[c1 1]]]", "XPENDING", "s", "g")
	if got := stripIdle(c.str("XPENDING", "s", "g", "-", "+", "10")); got != stripIdle(xnackPending) {
		t.Fatalf("XPENDING = %s", got)
	}
	// Released entries have no delivery time, so IDLE never filters them.
	expect("[[1-0  -1 9223372036854775807] [2-0  -1 0] [4-0  -1 5]]", "XPENDING", "s", "g", "IDLE", "100000", "-", "+", "10")
	if got := c.str("XPENDING", "s", "g", "-", "+", "10", "c1"); !strings.HasPrefix(got, "[[3-0 c1 ") {
		t.Fatalf("XPENDING c1 = %s", got)
	}
	if got := c.str("XINFO", "CONSUMERS", "s", "g"); !strings.Contains(got, "pending 1 ") {
		t.Fatalf("XINFO CONSUMERS = %s", got)
	}
	full := c.str("XINFO", "STREAM", "s", "FULL")
	if !strings.Contains(full, "pel-count 4 nacked-count 3 pending [[1-0  0 9223372036854775807] [2-0  0 0] [3-0 c1 ") {
		t.Fatalf("XINFO STREAM FULL = %s", full)
	}

	// CLAIM takes the released entries first, in the order XNACK released
	// them, whatever the minimum idle time; a delivery count at the
	// maximum stays there.
	got := c.str("XREADGROUP", "GROUP", "g", "c2", "COUNT", "2", "CLAIM", "100000", "STREAMS", "s", ">")
	if !strings.HasPrefix(got, "[[s [[2-0 [f v2] ") || !strings.Contains(got, " 0] [1-0 [f v1] ") || !strings.HasSuffix(got, " 9223372036854775807]]]]") {
		t.Fatalf("XREADGROUP CLAIM = %s", got)
	}
	if got := stripIdle(c.str("XPENDING", "s", "g", "1-0", "2-0", "10")); got != stripIdle("[[1-0 c2 0 9223372036854775807] [2-0 c2 0 1]]") {
		t.Fatalf("XPENDING after CLAIM = %s", got)
	}
	// XAUTOCLAIM and XCLAIM ignore the minimum idle time for released
	// entries too.
	expect("[0-0 [4-0] []]", "XAUTOCLAIM", "s", "g", "c3", "100000", "0", "JUSTID")
	expect("1", "XNACK", "s", "g", "FAIL", "IDS", "1", "3-0")
	expect("[3-0]", "XCLAIM", "s", "g", "c4", "100000", "3-0", "JUSTID")
	expect("[4 1-0 4-0 [[c2 2] [c3 1] [c4 1]]]", "XPENDING", "s", "g")
	expect("0", "XNACK", "s", "g", "FAIL", "IDS", "1", "7-0")
}

// Released entries reach replicas, survive an AOF rewrite and restart, and
// DUMP and RESTORE keep them and their order.
func TestXNackPersists(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	xnackFixture(mc)
	waitSynced(t, replica, master)
	if got := stripIdle(rc.str("XPENDING", "s", "g", "-", "+", "10")); got != stripIdle(xnackPending) {
		t.Fatalf("replica XPENDING = %s", got)
	}

	dump := mc.do("DUMP", "s")
	mc.do("RESTORE", "copy", "0", string(dump.Bulk))
	if got := stripIdle(mc.str("XPENDING", "copy", "g", "-", "+", "10")); got != stripIdle(xnackPending) {
		t.Fatalf("restored XPENDING = %s", got)
	}

	mc.do("BGREWRITEAOF")
	waitAOFRewrites(t, master, 1)
	stopReplServer(master)
	restarted, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	c := dialRepl(t, restarted)
	for _, key := range []string{"s", "copy"} {
		if got := stripIdle(c.str("XPENDING", key, "g", "-", "+", "10")); got != stripIdle(xnackPending) {
			t.Fatalf("%s XPENDING after restart = %s", key, got)
		}
		got := c.str("XREADGROUP", "GROUP", "g", "c9", "COUNT", "3", "CLAIM", "100000", "STREAMS", key, ">")
		var ids []string
		for _, m := range claimedIDs.FindAllStringSubmatch(got, -1) {
			ids = append(ids, m[1])
		}
		if strings.Join(ids, ",") != "2-0,1-0,4-0" {
			t.Fatalf("%s CLAIM order after restart = %s", key, got)
		}
	}
}

var claimedIDs = regexp.MustCompile(`\[(\d+-\d+) \[f `)

// XCLAIM and XAUTOCLAIM reach replicas and the AOF as XCLAIMs with the
// delivery time and count spelled out, as Redis sends them, so a minimum
// idle time is not judged again when the AOF is loaded later.
func TestXClaimPropagatesEffects(t *testing.T) {
	dir := t.TempDir()
	cfg := aofRewriteConfig(t, dir, nil)
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	for _, id := range []string{"1-0", "2-0", "3-0", "4-0"} {
		mc.do("XADD", "s", id, "f", "v")
	}
	mc.do("XGROUP", "CREATE", "s", "g", "0")
	mc.do("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", ">")
	mc.do("XNACK", "s", "g", "FAIL", "IDS", "1", "4-0")
	mc.do("XDEL", "s", "3-0", "4-0")
	// Too fresh to claim now; a later AOF load must not claim them either.
	mc.do("XAUTOCLAIM", "s", "g", "late", "3600000", "0")
	mc.do("XCLAIM", "s", "g", "late2", "3600000", "1-0")
	mc.do("XAUTOCLAIM", "s", "g", "c2", "0", "0", "COUNT", "1")
	mc.do("XCLAIM", "s", "g", "c3", "0", "2-0", "3-0", "4-0", "RETRYCOUNT", "7")
	mc.do("XCLAIM", "s", "g", "c4", "0", "9-0", "LASTID", "8-0")
	waitSynced(t, replica, master)
	want := "[[1-0 c2 _ 2] [2-0 c3 _ 7]]"
	for name, c := range map[string]*replClient{"master": mc, "replica": rc} {
		if got := stripIdle(c.str("XPENDING", "s", "g", "-", "+", "10")); got != stripIdle(want) {
			t.Fatalf("%s XPENDING = %s", name, got)
		}
		if got := groupField(t, c, "last-delivered-id"); got != "8-0" {
			t.Fatalf("%s last-delivered-id = %s", name, got)
		}
		if got := groupField(t, c, "consumers"); got != "6" {
			t.Fatalf("%s consumers = %s", name, got)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "XAUTOCLAIM") || strings.Contains(string(data), "3600000") {
		t.Fatalf("AOF holds the claim commands verbatim:\n%q", data)
	}
}
