package command

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// sliceCleanup removes the entries this test created from the package-global
// registries, so the test leaves no state behind for the rest of the package's
// suite.
func sliceCleanup(t *testing.T, heapNames, windowNames, logNames []string) {
	t.Helper()
	t.Cleanup(func() {
		heapsMu.Lock()
		for _, name := range heapNames {
			delete(heaps, name)
		}
		heapsMu.Unlock()

		windowsMu.Lock()
		for _, name := range windowNames {
			delete(windows, name)
		}
		windowsMu.Unlock()

		logsXMux.Lock()
		for _, name := range logNames {
			delete(logsX, name)
		}
		logsXMux.Unlock()
	})
}

// TestHeapPeekIsSafeWhileHeapMutatorsWrite reads a heap's minimum while
// another client pushes to and pops from it.
//
// The read path took the map lock only to look the heap up and then released
// it, reading the heap's Data slice with no lock at all while HEAP.PUSH grew
// that slice and HEAP.POP shifted its elements in place under the write lock.
// That is a data race, and it can also crash the server: the reader sizes
// and indexes the slice from a header a concurrent writer had already
// replaced, so the index can point past the end of the backing array.
//
// Run with -race this pins the data race; without it, the absence of a panic
// pins the crash.
func TestHeapPeekIsSafeWhileHeapMutatorsWrite(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	sliceCleanup(t, []string{"hp-race"}, nil, nil)

	// HEAP.PUSH creates the heap when it does not exist yet.
	if reply := registryRun(t, router, s, "HEAP.PUSH", "hp-race", "1"); !strings.HasPrefix(reply, ":") {
		t.Fatalf("HEAP.PUSH reply = %q, want an integer", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Writer: HEAP.PUSH grows the slice, HEAP.POP shifts its elements in
	// place. Both run under the write lock.
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				cmd := "HEAP.PUSH"
				args := []string{"hp-race", strconv.Itoa(i)}
				if w == 1 {
					cmd = "HEAP.POP"
					args = []string{"hp-race"}
				}
				ctx, _ := bufCtx(cmd, bytesArgs(args...), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("%s: unexpected error: %v", cmd, err)
					return
				}
			}
		}()
	}

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("HEAP.PEEK", bytesArgs("hp-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("HEAP.PEEK: unexpected error: %v", err)
					return
				}
				reply := buf.String()
				if !strings.HasPrefix(reply, ":") && !strings.HasPrefix(reply, "$-1") {
					t.Errorf("HEAP.PEEK: expected an integer or null reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestHeapSizeIsSafeWhileHeapPushWrites reads a heap's size while another
// client pushes to it.
//
// HEAP.SIZE released the map lock right after the lookup and then read
// len(h.Data) with no lock at all, while HEAP.PUSH replaced that slice header
// under the write lock: a data race on the slice header.
func TestHeapSizeIsSafeWhileHeapPushWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	sliceCleanup(t, []string{"hs-race"}, nil, nil)

	if reply := registryRun(t, router, s, "HEAP.PUSH", "hs-race", "1"); !strings.HasPrefix(reply, ":") {
		t.Fatalf("HEAP.PUSH reply = %q, want an integer", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("HEAP.PUSH", bytesArgs("hs-race", strconv.Itoa(i)), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("HEAP.PUSH: unexpected error: %v", err)
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
				ctx, buf := bufCtx("HEAP.SIZE", bytesArgs("hs-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("HEAP.SIZE: unexpected error: %v", err)
					return
				}
				reply := buf.String()
				if !strings.HasPrefix(reply, ":") {
					t.Errorf("HEAP.SIZE: expected an integer reply, got %q", reply)
					return
				}
				n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(reply, ":"), "\r\n"))
				if err != nil {
					t.Errorf("HEAP.SIZE: malformed reply %q: %v", reply, err)
					return
				}
				if n < 0 || n > iterations+1 {
					t.Errorf("HEAP.SIZE returned %d, want between 0 and %d: %q", n, iterations+1, reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestWindowGetIsSafeWhileWindowAddWrites reads a window while another client
// adds values to it.
//
// The read path took the map lock only to look the window up and then
// released it, reading the window's Data slice with no lock at all while
// WINDOW.ADD grew that slice and trimmed it under the write lock. The reply
// is sized from the slice it read, so a torn header produces a reply that
// does not match the window it came from.
func TestWindowGetIsSafeWhileWindowAddWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const windowSize = 8
	const iterations = 2000

	sliceCleanup(t, nil, []string{"wg-race"}, nil)

	if reply := registryRun(t, router, s, "WINDOW.CREATE", "wg-race", strconv.Itoa(windowSize)); reply != "+OK\r\n" {
		t.Fatalf("WINDOW.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("WINDOW.ADD", bytesArgs("wg-race", "1.500000"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("WINDOW.ADD: unexpected error: %v", err)
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
				ctx, buf := bufCtx("WINDOW.GET", bytesArgs("wg-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("WINDOW.GET: unexpected error: %v", err)
					return
				}
				reply := buf.String()
				n, err := ringBufferArrayLenX(reply)
				if err != nil {
					t.Errorf("WINDOW.GET: %v", err)
					return
				}
				if n < 0 || n > windowSize {
					t.Errorf("WINDOW.GET returned %d elements, want between 0 and %d: %q",
						n, windowSize, reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestWindowAggregateIsSafeWhileWindowAddWrites aggregates a window while
// another client adds values to it.
//
// The read path released the map lock right after the lookup and then read
// the window's Data slice with no lock at all — including indexing element 0
// for the min and max aggregations — while WINDOW.ADD grew and trimmed that
// slice under the write lock.
func TestWindowAggregateIsSafeWhileWindowAddWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const windowSize = 8
	const iterations = 2000

	sliceCleanup(t, nil, []string{"wa-race"}, nil)

	if reply := registryRun(t, router, s, "WINDOW.CREATE", "wa-race", strconv.Itoa(windowSize)); reply != "+OK\r\n" {
		t.Fatalf("WINDOW.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("WINDOW.ADD", bytesArgs("wa-race", "1.500000"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("WINDOW.ADD: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			// Every aggregation is exercised: min and max index element 0
			// of the window, the read most exposed to a stale slice header.
			for i := 0; i < iterations; i++ {
				for _, agg := range []string{"sum", "avg", "min", "max"} {
					ctx, buf := bufCtx("WINDOW.AGGREGATE", bytesArgs("wa-race", agg), s)
					if err := router.Execute(ctx); err != nil {
						t.Errorf("WINDOW.AGGREGATE: unexpected error: %v", err)
						return
					}
					reply := buf.String()
					if !strings.HasPrefix(reply, "$") {
						t.Errorf("WINDOW.AGGREGATE: expected a bulk string reply, got %q", reply)
						return
					}
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestLogxReadIsSafeWhileLogxWriteAppends reads a log while another client
// appends to it.
//
// The read path released the map lock right after the lookup and then read
// the log's entry slice with no lock at all, while LOGX.WRITE appended to
// that slice under the write lock.
func TestLogxReadIsSafeWhileLogxWriteAppends(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	sliceCleanup(t, nil, nil, []string{"lx-race"})

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("LOGX.WRITE", bytesArgs("lx-race", "info", "message"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("LOGX.WRITE: unexpected error: %v", err)
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
				ctx, buf := bufCtx("LOGX.READ", bytesArgs("lx-race", "10"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("LOGX.READ: unexpected error: %v", err)
					return
				}
				reply := buf.String()
				if !strings.HasPrefix(reply, "*") {
					t.Errorf("LOGX.READ: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestLogxSearchIsSafeWhileLogxWriteAppends searches a log while another
// client appends to it.
//
// LOGX.SEARCH has the same shape as LOGX.READ: it releases the map lock right
// after the lookup and then walks the log's entry slice with no lock. The
// registry stores the slice by value, so the lookup hands the handler its own
// copy of the slice header and the walk only touches goroutine-local state
// plus the entries the writer never mutates. The test pins that conclusion
// rather than assuming it.
func TestLogxSearchIsSafeWhileLogxWriteAppends(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	sliceCleanup(t, nil, nil, []string{"ls-race"})

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("LOGX.WRITE", bytesArgs("ls-race", "info", "needle"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("LOGX.WRITE: unexpected error: %v", err)
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
				ctx, buf := bufCtx("LOGX.SEARCH", bytesArgs("ls-race", "needle"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("LOGX.SEARCH: unexpected error: %v", err)
					return
				}
				reply := buf.String()
				if !strings.HasPrefix(reply, "*") {
					t.Errorf("LOGX.SEARCH: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestHeapAndWindowSequentialReadsAreExact is the control case: with no
// concurrency the readers must report exactly what the writers recorded. It
// must pass both before and after the fix, so a broken harness cannot
// masquerade as the defect.
func TestHeapAndWindowSequentialReadsAreExact(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	sliceCleanup(t, []string{"h-ctl"}, []string{"w-ctl"}, nil)

	if reply := registryRun(t, router, s, "HEAP.PUSH", "h-ctl", "7"); !strings.HasPrefix(reply, ":") {
		t.Fatalf("HEAP.PUSH reply = %q, want an integer", reply)
	}
	for _, v := range []string{"3", "9"} {
		if reply := registryRun(t, router, s, "HEAP.PUSH", "h-ctl", v); !strings.HasPrefix(reply, ":") {
			t.Fatalf("HEAP.PUSH reply = %q, want an integer", reply)
		}
	}
	if reply := registryRun(t, router, s, "HEAP.SIZE", "h-ctl"); reply != ":3\r\n" {
		t.Fatalf("HEAP.SIZE reply = %q, want :3", reply)
	}
	if reply := registryRun(t, router, s, "HEAP.PEEK", "h-ctl"); reply != ":3\r\n" {
		t.Fatalf("HEAP.PEEK reply = %q, want :3 (the minimum)", reply)
	}

	if reply := registryRun(t, router, s, "WINDOW.CREATE", "w-ctl", "4"); reply != "+OK\r\n" {
		t.Fatalf("WINDOW.CREATE reply = %q, want +OK", reply)
	}
	for _, v := range []string{"1.000000", "2.000000"} {
		if reply := registryRun(t, router, s, "WINDOW.ADD", "w-ctl", v); reply != "+OK\r\n" {
			t.Fatalf("WINDOW.ADD reply = %q, want +OK", reply)
		}
	}
	reply := registryRun(t, router, s, "WINDOW.GET", "w-ctl")
	n, err := ringBufferArrayLenX(reply)
	if err != nil {
		t.Fatalf("WINDOW.GET: %v", err)
	}
	if n != 2 {
		t.Fatalf("WINDOW.GET returned %d elements, want 2: %q", n, reply)
	}
	if reply := registryRun(t, router, s, "WINDOW.AGGREGATE", "w-ctl", "sum"); reply != "$8\r\n3.000000\r\n" {
		t.Fatalf("WINDOW.AGGREGATE reply = %q, want the sum 3.000000", reply)
	}
}
