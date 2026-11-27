package server

import (
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var clusterStressTags = []string{"t0", "t1", "t2", "t3", "t4"}

// clusterStressKeys are keys spread over the three masters.
func clusterStressKeys() []string {
	keys := make([]string, 0, 30)
	for i := 0; len(keys) < 30; i++ {
		keys = append(keys, "ck"+strconv.Itoa(i))
	}
	return keys
}

// localTagState checks, on one node, that every key's tags and every tag's
// keys agree, using LOCAL so that nothing is asked of the other masters,
// and returns the state for comparing a master with its replica.
func localTagState(t *testing.T, n *busNode, keys []string) string {
	t.Helper()
	c := dialReplPort(t, n.port)
	c.do("READONLY")
	byTag := map[string][]string{}
	var b strings.Builder
	for _, k := range keys {
		r := c.do("TAGS", k)
		if strings.HasPrefix(str(r), "ERR:MOVED") {
			continue
		}
		var tags []string
		for _, v := range r.Array {
			tags = append(tags, string(v.Bulk))
			byTag[string(v.Bulk)] = append(byTag[string(v.Bulk)], k)
		}
		sort.Strings(tags)
		if c.do("EXISTS", k).Int == 1 {
			b.WriteString(k + "=" + c.str("GET", k) + "[" + strings.Join(tags, ",") + "] ")
		} else if len(tags) != 0 {
			t.Fatalf("%s: missing key %s has tags %v", n.id, k, tags)
		}
	}
	for _, tag := range clusterStressTags {
		var got []string
		for _, v := range c.do("TAGKEYS", tag, "LOCAL").Array {
			got = append(got, string(v.Bulk))
		}
		sort.Strings(got)
		want := byTag[tag]
		sort.Strings(want)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s: TAGKEYS %s LOCAL = %v, keys with the tag %v", n.id, tag, got, want)
		}
	}
	return b.String()
}

// hierarchyOf is every tag's children on a node.
func hierarchyOf(c *replClient) string {
	var b strings.Builder
	for _, tag := range clusterStressTags {
		b.WriteString(tag + ">" + sortedArray(c.str("TAGCHILDREN", tag)) + " ")
	}
	return b.String()
}

// Clients on every master write tagged keys (each to the master serving
// it), retag, delete and expire them, and invalidate tags and change the
// hierarchy cluster-wide, all at once. Afterwards every node has one
// hierarchy, every master's tag index agrees with its keys, every replica
// with its master, and an INVALIDATE ... CASCADE sent to any one node
// leaves no key of the tags anywhere.
func TestClusterWideTagStressX(t *testing.T) {
	sc := startShardedCluster(t)
	keys := clusterStressKeys()
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for id := 0; id < 6; id++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(id)))
			own := make([]*replClient, len(sc.masters))
			for i, m := range sc.masters {
				own[i] = dialReplPort(t, m.port)
			}
			home := own[id%len(own)]
			tag := func() string { return clusterStressTags[rng.Intn(len(clusterStressTags))] }
			for r := 0; r < 400; r++ {
				key := keys[rng.Intn(len(keys))]
				c := own[ownerOf(key)]
				var cmd []string
				switch rng.Intn(15) {
				case 0, 1, 2:
					cmd = []string{"SET", key, strconv.Itoa(r), "TAGS", tag(), tag()}
				case 3:
					cmd = []string{"ADDTAG", key, tag()}
				case 4:
					cmd = []string{"REMTAG", key, tag()}
				case 5:
					cmd = []string{"DEL", key}
				case 6:
					cmd = []string{"PEXPIRE", key, strconv.Itoa(1 + rng.Intn(5))}
				case 7:
					cmd = []string{"SET", key, "e", "PX", strconv.Itoa(1 + rng.Intn(5)), "TAGS", tag()}
				case 8, 9:
					c, cmd = home, []string{"INVALIDATE", tag()}
				case 10:
					c, cmd = home, []string{"INVALIDATE", tag(), "CASCADE"}
				case 11:
					c, cmd = home, []string{"TAGLINK", tag(), tag()}
				case 12:
					c, cmd = home, []string{"TAGUNLINK", tag(), tag()}
				case 13:
					// Made on this node only; the hierarchy digest the nodes
					// exchange spreads it.
					c, cmd = home, []string{"TAGLINK", tag(), tag(), "LOCAL"}
				default:
					c, cmd = home, []string{"TAGCOUNT", tag()}
				}
				got := c.str(cmd...)
				if strings.HasPrefix(got, "ERR:") && !strings.Contains(got, "own descendant") && !strings.Contains(got, "link a tag to itself") {
					errs <- strings.Join(cmd, " ") + " -> " + got
					return
				}
			}
		}(id)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(90 * time.Second):
		t.Fatal("the clients did not finish")
	}
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	time.Sleep(20 * time.Millisecond)

	// One hierarchy on every node.
	var views []string
	converged := func() bool {
		views = views[:0]
		for _, n := range sc.all() {
			views = append(views, n.id[:8]+": "+hierarchyOf(n.c))
		}
		for _, v := range views[1:] {
			if v[10:] != views[0][10:] {
				return false
			}
		}
		return true
	}
	deadline := time.Now().Add(15 * time.Second)
	for !converged() {
		if time.Now().After(deadline) {
			t.Fatalf("the nodes keep different hierarchies:\n%s", strings.Join(views, "\n"))
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Each master's index agrees with its keys, and its replica with it.
	for i, m := range sc.masters {
		state := localTagState(t, m, keys)
		waitShardSynced(t, m, sc.replicas[i])
		if got := localTagState(t, sc.replicas[i], keys); got != state {
			t.Fatalf("replica %d differs\nmaster:  %s\nreplica: %s", i, state, got)
		}
	}
	// A cluster-wide INVALIDATE of every tag from one node leaves none.
	args := append([]string{"INVALIDATE"}, clusterStressTags...)
	if got := sc.masters[1].c.str(append(args, "CASCADE")...); strings.HasPrefix(got, "ERR") {
		t.Fatalf("final INVALIDATE: %s", got)
	}
	for _, n := range sc.all() {
		for _, tag := range clusterStressTags {
			if got := n.c.str("TAGCOUNT", tag); got != "0" {
				t.Fatalf("TAGCOUNT %s on %s after INVALIDATE of every tag = %s", tag, n.id, got)
			}
		}
	}
	for i, m := range sc.masters {
		waitShardSynced(t, m, sc.replicas[i])
		for _, n := range []*busNode{m, sc.replicas[i]} {
			c := dialReplPort(t, n.port)
			c.do("READONLY")
			for _, k := range keys {
				if ownerOf(k) != i {
					continue
				}
				if tags := c.str("TAGS", k); tags != "[]" {
					t.Fatalf("%s: %s still has tags %s", n.id, k, tags)
				}
			}
		}
	}
}
