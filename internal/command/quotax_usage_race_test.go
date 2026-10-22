package command

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// quotaxRun executes a command through the router without touching t, so it
// can be called from the goroutines of a race test.
func quotaxRun(router *Router, s *store.Store, cmd string, args ...string) error {
	argBytes := make([][]byte, len(args))
	for i, a := range args {
		argBytes[i] = []byte(a)
	}

	buf := &bytes.Buffer{}
	ctx := NewContext(cmd, argBytes, s, resp.NewWriter(buf))
	return router.Execute(ctx)
}

// TestQuotaXUsageReadIsSynchronisedWithCheckWrite pins the concurrency
// contract of the QUOTAX family: QUOTAX.USAGE reports the counter that
// QUOTAX.CHECK maintains, so the read has to happen under the same lock as
// the write.
//
// Before the fix cmdQUOTAXUSAGE released quotasXMux.RUnlock and only then
// read q.Used and q.Limit, so the read raced with the write cmdQUOTAXCHECK
// performs under the write lock. This test only detects that under the race
// detector (`go test -race`), which is how CI runs the suite.
func TestQuotaXUsageReadIsSynchronisedWithCheckWrite(t *testing.T) {
	router := NewRouter()
	RegisterMoreCommands(router)
	s := store.NewStore()

	key := "quotax-usage-race-proof"
	t.Cleanup(func() { quotaxExec(t, router, s, "QUOTAX.DELETE", key) })

	// A limit high enough that the counter is never refused, so the writer
	// keeps writing for the whole run.
	if got := quotaxExec(t, router, s, "QUOTAX.CREATE", key, "100000000", "60000"); got != "+OK\r\n" {
		t.Fatalf("QUOTAX.CREATE reply = %q, want %q", got, "+OK\r\n")
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writers: QUOTAX.CHECK updates the counter under the write lock.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if err := quotaxRun(router, s, "QUOTAX.CHECK", key, "1"); err != nil {
					return
				}
			}
		}()
	}

	// Readers: QUOTAX.USAGE reports the counter.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if err := quotaxRun(router, s, "QUOTAX.USAGE", key); err != nil {
					return
				}
			}
		}()
	}

	time.Sleep(250 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// TestQuotaXConcurrentChecksAreSynchronised is the control for the case
// above: writers that all take the write lock never race with each other.
func TestQuotaXConcurrentChecksAreSynchronised(t *testing.T) {
	router := NewRouter()
	RegisterMoreCommands(router)
	s := store.NewStore()

	key := "quotax-check-race-control"
	t.Cleanup(func() { quotaxExec(t, router, s, "QUOTAX.DELETE", key) })

	if got := quotaxExec(t, router, s, "QUOTAX.CREATE", key, "100000000", "60000"); got != "+OK\r\n" {
		t.Fatalf("QUOTAX.CREATE reply = %q, want %q", got, "+OK\r\n")
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if err := quotaxRun(router, s, "QUOTAX.CHECK", key, "1"); err != nil {
					return
				}
			}
		}()
	}

	time.Sleep(250 * time.Millisecond)
	close(stop)
	wg.Wait()
}
