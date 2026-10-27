package server

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

const busTestTimeout = 400

type busNode struct {
	t    *testing.T
	s    *Server
	c    *replClient
	dir  string
	id   string
	port int
	bus  int
}

func startBusNode(t *testing.T, dir string, mutate func(*config.Config)) *busNode {
	t.Helper()
	s, c := startClusterServer(t, dir, func(cfg *config.Config) {
		cfg.Cluster.NodeTimeout = busTestTimeout
		if mutate != nil {
			mutate(cfg)
		}
	})
	return &busNode{t: t, s: s, c: c, dir: dir, id: c.str("CLUSTER", "MYID"), port: s.testPort(), bus: s.testBusPort()}
}

func startBusNodes(t *testing.T, n int) []*busNode {
	t.Helper()
	nodes := make([]*busNode, n)
	for i := range nodes {
		nodes[i] = startBusNode(t, t.TempDir(), nil)
	}
	return nodes
}

// restart starts the node again on the same ports and data directory.
func (n *busNode) restart() {
	n.t.Helper()
	stopReplServer(n.s)
	port, bus := n.port, n.bus
	*n = *startBusNode(n.t, n.dir, func(cfg *config.Config) {
		cfg.Server.Port = port
		cfg.Cluster.BusPort = bus
	})
}

func (n *busNode) meet(o *busNode) {
	n.t.Helper()
	if got := n.c.str("CLUSTER", "MEET", "127.0.0.1", strconv.Itoa(o.port), strconv.Itoa(o.bus)); got != "OK" {
		n.t.Fatalf("CLUSTER MEET: %s", got)
	}
}

type nodeLine struct {
	id, addr, flags, master, link, slots string
	epoch                                uint64
}

func (n *busNode) nodes() map[string]nodeLine {
	n.t.Helper()
	out := map[string]nodeLine{}
	for _, line := range strings.Split(strings.TrimSpace(n.c.str("CLUSTER", "NODES")), "\n") {
		f := strings.Fields(line)
		if len(f) < 8 {
			n.t.Fatalf("CLUSTER NODES line %q", line)
		}
		epoch, _ := strconv.ParseUint(f[6], 10, 64)
		out[f[0]] = nodeLine{id: f[0], addr: f[1], flags: strings.TrimPrefix(f[2], "myself,"), master: f[3], link: f[7], slots: strings.Join(f[8:], " "), epoch: epoch}
	}
	return out
}

func (n *busNode) clusterInfo() map[string]string {
	n.t.Helper()
	out := map[string]string{}
	for _, line := range strings.Split(n.c.str("CLUSTER", "INFO"), "\r\n") {
		if k, v, ok := strings.Cut(line, ":"); ok {
			out[k] = v
		}
	}
	return out
}

// meshed reports whether every node knows every other one, by ID, over a
// connected link.
func meshed(nodes []*busNode) bool {
	for _, n := range nodes {
		lines := n.nodes()
		if len(lines) != len(nodes) {
			return false
		}
		for _, o := range nodes {
			l, ok := lines[o.id]
			if !ok || l.link != "connected" || l.flags != "master" {
				return false
			}
		}
	}
	return true
}

// converged reports whether every node has the same view of the slots and
// epochs of all of them.
func converged(nodes []*busNode) bool {
	var want string
	for i, n := range nodes {
		lines := n.nodes()
		var parts []string
		for _, l := range lines {
			parts = append(parts, l.id+" "+l.flags+" "+strconv.FormatUint(l.epoch, 10)+" "+l.slots)
		}
		sort.Strings(parts)
		got := strings.Join(parts, "\n")
		if i == 0 {
			want = got
		} else if got != want {
			return false
		}
	}
	return true
}

func formMesh(t *testing.T, nodes []*busNode) {
	t.Helper()
	for i := 1; i < len(nodes); i++ {
		nodes[i].meet(nodes[i-1])
	}
	eventually(t, 10*time.Second, "a full mesh", func() bool { return meshed(nodes) })
}

