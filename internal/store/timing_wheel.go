package store

import (
	"sync"
	"time"
)

type wheelBucket struct {
	mu   sync.Mutex
	keys map[string]int64
}

func newWheelBucket() *wheelBucket {
	return &wheelBucket{keys: make(map[string]int64)}
}

type wheelLevel struct {
	mu       sync.Mutex
	slots    []*wheelBucket
	current  int
	tickSize time.Duration
	numSlots int
}

func newWheelLevel(numSlots int, tickSize time.Duration) *wheelLevel {
	slots := make([]*wheelBucket, numSlots)
	for i := 0; i < numSlots; i++ {
		slots[i] = newWheelBucket()
	}
	return &wheelLevel{
		slots:    slots,
		current:  0,
		tickSize: tickSize,
		numSlots: numSlots,
	}
}

// wheelTickInterval is the wheel's sweep cadence. Slot pointers advance once
// per slot-second (time.Second / wheelTickInterval sweeps), and every sweep
// re-checks the current slot with a wall-clock comparison.
const wheelTickInterval = 100 * time.Millisecond

type TimingWheel struct {
	levels      [4]*wheelLevel
	farFuture   *wheelBucket
	store       *Store
	stopCh      chan struct{}
	wg          sync.WaitGroup
	mu          sync.RWMutex
	subtick     int
	hourElapsed int
}

func NewTimingWheel(s *Store) *TimingWheel {
	tw := &TimingWheel{
		store:     s,
		farFuture: newWheelBucket(),
		stopCh:    make(chan struct{}),
	}

	tw.levels[0] = newWheelLevel(3600, time.Second)
	tw.levels[1] = newWheelLevel(1440, time.Minute)
	tw.levels[2] = newWheelLevel(720, time.Hour)
	tw.levels[3] = newWheelLevel(365, 24*time.Hour)

	return tw
}

func (tw *TimingWheel) Add(key string, expiresAt int64) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	now := time.Now().UnixNano()
	duration := time.Duration(expiresAt - now)

	switch {
	case duration <= 0:
		return
	case duration < time.Hour:
		tw.addToLevel(0, key, expiresAt, duration)
	case duration < 24*time.Hour:
		tw.addToLevel(1, key, expiresAt, duration)
	case duration < 30*24*time.Hour:
		tw.addToLevel(2, key, expiresAt, duration)
	case duration < 365*24*time.Hour:
		tw.addToLevel(3, key, expiresAt, duration)
	default:
		tw.farFuture.mu.Lock()
		tw.farFuture.keys[key] = expiresAt
		tw.farFuture.mu.Unlock()
	}
}

func (tw *TimingWheel) addToLevel(level int, key string, expiresAt int64, duration time.Duration) {
	l := tw.levels[level]
	slot := int(duration / l.tickSize)
	slot = (l.current + slot) % l.numSlots
	if slot < 0 {
		slot = 0
	}
	l.slots[slot].mu.Lock()
	l.slots[slot].keys[key] = expiresAt
	l.slots[slot].mu.Unlock()
}

func (tw *TimingWheel) Remove(key string) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	for _, level := range tw.levels {
		for _, bucket := range level.slots {
			bucket.mu.Lock()
			delete(bucket.keys, key)
			bucket.mu.Unlock()
		}
	}

	tw.farFuture.mu.Lock()
	delete(tw.farFuture.keys, key)
	tw.farFuture.mu.Unlock()
}

func (tw *TimingWheel) Start() {
	tw.wg.Add(1)
	go tw.tickLoop()
	tw.wg.Add(1)
	go tw.farFutureCleanup()
}

func (tw *TimingWheel) Stop() {
	close(tw.stopCh)
	tw.wg.Wait()
	// Reset the channel so a stopped wheel can be started again.
	tw.stopCh = make(chan struct{})
}

func (tw *TimingWheel) tickLoop() {
	defer tw.wg.Done()

	ticker := time.NewTicker(wheelTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tw.stopCh:
			return
		case <-ticker.C:
			tw.tick()
		}
	}
}

func (tw *TimingWheel) tick() {
	now := time.Now().UnixNano()

	tw.levels[0].mu.Lock()
	bucket := tw.levels[0].slots[tw.levels[0].current]
	advance := false
	tw.subtick++
	if tw.subtick >= int(time.Second/wheelTickInterval) {
		tw.subtick = 0
		tw.levels[0].current = (tw.levels[0].current + 1) % tw.levels[0].numSlots
		advance = true
	}
	tw.levels[0].mu.Unlock()

	// Sweep the current slot every tick: expireBucket's wall-clock check makes
	// repeated sweeps exact for sub-second remainders, while the slot pointer
	// only advances once per slot-second — matching the slot math in Add,
	// where slot = duration / level tickSize.
	tw.expireBucket(bucket, now)

	if advance && tw.levels[0].current == 0 {
		tw.cascadeHour(now)
	}
}

