package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SPATIAL, CHAIN, ANALYTICS, ROLLUP, COOLDOWN and QUOTA are keys: their
// objects survive a restart through an AOF holding only Redis commands.
func TestMVCCNamespacesAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	ok := func(args ...string) string {
		t.Helper()
		v := c.do(args...)
		if v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
		return string(v.Bulk)
	}
	ok("SPATIAL.CREATE", "geo")
	ok("SPATIAL.ADD", "geo", "home", "41.0", "29.0", "istanbul")
	ok("SPATIAL.ADD", "geo", "far", "-33.9", "151.2")
	ok("SPATIAL.ADD", "geo", "gone", "0", "0")
	ok("SPATIAL.DELETE", "geo", "gone")
	ok("CHAIN.CREATE", "ch")
	ok("CHAIN.ADD", "ch", "genesis tx")
	ok("CHAIN.ADD", "ch", "second")
	ok("ANALYTICS.INCR", "an", "4")
	ok("ANALYTICS.DECR", "an", "1")
	ok("ROLLUP.CREATE", "ru", "60")
	ok("ROLLUP.ADD", "ru", "61", "1.5")
	ok("ROLLUP.ADD", "ru", "119", "2")
	ok("COOLDOWN.SET", "cd", "3600000")
	if r := c.do("COOLDOWN.CHECK", "cd"); r.Int != 1 {
		t.Fatalf("first COOLDOWN.CHECK: %+v", r)
	}
	ok("QUOTA.SET", "q", "10", "0")
	if r := c.do("QUOTA.USE", "q", "7"); r.Int != 1 {
		t.Fatalf("QUOTA.USE: %+v", r)
	}
	if r := c.do("QUOTA.USE", "q", "7"); r.Int != 0 {
		t.Fatalf("QUOTA.USE over the limit: %+v", r)
	}

	ok("POOL.CREATE", "pool", "2")
	ok("POOL.PUT", "pool", "conn-a")
	ok("POOL.PUT", "pool", "conn-b")
	if r := c.do("POOL.PUT", "pool", "conn-c"); r.Int != 0 {
		t.Fatalf("POOL.PUT into a full pool: %+v", r)
	}
	if got := ok("POOL.GET", "pool"); got != "conn-a" {
		t.Fatalf("POOL.GET = %q", got)
	}
	ok("POOL.PUT", "pool", "conn-c")

	ok("SET", "plain", "v")
	for _, args := range [][]string{{"SPATIAL.ADD", "plain", "a", "1", "1"}, {"CHAIN.ADD", "geo", "x"}, {"ANALYTICS.INCR", "ch", "1"},
		{"ROLLUP.ADD", "an", "1", "1"}, {"COOLDOWN.CHECK", "q"}, {"QUOTA.USE", "cd", "1"}, {"POOL.PUT", "q", "x"}} {
		if r := c.do(args...); r.Type != '-' || !strings.HasPrefix(r.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, r)
		}
	}
	for _, args := range [][]string{{"SPATIAL.ADD", "geo", "a", "nan", "1"}, {"SPATIAL.ADD", "geo", "a", "91", "1"},
		{"SPATIAL.ADD", "geo", "__spatial", "1", "1"}, {"ROLLUP.CREATE", "r2", "0"}, {"QUOTA.SET", "q2", "-1", "0"},
		{"COOLDOWN.SET", "c2", "x"}, {"ANALYTICS.INCR", "a2", "inf"}, {"CHAIN.ADD", "nochain", "x"}} {
		if r := c.do(args...); r.Type != '-' {
			t.Fatalf("%v accepted: %v", args, r)
		}
	}
	stopReplServer(s)

	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SPATIAL.", "CHAIN.", "ANALYTICS.", "ROLLUP.", "COOLDOWN.", "QUOTA.", "POOL."} {
		if strings.Contains(string(data), name) {
			t.Fatalf("the AOF records %s commands", name)
		}
	}

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if r := renderReply(c.do(args...), ","); r != want {
			t.Fatalf("%v after restart = %q, want %q", args, r, want)
		}
	}
	expect("geo", "SPATIAL.LIST")
	expect("home", "SPATIAL.WITHIN", "geo", "40", "28", "42", "30")
	expect("home,0.000000", "SPATIAL.NEARBY", "geo", "41", "29", "1")
	expect("2", "CHAIN.LENGTH", "ch")
	expect("1", "CHAIN.VALIDATE", "ch")
	if r := c.do("CHAIN.GET", "ch", "0"); string(r.Array[5].Bulk) != "genesis tx" {
		t.Fatalf("CHAIN.GET after restart: %+v", r)
	}
	expect("2", "ANALYTICS.COUNT", "an")
	expect("3.00", "ANALYTICS.SUM", "an")
	expect("-1.00", "ANALYTICS.MIN", "an")
	expect("3.50", "ROLLUP.GET", "ru", "100")
	expect("cd,3600000", "COOLDOWN.LIST")
	if r := c.do("COOLDOWN.CHECK", "cd"); len(r.Array) != 2 || r.Array[0].Int != 0 {
		t.Fatalf("COOLDOWN.CHECK after restart: %+v", r)
	}
	expect("q,10,7", "QUOTA.LIST")
	expect("1", "QUOTA.RESET", "q")
	expect("1", "QUOTA.USE", "q", "10")
	expect("name,pool,size,2,max_size,2,available,0", "POOL.STATS", "pool")
	expect("conn-b", "POOL.GET", "pool")
	expect("conn-c", "POOL.GET", "pool")
}