func assignThirds(t *testing.T, nodes []*busNode) {
	t.Helper()
	ranges := [][2]string{{"0", "5460"}, {"5461", "10922"}, {"10923", "16383"}}
	for i, n := range nodes {
		if got := n.c.str("CLUSTER", "ADDSLOTSRANGE", ranges[i][0], ranges[i][1]); got != "OK" {
			t.Fatalf("ADDSLOTSRANGE: %s", got)
		}
	}
	eventually(t, 10*time.Second, "slot ownership to converge", func() bool {
		if !converged(nodes) {
			return false
		}
		for _, n := range nodes {
			info := n.clusterInfo()
			if info["cluster_state"] != "ok" || info["cluster_known_nodes"] != "3" || info["cluster_size"] != "3" {
				return false
			}
		}
		return true
	})
}

func TestClusterBusMeshSlotsAndRedirects(t *testing.T) {
	nodes := startBusNodes(t, 3)
	a := nodes[0]
	if got := a.c.str("CLUSTER", "MEET", "localhost", "7000"); got != "ERR:ERR Invalid node address specified: localhost:7000" {
		t.Fatalf("MEET with a host name: %s", got)
	}
	if got := a.c.str("CLUSTER", "MEET", "127.0.0.1", "x"); got != "ERR:ERR Invalid base port specified: x" {
		t.Fatalf("MEET with a bad port: %s", got)
	}
	if got := a.c.str("CLUSTER", "MEET", "127.0.0.1", "7000", "y"); got != "ERR:ERR Invalid bus port specified: y" {
		t.Fatalf("MEET with a bad bus port: %s", got)
	}
	formMesh(t, nodes)
	assignThirds(t, nodes)

	epochs := map[uint64]bool{}
	for _, l := range a.nodes() {
		epochs[l.epoch] = true
	}
	if len(epochs) != 3 {
		t.Fatalf("config epochs are not distinct: %v", a.nodes())
	}
	for _, n := range nodes {
		lines := n.nodes()
		for _, o := range nodes {
			if want := "127.0.0.1:" + strconv.Itoa(o.port) + "@" + strconv.Itoa(o.bus); lines[o.id].addr != want {
				t.Fatalf("address of %s: %s, want %s", o.id, lines[o.id].addr, want)
			}
		}
	}

	for slot, owner := range map[int]*busNode{0: nodes[0], 5461: nodes[1], 16383: nodes[2]} {
		key := keyInSlot(t, "k", slot)
		if got := owner.c.str("SET", key, "v"); got != "OK" {
			t.Fatalf("SET on the owner: %s", got)
		}
		for _, n := range nodes {
			want := "v"
			if n != owner {
				want = "ERR:MOVED " + strconv.Itoa(slot) + " 127.0.0.1:" + strconv.Itoa(owner.port)
			}
			if got := n.c.str("GET", key); got != want {
				t.Fatalf("GET %s on %s: %s, want %s", key, n.id, got, want)
			}
		}
	}

	eventually(t, 5*time.Second, "four links per node", func() bool {
		for _, n := range nodes {
			v := n.c.do("CLUSTER", "LINKS")
			if len(v.Array) != 4 {
				return false
			}
		}
		return true
	})
	link := str(a.c.do("CLUSTER", "LINKS").Array[0])
	if !strings.HasPrefix(link, "[direction ") || !strings.Contains(link, " create-time ") || !strings.Contains(link, " send-buffer-used ") {
		t.Fatalf("CLUSTER LINKS entry: %s", link)
	}
	info := a.clusterInfo()
	if info["cluster_stats_messages_ping_sent"] == "" || info["cluster_stats_messages_pong_received"] == "" || info["cluster_stats_messages_sent"] == "0" {
		t.Fatalf("CLUSTER INFO message stats: %v", info)
	}
	if got := a.c.str("CLUSTER", "COUNT-FAILURE-REPORTS", nodes[1].id); got != "0" {
		t.Fatalf("COUNT-FAILURE-REPORTS: %s", got)
	}
	eventually(t, 5*time.Second, "nodes.conf to list the other nodes' slots", func() bool {
		conf, err := os.ReadFile(filepath.Join(a.dir, "nodes.conf"))
		return err == nil && strings.Contains(string(conf), nodes[1].id+" 127.0.0.1:"+strconv.Itoa(nodes[1].port)+"@") &&
			strings.Contains(string(conf), " 5461-10922\n") && strings.Contains(string(conf), " 10923-16383\n")
	})

	// The cluster survives a restart of a node, which keeps its config.
	nodes[2].restart()
	eventually(t, 10*time.Second, "the restarted node to rejoin", func() bool { return meshed(nodes) && converged(nodes) })
}

