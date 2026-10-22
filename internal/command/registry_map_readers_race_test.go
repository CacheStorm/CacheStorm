package command

import (
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// The GRID, MERKLE, CRDT.PNCounter and DOC families read a Go map after
// releasing the registry lock. A map read or iteration that overlaps a map
// write is a FATAL Go runtime error — it cannot be recovered and it
// terminates the whole server process.
//
// Each test drives the family's writer against the family's readers. Run with
// -race the report names the exact handler and line; without it, the absence
// of the fatal error pins the process-wide crash.

// TestGridReadersAreSafeWhileGridSetWrites reads a grid while another client
// writes cells into it.
func TestGridReadersAreSafeWhileGridSetWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	const iterations = 2000

	registryCleanup(t, &gridsMu, grids, "grid-race")
	if reply := registryRun(t, router, s, "GRID.CREATE", "grid-race", "4", "4"); reply != "+OK\r\n" {
		t.Fatalf("GRID.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("GRID.SET", bytesArgs("grid-race", "1", "2", "value"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("GRID.SET: unexpected error: %v", err)
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
				// GRID.GET answers with a bulk string or null, GRID.QUERY
				// with an array.
				ctx, buf := bufCtx("GRID.GET", bytesArgs("grid-race", "1", "2"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("GRID.GET: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "$") && !strings.HasPrefix(reply, "_") {
					t.Errorf("GRID.GET: expected a bulk string or null reply, got %q", reply)
					return
				}

				ctx, buf = bufCtx("GRID.QUERY", bytesArgs("grid-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("GRID.QUERY: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("GRID.QUERY: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestMerkleReadersAreSafeWhileMerkleAddWrites reads a merkle tree while
// another client adds nodes to it.
func TestMerkleReadersAreSafeWhileMerkleAddWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	const iterations = 2000

	registryCleanup(t, &merkleTreesMu, merkleTrees, "merkle-race")
	if reply := registryRun(t, router, s, "MERKLE.CREATE", "merkle-race"); reply != "+OK\r\n" {
		t.Fatalf("MERKLE.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("MERKLE.ADD", bytesArgs("merkle-race", "payload"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("MERKLE.ADD: unexpected error: %v", err)
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
				for _, cmd := range []string{"MERKLE.VERIFY", "MERKLE.PROOF"} {
					ctx, buf := bufCtx(cmd, bytesArgs("merkle-race", "hash"), s)
					if err := router.Execute(ctx); err != nil {
						t.Errorf("%s: unexpected error: %v", cmd, err)
						return
					}
					if reply := buf.String(); !strings.HasPrefix(reply, ":") && !strings.HasPrefix(reply, "*") && !strings.HasPrefix(reply, "_") {
						t.Errorf("%s: expected an integer, array or null reply, got %q", cmd, reply)
						return
					}
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestCRDTPNCounterGetIsSafeWhileCounterIncrements reads a PN counter while
// another client increments it.
func TestCRDTPNCounterGetIsSafeWhileCounterIncrements(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	const iterations = 2000

	registryCleanup(t, &crdtPNCounterMu, crdtPNCounter, "pnc-race")
	if reply := registryRun(t, router, s, "CRDT.PNCounter.INCR", "pnc-race", "node"); reply != "+OK\r\n" {
		t.Fatalf("CRDT.PNCounter.INCR reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("CRDT.PNCounter.INCR", bytesArgs("pnc-race", "node"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("CRDT.PNCounter.INCR: unexpected error: %v", err)
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
				ctx, buf := bufCtx("CRDT.PNCounter.GET", bytesArgs("pnc-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("CRDT.PNCounter.GET: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, ":") {
					t.Errorf("CRDT.PNCounter.GET: expected an integer reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestDocReadersAreSafeWhileDocInsertWrites reads a document store while
// another client inserts documents into it.
func TestDocReadersAreSafeWhileDocInsertWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	const iterations = 2000

	registryCleanup(t, &docStoresMu, docStores, "doc-race")
	if reply := registryRun(t, router, s, "DOC.INSERT", "doc-race", "d1", "field", "value"); !strings.HasPrefix(reply, "$") {
		t.Fatalf("DOC.INSERT reply = %q, want a bulk string", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("DOC.INSERT", bytesArgs("doc-race", "d2", "field", "value"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("DOC.INSERT: unexpected error: %v", err)
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
				for _, args := range [][]string{
					{"doc-race", "field", "value"},
					{"doc-race"},
				} {
					cmd := "DOC.FIND"
					if len(args) == 1 {
						cmd = "DOC.COUNT"
					}
					ctx, buf := bufCtx(cmd, bytesArgs(args...), s)
					if err := router.Execute(ctx); err != nil {
						t.Errorf("%s: unexpected error: %v", cmd, err)
						return
					}
					if reply := buf.String(); !strings.HasPrefix(reply, "*") && !strings.HasPrefix(reply, ":") {
						t.Errorf("%s: expected a RESP array or integer reply, got %q", cmd, reply)
						return
					}
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestRegistryMapReadersAreExactSequentially is the control case: with no
// concurrency the readers must report exactly what the writers recorded.
func TestRegistryMapReadersAreExactSequentially(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)
	RegisterExtendedCommands(router)

	registryCleanup(t, &gridsMu, grids, "grid-ctl")
	registryCleanup(t, &docStoresMu, docStores, "doc-ctl")

	if reply := registryRun(t, router, s, "GRID.CREATE", "grid-ctl", "4", "4"); reply != "+OK\r\n" {
		t.Fatalf("GRID.CREATE reply = %q, want +OK", reply)
	}
	if reply := registryRun(t, router, s, "GRID.SET", "grid-ctl", "1", "2", "value"); reply != "+OK\r\n" {
		t.Fatalf("GRID.SET reply = %q, want +OK", reply)
	}
	if reply := registryRun(t, router, s, "GRID.GET", "grid-ctl", "1", "2"); !strings.Contains(reply, "value") {
		t.Fatalf("GRID.GET reply = %q, want it to contain the cell value", reply)
	}

	if reply := registryRun(t, router, s, "DOC.INSERT", "doc-ctl", "d1", "field", "value"); !strings.HasPrefix(reply, "$") {
		t.Fatalf("DOC.INSERT reply = %q, want a bulk string", reply)
	}
	if reply := registryRun(t, router, s, "DOC.COUNT", "doc-ctl"); reply != ":1\r\n" {
		t.Fatalf("DOC.COUNT reply = %q, want :1", reply)
	}
}
