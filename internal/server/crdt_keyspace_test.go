package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The CRDT types are keys: registers, counters and sets survive a restart
// through an AOF holding only hash and set commands, and an OR-set keeps its
// tombstones, so a removed tag stays removed.
func TestCRDTsAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	ok := func(args ...string) {
		t.Helper()
		if v := c.do(args...); v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
	}
	ok("CRDT.LWW.SET", "reg", "old", "100")
	ok("CRDT.LWW.SET", "reg", "new", "200")
	ok("CRDT.LWW.SET", "reg", "stale", "150")
	ok("CRDT.GCOUNTER.INCR", "gc", "n1", "5")
	ok("CRDT.GCOUNTER.INCR", "gc", "n2")
	ok("CRDT.PNCounter.INCR", "pn", "n1")
	ok("CRDT.PNCounter.INCR", "pn", "n1")
	ok("CRDT.PNCounter.DECR", "pn", "n2")
	ok("CRDT.GSET.ADD", "gs", "b")
	ok("CRDT.GSET.ADD", "gs", "a")
	ok("CRDT.ORSET.ADD", "os", "x", "t1")
	ok("CRDT.ORSET.ADD", "os", "x", "t2")
	ok("CRDT.ORSET.ADD", "os", "y")
	ok("CRDT.ORSET.REMOVE", "os", "x", "t1")
	ok("SET", "plain", "v")
	for _, args := range [][]string{
		{"CRDT.LWW.SET", "plain", "v"}, {"CRDT.GCOUNTER.INCR", "reg", "n"}, {"CRDT.PNCounter.GET", "gc"},
		{"CRDT.GSET.ADD", "reg", "v"}, {"CRDT.ORSET.ADD", "gc", "v"}, {"CRDT.ORSET.GET", "plain"},
	} {
		if got := c.do(args...); got.Type != '-' || !strings.HasPrefix(got.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, got)
		}
	}
	for _, args := range [][]string{{"CRDT.GCOUNTER.INCR", "gc", "n1", "-1"}, {"CRDT.LWW.SET", "reg", "v", "x"}} {
		if got := c.do(args...); got.Type != '-' {
			t.Fatalf("%v accepted: %v", args, got)
		}
	}
	stopReplServer(s)

	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "CRDT.") {
		t.Fatal("the AOF records CRDT commands")
	}

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	if got := c.do("CRDT.LWW.GET", "reg"); string(got.Bulk) != "new" {
		t.Fatalf("CRDT.LWW.GET after restart: %+v", got)
	}
	ok("CRDT.LWW.SET", "reg", "older", "199")
	if got := c.do("CRDT.LWW.GET", "reg"); string(got.Bulk) != "new" {
		t.Fatalf("an older write replaced the register after restart: %+v", got)
	}
	if got := c.do("CRDT.GCOUNTER.GET", "gc"); got.Int != 6 {
		t.Fatalf("CRDT.GCOUNTER.GET after restart: %v", got)
	}
	if got := c.do("CRDT.PNCounter.GET", "pn"); got.Int != 1 {
		t.Fatalf("CRDT.PNCounter.GET after restart: %v", got)
	}
	if got := c.do("CRDT.GSET.GET", "gs"); len(got.Array) != 2 || string(got.Array[0].Bulk) != "a" {
		t.Fatalf("CRDT.GSET.GET after restart: %+v", got)
	}
	if got := c.do("CRDT.ORSET.TAGS", "os", "x"); len(got.Array) != 1 || string(got.Array[0].Bulk) != "t2" {
		t.Fatalf("CRDT.ORSET.TAGS after restart: %+v", got)
	}
	ok("CRDT.ORSET.ADD", "os", "x", "t1")
	if got := c.do("CRDT.ORSET.TAGS", "os", "x"); len(got.Array) != 1 {
		t.Fatalf("a removed tag came back after restart: %+v", got)
	}
	if got := c.do("CRDT.ORSET.REMOVE", "os", "x"); got.Int != 1 {
		t.Fatalf("CRDT.ORSET.REMOVE: %v", got)
	}
	if got := c.do("CRDT.ORSET.GET", "os"); len(got.Array) != 1 || string(got.Array[0].Bulk) != "y" {
		t.Fatalf("CRDT.ORSET.GET: %+v", got)
	}
	if got := c.do("CRDT.LWW.DELETE", "reg"); got.Int != 1 {
		t.Fatalf("CRDT.LWW.DELETE: %v", got)
	}
}

