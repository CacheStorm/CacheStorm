package server

import (
	"context"
	"strings"
	"testing"
)

func streamInfoField(t *testing.T, c *replClient, key, field string) string {
	t.Helper()
	v := c.do("XINFO", "STREAM", key)
	for i := 0; i+1 < len(v.Array); i += 2 {
		if string(v.Array[i].Bulk) == field {
			return str(v.Array[i+1])
		}
	}
	t.Fatalf("XINFO STREAM %s has no %s: %s", key, field, str(v))
	return ""
}

func idmpCounts(t *testing.T, c *replClient, key string) string {
	t.Helper()
	var out []string
	for _, f := range []string{"idmp-duration", "idmp-maxsize", "pids-tracked", "iids-tracked", "iids-added", "iids-duplicates"} {
		out = append(out, streamInfoField(t, c, key, f))
	}
	return strings.Join(out, " ")
}

// Redis 8.6's XADD IDMP and IDMPAUTO add an entry once per producer and
// idempotent ID; XCFGSET and XIDMPRECORD tune and fill that memory.
func TestXAddIDMP(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	expect("ERR:ERR syntax error, IDMP/IDMPAUTO specified multiple times", "XADD", "s", "IDMP", "p", "1", "IDMPAUTO", "p", "*", "f", "v")
	expect("ERR:ERR syntax error, IDMP requires a non-empty producer ID", "XADD", "s", "IDMP", "", "1", "*", "f", "v")
	expect("ERR:ERR syntax error, IDMP requires a non-empty idempotent ID", "XADD", "s", "IDMP", "p", "", "*", "f", "v")
	expect("ERR:ERR syntax error, IDMPAUTO requires a non-empty producer ID", "XADD", "s", "IDMPAUTO", "", "*", "f", "v")
	expect("ERR:ERR syntax error, IDMP/IDMPAUTO can be used only with auto-generated IDs", "XADD", "s", "IDMP", "p", "1", "5-0", "f", "v")
	expect("<null>", "XADD", "s", "NOMKSTREAM", "IDMP", "p", "1", "*", "f", "v")

	first := c.str("XADD", "s", "IDMP", "p", "1", "*", "f", "v")
	expect(first, "XADD", "s", "IDMP", "p", "1", "*", "other", "fields")
	other := c.str("XADD", "s", "IDMP", "q", "1", "*", "f", "v")
	if other == first {
		t.Fatalf("another producer's iid 1 was taken for a duplicate")
	}
	auto := c.str("XADD", "s", "IDMPAUTO", "p", "*", "a", "1", "b", "2")
	expect(auto, "XADD", "s", "IDMPAUTO", "p", "*", "b", "2", "a", "1")
	expect("3", "XLEN", "s")
	if got := idmpCounts(t, c, "s"); got != "100 100 2 3 3 2" {
		t.Fatalf("IDMP fields = %s", got)
	}

	// IDMP-MAXSIZE forgets a producer's oldest iid; changing it forgets
	// them all, setting the same value forgets nothing.
	expect("ERR:ERR At least one parameter must be specified", "XCFGSET", "s")
	expect("ERR:ERR IDMP-DURATION must be between 1 and 86400 seconds", "XCFGSET", "s", "IDMP-DURATION", "0")
	expect("ERR:ERR IDMP-MAXSIZE must be between 1 and 10000 entries", "XCFGSET", "s", "IDMP-MAXSIZE", "10001")
	expect("ERR:ERR IDMP-MAXSIZE specified multiple times", "XCFGSET", "s", "IDMP-MAXSIZE", "1", "IDMP-MAXSIZE", "1")
	expect("ERR:ERR syntax error", "XCFGSET", "s", "BOGUS", "1")
	expect("ERR:ERR no such key", "XCFGSET", "missing", "IDMP-MAXSIZE", "1")
	expect("OK", "XCFGSET", "s", "IDMP-DURATION", "100")
	if got := idmpCounts(t, c, "s"); got != "100 100 2 3 3 2" {
		t.Fatalf("IDMP fields after XCFGSET to the same value = %s", got)
	}
	expect("OK", "XCFGSET", "s", "IDMP-MAXSIZE", "2")
	if got := idmpCounts(t, c, "s"); got != "100 2 0 0 3 2" {
		t.Fatalf("IDMP fields after XCFGSET = %s", got)
	}
	c.do("XADD", "s", "IDMP", "p", "x", "*", "f", "v")
	c.do("XADD", "s", "IDMP", "p", "y", "*", "f", "v")
	z := c.str("XADD", "s", "IDMP", "p", "z", "*", "f", "v")
	expect(z, "XADD", "s", "IDMP", "p", "z", "*", "f", "v")
	if got := c.str("XADD", "s", "IDMP", "p", "x", "*", "f", "v"); got == z || strings.HasPrefix(got, "ERR") {
		t.Fatalf("evicted iid x was still remembered: %s", got)
	}

	// XIDMPRECORD remembers an existing entry; an iid whose entry is older
	// than IDMP-DURATION is forgotten.
	expect("ERR:ERR producer ID must be non-empty", "XIDMPRECORD", "s", "", "i", "1-0")
	expect("ERR:ERR idempotent ID must be non-empty", "XIDMPRECORD", "s", "p", "", "1-0")
	expect("ERR:ERR no such key", "XIDMPRECORD", "missing", "p", "i", "1-0")
	expect("ERR:ERR No such message in stream", "XIDMPRECORD", "s", "p", "i", "1-0")
	expect("ERR:ERR Invalid stream ID specified as stream command argument", "XIDMPRECORD", "s", "p", "i", "bad")
	expect("OK", "XIDMPRECORD", "s", "r", "i", first)
	expect("OK", "XIDMPRECORD", "s", "r", "i", first)
	expect("ERR:ERR IID already exists for this producer with a different stream ID", "XIDMPRECORD", "s", "r", "i", other)
	expect(first, "XADD", "s", "IDMP", "r", "i", "*", "f", "v")
	c.do("XADD", "old", "1-0", "f", "v")
	expect("OK", "XIDMPRECORD", "old", "p", "i", "1-0")
	if got := streamInfoField(t, c, "old", "iids-tracked"); got != "0" {
		t.Fatalf("an expired iid is still tracked: %s", got)
	}

	// New streams start with the stream-idmp-* configs.
	expect("ERR:ERR CONFIG SET failed (possibly related to argument 'stream-idmp-maxsize') - argument must be between 1 and 10000 inclusive", "CONFIG", "SET", "stream-idmp-maxsize", "0")
	expect("OK", "CONFIG", "SET", "stream-idmp-duration", "50", "stream-idmp-maxsize", "7")
	c.do("XADD", "fresh", "*", "f", "v")
	if got := idmpCounts(t, c, "fresh"); got != "50 7 0 0 0 0" {
		t.Fatalf("new stream's IDMP fields = %s", got)
	}
	if got := streamInfoField(t, c, "s", "idmp-duration"); got != "100" {
		t.Fatalf("an existing stream took the new default: %s", got)
	}
	expect("OK", "CONFIG", "SET", "stream-idmp-duration", "100", "stream-idmp-maxsize", "100")
}

