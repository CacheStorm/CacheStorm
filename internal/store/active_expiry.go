package store

import (
	"sync"
	"time"
)

// Active expiry removes keys past their expiry that nothing reads, as
// Redis' active expire cycle does: every activeExpireInterval it samples
// the keys with an expiry of each shard in turn and removes those due,
// sampling a shard again while more than activeExpireRepeatPct of a sample
// was due, until every shard has been visited or the cycle has used
// activeExpireBudget. A key read after its expiry is removed by the read.
//
// The keys with an expiry are a set per shard (Shard.vol), so a key
// rescheduled any number of times is held once: a timing wheel held one
// entry per schedule until each schedule's slot came round, which under
// keys rewritten with an expiry grew without bound.
const (
	activeExpireInterval  = 100 * time.Millisecond
	activeExpireBudget    = 25 * time.Millisecond
	activeExpireSample    = 20
	activeExpireRepeatPct = 10
)

type activeExpiry struct {
	store  *Store
	stopCh chan struct{}
	wg     sync.WaitGroup
	// next is the shard the next cycle starts at, so a cycle cut short by
	// its budget does not visit the same shards every time.
	next int
}

func newActiveExpiry(s *Store) *activeExpiry {
	return &activeExpiry{store: s, stopCh: make(chan struct{})}
}

func (a *activeExpiry) Start() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(activeExpireInterval)
		defer ticker.Stop()
		for {
			select {
			case <-a.stopCh:
				return
			case <-ticker.C:
				a.cycle()
				a.store.activeExpireHashFields(time.Now().UnixMilli())
			}
		}
	}()
}

func (a *activeExpiry) Stop() {
	close(a.stopCh)
	a.wg.Wait()
	// A stopped cycle can be started again.
	a.stopCh = make(chan struct{})
}

// cycle runs one active expire cycle and reports whether it stopped at its
// time budget.
func (a *activeExpiry) cycle() bool {
	start := time.Now()
	defer func() { GlobalMetrics.ExpireCycleNanos.Add(int64(time.Since(start))) }()
	keys := make([]string, 0, activeExpireSample)
	for visited := 0; visited < NumShards; visited++ {
		shard := a.store.shards[a.next]
		a.next = (a.next + 1) % NumShards
		for {
			var sampled int
			keys, sampled = a.sample(shard, keys[:0], time.Now().UnixMilli())
			for _, key := range keys {
				a.expireKey(key)
			}
			if time.Since(start) >= activeExpireBudget {
				GlobalMetrics.ExpireCycleTimeCap.Add(1)
				return true
			}
			if sampled < activeExpireSample || len(keys)*100 <= sampled*activeExpireRepeatPct {
				break
			}
		}
	}
	return false
}

// sample appends to keys up to activeExpireSample keys of shard with an
// expiry and returns those due at now, with how many it looked at.
func (a *activeExpiry) sample(shard *Shard, keys []string, now int64) ([]string, int) {
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	n := 0
	// Iteration starts at a random place in the map.
	for key := range shard.vol {
		if e := shard.data[key]; e != nil {
			if at := e.ExpiresAt.Load(); at > 0 && at <= now {
				keys = append(keys, key)
			}
		}
		n++
		if n == activeExpireSample {
			break
		}
	}
	return keys, n
}

func (a *activeExpiry) expireKey(key string) {
	// DeleteIfExpired checks and removes under the shard lock, so a key
	// rewritten or persisted since it was sampled stays. Active expirations
	// count as expirations, not client misses.
	release := a.store.expiryGuard(key)
	defer release()
	if a.store.DeleteIfExpired(key) {
		GlobalMetrics.RecordExpiration()
	}
}