func TestClusterBusFailureDetection(t *testing.T) {
	nodes := startBusNodes(t, 3)
	formMesh(t, nodes)
	assignThirds(t, nodes)
	a, b, c := nodes[0], nodes[1], nodes[2]

	stopReplServer(c.s)
	sawPFail, sawReport := false, false
	eventually(t, 10*time.Second, "the stopped node to be FAIL", func() bool {
		for _, n := range []*busNode{a, b} {
			l := n.nodes()[c.id]
			if l.flags == "master,fail?" {
				sawPFail = true
			}
			if l.flags != "master,fail" || l.link != "disconnected" {
				return false
			}
		}
		return true
	})
	if !sawPFail {
		t.Log("PFAIL was not observed before FAIL")
	}
	eventually(t, 5*time.Second, "a failure report", func() bool {
		sawReport = a.c.str("CLUSTER", "COUNT-FAILURE-REPORTS", c.id) == "1"
		return sawReport
	})
	for _, n := range []*busNode{a, b} {
		info := n.clusterInfo()
		if info["cluster_state"] != "fail" || info["cluster_slots_fail"] != "5461" {
			t.Fatalf("CLUSTER INFO with a failed master: %v", info)
		}
		if got := n.c.str("GET", keyInSlot(t, "k", 0)); !strings.HasPrefix(got, "ERR:CLUSTERDOWN") {
			t.Fatalf("GET while the cluster is down: %s", got)
		}
	}

	// A FAIL master that answers again is cleared.
	c.restart()
	eventually(t, 10*time.Second, "the cluster to recover", func() bool {
		if !meshed(nodes) {
			return false
		}
		for _, n := range nodes {
			if n.clusterInfo()["cluster_state"] != "ok" {
				return false
			}
		}
		return true
	})
}

