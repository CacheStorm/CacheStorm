package store

import (
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/logger"
)

type EvictionPolicy int

const (
	EvictionNoEviction EvictionPolicy = iota
	EvictionAllKeysLRU
	EvictionAllKeysLFU
	EvictionVolatileLRU
	EvictionAllKeysRandom
)

type EvictionController struct {
	policy     EvictionPolicy
	maxMemory  int64
	store      *Store
	tagIndex   *TagIndex
	memTracker *MemoryTracker
	sampleSize int
	onEvict    func(key string, entry *Entry)
	rnd        *rand.Rand
	rndMu      sync.Mutex
}

func NewEvictionController(policy EvictionPolicy, maxMemory int64, s *Store, mt *MemoryTracker, sampleSize int) *EvictionController {
	return &EvictionController{
		policy:     policy,
		maxMemory:  maxMemory,
		store:      s,
		memTracker: mt,
		sampleSize: sampleSize,
		rnd:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (ec *EvictionController) SetOnEvict(fn func(key string, entry *Entry)) {
	ec.onEvict = fn
}

func (ec *EvictionController) CheckAndEvict() error {
	if ec.maxMemory == 0 {
		return nil
	}

	pressure := ec.memTracker.Pressure()

	switch pressure {
	case PressureEmergency:
		logger.Warn().
			Int64("usage", ec.memTracker.Usage()).
			Int64("max", ec.maxMemory).
			Msg("memory pressure emergency, aggressive eviction")
		ec.evictUntil(0.85)
	case PressureCritical:
		logger.Warn().
			Int64("usage", ec.memTracker.Usage()).
			Int64("max", ec.maxMemory).
			Msg("memory pressure critical, evicting keys")
		ec.evictKeys(100)
	case PressureWarning:
		logger.Debug().
			Int64("usage", ec.memTracker.Usage()).
			Int64("max", ec.maxMemory).
			Msg("memory pressure warning, evicting keys")
		ec.evictKeys(10)
	}

	return nil
}

func (ec *EvictionController) evictUntil(targetPct float64) {
	target := int64(float64(ec.maxMemory) * targetPct)
	for ec.memTracker.Usage() > target {
		if !ec.evictOne() {
			break
		}
	}
}

func (ec *EvictionController) evictKeys(count int) {
	for i := 0; i < count; i++ {
		if !ec.evictOne() {
			break
		}
	}
}

func (ec *EvictionController) evictOne() bool {
	key := ec.selectVictim()
	if key == "" {
		return false
	}

	// Get entry before deleting to avoid race condition
	// where key could be re-created between Delete and Get
	var entry *Entry
	var exists bool
	if ec.onEvict != nil {
		entry, exists = ec.store.Get(key)
	}

	ec.store.Delete(key)

	if exists && ec.onEvict != nil {
		ec.onEvict(key, entry)
	}
	return true
}

func (ec *EvictionController) selectVictim() string {
	switch ec.policy {
	case EvictionAllKeysLRU:
		return ec.selectLRU()
	case EvictionAllKeysLFU:
		return ec.selectLFU()
	case EvictionVolatileLRU:
		return ec.selectVolatileLRU()
	case EvictionAllKeysRandom:
		return ec.selectRandom()
	default:
		return ""
	}
}

type candidate struct {
	key         string
	lastAccess  int64
	accessCount uint64
}

func (ec *EvictionController) selectLRU() string {
	candidates := ec.sampleKeys()
	if len(candidates) == 0 {
		return ""
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].lastAccess < candidates[j].lastAccess
	})

	return candidates[0].key
}

func (ec *EvictionController) selectLFU() string {
	candidates := ec.sampleKeys()
	if len(candidates) == 0 {
		return ""
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].accessCount < candidates[j].accessCount
	})

	return candidates[0].key
}

func (ec *EvictionController) selectVolatileLRU() string {
	candidates := ec.sampleVolatileKeys()
	if len(candidates) == 0 {
		return ec.selectLRU()
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].lastAccess < candidates[j].lastAccess
	})

	return candidates[0].key
}

func (ec *EvictionController) selectRandom() string {
	// Sample a random key from a random shard instead of materializing all keys
	ec.rndMu.Lock()
	shardIdx := ec.rnd.Intn(NumShards)
	ec.rndMu.Unlock()

	shard := ec.store.shards[shardIdx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	for key := range shard.data {
		return key
	}
	return ""
}

func (ec *EvictionController) sampleKeys() []candidate {
	candidates := make([]candidate, 0, ec.sampleSize)

	for i := 0; i < ec.sampleSize; i++ {
		ec.rndMu.Lock()
		shardIdx := ec.rnd.Intn(NumShards)
		ec.rndMu.Unlock()
		shard := ec.store.shards[shardIdx]

		shard.mu.RLock()
		if len(shard.data) == 0 {
			shard.mu.RUnlock()
			continue
		}

		for key, entry := range shard.data {
			candidates = append(candidates, candidate{
				key:         key,
				lastAccess:  entry.LastAccess.Load(),
				accessCount: entry.AccessCount.Load(),
			})
			break
		}
		shard.mu.RUnlock()
	}

	return candidates
}

func (ec *EvictionController) sampleVolatileKeys() []candidate {
	candidates := make([]candidate, 0, ec.sampleSize)

	for i := 0; i < ec.sampleSize*2 && len(candidates) < ec.sampleSize; i++ {
		ec.rndMu.Lock()
		shardIdx := ec.rnd.Intn(NumShards)
		ec.rndMu.Unlock()
		shard := ec.store.shards[shardIdx]

		shard.mu.RLock()
		for key, entry := range shard.data {
			if entry.ExpiresAt > 0 {
				candidates = append(candidates, candidate{
					key:        key,
					lastAccess: entry.LastAccess.Load(),
				})
			}
			break
		}
		shard.mu.RUnlock()
	}

	return candidates
}

func (ec *EvictionController) ForceEvict(n int) int {
	evicted := 0
	for i := 0; i < n; i++ {
		if ec.evictOne() {
			evicted++
		} else {
			break
		}
	}
	if evicted > 0 {
		logger.Info().Int("evicted", evicted).Int("requested", n).Msg("force eviction completed")
	}
	return evicted
}

