package command

import (
	"sync"
	"testing"
)

// cmdCACHELOCKED copied cacheLocks[key]'s *CacheLock pointer under RLock,
// released the lock, and then read lock.ExpiresAt. cacheLocks is
// map[string]*CacheLock, so the RLock covered only the map lookup:
// cmdCACHEREFRESH could rewrite that field under the write lock while this
// handler was still reading it unlocked.
//
// Same shape as the OBJECT round (r49), narrowed to one scalar field.
//
// Only meaningful under -race, the project's CI gate.
func TestCacheLockedDoesNotRaceWithCacheRefresh(t *testing.T) {
	const key = "race-cachelock"

	router := newRouterForRace()

	t.Cleanup(func() {
		cacheLocksMu.Lock()
		delete(cacheLocks, key)
		cacheLocksMu.Unlock()
	})

	ttl := []string{"holder", "60000"}
	if err := runArrayCmd(router, append([]string{"CACHE.LOCK", key}, ttl...)...); err != nil {
		t.Fatalf("CACHE.LOCK errored: %v", err)
	}

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				_ = runArrayCmd(router, "CACHE.LOCKED", key)
			}
		}()
	}
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				_ = runArrayCmd(router, append([]string{"CACHE.REFRESH", key}, ttl...)...)
			}
		}()
	}
	wg.Wait()

	if err := runArrayCmd(router, "CACHE.LOCKED", key); err != nil {
		t.Errorf("post-race CACHE.LOCKED errored: %v", err)
	}
}
