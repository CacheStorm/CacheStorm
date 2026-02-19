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

type TimingWheel struct {
	levels    [4]*wheelLevel
	farFuture *wheelBucket
	store     *Store
	tagIndex  *TagIndex
	stopCh    chan struct{}
	wg        sync.WaitGroup
	mu        sync.RWMutex
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
}

func (tw *TimingWheel) Stop() {
	close(tw.stopCh)
	tw.wg.Wait()
}

func (tw *TimingWheel) tickLoop() {
	defer tw.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
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
	tw.levels[0].current = (tw.levels[0].current + 1) % tw.levels[0].numSlots
	tw.levels[0].mu.Unlock()

	tw.expireBucket(bucket, now)

	if tw.levels[0].current == 0 {
		tw.cascade(1, now)
	}
}

func (tw *TimingWheel) cascade(level int, now int64) {
	if level > 3 {
		return
	}

	tw.levels[level].mu.Lock()
	bucket := tw.levels[level].slots[tw.levels[level].current]
	tw.levels[level].current = (tw.levels[level].current + 1) % tw.levels[level].numSlots
	tw.levels[level].mu.Unlock()

	bucket.mu.Lock()
	keys := make(map[string]int64, len(bucket.keys))
	for k, v := range bucket.keys {
		keys[k] = v
	}
	bucket.keys = make(map[string]int64)
	bucket.mu.Unlock()

	for key, expiresAt := range keys {
		duration := time.Duration(expiresAt - now)
		if duration <= 0 {
			tw.expireKey(key)
		} else if level == 1 {
			tw.addToLevel(0, key, expiresAt, duration)
		} else if level == 2 {
			tw.addToLevel(1, key, expiresAt, duration)
		} else {
			tw.addToLevel(2, key, expiresAt, duration)
		}
	}

	if tw.levels[level].current == 0 {
		tw.cascade(level+1, now)
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
	tw.store.Delete(key)
	if tw.tagIndex != nil {
		tw.tagIndex.RemoveKey(key, nil)
	}
}
