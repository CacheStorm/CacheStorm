package command

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// freqRun executes cmd through the real Router and returns the raw RESP reply,
// so the test drives the production dispatch path (Router.Execute ->
// cmdFREQADD / cmdFREQTOP) rather than a copy of the logic.
func freqRun(t *testing.T, router *Router, s *store.Store, cmd string, args ...string) string {
	t.Helper()
	ctx, buf := bufCtx(cmd, bytesArgs(args...), s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: unexpected error: %v", cmd, err)
	}
	return buf.String()
}

// freqCleanup removes the counter this test created from the package-global
// registry, so the test leaves no state behind for the rest of the package's
// suite.
func freqCleanup(t *testing.T, names ...string) {
	t.Helper()
	t.Cleanup(func() {
		freqsMu.Lock()
		for _, name := range names {
			delete(freqs, name)
		}
		freqsMu.Unlock()
	})
}

// TestFreqTopIsSafeWhileFreqAddWrites reads a frequency counter while
// another client writes to it.
//
// The read path took the map lock only to look the counter up, then released
// it and iterated the counter's Items map with no lock at all, while FREQ.ADD
// wrote that same map under the write lock. Concurrent map iteration and map
// write is a FATAL Go runtime error ("fatal error: concurrent map iteration
// and map write"): it cannot be caught by recover() and it terminates the
// whole server process, not just the offending connection.
//
// Run with -race this pins the data race; without it, the absence of the
// fatal error pins the process-wide crash.
func TestFreqTopIsSafeWhileFreqAddWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	freqCleanup(t, "fq-race")

	if reply := freqRun(t, router, s, "FREQ.CREATE", "fq-race"); reply != "+OK\r\n" {
		t.Fatalf("FREQ.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("FREQ.ADD", bytesArgs("fq-race", "item-"+strconv.Itoa(i)), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("FREQ.ADD: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("FREQ.TOP", bytesArgs("fq-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("FREQ.TOP: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("FREQ.TOP: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestFreqSequentialTopReportsAddedItems is the control case: with no
// concurrency, FREQ.TOP must report the items that were added, each with its
// count. It must pass both before and after the fix, so a broken harness
// cannot masquerade as the defect.
func TestFreqSequentialTopReportsAddedItems(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	freqCleanup(t, "fq-ctl")

	if reply := freqRun(t, router, s, "FREQ.CREATE", "fq-ctl"); reply != "+OK\r\n" {
		t.Fatalf("FREQ.CREATE reply = %q, want +OK", reply)
	}
	for _, item := range []string{"alpha", "beta", "gamma"} {
		if reply := freqRun(t, router, s, "FREQ.ADD", "fq-ctl", item); reply != ":1\r\n" {
			t.Fatalf("FREQ.ADD reply = %q, want :1", reply)
		}
	}

	reply := freqRun(t, router, s, "FREQ.TOP", "fq-ctl")
	if !strings.HasPrefix(reply, "*3\r\n") {
		t.Fatalf("FREQ.TOP reply = %q, want an array of 3 elements", reply)
	}
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(reply, want) {
			t.Errorf("FREQ.TOP reply %q is missing item %q", reply, want)
		}
	}
	if !strings.Contains(reply, ":1\r\n") {
		t.Errorf("FREQ.TOP reply %q does not report a count of 1", reply)
	}
}
