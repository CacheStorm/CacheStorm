package server

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// A write queued on a busy stripe is run by the holder before it lets go.
func TestCombineHolderRunsQueuedWriteX(t *testing.T) {
	var g writeGate
	g.stripes[7].Lock()
	ran := false
	r := newCombineReq(func() { ran = true })
	type result struct{ held, done bool }
	out := make(chan result, 1)
	go func() {
		held, done := g.combine(7, r)
		out <- result{held, done}
	}()
	waitFor(t, "write queued", func() bool { return g.queues[7].waiting.Load() == 1 })
	g.drain(7, maxCombine)
	res := <-out
	if !ran || res.held || !res.done {
		t.Fatalf("ran=%v held=%v done=%v, want the holder to run it", ran, res.held, res.done)
	}
	g.stripes[7].Unlock()
	if !g.stripes[7].TryLock() {
		t.Fatal("stripe left locked")
	}
}

// A release that finds a write still queued hands the stripe to it.
func TestCombineReleasePassesTurnX(t *testing.T) {
	var g writeGate
	h := &gateHold{g: &g}
	h.stripes = append(h.inline[:0], 3)
	h.lock()
	ran := false
	r := newCombineReq(func() { ran = true })
	out := make(chan [2]bool, 1)
	go func() {
		g.global.RLock()
		held, done := g.combine(3, r)
		out <- [2]bool{held, done}
	}()
	waitFor(t, "write queued", func() bool { return g.queues[3].waiting.Load() == 1 })
	h.unlock()
	res := <-out
	if !res[0] || res[1] || ran {
		t.Fatalf("held=%v done=%v ran=%v, want the stripe handed over unrun", res[0], res[1], ran)
	}
	if g.stripes[3].TryLock() {
		t.Fatal("stripe not held by the write it was handed to")
	}
	g.stripes[3].Unlock()
	g.global.RUnlock()
}

// A queued write that panics does not take the holder down: the panic goes
// back to the write's own goroutine.
func TestCombinePanicReturnsToWriterX(t *testing.T) {
	var g writeGate
	g.stripes[1].Lock()
	r := newCombineReq(func() { panic("boom") })
	out := make(chan any, 1)
	go func() {
		g.combine(1, r)
		out <- r.panicked
	}()
	waitFor(t, "write queued", func() bool { return g.queues[1].waiting.Load() == 1 })
	g.drain(1, maxCombine)
	if p := <-out; p != "boom" {
		t.Fatalf("panicked = %v", p)
	}
	g.stripes[1].Unlock()
}

// Many pipelining clients writing one key: every write is applied once, each
// client sees its own writes in order, and a replica ends up the same.
func TestHotKeyCombiningKeepsEveryWriteX(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	replicate(t, replica, master)

	const clients, rounds, depth = 24, 30, 16
	var wg sync.WaitGroup
	errs := make(chan error, clients)
	for c := 0; c < clients; c++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cl := dialRepl(t, master)
			last := int64(0)
			for round := 0; round < rounds; round++ {
				var batch []byte
				for i := 0; i < depth; i++ {
					switch i % 4 {
					case 0, 2:
						batch = append(batch, encodeArgs("INCR", "hot")...)
					case 1:
						batch = append(batch, encodeArgs("RPUSH", "hotlist", fmt.Sprintf("%d:%d:%d", id, round, i))...)
					case 3:
						if round%5 == 0 {
							batch = append(batch, encodeArgs("MSET", "hot2", strconv.Itoa(id), "other"+strconv.Itoa(id), "x")...)
						} else {
							batch = append(batch, encodeArgs("GET", "hot")...)
						}
					}
				}
				_ = cl.conn.SetDeadline(time.Now().Add(20 * time.Second))
				if _, err := cl.conn.Write(batch); err != nil {
					errs <- err
					return
				}
				for i := 0; i < depth; i++ {
					v, err := cl.rd.ReadValue()
					if err != nil {
						errs <- err
						return
					}
					if v.Type == resp.TypeError {
						errs <- fmt.Errorf("client %d: %s", id, v.Err)
						return
					}
					if i%4 == 0 || i%4 == 2 {
						if v.Int <= last {
							errs <- fmt.Errorf("client %d: INCR went %d -> %d", id, last, v.Int)
							return
						}
						last = v.Int
					}
				}
			}
		}(c)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	mc := dialRepl(t, master)
	incrs := clients * rounds * depth / 2
	if got := mc.str("GET", "hot"); got != strconv.Itoa(incrs) {
		t.Fatalf("hot = %s, want %d", got, incrs)
	}
	pushes := clients * rounds * depth / 4
	if got := mc.do("LLEN", "hotlist").Int; got != int64(pushes) {
		t.Fatalf("LLEN = %d, want %d", got, pushes)
	}
	// Each client's pushes are in the list in the order it sent them.
	seen := map[string]int{}
	for _, e := range mc.do("LRANGE", "hotlist", "0", "-1").Array {
		var id, round, i int
		if _, err := fmt.Sscanf(string(e.Bulk), "%d:%d:%d", &id, &round, &i); err != nil {
			t.Fatal(err)
		}
		seq := round*depth + i
		key := strconv.Itoa(id)
		if prev, ok := seen[key]; ok && seq <= prev {
			t.Fatalf("client %d: push %d after %d", id, seq, prev)
		}
		seen[key] = seq
	}

	waitSynced(t, replica, master)
	rc := dialRepl(t, replica)
	m, r := dumpAll(t, mc), dumpAll(t, rc)
	if len(m) != len(r) {
		t.Fatalf("master has %d keys, replica %d", len(m), len(r))
	}
	for k, v := range m {
		if r[k] != v {
			t.Fatalf("%s: master %q, replica %q", k, v, r[k])
		}
	}
}
