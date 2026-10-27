package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/hashslot"
	"github.com/cachestorm/cachestorm/internal/resp"
)

const (
	testMyID    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testOtherID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func clusterTestConfig(t *testing.T, dir string, mutate func(*config.Config)) *config.Config {
	t.Helper()
	return replTestConfig(t, func(c *config.Config) {
		c.Cluster.Enabled = true
		c.Persistence.DataDir = dir
		if mutate != nil {
			mutate(c)
		}
	})
}

func startClusterServer(t *testing.T, dir string, mutate func(*config.Config)) (*Server, *replClient) {
	t.Helper()
	s := startReplServerCfg(t, clusterTestConfig(t, dir, mutate))
	return s, dialRepl(t, s)
}

// raw sends a command and reads exactly len(want) bytes of reply.
func (c *replClient) raw(want string, args ...string) string {
	c.t.Helper()
	_ = c.conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.conn.Write(encodeArgs(args...)); err != nil {
		c.t.Fatalf("%v: %v", args, err)
	}
	buf := make([]byte, len(want))
	if _, err := io.ReadFull(c.conn, buf); err != nil {
		c.t.Fatalf("%v: read %q: %v", args, buf, err)
	}
	return string(buf)
}

func keyInSlot(t *testing.T, prefix string, slot int) string {
	t.Helper()
	for i := 0; i < 1000000; i++ {
		k := prefix + strconv.Itoa(i)
		if hashslot.Key(k) == slot {
			return k
		}
	}
	t.Fatalf("no key for slot %d", slot)
	return ""
}

func TestClusterSingleNode(t *testing.T) {
	dir := t.TempDir()
	s, c := startClusterServer(t, dir, nil)
	port := s.testPort()

	if got := c.info("cluster")["cluster_enabled"]; got != "1" {
		t.Fatalf("cluster_enabled = %q", got)
	}
	info := c.str("CLUSTER", "INFO")
	for _, want := range []string{"cluster_state:fail\r\n", "cluster_slots_assigned:0\r\n", "cluster_known_nodes:1\r\n", "cluster_size:0\r\n"} {
		if !strings.Contains(info, want) {
			t.Fatalf("fresh CLUSTER INFO lacks %q:\n%s", want, info)
		}
	}
	if got := c.str("SET", "foo", "bar"); got != "ERR:CLUSTERDOWN Hash slot not served" {
		t.Fatalf("SET with no slots: %s", got)
	}
	if got := c.str("PING"); got != "PONG" {
		t.Fatalf("a keyless command must not be redirected: %s", got)
	}
	if got := c.str("CLUSTER", "ADDSLOTSRANGE", "0", "8000", "8001"); got != "ERR:ERR wrong number of arguments for 'cluster|addslotsrange' command" {
		t.Fatalf("odd ADDSLOTSRANGE: %s", got)
	}
	if got := c.str("CLUSTER", "ADDSLOTSRANGE", "10", "5"); got != "ERR:ERR start slot number 10 is greater than end slot number 5" {
		t.Fatalf("reversed range: %s", got)
	}
	if got := c.str("CLUSTER", "ADDSLOTS", "16384"); got != "ERR:ERR Invalid or out of range slot" {
		t.Fatalf("slot out of range: %s", got)
	}
	if got := c.str("CLUSTER", "ADDSLOTSRANGE", "0", "16383"); got != "OK" {
		t.Fatalf("ADDSLOTSRANGE: %s", got)
	}
	if got := c.str("CLUSTER", "ADDSLOTS", "5"); got != "ERR:ERR Slot 5 is already busy" {
		t.Fatalf("busy slot: %s", got)
	}
	want := "cluster_state:ok\r\ncluster_slots_assigned:16384\r\ncluster_slots_ok:16384\r\ncluster_slots_pfail:0\r\ncluster_slots_fail:0\r\n" +
		"cluster_known_nodes:1\r\ncluster_size:1\r\ncluster_current_epoch:0\r\ncluster_my_epoch:0\r\n" +
		"cluster_stats_messages_sent:0\r\ncluster_stats_messages_received:0\r\ntotal_cluster_links_buffer_limit_exceeded:0\r\n"
	if got := c.str("CLUSTER", "INFO"); got != want {
		t.Fatalf("CLUSTER INFO:\n%q\nwant\n%q", got, want)
	}

	id := c.str("CLUSTER", "MYID")
	conf, err := os.ReadFile(filepath.Join(dir, "nodes.conf"))
	if err != nil || !strings.HasPrefix(string(conf), id+" 127.0.0.1:") || !strings.HasSuffix(string(conf), "vars currentEpoch 0 lastVoteEpoch 0\n") {
		t.Fatalf("nodes.conf (%v):\n%s", err, conf)
	}
	shard := c.str("CLUSTER", "MYSHARDID")
	if len(id) != 40 || len(shard) != 40 || !strings.Contains(string(conf), "shard-id="+shard) {
		t.Fatalf("ids %q %q", id, shard)
	}
	addr := fmt.Sprintf("127.0.0.1:%d@%d", port, port+10000)
	if got := c.str("CLUSTER", "NODES"); got != id+" "+addr+" myself,master - 0 0 0 connected 0-16383\n" {
		t.Fatalf("CLUSTER NODES: %q", got)
	}
	slots := fmt.Sprintf("*1\r\n*3\r\n:0\r\n:16383\r\n*4\r\n$9\r\n127.0.0.1\r\n:%d\r\n$40\r\n%s\r\n*0\r\n", port, id)
	if got := c.raw(slots, "CLUSTER", "SLOTS"); got != slots {
		t.Fatalf("CLUSTER SLOTS:\n%q\nwant\n%q", got, slots)
	}
	sh := c.do("CLUSTER", "SHARDS")
	if len(sh.Array) != 1 || str(sh.Array[0].Array[0]) != "slots" || str(sh.Array[0].Array[1]) != "[0 16383]" || str(sh.Array[0].Array[2]) != "nodes" {
		t.Fatalf("CLUSTER SHARDS: %s", str(sh))
	}
	node := str(sh.Array[0].Array[3].Array[0])
	wantNode := fmt.Sprintf("[id %s port %d ip 127.0.0.1 endpoint 127.0.0.1 role master replication-offset ", id, port)
	if !strings.HasPrefix(node, wantNode) || !strings.HasSuffix(node, " health online]") {
		t.Fatalf("CLUSTER SHARDS node: %s", node)
	}
	if got := c.str("CLUSTER", "LINKS"); got != "[]" {
		t.Fatalf("CLUSTER LINKS: %s", got)
	}
	if got := c.str("CLUSTER", "COUNT-FAILURE-REPORTS", id); got != "0" {
		t.Fatalf("COUNT-FAILURE-REPORTS: %s", got)
	}
	if got := c.str("CLUSTER", "HELP"); !strings.HasPrefix(got, "[CLUSTER <subcommand> [<arg> [value] [opt] ...]. Subcommands are: COUNTKEYSINSLOT <slot>") {
		t.Fatalf("CLUSTER HELP: %s", got)
	}

	if got := c.str("CLUSTER", "KEYSLOT", "foo"); got != "12182" {
		t.Fatalf("KEYSLOT foo = %s", got)
	}
	if a, b := c.str("CLUSTER", "KEYSLOT", "{user1000}.following"), c.str("CLUSTER", "KEYSLOT", "{user1000}.followers"); a != b {
		t.Fatalf("hash tags: %s %s", a, b)
	}
	if got := c.str("MSET", "{t}a", "1", "{t}b", "2"); got != "OK" {
		t.Fatalf("same-slot MSET: %s", got)
	}
	if got := c.str("MSET", "a", "1", "b", "2"); got != "ERR:CROSSSLOT Keys in request don't hash to the same slot" {
		t.Fatalf("cross-slot MSET: %s", got)
	}
	slot := strconv.Itoa(hashslot.Key("t"))
	if got := c.str("CLUSTER", "COUNTKEYSINSLOT", slot); got != "2" {
		t.Fatalf("COUNTKEYSINSLOT: %s", got)
	}
	if got := c.str("CLUSTER", "GETKEYSINSLOT", slot, "1"); got != "[{t}a]" && got != "[{t}b]" {
		t.Fatalf("GETKEYSINSLOT: %s", got)
	}
	if got := c.str("CLUSTER", "COUNTKEYSINSLOT", "16384"); got != "ERR:ERR Invalid slot" {
		t.Fatalf("COUNTKEYSINSLOT range: %s", got)
	}
	if got := c.str("CLUSTER", "GETKEYSINSLOT", "1", "-1"); got != "ERR:ERR Invalid slot or number of keys" {
		t.Fatalf("GETKEYSINSLOT range: %s", got)
	}
	if got := c.str("CLUSTER", "FLUSHSLOTS"); got != "ERR:ERR DB must be empty to perform CLUSTER FLUSHSLOTS." {
		t.Fatalf("FLUSHSLOTS with keys: %s", got)
	}
	if got := c.str("CLUSTER", "RESET"); got != "ERR:ERR CLUSTER RESET can't be called with master nodes containing keys" {
		t.Fatalf("RESET with keys: %s", got)
	}

	for args, want := range map[string]string{
		"SELECT 1":              "ERR:ERR SELECT is not allowed in cluster mode",
		"SELECT 0":              "OK",
		"SWAPDB 0 1":            "ERR:ERR SWAPDB is not allowed in cluster mode",
		"MOVE {t}a 1":           "ERR:ERR MOVE is not allowed in cluster mode",
		"COPY {t}a {t}c DB 1":   "ERR:ERR DB index is out of range",
		"REPLICAOF 127.0.0.1 1": "ERR:ERR REPLICAOF not allowed in cluster mode.",
	} {
		if got := c.str(strings.Fields(args)...); got != want {
			t.Fatalf("%s: %s, want %s", args, got, want)
		}
	}

	// A transaction is routed by the keys of all its commands.
	c.str("MULTI")
	c.str("SET", "{t}x", "1")
	c.str("SET", "other", "1")
	if got := c.str("EXEC"); got != "ERR:CROSSSLOT Keys in request don't hash to the same slot" {
		t.Fatalf("cross-slot EXEC: %s", got)
	}
	if got := c.str("GET", "{t}x"); got != "<null>" {
		t.Fatalf("a refused EXEC ran: %s", got)
	}
	c.str("MULTI")
	c.str("SET", "{t}x", "1")
	c.str("INCR", "{t}y")
	if got := c.str("EXEC"); got != "[OK 1]" {
		t.Fatalf("same-slot EXEC: %s", got)
	}

	// Scripts are routed by their declared keys, and their commands must
	// stay in that slot.
	if got := c.str("EVAL", "return redis.call('GET', KEYS[1])", "1", "{t}a"); got != "1" {
		t.Fatalf("EVAL: %s", got)
	}
	if got := c.str("EVAL", "return 1", "2", "a", "b"); got != "ERR:CROSSSLOT Keys in request don't hash to the same slot" {
		t.Fatalf("cross-slot EVAL: %s", got)
	}
	if got := c.str("EVAL", "return redis.call('GET', 'b')", "1", "{t}a"); !strings.Contains(got, "Script attempted to access keys that do not hash to the same slot") {
		t.Fatalf("undeclared key in another slot: %s", got)
	}
	if got := c.str("EVAL", "redis.call('GET', 'a') return redis.call('GET', 'b')", "0"); !strings.Contains(got, "Script attempted to access keys that do not hash to the same slot") {
		t.Fatalf("keyless script crossing slots: %s", got)
	}
	if got := c.str("EVAL", "return redis.call('MGET', 'a', 'b')", "0"); !strings.Contains(got, "Command 'mget' in script attempted to access keys that don't hash to the same slot") {
		t.Fatalf("cross-slot command in script: %s", got)
	}
	if got := c.str("EVAL", "#!lua flags=allow-cross-slot-keys\nredis.call('GET', 'a') return redis.call('GET', 'b')", "0"); got != "<null>" {
		t.Fatalf("allow-cross-slot-keys: %s", got)
	}
	if got := c.str("EVAL", "#!lua flags=no-cluster\nreturn 1", "0"); got != "ERR:ERR Can not run script on cluster, 'no-cluster' flag is set." {
		t.Fatalf("no-cluster: %s", got)
	}

	// RESP3 sends CLUSTER INFO and NODES as verbatim strings and SHARDS as maps.
	c.do("HELLO", "3")
	body := id + " " + addr + " myself,master - 0 0 0 connected 0-16383\n"
	nodes := fmt.Sprintf("=%d\r\ntxt:%s\r\n", len(body)+4, body)
	if got := c.raw(nodes, "CLUSTER", "NODES"); got != nodes {
		t.Fatalf("RESP3 CLUSTER NODES: %q", got)
	}
	if v := c.do("CLUSTER", "SHARDS"); v.Array[0].Type != resp.TypeMap {
		t.Fatalf("RESP3 CLUSTER SHARDS: %#v", v.Array[0])
	}
	slots3 := fmt.Sprintf("*1\r\n*3\r\n:0\r\n:16383\r\n*4\r\n$9\r\n127.0.0.1\r\n:%d\r\n$40\r\n%s\r\n%%0\r\n", port, id)
	if got := c.raw(slots3, "CLUSTER", "SLOTS"); got != slots3 {
		t.Fatalf("RESP3 CLUSTER SLOTS: %q", got)
	}

	// The node keeps its identity and slots across a restart.
	stopReplServer(s)
	s2, c2 := startClusterServer(t, dir, nil)
	defer stopReplServer(s2)
	if got := c2.str("CLUSTER", "MYID"); got != id {
		t.Fatalf("restarted with ID %s, want %s", got, id)
	}
	if got := c2.str("CLUSTER", "INFO"); !strings.Contains(got, "cluster_state:ok\r\n") {
		t.Fatalf("restarted CLUSTER INFO:\n%s", got)
	}
}

func TestClusterRedirections(t *testing.T) {
	dir := t.TempDir()
	conf := testMyID + " 127.0.0.1:0@0 myself,master - 0 0 0 connected\n" +
		testOtherID + " 127.0.0.1:6381@16381 master - 0 0 0 connected\n" +
		"vars currentEpoch 0 lastVoteEpoch 0\n"
	if err := os.WriteFile(filepath.Join(dir, "nodes.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	s, c := startClusterServer(t, dir, nil)
	if got := c.str("CLUSTER", "MYID"); got != testMyID {
		t.Fatalf("MYID %s", got)
	}
	c.str("CLUSTER", "ADDSLOTSRANGE", "0", "16383")
	if got := c.str("CLUSTER", "SETSLOT", "3999", "NODE", testOtherID); got != "OK" {
		t.Fatalf("SETSLOT NODE: %s", got)
	}
	if got := c.str("CLUSTER", "SETSLOT", "3999", "NODE", "cccccccccccccccccccccccccccccccccccccccc"); got != "ERR:ERR Unknown node cccccccccccccccccccccccccccccccccccccccc" {
		t.Fatalf("SETSLOT unknown node: %s", got)
	}
	if got := c.str("CLUSTER", "SETSLOT", "3999", "BOGUS"); got != "ERR:ERR Invalid CLUSTER SETSLOT action or number of arguments. Try CLUSTER HELP" {
		t.Fatalf("SETSLOT bad action: %s", got)
	}
	info := c.str("CLUSTER", "INFO")
	if !strings.Contains(info, "cluster_known_nodes:2\r\n") || !strings.Contains(info, "cluster_size:2\r\n") || !strings.Contains(info, "cluster_state:ok\r\n") {
		t.Fatalf("CLUSTER INFO:\n%s", info)
	}
	remote := keyInSlot(t, "k", 3999)
	if got := c.str("GET", remote); got != "ERR:MOVED 3999 127.0.0.1:6381" {
		t.Fatalf("GET of a foreign slot: %s", got)
	}
	if got := c.str("SPUBLISH", remote, "m"); got != "ERR:MOVED 3999 127.0.0.1:6381" {
		t.Fatalf("SPUBLISH of a foreign slot: %s", got)
	}
	if got := c.str("PUBLISH", remote, "m"); got != "0" {
		t.Fatalf("PUBLISH is not routed: %s", got)
	}
	c.str("MULTI")
	if got := c.str("GET", remote); got != "ERR:MOVED 3999 127.0.0.1:6381" {
		t.Fatalf("queued GET of a foreign slot: %s", got)
	}
	if got := c.str("EXEC"); !strings.HasPrefix(got, "ERR:EXECABORT") {
		t.Fatalf("EXEC after a redirected command: %s", got)
	}
	if got := c.str("EVAL", "return 1", "1", remote); got != "ERR:MOVED 3999 127.0.0.1:6381" {
		t.Fatalf("EVAL with a foreign key: %s", got)
	}
	if got := c.str("EVAL", "return redis.call('GET', ARGV[1])", "0", remote); !strings.HasPrefix(got, "ERR:ERR Script attempted to access a non local key in a cluster node script") {
		t.Fatalf("script with an undeclared foreign key: %s", got)
	}
	nodes := c.str("CLUSTER", "NODES")
	if !strings.Contains(nodes, testOtherID+" 127.0.0.1:6381@16381 master - 0 0 0 disconnected 3999\n") ||
		!strings.Contains(nodes, " myself,master - 0 0 0 connected 0-3998 4000-16383\n") {
		t.Fatalf("CLUSTER NODES:\n%s", nodes)
	}
	if got := c.str("CLUSTER", "SET-CONFIG-EPOCH", "3"); got != "ERR:ERR The user can assign a config epoch only when the node does not know any other node." {
		t.Fatalf("SET-CONFIG-EPOCH: %s", got)
	}

	// Migrating: keys still here are served, missing keys get ASK, and a
	// multi-key command that finds only some of them gets TRYAGAIN.
	mslot := hashslot.Key("m")
	ms := strconv.Itoa(mslot)
	c.str("SET", "{m}here", "1")
	if got := c.str("CLUSTER", "SETSLOT", ms, "MIGRATING", testOtherID); got != "OK" {
		t.Fatalf("SETSLOT MIGRATING: %s", got)
	}
	if got := c.str("GET", "{m}here"); got != "1" {
		t.Fatalf("GET of a key not migrated yet: %s", got)
	}
	if got := c.str("GET", "{m}gone"); got != "ERR:ASK "+ms+" 127.0.0.1:6381" {
		t.Fatalf("GET of a migrated key: %s", got)
	}
	if got := c.str("MGET", "{m}here", "{m}gone"); got != "ERR:TRYAGAIN Multiple keys request during rehashing of slot" {
		t.Fatalf("MGET during migration: %s", got)
	}
	if !strings.Contains(c.str("CLUSTER", "NODES"), " ["+ms+"->-"+testOtherID+"]") {
		t.Fatal("CLUSTER NODES does not show the migrating slot")
	}
	if got := c.str("CLUSTER", "SETSLOT", ms, "NODE", testOtherID); !strings.Contains(got, "while I still hold keys for this hash slot") {
		t.Fatalf("SETSLOT NODE with keys left: %s", got)
	}
	c.str("CLUSTER", "SETSLOT", ms, "STABLE")
	if got := c.str("GET", "{m}gone"); got != "<null>" {
		t.Fatalf("GET after STABLE: %s", got)
	}

	// Importing: served only after ASKING, which lasts one command.
	tag := keyInSlot(t, "t", 3999)
	a, b := "{"+tag+"}a", "{"+tag+"}b"
	if got := c.str("CLUSTER", "SETSLOT", "3999", "IMPORTING", testOtherID); got != "OK" {
		t.Fatalf("SETSLOT IMPORTING: %s", got)
	}
	if got := c.str("GET", a); got != "ERR:MOVED 3999 127.0.0.1:6381" {
		t.Fatalf("GET without ASKING: %s", got)
	}
	c.str("ASKING")
	if got := c.str("SET", a, "v"); got != "OK" {
		t.Fatalf("SET after ASKING: %s", got)
	}
	if got := c.str("GET", a); got != "ERR:MOVED 3999 127.0.0.1:6381" {
		t.Fatalf("ASKING outlived its command: %s", got)
	}
	c.str("ASKING")
	if got := c.str("MGET", a, b); got != "ERR:TRYAGAIN Multiple keys request during rehashing of slot" {
		t.Fatalf("MGET with a key not imported yet: %s", got)
	}
	if !strings.Contains(c.str("CLUSTER", "NODES"), " [3999-<-"+testOtherID+"]") {
		t.Fatal("CLUSTER NODES does not show the importing slot")
	}
	if got := c.str("CLUSTER", "SETSLOT", "3999", "NODE", testMyID); got != "OK" {
		t.Fatalf("closing the import: %s", got)
	}
	if got := c.str("GET", a); got != "v" {
		t.Fatalf("GET after the import: %s", got)
	}
	if got := c.str("CLUSTER", "INFO"); !strings.Contains(got, "cluster_my_epoch:1\r\n") || !strings.Contains(got, "cluster_size:1\r\n") {
		t.Fatalf("closing an import bumps the epoch:\n%s", got)
	}
	if got := c.str("CLUSTER", "BUMPEPOCH"); got != "STILL 1" {
		t.Fatalf("BUMPEPOCH: %s", got)
	}

	// Without full coverage the cluster is down.
	c.str("CLUSTER", "DELSLOTS", "1")
	if got := c.str("GET", a); got != "ERR:CLUSTERDOWN The cluster is down" {
		t.Fatalf("GET while down: %s", got)
	}
	if got := c.str("GET", keyInSlot(t, "z", 1)); got != "ERR:CLUSTERDOWN Hash slot not served" {
		t.Fatalf("GET of an unassigned slot: %s", got)
	}
	if got := c.str("CLUSTER", "DELSLOTS", "1"); got != "ERR:ERR Slot 1 is already unassigned" {
		t.Fatalf("DELSLOTS twice: %s", got)
	}
	stopReplServer(s)
	saved, _ := os.ReadFile(filepath.Join(dir, "nodes.conf"))
	if !strings.Contains(string(saved), " myself,master - 0 0 1 connected 0 2-16383\n") || !strings.Contains(string(saved), "vars currentEpoch 1 lastVoteEpoch 0\n") {
		t.Fatalf("nodes.conf:\n%s", saved)
	}
}

func TestClusterReadsWhenDown(t *testing.T) {
	_, c := startClusterServer(t, t.TempDir(), func(cfg *config.Config) { cfg.Cluster.AllowReadsWhenDown = true })
	c.str("CLUSTER", "ADDSLOTSRANGE", "0", "16383")
	c.str("SET", "{a}1", "x")
	c.str("CLUSTER", "DELSLOTS", "0")
	if got := c.str("GET", "{a}1"); got != "x" {
		t.Fatalf("read while down: %s", got)
	}
	if got := c.str("SET", "{a}1", "y"); got != "ERR:CLUSTERDOWN The cluster is down and only accepts read commands" {
		t.Fatalf("write while down: %s", got)
	}
}

func TestClusterLoadsExistingData(t *testing.T) {
	dir := t.TempDir()
	persist := func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.DataDir = dir
	}
	plain := startReplServer(t, persist)
	pc := dialRepl(t, plain)
	pc.str("SET", "{p}a", "1")
	pc.str("SET", "{p}b", "2")
	stopReplServer(plain)

	s, c := startClusterServer(t, dir, func(cfg *config.Config) { cfg.Persistence.Enabled = true })
	slot := strconv.Itoa(hashslot.Key("p"))
	if got := c.str("CLUSTER", "COUNTKEYSINSLOT", slot); got != "2" {
		t.Fatalf("keys loaded from the AOF are not indexed: %s", got)
	}
	if got := c.str("CLUSTER", "NODES"); !strings.Contains(got, " connected "+slot+"\n") {
		t.Fatalf("the slot holding keys was not claimed:\n%s", got)
	}
	if got := c.str("CLUSTER", "INFO"); !strings.Contains(got, "cluster_slots_assigned:1\r\n") {
		t.Fatalf("CLUSTER INFO:\n%s", got)
	}
	c.str("CLUSTER", "ADDSLOTSRANGE", "0", strconv.Itoa(hashslot.Key("p")-1))
	c.str("FLUSHALL")
	if got := c.str("CLUSTER", "COUNTKEYSINSLOT", slot); got != "0" {
		t.Fatalf("FLUSHALL left keys in the index: %s", got)
	}
	stopReplServer(s)

	plain = startReplServer(t, persist)
	pc = dialRepl(t, plain)
	pc.str("SELECT", "2")
	pc.str("SET", "x", "1")
	stopReplServer(plain)
	cfg := clusterTestConfig(t, dir, func(cfg *config.Config) { cfg.Persistence.Enabled = true })
	if s, err := New(cfg); err == nil {
		_ = s.Stop(context.Background())
		t.Fatal("cluster mode started with keys outside database 0")
	} else if !strings.Contains(err.Error(), "database 2 holds keys") {
		t.Fatalf("New: %v", err)
	}
}

func TestClusterCorruptNodesConf(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nodes.conf"), []byte("garbage line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s, err := New(clusterTestConfig(t, dir, nil)); err == nil {
		_ = s.Stop(context.Background())
		t.Fatal("started with a corrupt nodes.conf")
	}
}
