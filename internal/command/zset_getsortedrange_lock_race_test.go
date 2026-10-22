package command

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: four sorted-set handlers called zset.GetSortedRange() and
// zset.Remove() with NO lock.
//
//	internal/store/sorted_set.go
//	  GetSortedRange(): for member, score := range v.Members   <- ranges the map
//	  Remove():        delete(v.Members, member)              <- deletes, unlocked
//
// Neither takes the value mutex, so the contract is "the CALLER holds the lock".
// A brace-matched lock census over sortedset_commands.go showed the siblings
// honoured it and these four did not:
//
//	cmdZRANGE       GetSortedRange x1  lockcalls=2   (correct)
//	cmdZREVRANGE    GetSortedRange x1  lockcalls=2   (correct)
//	cmdZRANGESTORE  GetSortedRange x1  lockcalls=2   (correct)
//	cmdZMPOP        GetSortedRange x1  lockcalls=3   (correct)
//	cmdZSCAN        GetSortedRange x1  lockcalls=0   (unlocked)
//	cmdZPOPMIN      GetSortedRange x1  lockcalls=0   (unlocked, also Remove)
//	cmdZPOPMAX      GetSortedRange x1  lockcalls=0   (unlocked, also Remove)
//	cmdZRANDMEMBER  GetSortedRange x1  lockcalls=0   (unlocked)
//
// Meanwhile cmdZADD/cmdZREM/cmdZINCRBY mutate the same map under zset.Lock().
// Concurrent map iteration and map write is a PROCESS-FATAL runtime error, and
// because ZPOPMIN/ZPOPMAX also delete from the map they just ranged, two
// concurrent pops could return the same member twice.
//
// ZPOPMIN/ZPOPMAX mutate, so they take the WRITE lock; ZSCAN/ZRANDMEMBER only
// read the returned slice, so their read lock spans the GetSortedRange call
// alone (a lock held across a Store call would self-deadlock).

// zsetLockTry executes without ever calling t.Fatal, so it is safe to use from
// a spawned goroutine (t.FailNow is only legal on the test goroutine).
func zsetLockTry(s *store.Store, router *Router, cmd string, args ...string) string {
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		return "ERR:" + err.Error()
	}
	return buf.String()
}

// zsetLockRun is the asserting form, for single-goroutine use only.
func zsetLockRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	out := zsetLockTry(s, router, cmd, args...)
	if strings.HasPrefix(out, "ERR:") {
		t.Fatalf("%s: Execute returned error: %s", cmd, out)
	}
	return out
}

func zsetLockRouter() *Router {
	router := NewRouter()
	RegisterSortedSetCommands(router)
	return router
}

func zsetLockSeed(t *testing.T, s *store.Store, router *Router, key string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if out := zsetLockRun(t, s, router, "ZADD", key, fmt.Sprintf("%d", i), fmt.Sprintf("m%04d", i)); out == "" {
			t.Fatalf("ZADD seed failed for %s", key)
		}
	}
}

// TestZPopMinIsRaceFreeWithZAddWriter is the primary case: ZPOPMIN both ranges
// and deletes Members while cmdZADD writes it under the write lock.
func TestZPopMinIsRaceFreeWithZAddWriter(t *testing.T) {
	s := store.NewStore()
	router := zsetLockRouter()
	zsetLockSeed(t, s, router, "z", 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			zsetLockTry(s, router, "ZADD", "z", fmt.Sprintf("%d", i%16+1), fmt.Sprintf("w%04d", i%128))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			zsetLockTry(s, router, "ZPOPMIN", "z", "4")
			zsetLockTry(s, router, "ZADD", "z", "99", fmt.Sprintf("r%04d", i%128))
		}
	}()

	wg.Wait()
}

// TestZPopMaxIsRaceFreeWithZRemWriter covers the sibling pop.
func TestZPopMaxIsRaceFreeWithZRemWriter(t *testing.T) {
	s := store.NewStore()
	router := zsetLockRouter()
	zsetLockSeed(t, s, router, "z", 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			zsetLockTry(s, router, "ZREM", "z", fmt.Sprintf("m%04d", i%128))
			zsetLockTry(s, router, "ZADD", "z", "1", fmt.Sprintf("m%04d", i%128))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			zsetLockRun(t, s, router, "ZPOPMAX", "z", "4")
		}
	}()

	wg.Wait()
}

