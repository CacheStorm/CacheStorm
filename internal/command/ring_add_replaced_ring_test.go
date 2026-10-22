package command

import (
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// TestRingAddNeverMutatesAReplacedRing proves that RING.ADD cannot append to
// a ring that has already been replaced.
//
// RING.ADD looked the ring up under the registry read lock, released it and
// only then took the write lock to append the node. A RING.CREATE that
// replaced the ring in between left the append applied to the detached
// instance: the node was silently lost and the reply described a ring that
// was no longer live.
//
// The invariant: once a ring instance has been replaced it must never grow
// again.
func TestRingAddNeverMutatesAReplacedRing(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	run := func(cmd string, args ...string) string {
		ctx, buf := bufCtx(cmd, bytesArgs(args...), s)
		if err := router.Execute(ctx); err != nil {
			return "-ERR " + err.Error()
		}
		return buf.String()
	}

	if reply := run("RING.CREATE", "k", "3"); reply != "+OK\r\n" {
		t.Fatalf("RING.CREATE reply = %q, want +OK", reply)
	}

	var mu sync.Mutex
	replaced := map[*HashRing]int{}

	// The replacer replaces the ring over and over. Every instance it
	// replaces must keep the node count it had at that moment.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			ringsMu.RLock()
			inst := rings["k"]
			ringsMu.RUnlock()
			if inst == nil {
				continue
			}
			run("RING.CREATE", "k", "3")
			ringsMu.RLock()
			count := len(inst.Nodes)
			ringsMu.RUnlock()
			mu.Lock()
			if _, seen := replaced[inst]; !seen {
				replaced[inst] = count
			}
			mu.Unlock()
		}
	}()

	// The adder appends to the ring while it is being replaced.
	for i := 0; i < 20000; i++ {
		run("RING.ADD", "k", "node")
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	for inst, count := range replaced {
		ringsMu.RLock()
		now := len(inst.Nodes)
		ringsMu.RUnlock()
		if now != count {
			t.Fatalf("a replaced ring was mutated after it was replaced: node count %d, want %d", now, count)
		}
	}
}
