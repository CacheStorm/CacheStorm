package server

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func ask(slot int, n *busNode) string {
	return "ERR:ASK " + strconv.Itoa(slot) + " 127.0.0.1:" + strconv.Itoa(n.port)
}

// askingDo sends ASKING then the command on c, and returns the reply of the
// command.
func askingDo(c *replClient, args ...string) string {
	c.t.Helper()
	if got := c.str("ASKING"); got != "OK" {
		c.t.Fatalf("ASKING: %s", got)
	}
	return c.str(args...)
}

func nodesConf(t *testing.T, n *busNode) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(n.dir, "nodes.conf"))
	if err != nil {
		t.Fatalf("nodes.conf: %v", err)
	}
	return string(b)
}

// ownerEverywhere reports whether every node sees owner serving slot alone
// and holds no open slot.
func ownerEverywhere(nodes []*busNode, owner *busNode, slot int) bool {
	for _, n := range nodes {
		nodesText := n.c.str("CLUSTER", "NODES")
		if strings.Contains(nodesText, "[") {
			return false
		}
		for _, l := range n.nodes() {
			has := false
			for _, r := range strings.Fields(l.slots) {
				lo, hi, found := strings.Cut(r, "-")
				if !found {
					hi = lo
				}
				a, _ := strconv.Atoi(lo)
				b, _ := strconv.Atoi(hi)
				if slot >= a && slot <= b {
					has = true
				}
			}
			if has != (l.id == owner.id) {
				return false
			}
		}
	}
	return true
}

