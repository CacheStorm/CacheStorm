package server

import (
	"math/rand"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Clients adding to and merging HyperLogLogs that keep expiring leave the
// replica with the master's registers. PFMERGE read its destination as a
// source and then wrote it; when the destination expired in between, its
// expiry was recorded before the PFMERGE, so the replica merged without the
// registers the master had kept.
func TestConcurrentHLLMergesWithExpiryReplicateX(t *testing.T) {
	for run := 0; run < 6; run++ {
		master := startReplServer(t, nil)
		replica := startReplServer(t, nil)
		replicate(t, replica, master)
		var wg sync.WaitGroup
		for id := 0; id < 8; id++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				rng := rand.New(rand.NewSource(int64(id*100 + run)))
				c := dialRepl(t, master)
				for r := 0; r < 300; r++ {
					i, j := strconv.Itoa(rng.Intn(2)), strconv.Itoa(rng.Intn(2))
					switch rng.Intn(3) {
					case 0:
						c.do("PEXPIRE", "p"+i, strconv.Itoa(1+rng.Intn(4)))
					case 1:
						c.do("PFADD", "p"+i, "e"+strconv.Itoa(rng.Intn(500)))
					default:
						c.do("PFMERGE", "p"+i, "p"+j)
					}
				}
			}(id)
		}
		wg.Wait()
		// Let the expiries run out, and be noticed.
		time.Sleep(30 * time.Millisecond)
		mc := dialRepl(t, master)
		mc.do("EXISTS", "p0", "p1")
		want := dumpAll(t, mc)
		waitSynced(t, replica, master)
		if got := dumpAll(t, dialRepl(t, replica)); !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: the replica's HyperLogLogs differ from the master's", run)
		}
		stopReplServer(replica)
		stopReplServer(master)
	}
}