// IDMP state reaches replicas, a duplicate going out with the entry's ID,
// and survives DUMP and RESTORE and an AOF rewrite and restart.
func TestXAddIDMPPersists(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	first := mc.str("XADD", "s", "IDMP", "p", "1", "*", "f", "v")
	mc.do("XADD", "s", "IDMP", "p", "1", "*", "f", "v")
	mc.do("XADD", "s", "IDMPAUTO", "q", "*", "a", "b")
	mc.do("XADD", "s", "IDMPAUTO", "q", "*", "a", "b")
	mc.do("XCFGSET", "s", "IDMP-DURATION", "600")
	mc.do("XADD", "s", "IDMP", "p", "2", "*", "f", "v")
	mc.do("XIDMPRECORD", "s", "p", "1", first)
	waitSynced(t, replica, master)
	want := idmpCounts(t, mc, "s")
	if want != "600 100 1 2 4 2" {
		t.Fatalf("master IDMP fields = %s", want)
	}
	if got := idmpCounts(t, rc, "s"); got != want {
		t.Fatalf("replica IDMP fields = %s, master %s", got, want)
	}
	if got := rc.str("XLEN", "s"); got != "3" {
		t.Fatalf("replica XLEN = %s", got)
	}

	dump := mc.do("DUMP", "s")
	mc.do("RESTORE", "copy", "0", string(dump.Bulk))
	if got := idmpCounts(t, mc, "copy"); got != want {
		t.Fatalf("restored IDMP fields = %s, want %s", got, want)
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
		// The rewritten AOF re-records each iid, so only the remembered
		// iids and the settings are expected back.
		got := idmpCounts(t, c, key)
		if !strings.HasPrefix(got, "600 100 1 2 ") {
			t.Fatalf("%s IDMP fields after restart = %s", key, got)
		}
		if got := c.str("XADD", key, "IDMP", "p", "1", "*", "f", "v"); got != first {
			t.Fatalf("%s forgot iid 1 after restart: %s", key, got)
		}
	}
}
