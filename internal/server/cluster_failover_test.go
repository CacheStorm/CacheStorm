package server

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// shardedCluster is three masters serving a third of the slots each, and a
// replica of each: replicas[i] replicates masters[i].
type shardedCluster struct {
	t        *testing.T
	masters  []*busNode
	replicas []*busNode
}

func (sc *shardedCluster) all() []*busNode {
	return append(append([]*busNode(nil), sc.masters...), sc.replicas...)
}

var thirds = [][2]int{{0, 5460}, {5461, 10922}, {10923, 16383}}

func startShardedCluster(t *testing.T) *shardedCluster {
	t.Helper()
	nodes := startBusNodes(t, 6)
	sc := &shardedCluster{t: t, masters: nodes[:3], replicas: nodes[3:]}
	formMesh(t, nodes)
	for i, n := range sc.masters {
		if got := n.c.str("CLUSTER", "ADDSLOTSRANGE", strconv.Itoa(thirds[i][0]), strconv.Itoa(thirds[i][1])); got != "OK" {
			t.Fatalf("ADDSLOTSRANGE: %s", got)
		}
	}
	eventually(t, 10*time.Second, "the masters' slots to converge", func() bool {
		return converged(nodes) && sc.allOK(nodes)
	})
	for i, r := range sc.replicas {
		if got := r.c.str("CLUSTER", "REPLICATE", sc.masters[i].id); got != "OK" {
			t.Fatalf("REPLICATE: %s", got)
		}
	}
	sc.waitRoles(sc.masters, sc.replicas, nodes)
	return sc
}

func (sc *shardedCluster) allOK(nodes []*busNode) bool {
	for _, n := range nodes {
		if n.clusterInfo()["cluster_state"] != "ok" {
			return false
		}
	}
	return true
}

// waitRoles waits until every node of view sees masters[i] serving the
// i-th third and replicas[i] replicating it, and the cluster is ok.
func (sc *shardedCluster) waitRoles(masters, replicas, view []*busNode) {
	sc.t.Helper()
	eventually(sc.t, 15*time.Second, "the replica roles to converge", func() bool {
		for _, n := range view {
			lines := n.nodes()
			for i, m := range masters {
				want := strconv.Itoa(thirds[i][0]) + "-" + strconv.Itoa(thirds[i][1])
				if l := lines[m.id]; l.flags != "master" || l.slots != want || l.master != "-" {
					return false
				}
				if l := lines[replicas[i].id]; l.flags != "slave" || l.master != m.id || l.slots != "" {
					return false
				}
			}
		}
		return sc.allOK(view)
	})
}

// waitShardSynced waits until replica's replication offset reached master's.
func waitShardSynced(t *testing.T, master, replica *busNode) {
	t.Helper()
	eventually(t, 10*time.Second, "the replica to catch up", func() bool {
		mo := master.c.info("replication")["master_repl_offset"]
		ri := replica.c.info("replication")
		return ri["master_link_status"] == "up" && mo != "0" && ri["master_repl_offset"] == mo
	})
}

func moved(slot int, n *busNode) string {
	return "ERR:MOVED " + strconv.Itoa(slot) + " 127.0.0.1:" + strconv.Itoa(n.port)
}

// mapOf returns the map a CLUSTER SHARDS entry or node is, by key.
func mapOf(v *resp.Value) map[string]*resp.Value {
	out := map[string]*resp.Value{}
	items := v.Array
	for i := 0; i+1 < len(items); i += 2 {
		out[str(items[i])] = items[i+1]
	}
	for k, val := range v.Map {
		out[k] = val
	}
	return out
}

