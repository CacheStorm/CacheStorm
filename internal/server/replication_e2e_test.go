package server

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

// seedMaster writes keys of every core type, with and without TTLs, in
// databases 0, 1 and 3.
func seedMaster(t *testing.T, m *replClient) {
	t.Helper()
	cmds := [][]string{
		{"SET", "str", "v1"},
		{"SET", "ttlstr", "v2", "EX", "1000"},
		{"RPUSH", "list", "a", "b", "c"},
		{"SADD", "set", "x", "y", "z"},
		{"HSET", "hash", "f1", "1", "f2", "2"},
		{"ZADD", "zset", "1", "one", "2.5", "two"},
		{"XADD", "stream", "1-1", "f", "v"},
		{"XADD", "stream", "*", "g", "w"},
		{"PEXPIRE", "hash", "500000"},
		{"SELECT", "1"},
		{"SET", "db1key", "in-db1"},
		{"LPUSH", "db1list", "q"},
		{"SELECT", "3"},
		{"SET", "db3key", "in-db3", "PX", "900000"},
		{"SELECT", "0"},
	}
	for _, c := range cmds {
		if got := m.do(c...); got.Type == '-' {
			t.Fatalf("%v: %s", c, str(got))
		}
	}
}

func TestReplicationFullSyncAndLiveStream(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	seedMaster(t, mc)

	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	sameData(t, mc, rc)

	// Live writes, across databases.
	mc.do("SET", "live", "1")
	mc.do("INCR", "live")
	mc.do("SELECT", "2")
	mc.do("SADD", "db2set", "m")
	mc.do("SELECT", "0")
	mc.do("DEL", "str")
	mc.do("LPOP", "list")
	mc.do("XADD", "stream", "*", "h", "z")
	mc.do("SPOP", "set")
	mc.do("HINCRBY", "hash", "f1", "41")
	waitSynced(t, replica, master)
	sameData(t, mc, rc)

	info := mc.info("replication")
	if info["role"] != "master" || info["connected_slaves"] != "1" {
		t.Fatalf("master INFO: %v", info)
	}
	if !strings.Contains(info["slave0"], "state=online") ||
		!strings.Contains(info["slave0"], "port="+strconv.Itoa(replica.testPort())) {
		t.Fatalf("master slave0 line: %q", info["slave0"])
	}
	rinfo := rc.info("replication")
	if rinfo["role"] != "slave" || rinfo["master_port"] != strconv.Itoa(master.testPort()) ||
		rinfo["master_sync_in_progress"] != "0" || rinfo["slave_read_only"] != "1" ||
		rinfo["master_replid"] != info["master_replid"] {
		t.Fatalf("replica INFO: %v", rinfo)
	}
	if got := rc.info("server")["tcp_port"]; got != strconv.Itoa(replica.testPort()) {
		t.Fatalf("tcp_port %s, want %d", got, replica.testPort())
	}
	if got := rc.str("ROLE"); !strings.HasPrefix(got, "[slave 127.0.0.1 "+strconv.Itoa(master.testPort())+" connected ") {
		t.Fatalf("replica ROLE: %s", got)
	}
	if got := mc.str("ROLE"); !strings.HasPrefix(got, "[master ") || !strings.Contains(got, strconv.Itoa(replica.testPort())) {
		t.Fatalf("master ROLE: %s", got)
	}
}

func TestReplicationReadOnlyReplica(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("SET", "k", "v")
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)

	if got := rc.str("SET", "k", "other"); got != "ERR:READONLY You can't write against a read only replica." {
		t.Fatalf("SET on replica: %s", got)
	}
	if got := rc.str("GET", "k"); got != "v" {
		t.Fatalf("GET on replica: %s", got)
	}
	rc.do("MULTI")
	rc.do("INCR", "k")
	if got := rc.str("EXEC"); !strings.HasPrefix(got, "ERR:EXECABORT") {
		t.Fatalf("EXEC with a refused write: %s", got)
	}
	if got := rc.str("EVAL", "return redis.call('GET', KEYS[1])", "1", "k"); got != "v" {
		t.Fatalf("read-only EVAL on replica: %s", got)
	}
	if got := rc.str("EVAL", "return redis.call('SET', KEYS[1], 'x')", "1", "k"); !strings.Contains(got, "READONLY") {
		t.Fatalf("writing EVAL on replica: %s", got)
	}
	if got := rc.str("WAIT", "0", "0"); !strings.HasPrefix(got, "ERR:ERR WAIT cannot be used with replica instances") {
		t.Fatalf("WAIT on replica: %s", got)
	}
}

