package server

import (
	"bufio"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
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

func TestWAITAOF(t *testing.T) {
	plain := dialRepl(t, startReplServer(t, nil))
	if got := plain.do("WAITAOF", "1", "0", "0"); got.Type != resp.TypeError {
		t.Fatalf("WAITAOF 1 without AOF = %+v, want an error", got)
	}
	if got := plain.do("WAITAOF", "0", "0", "0"); len(got.Array) != 2 || got.Array[0].Int != 0 {
		t.Fatalf("WAITAOF 0 0 0 = %+v", got)
	}
	dir := t.TempDir()
	master := startReplServerCfg(t, replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.AOF = true
		c.Persistence.AOFSync = "no"
		c.Persistence.DataDir = dir
		c.Persistence.SnapshotInterval = "0"
	}))
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	replicate(t, replica, master)
	waitSynced(t, replica, master)
	mc.do("SET", "k", "v")
	got := mc.do("WAITAOF", "1", "1", "5000")
	if len(got.Array) != 2 || got.Array[0].Int != 1 || got.Array[1].Int != 1 {
		t.Fatalf("WAITAOF 1 1 = %+v, want [1 1]", got)
	}
	if got := dialRepl(t, replica).do("WAITAOF", "0", "0", "0"); got.Type != resp.TypeError {
		t.Fatalf("WAITAOF on a replica = %+v, want an error", got)
	}
}

// SYNC is the pre-PSYNC protocol: the master answers with the snapshot as
// one bulk payload, without +FULLRESYNC, and then streams the writes.
func TestLegacySYNCStreamsSnapshotAndWrites(t *testing.T) {
	master := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("SET", "before", "1")
	conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(master.testPort()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write(encodeArgs("SYNC")); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	rd := bufio.NewReader(conn)
	header, err := rd.ReadString('\n')
	if err != nil || !strings.HasPrefix(header, "$") {
		t.Fatalf("SYNC header = %q, %v; want the bulk length of the snapshot", header, err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(header[1:]))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := make([]byte, n)
	if _, err := io.ReadFull(rd, snapshot); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(snapshot), "before") {
		t.Fatalf("SYNC snapshot lacks the existing key: %q", snapshot)
	}
	mc.do("SET", "after", "2")
	r := resp.NewReader(rd)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		v, err := r.ReadValue()
		if err != nil {
			t.Fatalf("reading the stream: %v", err)
		}
		if v.Type == resp.TypeArray && len(v.Array) == 3 && strings.EqualFold(string(v.Array[0].Bulk), "SET") && string(v.Array[1].Bulk) == "after" {
			return
		}
	}
	t.Fatal("the write after SYNC was not streamed")
}

func TestMIGRATEMovesKeys(t *testing.T) {
	dir := t.TempDir()
	srcCfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.AOF = true
		c.Persistence.AOFSync = "always"
		c.Persistence.DataDir = dir
		c.Persistence.SnapshotInterval = "0"
	})
	src := startReplServerCfg(t, srcCfg)
	dst := startReplServer(t, func(c *config.Config) { c.Server.RequirePass = "pw" })
	sc := dialRepl(t, src)
	dc := dialRepl(t, dst)
	dc.do("AUTH", "pw")
	port := strconv.Itoa(dst.testPort())

	sc.do("SET", "s", "v", "EX", "1000")
	sc.do("HSET", "h", "f", "1")
	if got := sc.do("MIGRATE", "127.0.0.1", port, "s", "2", "5000", "AUTH", "pw"); got.Str != "OK" {
		t.Fatalf("MIGRATE = %+v", got)
	}
	if got := sc.do("EXISTS", "s"); got.Int != 0 {
		t.Fatal("MIGRATE without COPY left the source key")
	}
	dc.do("SELECT", "2")
	if got := dc.str("GET", "s"); got != "v" {
		t.Fatalf("migrated value = %q", got)
	}
	if ttl := dc.do("TTL", "s").Int; ttl < 990 || ttl > 1000 {
		t.Fatalf("migrated TTL = %d", ttl)
	}

	if got := sc.do("MIGRATE", "127.0.0.1", port, "h", "2", "5000", "COPY", "AUTH", "pw"); got.Str != "OK" {
		t.Fatalf("MIGRATE COPY = %+v", got)
	}
	if got := sc.do("EXISTS", "h"); got.Int != 1 {
		t.Fatal("MIGRATE COPY removed the source key")
	}
	if got := sc.do("MIGRATE", "127.0.0.1", port, "h", "2", "5000", "AUTH", "pw"); got.Type != resp.TypeError || !strings.Contains(got.Err, "BUSYKEY") {
		t.Fatalf("MIGRATE onto an existing key without REPLACE = %+v", got)
	}
	if got := sc.do("EXISTS", "h"); got.Int != 1 {
		t.Fatal("a refused MIGRATE removed the source key")
	}
	sc.do("SET", "a", "1")
	sc.do("SET", "b", "2")
	if got := sc.do("MIGRATE", "127.0.0.1", port, "", "2", "5000", "REPLACE", "AUTH", "pw", "KEYS", "a", "b", "h", "missing"); got.Str != "OK" {
		t.Fatalf("MIGRATE KEYS REPLACE = %+v", got)
	}
	if got := dc.do("MGET", "a", "b"); string(got.Array[0].Bulk) != "1" || string(got.Array[1].Bulk) != "2" {
		t.Fatalf("keys migrated with KEYS = %+v", got)
	}
	if got := sc.do("MIGRATE", "127.0.0.1", port, "missing", "2", "5000"); got.Str != "NOKEY" {
		t.Fatalf("MIGRATE of a missing key = %+v", got)
	}
	sc.do("SET", "unreachable", "x")
	if got := sc.do("MIGRATE", "127.0.0.1", "1", "unreachable", "0", "200"); got.Type != resp.TypeError || !strings.HasPrefix(got.Err, "IOERR") {
		t.Fatalf("MIGRATE to a closed port = %+v", got)
	}

	stopReplServer(src)
	restored := dialRepl(t, startReplServerCfg(t, srcCfg))
	if got := restored.do("EXISTS", "s", "a", "b", "h", "unreachable"); got.Int != 1 {
		t.Fatalf("after restart %d of the migrated keys exist, want only 'unreachable'", got.Int)
	}
}
