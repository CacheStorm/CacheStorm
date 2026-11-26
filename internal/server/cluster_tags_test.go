package server

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ownerOf returns the index of the master of sc serving key.
func ownerOf(key string) int {
	slot := hashSlotOf(key)
	for i, r := range thirds {
		if slot >= r[0] && slot <= r[1] {
			return i
		}
	}
	return -1
}

// tagKeysPerMaster writes n keys tagged tag on each master, each on the
// master serving it, and returns them sorted.
func tagKeysPerMaster(t *testing.T, sc *shardedCluster, prefix, tag string, n int) []string {
	t.Helper()
	per := make([]int, len(sc.masters))
	var keys []string
	for i := 0; len(keys) < n*len(sc.masters); i++ {
		key := fmt.Sprintf("%s%d", prefix, i)
		o := ownerOf(key)
		if per[o] == n {
			continue
		}
		per[o]++
		if got := sc.masters[o].c.str("SET", key, "v", "TAGS", tag); got != "OK" {
			t.Fatalf("SET %s on its master = %s", key, got)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedArray(s string) string {
	s = strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
	f := strings.Fields(s)
	sort.Strings(f)
	return "[" + strings.Join(f, " ") + "]"
}

// In a cluster the tag commands reach every master from any node: a client
// sends INVALIDATE, TAGKEYS or TAGCOUNT once, as it would to one server.
func TestClusterWideTagCommandsX(t *testing.T) {
	sc := startShardedCluster(t)
	keys := tagKeysPerMaster(t, sc, "k", "t", 4)
	for _, n := range sc.all() {
		if got := n.c.str("TAGCOUNT", "t"); got != "12" {
			t.Fatalf("TAGCOUNT t on %s = %s, want 12", n.id, got)
		}
		if got := sortedArray(n.c.str("TAGKEYS", "t")); got != "["+strings.Join(keys, " ")+"]" {
			t.Fatalf("TAGKEYS t on %s = %s", n.id, got)
		}
	}
	if got := sc.masters[0].c.str("TAGCOUNT", "t", "LOCAL"); got != "4" {
		t.Fatalf("TAGCOUNT t LOCAL = %s, want 4", got)
	}
	if got := sc.replicas[1].c.str("INVALIDATE", "t"); !strings.HasPrefix(got, "ERR:READONLY") {
		t.Fatalf("INVALIDATE on a replica = %s", got)
	}
	if got := sc.masters[1].c.str("INVALIDATE", "t"); got != "12" {
		t.Fatalf("INVALIDATE t = %s, want 12", got)
	}
	for i, m := range sc.masters {
		if got := m.c.str("DBSIZE"); got != "0" {
			t.Fatalf("master %d DBSIZE after INVALIDATE = %s", i, got)
		}
		waitShardSynced(t, m, sc.replicas[i])
		if got := sc.replicas[i].c.str("DBSIZE"); got != "0" {
			t.Fatalf("replica %d DBSIZE after INVALIDATE = %s", i, got)
		}
	}
	// LOCAL keeps the command on the node it is sent to.
	tagKeysPerMaster(t, sc, "l", "u", 2)
	if got := sc.masters[2].c.str("INVALIDATE", "u", "LOCAL"); got != "2" {
		t.Fatalf("INVALIDATE u LOCAL = %s, want 2", got)
	}
	if got := sc.masters[0].c.str("TAGCOUNT", "u"); got != "4" {
		t.Fatalf("TAGCOUNT u after a local INVALIDATE = %s, want 4", got)
	}
	// Inside MULTI the command runs on the node only.
	m := sc.masters[0].c
	m.do("MULTI")
	m.do("INVALIDATE", "u")
	if got := m.str("EXEC"); got != "[2]" {
		t.Fatalf("EXEC INVALIDATE u = %s, want [2]", got)
	}
	// Master 1's two keys are left; master 2 dropped its own with LOCAL.
	if got := sc.masters[1].c.str("TAGCOUNT", "u"); got != "2" {
		t.Fatalf("TAGCOUNT u = %s, want 2", got)
	}
}

// A master that does not answer in time makes the command report what it
// did and which master it did not reach; sent again, it completes.
func TestClusterWideInvalidatePartialX(t *testing.T) {
	sc := startShardedCluster(t)
	tagKeysPerMaster(t, sc, "k", "t", 3)
	for _, m := range sc.masters {
		m.c.do("CONFIG", "SET", "cluster-tag-fanout-timeout", "200")
	}
	if got := sc.masters[0].c.str("CONFIG", "GET", "cluster-tag-fanout-timeout"); got != "[cluster-tag-fanout-timeout 200]" {
		t.Fatalf("CONFIG GET cluster-tag-fanout-timeout = %s", got)
	}
	slow := sc.masters[2]
	slow.c.do("CLIENT", "PAUSE", "1500", "WRITE")
	got := sc.masters[0].c.str("INVALIDATE", "t")
	if !strings.HasPrefix(got, "ERR:TAGFANOUT 6 keys deleted; not reached: "+slow.id) {
		t.Fatalf("INVALIDATE with a paused master = %s", got)
	}
	slow.c.do("CLIENT", "UNPAUSE")
	eventually(t, 5*time.Second, "the retry to complete", func() bool {
		return sc.masters[0].c.str("INVALIDATE", "t") == "0" && sc.masters[1].c.str("TAGCOUNT", "t") == "0"
	})
}

// Masters sending each other cluster-wide INVALIDATEs at once do not wait
// on each other: the command asks the others after releasing its locks.
func TestClusterWideInvalidateConcurrentX(t *testing.T) {
	sc := startShardedCluster(t)
	var wg sync.WaitGroup
	errs := make(chan string, 3)
	for i, m := range sc.masters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := dialReplPort(t, m.port)
			for j := 0; j < 100; j++ {
				key := "w" + strconv.Itoa(i) + ":" + strconv.Itoa(j)
				c.do("SET", "{"+strconv.Itoa(i)+"}"+key, "v", "TAGS", "shared")
				if got := c.str("INVALIDATE", "shared"); strings.HasPrefix(got, "ERR") {
					errs <- got
					return
				}
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("concurrent cluster-wide INVALIDATEs did not finish")
	}
	close(errs)
	for e := range errs {
		t.Fatalf("INVALIDATE: %s", e)
	}
	if got := sc.masters[0].c.str("TAGCOUNT", "shared"); got != "0" {
		t.Fatalf("TAGCOUNT shared = %s", got)
	}
}

// The hierarchy is one for the whole cluster: a link made on any node
// reaches every node, CASCADE follows it on every master, and a link that
// would close a cycle is refused wherever it is sent.
func TestClusterTagHierarchyX(t *testing.T) {
	sc := startShardedCluster(t)
	// Masters[0] serves slot 0 and orders the changes; masters[1] forwards.
	if got := sc.masters[1].c.str("TAGLINK", "p", "c"); got != "OK" {
		t.Fatalf("TAGLINK p c = %s", got)
	}
	for _, n := range sc.all() {
		n := n
		eventually(t, 5*time.Second, "the link on every node", func() bool {
			return n.c.str("TAGCHILDREN", "p") == "[c]"
		})
	}
	if got := sc.masters[2].c.str("TAGLINK", "c", "p"); !strings.HasPrefix(got, "ERR:ERR the link would make the tag its own descendant") {
		t.Fatalf("TAGLINK c p = %s", got)
	}
	tagKeysPerMaster(t, sc, "k", "c", 2)
	if got := sc.masters[2].c.str("INVALIDATE", "p", "CASCADE"); got != "6" {
		t.Fatalf("INVALIDATE p CASCADE = %s, want 6", got)
	}
	if got := sc.masters[2].c.str("TAGUNLINK", "p", "c"); got != "OK" {
		t.Fatalf("TAGUNLINK p c = %s", got)
	}
	for _, n := range sc.all() {
		n := n
		eventually(t, 5*time.Second, "the unlink on every node", func() bool {
			return n.c.str("TAGCHILDREN", "p") == "[]"
		})
	}
	// A change no master was told of (LOCAL) reaches the others through
	// the hierarchy digest the nodes exchange, and their replicas with it.
	if got := sc.masters[0].c.str("TAGLINK", "a", "b", "LOCAL"); got != "OK" {
		t.Fatalf("TAGLINK a b LOCAL = %s", got)
	}
	for _, n := range sc.all() {
		n := n
		eventually(t, 10*time.Second, "the local link to spread", func() bool {
			return n.c.str("TAGCHILDREN", "a") == "[b]"
		})
	}
	// The removed link stays removed everywhere: the merge keeps the newer
	// change.
	for _, n := range sc.all() {
		if got := n.c.str("TAGCHILDREN", "p"); got != "[]" {
			t.Fatalf("TAGCHILDREN p on %s = %s after the merge", n.id, got)
		}
	}
	if got := sc.masters[1].c.str("TAGSYNC"); !strings.HasPrefix(got, "ERR:") {
		t.Fatalf("TAGSYNC from a client = %s", got)
	}
}

// A master that was down while the hierarchy changed catches up from the
// others once it is back.
func TestClusterTagHierarchyCatchesUpX(t *testing.T) {
	nodes := startBusNodes(t, 3)
	formMesh(t, nodes)
	assignThirds(t, nodes)
	down := nodes[2]
	stopReplServer(down.s)
	got := nodes[1].c.str("TAGLINK", "p", "c")
	if got != "OK" && !strings.HasPrefix(got, "ERR:TAGFANOUT") {
		t.Fatalf("TAGLINK p c with a master down = %s", got)
	}
	if got := nodes[0].c.str("TAGCHILDREN", "p"); got != "[c]" {
		t.Fatalf("TAGCHILDREN p on the coordinator = %s", got)
	}
	down.restart()
	eventually(t, 15*time.Second, "the restarted master to catch up", func() bool {
		return down.c.str("TAGCHILDREN", "p") == "[c]"
	})
}
