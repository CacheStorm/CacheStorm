package server

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

// freePort returns a TCP port that was free a moment ago.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func syncStats(s *Server) (int64, int64, int64) {
	return s.repl.SyncStats()
}

func TestReplicationLinkDropContinuesFromBacklog(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("SET", "a", "1")
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)

	master.repl.DisconnectReplicas()
	// Writes made while the link is down reach the replica through +CONTINUE.
	for i := 0; i < 50; i++ {
		mc.do("RPUSH", "l", strconv.Itoa(i))
	}
	waitLinkUp(t, rc)
	waitSynced(t, replica, master)
	sameData(t, mc, rc)
	full, ok, _ := syncStats(master)
	if full != 1 || ok != 1 {
		t.Fatalf("sync_full=%d sync_partial_ok=%d, want 1 and 1", full, ok)
	}
}

func TestReplicationReplicaRestartPartialAndFullResync(t *testing.T) {
	master := startReplServer(t, func(c *config.Config) { c.Replication.BacklogSize = 4096 })
	mc := dialRepl(t, master)
	seedMaster(t, mc)

	dir := t.TempDir()
	replicaCfg := func() *config.Config {
		return replTestConfig(t, func(c *config.Config) {
			c.Persistence.Enabled = true
			c.Persistence.AOF = true
			c.Persistence.AOFSync = "always"
			c.Persistence.DataDir = dir
			c.Replication.Role = "replica"
			c.Replication.MasterHost = "127.0.0.1"
			c.Replication.MasterPort = master.testPort()
		})
	}

	replica := startReplServerCfg(t, replicaCfg())
	rc := dialRepl(t, replica)
	waitLinkUp(t, rc)
	waitSynced(t, replica, master)
	mc.do("SET", "after-sync", "x")
	mc.do("SELECT", "2")
	mc.do("SET", "in-db2", "y")
	waitSynced(t, replica, master)
	stopReplServer(replica)

	// A small gap: the restarted replica continues from the backlog, from
	// the dataset and position its AOF recorded.
	mc.do("SET", "while-down", "z")
	mc.do("SELECT", "0")
	mc.do("INCR", "counter")
	replica = startReplServerCfg(t, replicaCfg())
	rc = dialRepl(t, replica)
	waitLinkUp(t, rc)
	waitSynced(t, replica, master)
	sameData(t, mc, rc)
	full, ok, _ := syncStats(master)
	if full != 1 || ok != 1 {
		t.Fatalf("after a short outage: sync_full=%d sync_partial_ok=%d, want 1 and 1", full, ok)
	}
	stopReplServer(replica)

	// A gap larger than the backlog forces a full resynchronization.
	big := strings.Repeat("v", 512)
	for i := 0; i < 40; i++ {
		mc.do("SET", "big"+strconv.Itoa(i), big)
	}
	replica = startReplServerCfg(t, replicaCfg())
	rc = dialRepl(t, replica)
	waitLinkUp(t, rc)
	waitSynced(t, replica, master)
	sameData(t, mc, rc)
	full, ok, _ = syncStats(master)
	if full != 2 || ok != 1 {
		t.Fatalf("after a long outage: sync_full=%d sync_partial_ok=%d, want 2 and 1", full, ok)
	}
	mc.do("DEL", "big0")
	waitSynced(t, replica, master)
	stopReplServer(replica)

	// The replica's AOF holds exactly what it applied: started on its own,
	// it serves the master's dataset.
	standalone := startReplServerCfg(t, replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.AOF = true
		c.Persistence.DataDir = dir
	}))
	sameData(t, mc, dialRepl(t, standalone))
}

func TestReplicationMasterRestartFullResync(t *testing.T) {
	port := freePort(t)
	masterCfg := func() *config.Config {
		return replTestConfig(t, func(c *config.Config) { c.Server.Port = port })
	}
	master := startReplServerCfg(t, masterCfg())
	mc := dialRepl(t, master)
	mc.do("SET", "old", "1")
	replica := startReplServer(t, nil)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)

	stopReplServer(master)
	eventually(t, 10*time.Second, "master link down", func() bool {
		return rc.info("replication")["master_link_status"] == "down"
	})
	var restarted *Server
	eventually(t, 10*time.Second, "master restarted on its port", func() bool {
		s, err := New(masterCfg())
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Start(t.Context()); err != nil {
			return false
		}
		restarted = s
		return true
	})
	t.Cleanup(func() { stopReplServer(restarted) })
	mc = dialRepl(t, restarted)
	mc.do("SET", "new", "2")

	waitLinkUp(t, rc)
	waitSynced(t, replica, restarted)
	sameData(t, mc, rc)
	if got := rc.str("GET", "old"); got != "<null>" {
		t.Fatalf("data of the old master survived a full resync: %s", got)
	}
	if full, _, _ := syncStats(restarted); full != 1 {
		t.Fatalf("restarted master served %d full resyncs, want 1", full)
	}
}