func TestClusterBusForget(t *testing.T) {
	nodes := startBusNodes(t, 3)
	formMesh(t, nodes)
	a, c := nodes[0], nodes[2]
	if got := a.c.str("CLUSTER", "FORGET", a.id); got != "ERR:ERR I tried hard but I can't forget myself..." {
		t.Fatalf("FORGET myself: %s", got)
	}
	unknown := strings.Repeat("e", 40)
	if got := a.c.str("CLUSTER", "FORGET", unknown); got != "ERR:ERR Unknown node "+unknown {
		t.Fatalf("FORGET unknown: %s", got)
	}
	if got := a.c.str("CLUSTER", "FORGET", c.id); got != "OK" {
		t.Fatalf("FORGET: %s", got)
	}
	if got := a.c.str("CLUSTER", "FORGET", c.id); got != "OK" {
		t.Fatalf("FORGET of a node forgotten already: %s", got)
	}
	// B keeps gossiping about C; the blacklist keeps A from adding it back.
	deadline := time.Now().Add(5 * busTestTimeout * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, ok := a.nodes()[c.id]; ok {
			t.Fatal("a forgotten node came back through gossip")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got := a.clusterInfo()["cluster_known_nodes"]; got != "2" {
		t.Fatalf("known nodes after FORGET: %s", got)
	}
	eventually(t, 5*time.Second, "nodes.conf to drop the forgotten node", func() bool {
		conf, err := os.ReadFile(filepath.Join(a.dir, "nodes.conf"))
		return err == nil && !strings.Contains(string(conf), c.id)
	})
}

func TestClusterBusPublish(t *testing.T) {
	nodes := startBusNodes(t, 3)
	formMesh(t, nodes)
	subs := make([]*replClient, len(nodes))
	for i, n := range nodes {
		subs[i] = dialRepl(t, n.s)
		if got := subs[i].str("SUBSCRIBE", "news"); got != "[subscribe news 1]" {
			t.Fatalf("SUBSCRIBE: %s", got)
		}
	}
	if got := nodes[0].c.str("PUBLISH", "news", "hello"); got != "1" {
		t.Fatalf("PUBLISH counts the local subscribers: %s", got)
	}
	for i, sub := range subs {
		_ = sub.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		v, err := sub.rd.ReadValue()
		if err != nil || str(v) != "[message news hello]" {
			t.Fatalf("subscriber on node %d: %v %s", i, err, str(v))
		}
	}
	// Each node delivers the message once: nothing is forwarded again.
	time.Sleep(3 * busTestTimeout * time.Millisecond)
	for i, sub := range subs {
		_ = sub.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		v, err := sub.rd.ReadValue()
		var ne net.Error
		if err == nil || !errors.As(err, &ne) || !ne.Timeout() {
			t.Fatalf("subscriber on node %d got a second delivery: %v %s", i, err, str(v))
		}
	}
}

func TestClusterBusEpochCollision(t *testing.T) {
	nodes := make([]*busNode, 2)
	for i := range nodes {
		nodes[i] = startBusNode(t, t.TempDir(), func(cfg *config.Config) { cfg.Cluster.RequireFullCoverage = false })
	}
	for _, n := range nodes {
		if got := n.c.str("CLUSTER", "SET-CONFIG-EPOCH", "5"); got != "OK" {
			t.Fatalf("SET-CONFIG-EPOCH: %s", got)
		}
		if got := n.c.str("CLUSTER", "ADDSLOTSRANGE", "0", "100"); got != "OK" {
			t.Fatalf("ADDSLOTSRANGE: %s", got)
		}
	}
	low, high := nodes[0], nodes[1]
	if low.id > high.id {
		low, high = high, low
	}
	key := keyInSlot(t, "k", 7)
	high.c.str("SET", key, "stale")
	formMesh(t, nodes)
	eventually(t, 10*time.Second, "the collision to be resolved", func() bool {
		if !converged(nodes) {
			return false
		}
		// The loser is left without slots, so it becomes a replica of the
		// winner, as in Redis.
		l := low.nodes()
		return l[low.id].epoch == 6 && l[high.id].flags == "slave" && l[low.id].slots == "0-100" && l[high.id].slots == ""
	})
	for _, n := range nodes {
		if got := n.clusterInfo()["cluster_current_epoch"]; got != "6" {
			t.Fatalf("current epoch: %s", got)
		}
	}
	eventually(t, 5*time.Second, "the loser to drop its keys", func() bool {
		return high.c.str("CLUSTER", "COUNTKEYSINSLOT", "7") == "0"
	})
	if got := high.c.str("GET", key); got != "ERR:MOVED 7 127.0.0.1:"+strconv.Itoa(low.port) {
		t.Fatalf("GET on the loser: %s", got)
	}
}

func TestClusterBusStopsItsGoroutines(t *testing.T) {
	base := runtime.NumGoroutine()
	nodes := make([]*busNode, 3)
	for i := range nodes {
		nodes[i] = startBusNode(t, t.TempDir(), nil)
	}
	formMesh(t, nodes)
	for _, n := range nodes {
		n.c.conn.Close()
		stopReplServer(n.s)
	}
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > base && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > base {
		buf := make([]byte, 1<<20)
		t.Fatalf("%d goroutines left, %d before:\n%s", got, base, buf[:runtime.Stack(buf, true)])
	}
}

func TestClusterBusHTTPJoin(t *testing.T) {
	nodes := startBusNodes(t, 2)
	h := newTestHTTPServer()
	h.srv = nodes[0].s
	post := func(body string) int {
		w := httptest.NewRecorder()
		h.handleClusterJoin(w, httptest.NewRequest("POST", "/api/cluster/join", strings.NewReader(body)))
		return w.Code
	}
	if code := post(`{"host":"no-such-host","port":7000}`); code != http.StatusBadRequest {
		t.Fatalf("join with a host name: %d", code)
	}
	if code := post(`{"host":"127.0.0.1","port":` + strconv.Itoa(nodes[1].port) + `,"bus_port":` + strconv.Itoa(nodes[1].bus) + `}`); code != http.StatusOK {
		t.Fatalf("join: %d", code)
	}
	eventually(t, 10*time.Second, "the HTTP join", func() bool { return meshed(nodes) })
}