// TestClusterSlotMigration moves a slot by hand the way redis-cli does:
// SETSLOT IMPORTING on the target, MIGRATING on the source, GETKEYSINSLOT
// and MIGRATE KEYS in batches, then SETSLOT NODE on the target and the
// source. It checks the ASK, ASKING and TRYAGAIN replies in the middle, MOVED
// after, that values, encodings and TTLs survive, that both replicas follow
// and that nodes.conf records each step.
func TestClusterSlotMigration(t *testing.T) {
	sc := startShardedCluster(t)
	src, dst := sc.masters[0], sc.masters[1]
	srcReplica, dstReplica := sc.replicas[0], sc.replicas[1]
	slot := 1234
	tag := keyInSlot(t, "mig", slot)
	k := func(name string) string { return "{" + tag + "}" + name }

	setup := [][]string{
		{"SET", k("str"), "hello", "PX", "600000"},
		{"HSET", k("hash"), "a", "1", "b", "2"},
		{"SADD", k("intset"), "1", "2", "3"},
		{"SADD", k("set"), "x", "y", "z"},
		{"ZADD", k("zset"), "1", "a", "2", "b"},
		{"RPUSH", k("list"), "a", "b", "c"},
		{"XADD", k("stream"), "1-1", "f", "v"},
		{"SET", k("int"), "12345"},
	}
	for _, cmd := range setup {
		if got := src.c.str(cmd...); strings.HasPrefix(got, "ERR:") {
			t.Fatalf("%v: %s", cmd, got)
		}
	}
	big := []string{"HSET", k("bighash")}
	for i := 0; i < 300; i++ {
		big = append(big, "f"+strconv.Itoa(i), strings.Repeat("v", 10))
	}
	src.c.str(big...)
	for i := 0; i < 40; i++ {
		src.c.str("SET", k("n"+strconv.Itoa(i)), "v"+strconv.Itoa(i), "EX", "3600")
	}
	keys := []string{}
	for _, v := range src.c.do("KEYS", "*").Array {
		keys = append(keys, str(v))
	}
	total := len(keys)
	encoding, dump := map[string]string{}, map[string]string{}
	for _, key := range keys {
		encoding[key] = src.c.str("OBJECT", "ENCODING", key)
		dump[key] = src.c.str("DUMP", key)
	}
	waitShardSynced(t, src, srcReplica)

	if got := dst.c.str("CLUSTER", "SETSLOT", strconv.Itoa(slot), "IMPORTING", src.id); got != "OK" {
		t.Fatalf("SETSLOT IMPORTING: %s", got)
	}
	if got := src.c.str("CLUSTER", "SETSLOT", strconv.Itoa(slot), "MIGRATING", dst.id); got != "OK" {
		t.Fatalf("SETSLOT MIGRATING: %s", got)
	}
	if got := src.c.str("CLUSTER", "SETSLOT", strconv.Itoa(slot), "MIGRATING", dst.id, "extra"); got != "ERR:ERR Invalid CLUSTER SETSLOT action or number of arguments. Try CLUSTER HELP" {
		t.Fatalf("SETSLOT MIGRATING with an extra argument: %s", got)
	}
	if !strings.Contains(src.c.str("CLUSTER", "NODES"), "["+strconv.Itoa(slot)+"->-"+dst.id+"]") ||
		!strings.Contains(nodesConf(t, src), "["+strconv.Itoa(slot)+"->-"+dst.id+"]") {
		t.Fatalf("the source does not show the migrating slot:\n%s", nodesConf(t, src))
	}
	if !strings.Contains(dst.c.str("CLUSTER", "NODES"), "["+strconv.Itoa(slot)+"-<-"+src.id+"]") ||
		!strings.Contains(nodesConf(t, dst), "["+strconv.Itoa(slot)+"-<-"+src.id+"]") {
		t.Fatalf("the target does not show the importing slot:\n%s", nodesConf(t, dst))
	}
	if got := src.c.str("CLUSTER", "COUNTKEYSINSLOT", strconv.Itoa(slot)); got != strconv.Itoa(total) {
		t.Fatalf("COUNTKEYSINSLOT: %s, want %d", got, total)
	}

	// Move two keys first.
	migrate := func(ks ...string) string {
		args := append([]string{"MIGRATE", "127.0.0.1", strconv.Itoa(dst.port), "", "0", "5000", "KEYS"}, ks...)
		return src.c.str(args...)
	}
	if got := migrate(k("str"), k("hash")); got != "OK" {
		t.Fatalf("MIGRATE: %s", got)
	}
	if got := migrate(k("str")); got != "NOKEY" {
		t.Fatalf("MIGRATE of a moved key: %s", got)
	}
	if got := src.c.str("GET", k("str")); got != ask(slot, dst) {
		t.Fatalf("GET of a moved key on the source: %s", got)
	}
	if got := src.c.str("SET", k("new"), "x"); got != ask(slot, dst) {
		t.Fatalf("SET of a new key on the source: %s", got)
	}
	if got := src.c.str("GET", k("int")); got != "12345" {
		t.Fatalf("GET of a key still on the source: %s", got)
	}
	if got := src.c.str("MGET", k("str"), k("int")); got != "ERR:TRYAGAIN Multiple keys request during rehashing of slot" {
		t.Fatalf("MGET across the migration on the source: %s", got)
	}
	if got := src.c.str("MGET", k("str"), k("hash")); got != ask(slot, dst) {
		t.Fatalf("MGET of moved keys on the source: %s", got)
	}
	if got := dst.c.str("GET", k("str")); got != moved(slot, src) {
		t.Fatalf("GET on the target without ASKING: %s", got)
	}
	if got := askingDo(dst.c, "GET", k("str")); got != "hello" {
		t.Fatalf("ASKING GET on the target: %s", got)
	}
	if got := dst.c.str("GET", k("str")); got != moved(slot, src) {
		t.Fatalf("ASKING must last one command: %s", got)
	}
	if got := askingDo(dst.c, "MGET", k("str"), k("int")); got != "ERR:TRYAGAIN Multiple keys request during rehashing of slot" {
		t.Fatalf("ASKING MGET with a key still on the source: %s", got)
	}
	if got := askingDo(dst.c, "MGET", k("str"), k("str")); got != "[hello hello]" {
		t.Fatalf("ASKING MGET of one moved key: %s", got)
	}
	if got := askingDo(dst.c, "SET", k("new"), "x"); got != "OK" {
		t.Fatalf("ASKING SET of a new key on the target: %s", got)
	}
	total++
	payload := src.c.str("DUMP", k("int"))
	if got := dst.c.str("RESTORE", k("restored"), "0", payload); got != moved(slot, src) {
		t.Fatalf("RESTORE on the target: %s", got)
	}
	if got := dst.c.str("RESTORE-ASKING", k("restored"), "0", payload); got != "OK" {
		t.Fatalf("RESTORE-ASKING on the target: %s", got)
	}
	total++
	if got := sc.masters[2].c.str("RESTORE-ASKING", k("restored"), "0", payload); got != moved(slot, src) {
		t.Fatalf("RESTORE-ASKING on a third node: %s", got)
	}

	// Then the rest, ten at a time, the way redis-cli's pipeline does.
	for {
		v := src.c.do("CLUSTER", "GETKEYSINSLOT", strconv.Itoa(slot), "10")
		if len(v.Array) == 0 {
			break
		}
		batch := make([]string, len(v.Array))
		for i, e := range v.Array {
			batch[i] = str(e)
		}
		if got := migrate(batch...); got != "OK" {
			t.Fatalf("MIGRATE %v: %s", batch, got)
		}
	}
	if got := src.c.str("CLUSTER", "COUNTKEYSINSLOT", strconv.Itoa(slot)); got != "0" {
		t.Fatalf("COUNTKEYSINSLOT on the emptied source: %s", got)
	}
	if got := dst.c.str("CLUSTER", "COUNTKEYSINSLOT", strconv.Itoa(slot)); got != strconv.Itoa(total) {
		t.Fatalf("COUNTKEYSINSLOT on the target: %s, want %d", got, total)
	}

	epochBefore := dst.nodes()[dst.id].epoch
	if got := dst.c.str("CLUSTER", "SETSLOT", strconv.Itoa(slot), "NODE", dst.id); got != "OK" {
		t.Fatalf("SETSLOT NODE on the target: %s", got)
	}
	lines := dst.nodes()
	for id, l := range lines {
		if id != dst.id && l.flags == "master" && l.epoch >= lines[dst.id].epoch {
			t.Fatalf("the target's epoch %d (was %d) is not the greatest: %s has %d", lines[dst.id].epoch, epochBefore, id, l.epoch)
		}
	}
	if conf := nodesConf(t, dst); strings.Contains(conf, "[") {
		t.Fatalf("the target's nodes.conf still has an open slot:\n%s", conf)
	}
	// The PONG the target broadcast hands the slot to the source; its
	// SETSLOT NODE then only clears its MIGRATING state.
	if got := src.c.str("CLUSTER", "SETSLOT", strconv.Itoa(slot), "NODE", dst.id); got != "OK" {
		t.Fatalf("SETSLOT NODE on the source: %s", got)
	}
	eventually(t, 10*time.Second, "every node to see the target owning the slot", func() bool {
		return ownerEverywhere(sc.all(), dst, slot) && sc.allOK(sc.all())
	})
	if conf := nodesConf(t, src); strings.Contains(conf, "[") || !strings.Contains(conf, " "+strconv.Itoa(slot)+" ") && !strings.Contains(conf, " "+strconv.Itoa(slot)+"\n") {
		t.Fatalf("the source's nodes.conf:\n%s", conf)
	}

	for _, key := range keys {
		if got := src.c.str("TYPE", key); got != moved(slot, dst) {
			t.Fatalf("TYPE %s on the source after the migration: %s", key, got)
		}
		if got := dst.c.str("OBJECT", "ENCODING", key); got != encoding[key] {
			t.Fatalf("encoding of %s: %s, want %s", key, got, encoding[key])
		}
		if got := dst.c.str("DUMP", key); got != dump[key] {
			t.Fatalf("DUMP of %s changed", key)
		}
	}
	if pttl, _ := strconv.Atoi(dst.c.str("PTTL", k("str"))); pttl <= 0 || pttl > 600000 {
		t.Fatalf("PTTL of the moved key: %d", pttl)
	}
	if ttl, _ := strconv.Atoi(dst.c.str("TTL", k("n7"))); ttl <= 3500 || ttl > 3600 {
		t.Fatalf("TTL of a moved key: %d", ttl)
	}
	if got := dst.c.str("TTL", k("int")); got != "-1" {
		t.Fatalf("TTL of a moved persistent key: %s", got)
	}
	if got := sc.masters[2].c.str("GET", k("int")); got != moved(slot, dst) {
		t.Fatalf("GET on a third node: %s", got)
	}

	// The source's replica saw the DELs, the target's the RESTOREs.
	waitShardSynced(t, src, srcReplica)
	waitShardSynced(t, dst, dstReplica)
	if got := srcReplica.c.str("CLUSTER", "COUNTKEYSINSLOT", strconv.Itoa(slot)); got != "0" {
		t.Fatalf("COUNTKEYSINSLOT on the source's replica: %s", got)
	}
	if got := dstReplica.c.str("CLUSTER", "COUNTKEYSINSLOT", strconv.Itoa(slot)); got != strconv.Itoa(total) {
		t.Fatalf("COUNTKEYSINSLOT on the target's replica: %s, want %d", got, total)
	}
	if got := dstReplica.c.str("READONLY"); got != "OK" {
		t.Fatalf("READONLY: %s", got)
	}
	for _, key := range keys {
		if got := dstReplica.c.str("DUMP", key); got != dump[key] {
			t.Fatalf("DUMP of %s on the target's replica changed: %s", key, got)
		}
	}
	if pttl, _ := strconv.Atoi(dstReplica.c.str("PTTL", k("str"))); pttl <= 0 || pttl > 600000 {
		t.Fatalf("PTTL on the target's replica: %d", pttl)
	}
}