func TestReplicationWritableReplica(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, func(c *config.Config) { c.Replication.ReadOnly = false })
	rc := replicate(t, replica, master)
	if got := rc.str("SET", "local", "1"); got != "OK" {
		t.Fatalf("SET on writable replica: %s", got)
	}
}

func TestReplicationTransactionsScriptsAndExpiry(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	rc := replicate(t, replica, master)

	mc.do("MULTI")
	mc.do("SET", "tx1", "a")
	mc.do("SELECT", "1")
	mc.do("INCRBY", "tx2", "5")
	mc.do("SELECT", "0")
	mc.do("RPUSH", "txl", "1", "2")
	if got := mc.str("EXEC"); got != "[OK OK 5 OK 2]" {
		t.Fatalf("EXEC: %s", got)
	}
	if got := mc.str("EVAL", "redis.call('SET', KEYS[1], ARGV[1]); redis.call('SPOP', KEYS[2]); return redis.call('XADD', KEYS[3], '*', 'a', 'b')",
		"3", "lua1", "luaset", "luastream", "fromlua"); strings.HasPrefix(got, "ERR") {
		t.Fatalf("EVAL: %s", got)
	}
	mc.do("SADD", "luaset", "p", "q", "r")
	sha := mc.str("SCRIPT", "LOAD", "return redis.call('SPOP', KEYS[1])")
	mc.do("EVALSHA", sha, "1", "luaset")

	mc.do("SET", "short", "gone", "PX", "50")
	mc.do("SELECT", "1")
	mc.do("SET", "short1", "gone", "PX", "50")
	mc.do("SELECT", "0")
	waitSynced(t, replica, master)
	sameData(t, mc, rc)

	// Expired keys are reaped on the master and streamed as DEL; the
	// replica never expires keys on its own.
	time.Sleep(200 * time.Millisecond)
	mc.do("GET", "short")
	eventually(t, 10*time.Second, "expired keys removed on the replica", func() bool {
		_, ok1 := replica.dbs.DB(0).GetShard("short").Get("short")
		_, ok2 := replica.dbs.DB(1).GetShard("short1").Get("short1")
		return !ok1 && !ok2
	})
	waitSynced(t, replica, master)
	sameData(t, mc, rc)
}

func TestReplicationPromoteAndWait(t *testing.T) {
	master := startReplServer(t, nil)
	r1 := startReplServer(t, nil)
	r2 := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("SET", "a", "1")
	rc1 := replicate(t, r1, master)
	rc2 := replicate(t, r2, master)
	waitSynced(t, r1, master)
	waitSynced(t, r2, master)

	mc.do("SET", "b", "2")
	if got := mc.str("WAIT", "2", "5000"); got != "2" {
		t.Fatalf("WAIT 2: %s", got)
	}
	start := time.Now()
	if got := mc.str("WAIT", "3", "200"); got != "2" {
		t.Fatalf("WAIT 3 with two replicas: %s", got)
	}
	if time.Since(start) < 150*time.Millisecond {
		t.Fatal("WAIT returned before its timeout")
	}
	sameData(t, mc, rc1)
	sameData(t, mc, rc2)

	// Promote r1; r2 follows it and continues from its backlog.
	oldID := rc1.info("replication")["master_replid"]
	if got := rc1.str("REPLICAOF", "NO", "ONE"); got != "OK" {
		t.Fatalf("REPLICAOF NO ONE: %s", got)
	}
	info := rc1.info("replication")
	if info["role"] != "master" || info["master_replid"] == oldID || info["master_replid2"] != oldID {
		t.Fatalf("promoted INFO: %v", info)
	}
	if got := rc1.str("GET", "b"); got != "2" {
		t.Fatalf("promoted replica lost data: %s", got)
	}
	if got := rc1.str("SET", "c", "3"); got != "OK" {
		t.Fatalf("write on promoted replica: %s", got)
	}
	if got := rc2.str("REPLICAOF", "127.0.0.1", strconv.Itoa(r1.testPort())); got != "OK" {
		t.Fatalf("REPLICAOF new master: %s", got)
	}
	waitLinkUp(t, rc2)
	waitSynced(t, r2, r1)
	sameData(t, rc1, rc2)
	if got := rc1.str("WAIT", "1", "5000"); got != "1" {
		t.Fatalf("WAIT on promoted master: %s", got)
	}
}
