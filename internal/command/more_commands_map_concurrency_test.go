package command

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// registryRun executes cmd through the real Router and returns the raw RESP
// reply, so the test drives the production dispatch path rather than a copy
// of the logic.
func registryRun(t *testing.T, router *Router, s *store.Store, cmd string, args ...string) string {
	t.Helper()
	ctx, buf := bufCtx(cmd, bytesArgs(args...), s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: unexpected error: %v", cmd, err)
	}
	return buf.String()
}

// experimentCleanup removes the experiment this test created from the
// package-global registry, so the test leaves no state behind.
func experimentCleanup(t *testing.T, names ...string) {
	t.Helper()
	t.Cleanup(func() {
		experimentsMu.Lock()
		for _, name := range names {
			delete(experiments, name)
		}
		experimentsMu.Unlock()
	})
}

// TestExperimentResultsIsSafeWhileExperimentAssignWrites reads an experiment's
// results while another client assigns users to it.
//
// The read path took the map lock only to look the experiment up, then
// released it and iterated the experiment's Assignments map with no lock at
// all, while EXPERIMENT.ASSIGN wrote that same map under the write lock.
// Concurrent map iteration and map write is a FATAL Go runtime error ("fatal
// error: concurrent map iteration and map write"): it cannot be caught by
// recover() and it terminates the whole server process, not just the
// offending connection.
//
// Run with -race this pins the data race; without it, the absence of the
// fatal error pins the process-wide crash.
func TestExperimentResultsIsSafeWhileExperimentAssignWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	experimentCleanup(t, "exp-race")

	if reply := registryRun(t, router, s, "EXPERIMENT.CREATE", "exp-race", "alpha", "beta"); reply != "+OK\r\n" {
		t.Fatalf("EXPERIMENT.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("EXPERIMENT.ASSIGN", bytesArgs("exp-race", "user-"+strconv.Itoa(i)), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("EXPERIMENT.ASSIGN: unexpected error: %v", err)
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
				ctx, buf := bufCtx("EXPERIMENT.RESULTS", bytesArgs("exp-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("EXPERIMENT.RESULTS: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("EXPERIMENT.RESULTS: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestFreqCountIsSafeWhileFreqAddWrites reads a frequency counter while
// another client writes to it.
//
// FREQ.COUNT took the map lock only to look the counter up, then released it
// and read the counter's Items map with no lock at all, while FREQ.ADD wrote
// that same map under the write lock. Concurrent map read and map write is
// the same FATAL Go runtime error as concurrent map iteration and map write:
// it cannot be caught by recover() and it terminates the whole server process.
//
// Run with -race this pins the data race; without it, the absence of the
// fatal error pins the process-wide crash.
func TestFreqCountIsSafeWhileFreqAddWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	freqCleanupX(t, "fc-race")

	if reply := registryRun(t, router, s, "FREQ.CREATE", "fc-race"); reply != "+OK\r\n" {
		t.Fatalf("FREQ.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("FREQ.ADD", bytesArgs("fc-race", "item-"+strconv.Itoa(i)), s)
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
				ctx, buf := bufCtx("FREQ.COUNT", bytesArgs("fc-race", "item-0"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("FREQ.COUNT: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, ":") {
					t.Errorf("FREQ.COUNT: expected a RESP integer reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestExperimentResultsSequentialCountsAssignments is the control case: with
// no concurrency, EXPERIMENT.RESULTS must report the assignments. It must
// pass both before and after the fix, so a broken harness cannot masquerade
// as the defect.
func TestExperimentResultsSequentialCountsAssignments(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	experimentCleanup(t, "exp-ctl")

	if reply := registryRun(t, router, s, "EXPERIMENT.CREATE", "exp-ctl", "alpha", "beta"); reply != "+OK\r\n" {
		t.Fatalf("EXPERIMENT.CREATE reply = %q, want +OK", reply)
	}
	for _, user := range []string{"u1", "u2"} {
		if reply := registryRun(t, router, s, "EXPERIMENT.ASSIGN", "exp-ctl", user); !strings.HasPrefix(reply, "$") {
			t.Fatalf("EXPERIMENT.ASSIGN reply = %q, want a bulk string", reply)
		}
	}

	reply := registryRun(t, router, s, "EXPERIMENT.RESULTS", "exp-ctl")
	if !strings.Contains(reply, "$11\r\nassignments\r\n:2\r\n") {
		t.Fatalf("EXPERIMENT.RESULTS reply = %q, want it to report 2 assignments", reply)
	}
}

// TestFreqCountSequentialReportsCount is the control case: with no
// concurrency, FREQ.COUNT must report the count that FREQ.ADD recorded.
func TestFreqCountSequentialReportsCount(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	freqCleanupX(t, "fc-ctl")

	if reply := registryRun(t, router, s, "FREQ.CREATE", "fc-ctl"); reply != "+OK\r\n" {
		t.Fatalf("FREQ.CREATE reply = %q, want +OK", reply)
	}
	if reply := registryRun(t, router, s, "FREQ.ADD", "fc-ctl", "alpha", "3"); reply != ":3\r\n" {
		t.Fatalf("FREQ.ADD reply = %q, want :3", reply)
	}
	if reply := registryRun(t, router, s, "FREQ.COUNT", "fc-ctl", "alpha"); reply != ":3\r\n" {
		t.Fatalf("FREQ.COUNT reply = %q, want :3", reply)
	}
}
