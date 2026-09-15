package command

import (
	"fmt"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: the multi-key set commands (SDIFF, SDIFFSTORE, SUNIONSTORE,
// SINTERSTORE, and SINTER's non-first keys) iterated set.Members without
// holding the set's RLock, while SADD/SMOVE/SREM mutate under the lock —
// a data race between every locked writer and these unlocked readers.

func newSetRaceRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterSetCommands(router)
	return router
}

func TestSetMultiKeyCommandsAreRaceFree(t *testing.T) {
	s := store.NewStore()
	router := newSetRaceRouter(s)

	if reply := runCmd(t, s, router, "SADD", "src", "a", "b", "c"); reply == "" {
		t.Fatal("SADD setup failed")
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// The locked writer: SADD mutates src under the set lock.
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			runCmd(t, s, router, "SADD", "src", fmt.Sprintf("m%d", i%7))
		}
	}()

	// The unlocked readers (pre-fix): SDIFFSTORE/SUNIONSTORE iterate
	// src.Members with no RLock.
	go func() {
		defer wg.Done()
		for i := 0; i < 150; i++ {
			runCmd(t, s, router, "SDIFFSTORE", "dst", "src", "absent")
			runCmd(t, s, router, "SUNIONSTORE", "dst2", "src")
			runCmd(t, s, router, "SDIFF", "src", "absent")
		}
	}()

	wg.Wait()
}
