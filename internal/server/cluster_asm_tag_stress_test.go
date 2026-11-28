package server

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// movedPort returns the port a MOVED reply names, or 0.
func movedPort(reply string) int {
	if !strings.HasPrefix(reply, "ERR:MOVED ") {
		return 0
	}
	i := strings.LastIndexByte(reply, ':')
	port, _ := strconv.Atoi(reply[i+1:])
	return port
}

// Clients write tagged keys, retag, delete and expire them, invalidate tags
// cluster-wide and increment counters, all in slots that move from one
// master to another and back with CLUSTER MIGRATION IMPORT, following
// MOVED as they go. Afterwards every master's tag index agrees with its
// keys and holds only keys of its slots, every replica agrees with its
// master, no acknowledged increment is lost, and one INVALIDATE leaves no
// tagged key anywhere.
func TestClusterTagsDuringSlotMigrationX(t *testing.T) {
	sc := startShardedCluster(t)
	src, dst := sc.masters[0], sc.masters[1]
	var keys []string
	for slot := 100; slot < 200; slot += 4 {
		keys = append(keys, keyInSlot(t, "mk", slot))
	}
	// Keys outside the moving slots, on the first and the third master.
	stay := map[string]bool{keyInSlot(t, "mstay", 300): true, keyInSlot(t, "mfar", 12000): true}
	for k := range stay {
		keys = append(keys, k)
	}
	const writers = 4
	counters := make([]string, writers)
	for i := range counters {
		counters[i] = keyInSlot(t, "mctr", 110+i*20)
	}
	byPort := map[int]*busNode{}
	for _, n := range sc.masters {
		byPort[n.port] = n
	}

	var (
		stop  = make(chan struct{})
		wg    sync.WaitGroup
		acked = make([]int, writers)
		moves atomic.Int64
		ops   atomic.Int64
		errs  = make(chan string, writers)
	)
	for id := 0; id < writers; id++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))
			conns := map[int]*replClient{}
			conn := func(n *busNode) *replClient {
				if c := conns[n.port]; c != nil {
					return c
				}
				c := dialReplPort(t, n.port)
				conns[n.port] = c
				return c
			}
			// route remembers which master serves a key, as a cluster
			// client's slot map does, learning from MOVED.
			route := map[string]*busNode{}
			send := func(key string, cmd ...string) (string, bool) {
				n := route[key]
				if n == nil {
					n = src
					if stay[key] || key == "" {
						n = sc.masters[rng.Intn(len(sc.masters))]
					}
				}
				for try := 0; try < 10; try++ {
					got := conn(n).str(cmd...)
					if p := movedPort(got); p != 0 && byPort[p] != nil {
						moves.Add(1)
						n = byPort[p]
						if key != "" {
							route[key] = n
						}
						continue
					}
					if strings.HasPrefix(got, "ERR:TRYAGAIN") {
						time.Sleep(time.Millisecond)
						continue
					}
					if strings.HasPrefix(got, "ERR:") {
						errs <- strings.Join(cmd, " ") + " -> " + got
						return got, false
					}
					return got, true
				}
				errs <- strings.Join(cmd, " ") + ": no node took it"
				return "", false
			}
			tag := func() string { return clusterStressTags[rng.Intn(len(clusterStressTags))] }
			for r := 0; ; r++ {
				select {
				case <-stop:
					return
				default:
				}
				key := keys[rng.Intn(len(keys))]
				var ok bool
				switch rng.Intn(12) {
				case 0, 1, 2:
					_, ok = send(key, "SET", key, strconv.Itoa(r), "TAGS", tag(), tag())
				case 3:
					_, ok = send(key, "ADDTAG", key, tag())
				case 4:
					_, ok = send(key, "REMTAG", key, tag())
				case 5:
					_, ok = send(key, "DEL", key)
				case 6:
					_, ok = send(key, "PEXPIRE", key, strconv.Itoa(1+rng.Intn(5)))
				case 7:
					_, ok = send(key, "SET", key, "e", "PX", strconv.Itoa(1+rng.Intn(5)), "TAGS", tag())
				case 8:
					_, ok = send("", "INVALIDATE", tag())
				case 9:
					_, ok = send("", "INVALIDATE", tag(), "CASCADE")
				default:
					if _, ok = send(counters[id], "INCR", counters[id]); ok {
						acked[id]++
					}
				}
				if !ok {
					return
				}
				ops.Add(1)
			}
		}(id)
	}

	// Move the slots there and back while the clients write.
	time.Sleep(50 * time.Millisecond)
	for _, hop := range []struct{ to, from *busNode }{{dst, src}, {src, dst}, {dst, src}} {
		id := hop.to.c.str("CLUSTER", "MIGRATION", "IMPORT", "100", "199")
		if len(id) != 40 {
			close(stop)
			wg.Wait()
			t.Fatalf("IMPORT: %s", id)
		}
		waitMigration(t, hop.to, id, "completed")
		waitMigration(t, hop.from, id, "completed")
		time.Sleep(30 * time.Millisecond)
	}
	close(stop)
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	owner := dst
	for _, n := range sc.all() {
		eventually(t, 10*time.Second, "every node to send the slots to their owner", func() bool {
			return n == owner || n.c.str("GET", counters[0]) == moved(hashSlotOf(counters[0]), owner)
		})
	}
	time.Sleep(20 * time.Millisecond)
	t.Logf("%d commands, %d MOVED followed, increments %v", ops.Load(), moves.Load(), acked)
	if moves.Load() == 0 {
		t.Fatal("no client was redirected: the slots moved before the clients wrote")
	}

	for i, k := range counters {
		if got := owner.c.str("GET", k); got != strconv.Itoa(acked[i]) && (got != "<null>" || acked[i] != 0) {
			t.Fatalf("counter %s = %s, %d increments were acknowledged", k, got, acked[i])
		}
	}
	// Each master's index agrees with its keys, which are all of its own
	// slots, and its replica agrees with it.
	all := append(append([]string{}, keys...), counters...)
	for i, m := range sc.masters {
		for slot := 100; slot < 200; slot++ {
			if m == owner {
				continue
			}
			if n := m.c.str("CLUSTER", "COUNTKEYSINSLOT", strconv.Itoa(slot)); n != "0" {
				eventually(t, 10*time.Second, "the old owner's slots to be trimmed", func() bool {
					return m.c.str("CLUSTER", "COUNTKEYSINSLOT", strconv.Itoa(slot)) == "0"
				})
			}
		}
		state := localTagState(t, m, all)
		waitShardSynced(t, m, sc.replicas[i])
		if got := localTagState(t, sc.replicas[i], all); got != state {
			t.Fatalf("replica %d differs\nmaster:  %s\nreplica: %s", i, state, got)
		}
	}
	// One cluster-wide INVALIDATE of every tag leaves no tagged key.
	args := append([]string{"INVALIDATE"}, clusterStressTags...)
	if got := sc.masters[2].c.str(append(args, "CASCADE")...); strings.HasPrefix(got, "ERR") {
		t.Fatalf("final INVALIDATE: %s", got)
	}
	for _, n := range sc.all() {
		for _, tag := range clusterStressTags {
			if got := n.c.str("TAGCOUNT", tag); got != "0" {
				t.Fatalf("TAGCOUNT %s on %s after INVALIDATE of every tag = %s", tag, n.id, got)
			}
		}
	}
}