func TestClusterReplicateShape(t *testing.T) {
	nodes := startBusNodes(t, 3)
	a, b, c := nodes[0], nodes[1], nodes[2]
	unknown := strings.Repeat("e", 40)
	if got := a.c.str("CLUSTER", "REPLICATE", unknown); got != "ERR:ERR Unknown node "+unknown {
		t.Fatalf("REPLICATE unknown: %s", got)
	}
	if got := a.c.str("CLUSTER", "REPLICATE", a.id); got != "ERR:ERR Can't replicate myself" {
		t.Fatalf("REPLICATE myself: %s", got)
	}
	if got := a.c.str("CLUSTER", "FAILOVER"); got != "ERR:ERR You should send CLUSTER FAILOVER to a replica" {
		t.Fatalf("FAILOVER on a master: %s", got)
	}
	if got := a.c.str("CLUSTER", "FAILOVER", "SOON"); got != "ERR:ERR syntax error" {
		t.Fatalf("FAILOVER with a bad option: %s", got)
	}
	formMesh(t, nodes)
	if got := a.c.str("CLUSTER", "ADDSLOTSRANGE", "0", "16383"); got != "OK" {
		t.Fatalf("ADDSLOTSRANGE: %s", got)
	}
	if got := a.c.str("CLUSTER", "REPLICATE", b.id); got != "ERR:ERR To set a master the node must be empty and without assigned slots." {
		t.Fatalf("REPLICATE from a master with slots: %s", got)
	}
	eventually(t, 10*time.Second, "B to learn A's slots", func() bool {
		return strings.HasPrefix(b.c.str("SET", "{x}a", "1"), "ERR:MOVED")
	})
	if got := b.c.str("CLUSTER", "REPLICATE", a.id); got != "OK" {
		t.Fatalf("REPLICATE: %s", got)
	}
	eventually(t, 10*time.Second, "B to be seen as A's replica", func() bool {
		return a.nodes()[b.id].flags == "slave" && c.nodes()[b.id].master == a.id
	})
	if got := c.c.str("CLUSTER", "REPLICATE", b.id); got != "ERR:ERR I can only replicate a master, not a replica." {
		t.Fatalf("REPLICATE a replica: %s", got)
	}
	if got := b.c.str("CLUSTER", "MYSHARDID"); got != a.c.str("CLUSTER", "MYSHARDID") {
		t.Fatalf("the replica's shard ID %s is not its master's", got)
	}
	if got := b.c.str("CLUSTER", "FAILOVER", "FORCE", "NOW"); !strings.HasPrefix(got, "ERR:ERR unknown subcommand or wrong number of arguments") {
		t.Fatalf("FAILOVER with two options: %s", got)
	}
	conf, err := os.ReadFile(filepath.Join(b.dir, "nodes.conf"))
	if err != nil || !strings.Contains(string(conf), " myself,slave "+a.id+" ") {
		t.Fatalf("the replica's nodes.conf: %v\n%s", err, conf)
	}
}

func TestClusterReplicasTopologyAndReads(t *testing.T) {
	sc := startShardedCluster(t)
	m0, r0 := sc.masters[0], sc.replicas[0]

	for i, m := range sc.masters {
		for _, sub := range []string{"REPLICAS", "SLAVES"} {
			v := sc.replicas[2].c.do("CLUSTER", sub, m.id)
			if len(v.Array) != 1 || !strings.HasPrefix(str(v.Array[0]), sc.replicas[i].id+" 127.0.0.1:"+strconv.Itoa(sc.replicas[i].port)+"@") ||
				!strings.Contains(str(v.Array[0]), "slave "+m.id+" ") {
				t.Fatalf("CLUSTER %s: %s", sub, str(v))
			}
		}
	}
	if got := m0.c.str("CLUSTER", "REPLICAS", r0.id); got != "ERR:ERR The specified node is not a master" {
		t.Fatalf("REPLICAS of a replica: %s", got)
	}
	if got := r0.c.str("CLUSTER", "NODES"); !strings.Contains(got, r0.id+" 127.0.0.1:"+strconv.Itoa(r0.port)+"@"+strconv.Itoa(r0.bus)+" myself,slave "+m0.id+" ") {
		t.Fatalf("CLUSTER NODES on the replica:\n%s", got)
	}

	// Writes on a master reach its replica.
	keys := make([]string, 3)
	for i, m := range sc.masters {
		keys[i] = keyInSlot(t, "rk", thirds[i][0]+7)
		if got := m.c.str("SET", keys[i], "v"+strconv.Itoa(i)); got != "OK" {
			t.Fatalf("SET: %s", got)
		}
		waitShardSynced(t, m, sc.replicas[i])
	}

	slot := thirds[0][0] + 7
	if got := r0.c.str("GET", keys[0]); got != moved(slot, m0) {
		t.Fatalf("GET on a replica without READONLY: %s", got)
	}
	if got := r0.c.str("SET", keys[0], "x"); got != moved(slot, m0) {
		t.Fatalf("SET on a replica: %s", got)
	}
	if got := r0.c.str("READONLY"); got != "OK" {
		t.Fatalf("READONLY: %s", got)
	}
	if got := r0.c.str("GET", keys[0]); got != "v0" {
		t.Fatalf("GET on a READONLY replica: %s", got)
	}
	if got := r0.c.str("SET", keys[0], "x"); got != moved(slot, m0) {
		t.Fatalf("SET on a READONLY replica: %s", got)
	}
	if got := r0.c.str("GET", keys[1]); got != moved(thirds[1][0]+7, sc.masters[1]) {
		t.Fatalf("GET of another master's slot on a READONLY replica: %s", got)
	}
	if got := r0.c.str("FLUSHALL"); got != "ERR:READONLY You can't write against a read only replica." {
		t.Fatalf("FLUSHALL on a replica: %s", got)
	}
	if got := r0.c.str("READWRITE"); got != "OK" {
		t.Fatalf("READWRITE: %s", got)
	}
	if got := r0.c.str("GET", keys[0]); got != moved(slot, m0) {
		t.Fatalf("GET after READWRITE: %s", got)
	}

	// The offsets reach the other nodes through gossip; then CLUSTER SLOTS
	// lists each replica after its master and CLUSTER SHARDS reports them.
	eventually(t, 10*time.Second, "CLUSTER SLOTS to list the replicas", func() bool {
		for _, n := range sc.all() {
			v := n.c.do("CLUSTER", "SLOTS")
			if len(v.Array) != 3 {
				return false
			}
			for i, e := range v.Array {
				if len(e.Array) != 4 || str(e.Array[2].Array[2]) != sc.masters[i].id || str(e.Array[3].Array[2]) != sc.replicas[i].id ||
					str(e.Array[3].Array[1]) != strconv.Itoa(sc.replicas[i].port) {
					return false
				}
			}
		}
		return true
	})
	eventually(t, 10*time.Second, "CLUSTER SHARDS to report the replicas", func() bool {
		v := m0.c.do("CLUSTER", "SHARDS")
		if len(v.Array) != 3 {
			return false
		}
		for _, sh := range v.Array {
			nodes := mapOf(sh)["nodes"]
			if nodes == nil || len(nodes.Array) != 2 {
				return false
			}
			master, replica := mapOf(nodes.Array[0]), mapOf(nodes.Array[1])
			if str(master["role"]) != "master" || str(replica["role"]) != "replica" || str(replica["health"]) != "online" ||
				str(replica["replication-offset"]) == "0" || str(master["replication-offset"]) == "0" {
				return false
			}
		}
		return true
	})
}

