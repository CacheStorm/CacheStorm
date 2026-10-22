package command

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdZDIFFSTORE read source sorted-set maps with NO lock while
// cmdZADD/cmdZREM mutate those exact maps under zset.Lock().
//
// A map iteration concurrent with a map write is a PROCESS-FATAL runtime error
// ("fatal error: concurrent map iteration and map write"), so a client running
// ZDIFFSTORE against a sorted set another client is writing to could kill the
// server.
//
// In-repo basis: the sibling cmdZDIFF — the same logic without STORE — locks
// every one of its iterations, and the ZUNIONSTORE/ZINTERSTORE shared helper
// locks too. ZDIFFSTORE was the one handler ranging Members unlocked.
//
// Three regions needed locking: the first-set copy, the per-key delete loop,
// and the wholesale destZset.Members = result replacement.

func zdiffStoreRouter() *Router {
	router := NewRouter()
	RegisterSortedSetCommands(router)
	return router
}

func zdiffStoreSeed(t *testing.T, s *store.Store, router *Router, key string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if out := runStoreCommand(t, s, router, "ZADD", key, fmt.Sprintf("%d", i), fmt.Sprintf("m%03d", i)); out == "" {
			t.Fatalf("ZADD seed failed for %s", key)
		}
	}
}

func runStoreCommand(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s: Execute returned error: %v", cmd, err)
	}
	return buf.String()
}

// TestZDIFFStoreIsRaceFreeWithZADDWriter is the primary case.
func TestZDIFFStoreIsRaceFreeWithZADDWriter(t *testing.T) {
	s := store.NewStore()
	router := zdiffStoreRouter()
	zdiffStoreSeed(t, s, router, "src", 256)
	zdiffStoreSeed(t, s, router, "other", 64)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			runStoreCommand(t, s, router, "ZADD", "src", fmt.Sprintf("%d", 10000+i), fmt.Sprintf("w%d", i%64))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			runStoreCommand(t, s, router, "ZDIFFSTORE", "dst", "2", "src", "other")
		}
	}()

	wg.Wait()
}

// TestZDIFFStoreIsRaceFreeWithZREMWriter is the deleting writer.
func TestZDIFFStoreIsRaceFreeWithZREMWriter(t *testing.T) {
	s := store.NewStore()
	router := zdiffStoreRouter()
	zdiffStoreSeed(t, s, router, "src", 256)
	zdiffStoreSeed(t, s, router, "other", 64)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			runStoreCommand(t, s, router, "ZREM", "src", fmt.Sprintf("m%03d", i%128))
			runStoreCommand(t, s, router, "ZADD", "src", fmt.Sprintf("%d", 20000+i), fmt.Sprintf("r%d", i%64))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			runStoreCommand(t, s, router, "ZDIFFSTORE", "dst", "2", "src", "other")
		}
	}()

	wg.Wait()
}

// TestZDIFFStoreControlsUncontended is the control: against sets nobody mutates,
// ZDIFFSTORE must still compute the correct difference, overwrite an existing
// destination, and clear the destination when the result is empty.
func TestZDIFFStoreControlsUncontended(t *testing.T) {
	s := store.NewStore()
	router := zdiffStoreRouter()

	// src = {a,b,c}, other = {b}  =>  dst = {a,c}
	runStoreCommand(t, s, router, "ZADD", "src", "1", "a", "2", "b", "3", "c")
	runStoreCommand(t, s, router, "ZADD", "other", "1", "b")

	if out := runStoreCommand(t, s, router, "ZDIFFSTORE", "dst", "2", "src", "other"); out != ":2\r\n" {
		t.Fatalf("CONTROL FAIL: ZDIFFSTORE returned %q, want 2", out)
	}
	if got := runStoreCommand(t, s, router, "ZCARD", "dst"); got != ":2\r\n" {
		t.Fatalf("CONTROL FAIL: ZCARD dst = %q, want 2", got)
	}
	if got := runStoreCommand(t, s, router, "ZSCORE", "dst", "a"); got != "$1\r\n1\r\n" {
		t.Fatalf("CONTROL FAIL: ZSCORE dst a = %q, want 1", got)
	}
	// ZSCORE on an absent member replies with a RESP null (`_`).
	if got := runStoreCommand(t, s, router, "ZSCORE", "dst", "b"); got != "_\r\n" {
		t.Fatalf("CONTROL FAIL: ZSCORE dst b = %q, want a RESP null (b is in other)", got)
	}

	// Re-running must overwrite the destination rather than accumulate.
	runStoreCommand(t, s, router, "ZADD", "src2", "1", "x")
	if got := runStoreCommand(t, s, router, "ZDIFFSTORE", "dst", "1", "src2"); got != ":1\r\n" {
		t.Fatalf("CONTROL FAIL: second ZDIFFSTORE returned %q, want 1", got)
	}
	if got := runStoreCommand(t, s, router, "ZCARD", "dst"); got != ":1\r\n" {
		t.Fatalf("CONTROL FAIL: destination was not overwritten (ZCARD = %q, want 1)", got)
	}

	// An empty result must delete the destination rather than leave it stale.
	if got := runStoreCommand(t, s, router, "ZDIFFSTORE", "dst", "2", "src2", "src2"); got != ":0\r\n" {
		t.Fatalf("CONTROL FAIL: ZDIFFSTORE over identical sets = %q, want 0", got)
	}
	if got := runStoreCommand(t, s, router, "ZCARD", "dst"); got != ":0\r\n" {
		t.Fatalf("CONTROL FAIL: destination survived an empty ZDIFFSTORE (ZCARD = %q)", got)
	}
}
