package command

import (
	"fmt"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: SSCAN iterated SetValue.Members with no lock on the set, while
// SADD, SREM and SMOVE mutate the same map under SetValue.Lock(). Every
// locked writer on the same key raced SSCAN's map iteration — an unsynchronized
// map read concurrent with a map write, which is undefined behaviour and can
// abort the process with "concurrent map iteration and map write".
//
// The sibling multi-key set readers (SDIFF, SDIFFSTORE, SUNIONSTORE,
// SINTERSTORE, SINTER) were locked by an earlier round; SSCAN was not part of
// that fix and remained the only unlocked .Members reader in the file.

func newSSCANRaceRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterSetCommands(router)
	return router
}

// sscanRaceSeed fills a set wide enough that SSCAN's MATCH pass has real work
// to do, so the reader's map iteration overlaps the writers.
func sscanRaceSeed(t *testing.T, s *store.Store, router *Router, key string, n int, prefix string) {
	t.Helper()
	members := make([]string, 0, n)
	for i := 0; i < n; i++ {
		members = append(members, fmt.Sprintf("%s%03d", prefix, i))
	}
	if reply := runCmd(t, s, router, "SADD", append([]string{key}, members...)...); reply == "" {
		t.Fatalf("%s seed failed", prefix)
	}
}

// The primary case: the additive writer SADD against the unlocked reader.
func TestSSCANIsRaceFreeWithSADD(t *testing.T) {
	s := store.NewStore()
	router := newSSCANRaceRouter(s)
	sscanRaceSeed(t, s, router, "s", 256, "seed")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			runCmd(t, s, router, "SADD", "s", fmt.Sprintf("w%d", i%64))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			runCmd(t, s, router, "SSCAN", "s", "0")
			runCmd(t, s, router, "SSCAN", "s", "0", "COUNT", "1000")
		}
	}()

	wg.Wait()
}

// The secondary branch: the deleting writer SREM, which mutates the map with
// delete rather than assignment. A delete concurrent with an iteration is the
// variant most likely to surface the runtime's map-corruption abort.
func TestSSCANIsRaceFreeWithSREM(t *testing.T) {
	s := store.NewStore()
	router := newSSCANRaceRouter(s)
	sscanRaceSeed(t, s, router, "s", 256, "seed")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			runCmd(t, s, router, "SREM", "s", fmt.Sprintf("seed%03d", i%128))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			runCmd(t, s, router, "SSCAN", "s", "0", "MATCH", "seed*")
		}
	}()

	wg.Wait()
}

// Control: SSCAN on an uncontended set must still return every member and
// terminate its cursor. This proves the reader still works, so a race failure
// above is the defect rather than a broken harness.
func TestSSCANControlUncontendedStillReturnsAllMembers(t *testing.T) {
	s := store.NewStore()
	router := newSSCANRaceRouter(s)
	sscanRaceSeed(t, s, router, "s", 64, "m")

	if got := runCmd(t, s, router, "SSCAN", "s", "0", "COUNT", "1000"); got == "" {
		t.Fatal("SSCAN returned an empty reply")
	}
	if got := runCmd(t, s, router, "SCARD", "s"); got != ":64\r\n" {
		t.Fatalf("SCARD = %q, want 64 — the control set must be intact", got)
	}
}