func TestReplicationMasterAuth(t *testing.T) {
	master := startReplServer(t, func(c *config.Config) { c.Server.RequirePass = "s3cret" })
	mc := dialRepl(t, master)
	mc.do("AUTH", "s3cret")
	mc.do("SET", "k", "v")

	without := startReplServer(t, nil)
	wc := dialRepl(t, without)
	wc.do("REPLICAOF", "127.0.0.1", strconv.Itoa(master.testPort()))
	time.Sleep(300 * time.Millisecond)
	if got := wc.info("replication")["master_link_status"]; got != "down" {
		t.Fatalf("replica without masterauth: link %s", got)
	}

	replica := startReplServer(t, func(c *config.Config) { c.Replication.MasterAuth = "s3cret" })
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	if got := rc.str("GET", "k"); got != "v" {
		t.Fatalf("GET on replica: %s", got)
	}

	// A PSYNC without AUTH is refused like any other command.
	raw := dialRepl(t, master)
	if got := raw.str("PSYNC", "?", "-1"); !strings.HasPrefix(got, "ERR:NOAUTH") {
		t.Fatalf("unauthenticated PSYNC: %s", got)
	}
}

// A replica that stops reading is disconnected once its queue passes the
// limit; the master's clients are never held up by it.
func TestReplicationSlowReplicaDisconnected(t *testing.T) {
	master := startReplServer(t, func(c *config.Config) {
		c.Replication.ReplicaBufferLimit = 256 << 10
	})
	mc := dialRepl(t, master)

	stuck := dialRepl(t, master)
	stuck.do("REPLCONF", "listening-port", "1")
	if _, err := stuck.conn.Write(encodeArgs("PSYNC", "?", "-1")); err != nil {
		t.Fatal(err)
	}
	eventually(t, 5*time.Second, "stuck replica attached", func() bool {
		return mc.info("replication")["connected_slaves"] == "1"
	})

	value := strings.Repeat("x", 16<<10)
	start := time.Now()
	for i := 0; i < 2000; i++ {
		if got := mc.str("SET", fmt.Sprintf("k%d", i%50), value); got != "OK" {
			t.Fatalf("SET: %s", got)
		}
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("writes took %v with a stuck replica", elapsed)
	}
	eventually(t, 10*time.Second, "stuck replica disconnected", func() bool {
		return mc.info("replication")["connected_slaves"] == "0"
	})
}

// Writes running while a replica full-syncs land either in the snapshot or in
// the stream after it, never in both and never in neither.
func TestReplicationSnapshotConsistentUnderLoad(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	for i := 0; i < 2000; i++ {
		mc.do("SET", "pre"+strconv.Itoa(i), "v")
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		c := dialRepl(t, master)
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				key := "ctr" + strconv.Itoa(i%10)
				c.do("INCR", key)
				c.do("RPUSH", "list"+strconv.Itoa(w), strconv.Itoa(i))
				if i%7 == 0 {
					c.do("MULTI")
					c.do("INCRBY", "tx", "2")
					c.do("SADD", "txset", strconv.Itoa(i))
					c.do("EXEC")
				}
			}
		}(w)
	}
	rc := replicate(t, replica, master)
	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()
	waitSynced(t, replica, master)
	sameData(t, mc, rc)
}

func TestReplicationBlockingCommandsUnderGate(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	rc := replicate(t, replica, master)
	mc := dialRepl(t, master)
	blocked := dialRepl(t, master)
	done := make(chan string, 1)
	go func() {
		done <- str(blocked.do("BLPOP", "q", "5"))
	}()
	time.Sleep(100 * time.Millisecond)
	// The waiting BLPOP must not hold the write gate for q.
	mc.do("RPUSH", "q", "a", "b")
	if got := <-done; got != "[q a]" {
		t.Fatalf("BLPOP: %s", got)
	}
	waitSynced(t, replica, master)
	sameData(t, mc, rc)
	if got := rc.str("LRANGE", "q", "0", "-1"); got != "[b]" {
		t.Fatalf("replica list: %s", got)
	}
}

func TestClusterModeRefused(t *testing.T) {
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Cluster.Enabled = true
		c.Cluster.NodeName = "n1"
	})
	if _, err := New(cfg); err == nil || !strings.Contains(err.Error(), "cluster mode is not supported in this release") {
		t.Fatalf("New with cluster.enabled: %v", err)
	}
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	for _, args := range [][]string{{"CLUSTER", "INFO"}, {"CLUSTER", "NODES"}, {"READONLY"}, {"ASKING"}} {
		if got := c.str(args...); got != "ERR:ERR This instance has cluster support disabled" {
			t.Fatalf("%v: %s", args, got)
		}
	}
}
