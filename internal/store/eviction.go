package store

import (
	"math/rand"
	"sort"
	"time"
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
}

func NewEvictionController(policy EvictionPolicy, maxMemory int64, s *Store, mt *MemoryTracker, sampleSize int) *EvictionController {
	return &EvictionController{
		policy:     policy,
		maxMemory:  maxMemory,
		store:      s,
		memTracker: mt,
		sampleSize: sampleSize,
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
		ec.evictUntil(0.85)
	case PressureCritical:
		ec.evictKeys(100)
	case PressureWarning:
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

	ec.store.Delete(key)
	if ec.onEvict != nil {
		if entry, exists := ec.store.Get(key); exists {
			ec.onEvict(key, entry)
		}
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
	keys := ec.store.Keys()
	if len(keys) == 0 {
		return ""
	}
	return keys[rand.Intn(len(keys))]
}

func (ec *EvictionController) sampleKeys() []candidate {
	candidates := make([]candidate, 0, ec.sampleSize)

	for i := 0; i < ec.sampleSize; i++ {
		shardIdx := rand.Intn(NumShards)
		shard := ec.store.shards[shardIdx]

		shard.mu.RLock()
		if len(shard.data) == 0 {
			shard.mu.RUnlock()
			continue
		}

		for key, entry := range shard.data {
			candidates = append(candidates, candidate{
				key:         key,
				lastAccess:  entry.LastAccess,
				accessCount: entry.AccessCount,
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
		shardIdx := rand.Intn(NumShards)
		shard := ec.store.shards[shardIdx]

		shard.mu.RLock()
		for key, entry := range shard.data {
			if entry.ExpiresAt > 0 {
				candidates = append(candidates, candidate{
					key:        key,
					lastAccess: entry.LastAccess,
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
	return evicted
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