// TestZRandMemberIsRaceFreeWithZAddWriter covers the read-only arm.
func TestZRandMemberIsRaceFreeWithZAddWriter(t *testing.T) {
	s := store.NewStore()
	router := zsetLockRouter()
	zsetLockSeed(t, s, router, "z", 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			zsetLockTry(s, router, "ZADD", "z", fmt.Sprintf("%d", i%16+1), fmt.Sprintf("w%04d", i%128))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			zsetLockTry(s, router, "ZRANDMEMBER", "z", "8")
		}
	}()

	wg.Wait()
}

// TestZScanIsRaceFreeWithZAddWriter covers the scan arm.
func TestZScanIsRaceFreeWithZAddWriter(t *testing.T) {
	s := store.NewStore()
	router := zsetLockRouter()
	zsetLockSeed(t, s, router, "z", 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			zsetLockTry(s, router, "ZADD", "z", fmt.Sprintf("%d", i%16+1), fmt.Sprintf("w%04d", i%128))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			zsetLockRun(t, s, router, "ZSCAN", "z", "0")
		}
	}()

	wg.Wait()
}

// TestZSetRangeControlsUncontended is the control: against a set nobody
// mutates, every affected command must still behave correctly, and the
// already-locked sibling ZRANGE must keep agreeing with them. This proves the
// harness reaches real sorted-set behaviour, so a race FAIL is the defect and
// not a broken setup.
func TestZSetRangeControlsUncontended(t *testing.T) {
	s := store.NewStore()
	router := zsetLockRouter()
	zsetLockRun(t, s, router, "ZADD", "z", "1", "a", "2", "b", "3", "c")

	if got := zsetLockRun(t, s, router, "ZRANGE", "z", "0", "-1"); got != "*3\r\n$1\r\na\r\n$1\r\nb\r\n$1\r\nc\r\n" {
		t.Fatalf("CONTROL FAIL: ZRANGE = %q, want a b c", got)
	}
	if got := zsetLockRun(t, s, router, "ZRANDMEMBER", "z", "COUNT", "8"); got == "" {
		t.Fatalf("CONTROL FAIL: ZRANDMEMBER returned nothing for a populated set")
	}
	if got := zsetLockRun(t, s, router, "ZSCAN", "z", "0"); got == "" {
		t.Fatalf("CONTROL FAIL: ZSCAN returned nothing for a populated set")
	}

	// ZPOPMIN pops the lowest member and removes it.
	if got := zsetLockRun(t, s, router, "ZPOPMIN", "z"); got != "*2\r\n$1\r\na\r\n$1\r\n1\r\n" {
		t.Fatalf("CONTROL FAIL: ZPOPMIN = %q, want a with score 1", got)
	}
	if got := zsetLockRun(t, s, router, "ZSCORE", "z", "a"); got != "_\r\n" {
		t.Fatalf("CONTROL FAIL: ZSCORE after ZPOPMIN = %q, want nil (member was removed)", got)
	}

	// ZPOPMAX pops the highest member.
	if got := zsetLockRun(t, s, router, "ZPOPMAX", "z"); got != "*2\r\n$1\r\nc\r\n$1\r\n3\r\n" {
		t.Fatalf("CONTROL FAIL: ZPOPMAX = %q, want c with score 3", got)
	}
	if got := zsetLockRun(t, s, router, "ZCARD", "z"); got != ":1\r\n" {
		t.Fatalf("CONTROL FAIL: ZCARD = %q, want 1 after two pops", got)
	}

	// Boundary: count 0 returns empty and removes nothing.
	if got := zsetLockRun(t, s, router, "ZPOPMIN", "z", "0"); got != "*0\r\n" {
		t.Fatalf("CONTROL FAIL: ZPOPMIN z 0 = %q, want an empty array", got)
	}
	if got := zsetLockRun(t, s, router, "ZCARD", "z"); got != ":1\r\n" {
		t.Fatalf("CONTROL FAIL: ZCARD after ZPOPMIN 0 = %q, want 1", got)
	}
}