// DEDUP sets and vector clocks are keys: a seen id stays seen across a
// restart until its own TTL passes, and clocks keep their counters.
func TestDedupAndVectorClocksAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	for _, args := range [][]string{
		{"DEDUP.ADD", "seen", "m1"}, {"DEDUP.ADD", "seen", "m2", "1"},
		{"VECTOR_CLOCK.CREATE", "a", "n1"}, {"VECTOR_CLOCK.INCREMENT", "a", "n1"}, {"VECTOR_CLOCK.INCREMENT", "a", "n1"},
		{"VECTOR_CLOCK.INCREMENT", "b", "n1"}, {"VECTOR_CLOCK.INCREMENT", "b", "n2"},
	} {
		if v := c.do(args...); v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
	}
	if got := c.do("DEDUP.ADD", "seen", "m1"); got.Int != 0 {
		t.Fatalf("a second DEDUP.ADD of m1: %v", got)
	}
	if got := c.do("VECTOR_CLOCK.COMPARE", "a", "b"); string(got.Bulk) != "concurrent" {
		t.Fatalf("VECTOR_CLOCK.COMPARE: %+v", got)
	}
	for _, args := range [][]string{{"DEDUP.ADD", "seen", "m3", "0"}, {"VECTOR_CLOCK.COMPARE", "a", "nope"}} {
		if got := c.do(args...); got.Type != '-' {
			t.Fatalf("%v accepted: %v", args, got)
		}
	}
	c.do("SET", "plain", "v")
	for _, args := range [][]string{{"DEDUP.ADD", "plain", "x"}, {"VECTOR_CLOCK.INCREMENT", "seen", "n"}, {"VECTOR_CLOCK.GET", "plain"}} {
		if got := c.do(args...); got.Type != '-' || !strings.HasPrefix(got.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, got)
		}
	}
	stopReplServer(s)
	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"DEDUP.", "VECTOR_CLOCK."} {
		if strings.Contains(string(data), name) {
			t.Fatalf("the AOF records %s commands", name)
		}
	}

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	if got := c.do("DEDUP.CHECK", "seen", "m1"); got.Int != 1 {
		t.Fatalf("DEDUP.CHECK m1 after restart: %v", got)
	}
	if got := c.do("DEDUP.CHECK", "seen", "m2"); got.Int != 0 {
		t.Fatalf("DEDUP.CHECK of an id past its TTL: %v", got)
	}
	if got := c.do("HPTTL", "seen", "FIELDS", "1", "m1"); len(got.Array) != 1 || got.Array[0].Int <= 0 {
		t.Fatalf("m1 lost its TTL across the restart: %+v", got)
	}
	if got := c.do("VECTOR_CLOCK.MERGE", "a", "b"); got.Type == '-' {
		t.Fatalf("VECTOR_CLOCK.MERGE: %v", got.Err)
	}
	if got := c.do("VECTOR_CLOCK.COMPARE", "a", "b"); string(got.Bulk) != "after" {
		t.Fatalf("VECTOR_CLOCK.COMPARE after the merge: %+v", got)
	}
	if got := c.do("VECTOR_CLOCK.GET", "a"); len(got.Array) != 4 || got.Array[1].Int != 2 || got.Array[3].Int != 1 {
		t.Fatalf("VECTOR_CLOCK.GET after restart: %+v", got)
	}
	if got := c.do("DEDUP.CLEAR", "seen"); got.Int != 1 {
		t.Fatalf("DEDUP.CLEAR: %v", got)
	}
}