func TestClusterAutomaticFailover(t *testing.T) {
	sc := startShardedCluster(t)
	m0, r0 := sc.masters[0], sc.replicas[0]
	keys := make([]string, 50)
	for i := range keys {
		keys[i] = keyInSlot(t, "fk"+strconv.Itoa(i)+"-", thirds[0][0]+i)
		if got := m0.c.str("SET", keys[i], strconv.Itoa(i)); got != "OK" {
			t.Fatalf("SET: %s", got)
		}
	}
	waitShardSynced(t, m0, r0)
	epochBefore := m0.nodes()[m0.id].epoch

	stopReplServer(m0.s)
	alive := []*busNode{sc.masters[1], sc.masters[2], r0, sc.replicas[1], sc.replicas[2]}
	start := time.Now()
	eventually(t, 15*time.Second, "the replica to be promoted", func() bool {
		for _, n := range alive {
			l := n.nodes()[r0.id]
			if l.flags != "master" || l.slots != "0-5460" || l.epoch <= epochBefore {
				return false
			}
		}
		return sc.allOK(alive)
	})
	t.Logf("failover took %v", time.Since(start))
	for i, k := range keys {
		if got := sc.masters[1].c.str("GET", k); got != moved(thirds[0][0]+i, r0) {
			t.Fatalf("GET on another master after the failover: %s", got)
		}
		if got := r0.c.str("GET", k); got != strconv.Itoa(i) {
			t.Fatalf("GET %s on the new master: %s", k, got)
		}
	}
	if got := r0.c.str("SET", keys[0], "after"); got != "OK" {
		t.Fatalf("SET on the new master: %s", got)
	}
	if role := r0.c.info("replication")["role"]; role != "master" {
		t.Fatalf("the promoted replica's replication role: %s", role)
	}

	// The old master comes back and becomes a replica of the new one.
	m0.restart()
	sc.waitRoles([]*busNode{r0, sc.masters[1], sc.masters[2]}, []*busNode{m0, sc.replicas[1], sc.replicas[2]}, sc.all())
	waitShardSynced(t, r0, m0)
	if got := m0.c.str("GET", keys[0]); got != moved(thirds[0][0], r0) {
		t.Fatalf("GET on the old master: %s", got)
	}
	m0.c.str("READONLY")
	if got := m0.c.str("GET", keys[0]); got != "after" {
		t.Fatalf("READONLY GET on the old master: %s", got)
	}
}

