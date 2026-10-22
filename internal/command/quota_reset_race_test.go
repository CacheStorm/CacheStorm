package command

import (
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// TestQuotaResetIsSafeWhileQuotaUseWrites resets a quota's counter while
// another client consumes the quota.
//
// QUOTA.RESET took the map lock only to look the quota up, released it, and
// then wrote quota.Used = 0 with no lock at all, while QUOTA.USE updated that
// counter under the write lock: a data race on the counter, and a reset that
// can be lost by a concurrent use.
//
// Run with -race this pins the data race.
func TestQuotaResetIsSafeWhileQuotaUseWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	const iterations = 2000

	key := "quota-reset-race"
	t.Cleanup(func() { quotaExec(t, router, s, "QUOTA.DELETE", key) })

	if got := quotaExec(t, router, s, "QUOTA.SET", key, "100000000", "60000"); got != "+OK\r\n" {
		t.Fatalf("QUOTA.SET reply = %q, want +OK", got)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Writer: QUOTA.USE updates the counter under the write lock.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("QUOTA.USE", bytesArgs(key, "1"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("QUOTA.USE: unexpected error: %v", err)
				return
			}
		}
	}()

	// Resetter: QUOTA.RESET zeroes the counter.
	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("QUOTA.RESET", bytesArgs(key), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("QUOTA.RESET: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, ":") {
					t.Errorf("QUOTA.RESET: expected an integer reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestQuotaListIsSafeWhileQuotaResetWrites lists the quotas while another
// client resets one of them.
//
// QUOTA.LIST read every quota's counter, and QUOTA.RESET wrote it without
// holding any lock, so the read and the write were unsynchronised: a data
// race on the counter.
func TestQuotaListIsSafeWhileQuotaResetWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	const iterations = 2000

	key := "quota-list-race"
	t.Cleanup(func() { quotaExec(t, router, s, "QUOTA.DELETE", key) })

	if got := quotaExec(t, router, s, "QUOTA.SET", key, "100000000", "60000"); got != "+OK\r\n" {
		t.Fatalf("QUOTA.SET reply = %q, want +OK", got)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Resetter: QUOTA.RESET zeroes the counter.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("QUOTA.RESET", bytesArgs(key), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("QUOTA.RESET: unexpected error: %v", err)
				return
			}
		}
	}()

	// Lister: QUOTA.LIST reports every quota's counter.
	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("QUOTA.LIST", nil, s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("QUOTA.LIST: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("QUOTA.LIST: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestQuotaResetAndListAreConsistentSequentially is the control case: with no
// concurrency the reset must clear the counter and the list must report it.
// It must pass both before and after the fix, so a broken harness cannot
// masquerade as the defect.
func TestQuotaResetAndListAreConsistentSequentially(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	key := "quota-reset-control"
	t.Cleanup(func() { quotaExec(t, router, s, "QUOTA.DELETE", key) })

	if got := quotaExec(t, router, s, "QUOTA.SET", key, "5", "60000"); got != "+OK\r\n" {
		t.Fatalf("QUOTA.SET reply = %q, want +OK", got)
	}
	if got := quotaExec(t, router, s, "QUOTA.USE", key, "2"); got != ":1\r\n" {
		t.Fatalf("QUOTA.USE reply = %q, want :1", got)
	}

	list := quotaExec(t, router, s, "QUOTA.LIST")
	if !strings.Contains(list, key) {
		t.Fatalf("QUOTA.LIST reply = %q, want it to contain the quota key", list)
	}
	if !strings.Contains(list, ":2\r\n") {
		t.Fatalf("QUOTA.LIST reply = %q, want it to report 2 used", list)
	}

	if got := quotaExec(t, router, s, "QUOTA.RESET", key); got != ":1\r\n" {
		t.Fatalf("QUOTA.RESET reply = %q, want :1", got)
	}
	list = quotaExec(t, router, s, "QUOTA.LIST")
	if !strings.Contains(list, ":0\r\n") {
		t.Fatalf("QUOTA.LIST reply = %q, want it to report 0 used after the reset", list)
	}
}