// TestClusterSetSlotNodeEmptiesMaster checks that a master giving away its
// last slot becomes a replica of the node that took it, and refuses the
// SETSLOT that follows as Redis does (redis-cli accepts that refusal).
func TestClusterSetSlotNodeEmptiesMaster(t *testing.T) {
	nodes := startBusNodes(t, 3)
	a, b, c := nodes[0], nodes[1], nodes[2]
	formMesh(t, nodes)
	for _, step := range []struct {
		n          *busNode
		start, end string
	}{{a, "0", "5460"}, {b, "5461", "16382"}, {c, "16383", "16383"}} {
		if got := step.n.c.str("CLUSTER", "ADDSLOTSRANGE", step.start, step.end); got != "OK" {
			t.Fatalf("ADDSLOTSRANGE: %s", got)
		}
	}
	eventually(t, 10*time.Second, "the slots to converge", func() bool {
		return converged(nodes) && a.clusterInfo()["cluster_state"] == "ok"
	})
	key := keyInSlot(t, "last", 16383)
	if got := c.c.str("SET", key, "v"); got != "OK" {
		t.Fatalf("SET: %s", got)
	}
	if got := b.c.str("CLUSTER", "SETSLOT", "16383", "IMPORTING", c.id); got != "OK" {
		t.Fatalf("SETSLOT IMPORTING: %s", got)
	}
	if got := c.c.str("CLUSTER", "SETSLOT", "16383", "MIGRATING", b.id); got != "OK" {
		t.Fatalf("SETSLOT MIGRATING: %s", got)
	}
	if got := c.c.str("CLUSTER", "SETSLOT", "16383", "NODE", b.id); got != "ERR:ERR Can't assign hashslot 16383 to a different node while I still hold keys for this hash slot." {
		t.Fatalf("SETSLOT NODE with keys left: %s", got)
	}
	if got := c.c.str("MIGRATE", "127.0.0.1", strconv.Itoa(b.port), key, "0", "5000"); got != "OK" {
		t.Fatalf("MIGRATE: %s", got)
	}
	if got := b.c.str("CLUSTER", "SETSLOT", "16383", "NODE", b.id); got != "OK" {
		t.Fatalf("SETSLOT NODE on the target: %s", got)
	}
	eventually(t, 10*time.Second, "the emptied master to become a replica of the target", func() bool {
		for _, n := range nodes {
			if l := n.nodes()[c.id]; l.flags != "slave" || l.master != b.id {
				return false
			}
		}
		return true
	})
	if got := c.c.str("CLUSTER", "SETSLOT", "16383", "NODE", b.id); got != "ERR:ERR Please use SETSLOT only with masters." {
		t.Fatalf("SETSLOT NODE on the emptied master: %s", got)
	}
	if strings.Contains(c.c.str("CLUSTER", "NODES"), "[") {
		t.Fatalf("the emptied master kept an open slot:\n%s", c.c.str("CLUSTER", "NODES"))
	}
	if got := b.c.str("GET", key); got != "v" {
		t.Fatalf("GET on the new owner: %s", got)
	}
	eventually(t, 10*time.Second, "the new replica to sync", func() bool {
		return c.c.info("replication")["master_link_status"] == "up"
	})
}
