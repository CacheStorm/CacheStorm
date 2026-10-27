package server

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// Redis Sentinel promotes a replica with MULTI, SLAVEOF NO ONE, CONFIG
// REWRITE, CLIENT KILL TYPE normal, EXEC and reconfigures the others with the
// same transaction around SLAVEOF host port. Both must take effect.
func TestSentinelStylePromotionInsideMulti(t *testing.T) {
	master := startReplServer(t, nil)
	r1 := startReplServer(t, nil)
	r2 := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("SET", "k", "v")
	rc1 := replicate(t, r1, master)
	rc2 := replicate(t, r2, master)
	waitSynced(t, r1, master)
	waitSynced(t, r2, master)

	rc1.do("MULTI")
	rc1.do("SLAVEOF", "NO", "ONE")
	rc1.do("CONFIG", "REWRITE")
	rc1.do("CLIENT", "KILL", "TYPE", "normal")
	got := rc1.do("EXEC")
	if got.Type != resp.TypeArray || len(got.Array) != 3 || got.Array[0].Type == resp.TypeError {
		t.Fatalf("EXEC of the promotion = %+v", got)
	}
	eventually(t, 5*time.Second, "promotion inside MULTI", func() bool {
		return rc1.info("replication")["role"] == "master"
	})

	rc2.do("MULTI")
	rc2.do("SLAVEOF", "127.0.0.1", strconv.Itoa(r1.testPort()))
	if got := rc2.do("EXEC"); got.Type != resp.TypeArray || got.Array[0].Type == resp.TypeError {
		t.Fatalf("EXEC of the reconfiguration = %+v", got)
	}
	eventually(t, 5*time.Second, "replica follows the promoted master", func() bool {
		info := rc2.info("replication")
		return info["master_port"] == strconv.Itoa(r1.testPort()) && info["master_link_status"] == "up"
	})
	if got := rc1.str("SET", "after", "1"); got != "OK" {
		t.Fatalf("write on the promoted master: %s", got)
	}
	waitSynced(t, r2, r1)
	if got := rc2.str("GET", "after"); got != "1" {
		t.Fatalf("new master's write on the replica: %s", got)
	}
}

func TestInfoReportsRedisFields(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	c.do("SET", "a", "1")
	c.do("GET", "a")
	c.do("GET", "missing")
	all := string(c.do("INFO").Bulk)
	for _, section := range []string{"# Server", "# Clients", "# Memory", "# Persistence", "# Stats", "# Replication", "# CPU", "# Keyspace"} {
		if !strings.Contains(all, section) {
			t.Errorf("INFO lacks %s", section)
		}
	}
	server := c.info("server")
	if server["redis_version"] == "" || len(server["run_id"]) != 40 || server["process_id"] == "" || server["uptime_in_seconds"] == "" {
		t.Fatalf("INFO server = %v", server)
	}
	if c.info("clients")["connected_clients"] != "1" {
		t.Fatalf("INFO clients = %v", c.info("clients"))
	}
	stats := c.info("stats")
	if stats["keyspace_hits"] == "" || stats["keyspace_misses"] == "" || stats["total_commands_processed"] == "" || stats["instantaneous_ops_per_sec"] == "" {
		t.Fatalf("INFO stats = %v", stats)
	}
	mem := c.info("memory")
	if mem["maxmemory_policy"] == "" || mem["used_memory"] == "" || mem["used_memory_human"] == "" {
		t.Fatalf("INFO memory = %v", mem)
	}
	if got := c.info("keyspace")["db0"]; !strings.HasPrefix(got, "keys=1,") {
		t.Fatalf("INFO keyspace db0 = %q", got)
	}
	if sole := string(c.do("INFO", "cpu").Bulk); strings.Contains(sole, "# Server") || !strings.Contains(sole, "used_cpu_sys") {
		t.Fatalf("INFO cpu = %q", sole)
	}
}

func TestDebugDigestMatchesAcrossReplication(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	if got := mc.str("DEBUG", "DIGEST"); got != strings.Repeat("0", 40) {
		t.Fatalf("empty dataset digest = %s", got)
	}
	mc.do("SET", "s", "v", "PXAT", "16725225600000")
	mc.do("HSET", "h", "a", "1", "b", "2", "c", "3")
	mc.do("SADD", "set", "x", "y", "z")
	mc.do("ZADD", "z", "1", "a", "2", "b")
	mc.do("RPUSH", "l", "1", "2", "3")
	mc.do("SELECT", "1")
	mc.do("SET", "s", "other-db")
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	want := mc.str("DEBUG", "DIGEST")
	if got := rc.str("DEBUG", "DIGEST"); got != want {
		t.Fatalf("replica digest %s, master %s", got, want)
	}
	mc.do("HSET", "h", "a", "changed")
	if got := mc.str("DEBUG", "DIGEST"); got == want {
		t.Fatal("digest did not change with the data")
	}
}
