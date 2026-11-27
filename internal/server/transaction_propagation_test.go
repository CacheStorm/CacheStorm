package server

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// A script that writes and then fails keeps its writes, and Redis
// propagates them. Run inside MULTI, EXEC left a command that answered with
// an error out of what it recorded, script effects and all, so the replicas
// and the AOF never had the script's RPUSH.
func TestScriptThatFailsInsideATransactionKeepsItsWrites(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	replicate(t, replica, master)
	c := dialRepl(t, master)
	c.do("SET", "s", "str")
	c.do("MULTI")
	c.do("EVAL", "redis.call('RPUSH', KEYS[1], 'v') return redis.call('LPOP', KEYS[2])", "2", "l", "s")
	c.do("RPUSH", "l", "w")
	if got := str(c.do("EXEC")); got == "" {
		t.Fatal("EXEC gave nothing")
	}
	if got := str(c.do("LRANGE", "l", "0", "-1")); got != "[v w]" {
		t.Fatalf("master l = %s", got)
	}
	want := dumpAll(t, c)
	waitSynced(t, replica, master)
	if got := dumpAll(t, dialRepl(t, replica)); !reflect.DeepEqual(got, want) {
		t.Fatalf("replica %v, master %v", got, want)
	}
	restarted := checkAOFRestart(t, master, cfg)
	if got := dumpAll(t, dialRepl(t, restarted)); !reflect.DeepEqual(got, want) {
		t.Fatalf("after an AOF load %v, before %v", got, want)
	}
}

// A transaction that runs out of room under maxmemory halfway through does
// not evict keys then: Redis evicts only before a command runs, and the
// eviction's DEL was recorded ahead of the transaction, the first SET's key
// among the evicted, so the replicas and the AOF kept a key the master had
// evicted. The write that does not fit is refused, as when there is
// nothing to evict, and what the transaction did is recorded as it was.
func TestTransactionDoesNotEvictHalfwayThrough(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	replicate(t, replica, master)
	c := dialRepl(t, master)
	used, _ := strconv.ParseInt(c.info("memory")["used_memory"], 10, 64)
	c.do("CONFIG", "SET", "maxmemory-policy", "allkeys-lru")
	c.do("CONFIG", "SET", "maxmemory", strconv.FormatInt(used+100_000, 10))
	big := strings.Repeat("x", 60_000)
	// The eviction count is the process's, shared with other tests.
	evictedBefore := c.info("stats")["evicted_keys"]
	c.do("MULTI")
	c.do("SET", "a", big)
	c.do("SET", "b", big)
	if got := str(c.do("EXEC")); !strings.HasPrefix(got, "[OK ERR:OOM") {
		t.Fatalf("EXEC = %.80s", got)
	}
	if c.do("EXISTS", "a").Int != 1 || c.do("EXISTS", "b").Int != 0 {
		t.Fatal("the transaction evicted its own first key")
	}
	if n := c.info("stats")["evicted_keys"]; n != evictedBefore {
		t.Fatalf("evicted_keys went from %s to %s inside a transaction", evictedBefore, n)
	}
	want := dumpAll(t, c)
	waitSynced(t, replica, master)
	if got := dumpAll(t, dialRepl(t, replica)); !reflect.DeepEqual(got, want) {
		t.Fatalf("replica differs\n%s", mapDiff(want, got))
	}
	restarted := checkAOFRestart(t, master, cfg)
	if got := dumpAll(t, dialRepl(t, restarted)); !reflect.DeepEqual(got, want) {
		t.Fatalf("after an AOF load\n%s", mapDiff(want, got))
	}
}