func TestClusterManualFailover(t *testing.T) {
	sc := startShardedCluster(t)
	m0, r0 := sc.masters[0], sc.replicas[0]
	if got := m0.c.str("CLUSTER", "FAILOVER"); got != "ERR:ERR You should send CLUSTER FAILOVER to a replica" {
		t.Fatalf("FAILOVER on a master: %s", got)
	}

	// A writer keeps writing to the master while the roles swap; every
	// write it saw acknowledged must be on the new master.
	tag := "{" + keyInSlot(t, "mf", 42) + "}"
	writer := dialRepl(t, m0.s)
	var mu sync.Mutex
	var acked []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100000; i++ {
			key := tag + strconv.Itoa(i)
			_ = writer.conn.SetDeadline(time.Now().Add(20 * time.Second))
			if _, err := writer.conn.Write(encodeArgs("SET", key, "v")); err != nil {
				return
			}
			v, err := writer.rd.ReadValue()
			if err != nil || v.Type != resp.TypeSimpleString {
				return
			}
			mu.Lock()
			acked = append(acked, key)
			mu.Unlock()
		}
	}()
	eventually(t, 5*time.Second, "some writes", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(acked) > 50
	})
	if got := r0.c.str("CLUSTER", "FAILOVER"); got != "OK" {
		t.Fatalf("CLUSTER FAILOVER: %s", got)
	}
	sc.waitRoles([]*busNode{r0, sc.masters[1], sc.masters[2]}, []*busNode{m0, sc.replicas[1], sc.replicas[2]}, sc.all())
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the writer did not stop after the failover")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(acked) < 50 {
		t.Fatalf("only %d writes acknowledged", len(acked))
	}
	slot := 42
	for _, k := range acked {
		if got := r0.c.str("GET", k); got != "v" {
			t.Fatalf("acknowledged write %s lost after the manual failover: %s (%d acknowledged)", k, got, len(acked))
		}
	}
	if got := m0.c.str("GET", acked[0]); got != moved(slot, r0) {
		t.Fatalf("GET on the old master: %s", got)
	}
}

func TestClusterFailoverForceAndTakeover(t *testing.T) {
	sc := startShardedCluster(t)
	m0, r0 := sc.masters[0], sc.replicas[0]
	key := keyInSlot(t, "ft", 100)
	m0.c.str("SET", key, "1")
	waitShardSynced(t, m0, r0)

	if got := r0.c.str("CLUSTER", "FAILOVER", "FORCE"); got != "OK" {
		t.Fatalf("CLUSTER FAILOVER FORCE: %s", got)
	}
	sc.waitRoles([]*busNode{r0, sc.masters[1], sc.masters[2]}, []*busNode{m0, sc.replicas[1], sc.replicas[2]}, sc.all())
	if got := r0.c.str("GET", key); got != "1" {
		t.Fatalf("GET after FORCE: %s", got)
	}
	r0.c.str("SET", key, "2")
	waitShardSynced(t, r0, m0)
	epoch := r0.nodes()[r0.id].epoch

	if got := m0.c.str("CLUSTER", "FAILOVER", "TAKEOVER"); got != "OK" {
		t.Fatalf("CLUSTER FAILOVER TAKEOVER: %s", got)
	}
	sc.waitRoles(sc.masters, sc.replicas, sc.all())
	if got := m0.nodes()[m0.id].epoch; got <= epoch {
		t.Fatalf("TAKEOVER kept the config epoch %d (was %d)", got, epoch)
	}
	if got := m0.c.str("GET", key); got != "2" {
		t.Fatalf("GET after TAKEOVER: %s", got)
	}

	// A replica whose master is down needs FORCE.
	stopReplServer(sc.masters[1].s)
	r1 := sc.replicas[1]
	eventually(t, 10*time.Second, "the master to be FAIL", func() bool {
		return r1.nodes()[sc.masters[1].id].flags == "master,fail"
	})
	if got := r1.c.str("CLUSTER", "FAILOVER"); got != "ERR:ERR Master is down or failed, please use CLUSTER FAILOVER FORCE" &&
		got != "ERR:ERR You should send CLUSTER FAILOVER to a replica" {
		t.Fatalf("FAILOVER with a failed master: %s", got)
	}
}