// cascadeHour is called once per level-0 wrap (every hour of level-0 time):
// level 1 sweeps the sixty 1-minute slots that just elapsed, level 2 one
// 1-hour slot, and level 3 one 1-day slot per 24 wraps.
func (tw *TimingWheel) cascadeHour(now int64) {
	for i := 0; i < 60; i++ {
		tw.cascadeLevel(1, now)
	}
	tw.cascadeLevel(2, now)
	tw.hourElapsed++
	if tw.hourElapsed >= 24 {
		tw.hourElapsed = 0
		tw.cascadeLevel(3, now)
	}
}

// cascadeLevel sweeps the level's current slot — removing keys whose expiry
// has passed and moving future keys down one level as they come into range —
// then advances the pointer. Levels above 3 do not exist.
func (tw *TimingWheel) cascadeLevel(level int, now int64) {
	if level > 3 {
		return
	}

	l := tw.levels[level]
	l.mu.Lock()
	bucket := l.slots[l.current]
	l.current = (l.current + 1) % l.numSlots
	l.mu.Unlock()

	bucket.mu.Lock()
	var expired []string
	var toMove []struct {
		key       string
		expiresAt int64
	}

	for key, expiresAt := range bucket.keys {
		delete(bucket.keys, key)
		if expiresAt <= now {
			expired = append(expired, key)
		} else {
			toMove = append(toMove, struct {
				key       string
				expiresAt int64
			}{key, expiresAt})
		}
	}
	bucket.mu.Unlock()

	for _, key := range expired {
		tw.expireKey(key)
	}

	for _, item := range toMove {
		tw.addToLevel(level-1, item.key, item.expiresAt, time.Duration(item.expiresAt-now))
	}
}

func (tw *TimingWheel) expireBucket(bucket *wheelBucket, now int64) {
	bucket.mu.Lock()
	keys := make([]string, 0)
	for key, expiresAt := range bucket.keys {
		if expiresAt <= now {
			keys = append(keys, key)
		}
	}
	for _, k := range keys {
		delete(bucket.keys, k)
	}
	bucket.mu.Unlock()

	for _, key := range keys {
		tw.expireKey(key)
	}
}

func (tw *TimingWheel) expireKey(key string) {
	// DeleteIfExpired checks and removes under the shard lock: a stale
	// schedule (the key was deleted, rescheduled, or PERSISTed after it was
	// queued) must never remove a live key. Active expirations count as
	// expirations, not client misses.
	if tw.store.DeleteIfExpired(key) {
		GlobalMetrics.RecordExpiration()
	}
}

// farFutureCleanup periodically checks and cleans up expired keys in farFuture bucket
// and moves keys that are now within range to appropriate timing wheel levels
func (tw *TimingWheel) farFutureCleanup() {
	defer tw.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-tw.stopCh:
			return
		case <-ticker.C:
			tw.cleanupFarFuture()
		}
	}
}

func (tw *TimingWheel) cleanupFarFuture() {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	now := time.Now().UnixNano()

	tw.farFuture.mu.Lock()
	var expired []string
	var toMove map[string]int64

	for key, expiresAt := range tw.farFuture.keys {
		if expiresAt <= now {
			expired = append(expired, key)
		} else {
			duration := time.Duration(expiresAt - now)
			if duration < 365*24*time.Hour {
				if toMove == nil {
					toMove = make(map[string]int64)
				}
				toMove[key] = expiresAt
			}
		}
	}

	// Remove expired keys
	for _, key := range expired {
		delete(tw.farFuture.keys, key)
	}

	// Move keys that are now within range
	for key, expiresAt := range toMove {
		delete(tw.farFuture.keys, key)
		duration := time.Duration(expiresAt - now)
		switch {
		case duration < time.Hour:
			tw.addToLevel(0, key, expiresAt, duration)
		case duration < 24*time.Hour:
			tw.addToLevel(1, key, expiresAt, duration)
		case duration < 30*24*time.Hour:
			tw.addToLevel(2, key, expiresAt, duration)
		default:
			// Still in far future, keep it
			tw.farFuture.keys[key] = expiresAt
		}
	}
	tw.farFuture.mu.Unlock()

	// Expire keys that have passed their expiration time
	for _, key := range expired {
		tw.expireKey(key)
	}
}
