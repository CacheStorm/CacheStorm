package store

import (
	"math/rand"
	"sync"
	"sync/atomic"
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
	EvictionVolatileLFU
	EvictionVolatileRandom
	EvictionVolatileTTL
)

var evictionPolicyNames = map[EvictionPolicy]string{
	EvictionNoEviction:     "noeviction",
	EvictionAllKeysLRU:     "allkeys-lru",
	EvictionAllKeysLFU:     "allkeys-lfu",
	EvictionVolatileLRU:    "volatile-lru",
	EvictionAllKeysRandom:  "allkeys-random",
	EvictionVolatileLFU:    "volatile-lfu",
	EvictionVolatileRandom: "volatile-random",
	EvictionVolatileTTL:    "volatile-ttl",
}

func (p EvictionPolicy) String() string {
	return evictionPolicyNames[p]
}

// ParseEvictionPolicy maps a Redis maxmemory-policy name to its policy.
func ParseEvictionPolicy(name string) (EvictionPolicy, bool) {
	for p, n := range evictionPolicyNames {
		if n == name {
			return p, true
		}
	}
	return EvictionNoEviction, false
}

// UsesLFU reports whether the store evicts by access frequency, the only
// policy under which Redis tracks OBJECT FREQ.
func (s *Store) UsesLFU() bool {
	if s.evictor == nil {
		return false
	}
	p := s.evictor.Policy()
	return p == EvictionAllKeysLFU || p == EvictionVolatileLFU
}

// UsesLRU reports whether the store evicts by idle time, the policies under
// which Redis saves each key's idle time in an RDB file.
func (s *Store) UsesLRU() bool {
	if s.evictor == nil {
		return false
	}
	p := s.evictor.Policy()
	return p == EvictionAllKeysLRU || p == EvictionVolatileLRU
}

// SharesIntegers reports whether Redis would keep small integer strings as
// shared objects: not under an LRU or LFU policy with maxmemory set, where
// each value needs its own access clock.
func (s *Store) SharesIntegers() bool {
	if s.evictor == nil || s.memTracker == nil || s.memTracker.Max() <= 0 {
		return true
	}
	switch s.evictor.Policy() {
	case EvictionAllKeysLRU, EvictionVolatileLRU, EvictionAllKeysLFU, EvictionVolatileLFU:
		return false
	}
	return true
}

// evictionSettings are shared by the evictors of every database, so a runtime
// policy change reaches all of them at once.
type evictionSettings struct {
	policy     atomic.Int32
	sampleSize atomic.Int32
}

type EvictionController struct {
	settings   *evictionSettings
	store      *Store
	memTracker *MemoryTracker
	onEvict    func(key string, entry *Entry)
	onEvictMu  sync.RWMutex
	rnd        *rand.Rand
	rndMu      sync.Mutex
}

func NewEvictionController(policy EvictionPolicy, maxMemory int64, s *Store, mt *MemoryTracker, sampleSize int) *EvictionController {
	settings := &evictionSettings{}
	settings.policy.Store(int32(policy))
	if sampleSize <= 0 {
		sampleSize = 5
	}
	settings.sampleSize.Store(int32(sampleSize))
	return newEvictionController(settings, s, mt)
}

func newEvictionController(settings *evictionSettings, s *Store, mt *MemoryTracker) *EvictionController {
	return &EvictionController{
		settings:   settings,
		store:      s,
		memTracker: mt,
		rnd:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Policy returns the current eviction policy.
func (ec *EvictionController) Policy() EvictionPolicy {
	return EvictionPolicy(ec.settings.policy.Load())
}

// SetPolicy changes the eviction policy of every database sharing ec's
// settings.
func (ec *EvictionController) SetPolicy(p EvictionPolicy) {
	ec.settings.policy.Store(int32(p))
}

// SampleSize returns how many keys are sampled per eviction.
func (ec *EvictionController) SampleSize() int {
	return int(ec.settings.sampleSize.Load())
}

// SetSampleSize changes how many keys are sampled per eviction.
func (ec *EvictionController) SetSampleSize(n int) {
	if n > 0 {
		ec.settings.sampleSize.Store(int32(n))
	}
}

func (ec *EvictionController) evicts() bool {
	return ec != nil && ec.Policy() != EvictionNoEviction
}

func (ec *EvictionController) SetOnEvict(fn func(key string, entry *Entry)) {
	ec.onEvictMu.Lock()
	defer ec.onEvictMu.Unlock()
	ec.onEvict = fn
}

func (ec *EvictionController) CheckAndEvict() error {
	maxMemory := ec.memTracker.Max()
	if maxMemory <= 0 {
		return nil
	}

	pressure := ec.memTracker.Pressure()

	switch pressure {
	case PressureEmergency:
		logger.Warn().
			Int64("usage", ec.memTracker.Usage()).
			Int64("max", maxMemory).
			Msg("memory pressure emergency, aggressive eviction")
		ec.evictUntil(0.85)
	case PressureCritical:
		logger.Warn().
			Int64("usage", ec.memTracker.Usage()).
			Int64("max", maxMemory).
			Msg("memory pressure critical, evicting keys")
		ec.evictKeys(100)
	case PressureWarning:
		logger.Debug().
			Int64("usage", ec.memTracker.Usage()).
			Int64("max", maxMemory).
			Msg("memory pressure warning, evicting keys")
		ec.evictKeys(10)
	}

	return nil
}

// mayEvictAgain reports whether a write that could not reserve memory may
// evict another key, counting its evictions in remaining (-1 to start).
// Once a write has evicted freeEvictions keys the rest are bounded by the
// key count, so a write evicting keys that free nothing ends; counting keys
// locks every shard, so a write evicting a few does not.
func (s *Store) mayEvictAgain(remaining *int) bool {
	const freeEvictions = 64
	if *remaining < 0 {
		if *remaining > -freeEvictions {
			*remaining--
			return true
		}
		*remaining = int(s.KeyCount()) + 1
	}
	if *remaining == 0 {
		return false
	}
	*remaining--
	return true
}

func (ec *EvictionController) evictUntil(targetPct float64) {
	target := int64(float64(ec.memTracker.Limit()) * targetPct)
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

	// Read the victim through the shard directly: Store.Get would count this
	// internal lookup as a client hit and touch the LRU state of a key that
	// is about to be deleted.
	entry, _ := ec.store.GetShard(key).Get(key)

	if entry == nil || !ec.store.evictObserved(key, entry) {
		return false
	}
	GlobalMetrics.RecordEviction()
	ec.store.Notify(NotifyEvicted, "evicted", key)

	ec.onEvictMu.RLock()
	onEvict := ec.onEvict
	ec.onEvictMu.RUnlock()
	if onEvict != nil {
		onEvict(key, entry)
	}
	ec.store.fireEvictKey(key)
	return true
}

func (ec *EvictionController) selectVictim() string {
	switch ec.Policy() {
	case EvictionAllKeysLRU:
		return ec.selectLRU()
	case EvictionAllKeysLFU:
		return ec.selectLFU()
	case EvictionVolatileLRU:
		return pickCandidate(ec.sampleVolatileKeys(), func(a, b candidate) bool { return a.lastAccess < b.lastAccess })
	case EvictionVolatileLFU:
		return pickCandidate(ec.sampleVolatileKeys(), func(a, b candidate) bool { return a.accessCount < b.accessCount })
	case EvictionVolatileTTL:
		return pickCandidate(ec.sampleVolatileKeys(), func(a, b candidate) bool { return a.expiresAt < b.expiresAt })
	case EvictionVolatileRandom:
		return pickCandidate(ec.sampleVolatileKeys(), nil)
	case EvictionAllKeysRandom:
		return ec.selectRandom()
	default:
		return ""
	}
}

// pickCandidate returns the candidate that sorts first under less, or the
// first sampled one when less is nil (the sample is already random).
func pickCandidate(candidates []candidate, less func(a, b candidate) bool) string {
	if len(candidates) == 0 {
		return ""
	}
	best := candidates[0]
	if less != nil {
		for _, c := range candidates[1:] {
			if less(c, best) {
				best = c
			}
		}
	}
	return best.key
}

type candidate struct {
	key         string
	lastAccess  int64
	accessCount uint64
	expiresAt   int64
}

func (ec *EvictionController) selectLRU() string {
	return pickCandidate(ec.sampleKeys(), func(a, b candidate) bool { return a.lastAccess < b.lastAccess })
}

func (ec *EvictionController) selectLFU() string {
	return pickCandidate(ec.sampleKeys(), func(a, b candidate) bool { return a.accessCount < b.accessCount })
}

// visitShards calls fn on every shard once, in a random order, until fn
// returns false. An empty shard must not end a search for a victim while
// any key exists, and the walk allocates nothing: it runs for every
// eviction.
func (ec *EvictionController) visitShards(fn func(*Shard) bool) {
	ec.rndMu.Lock()
	r := ec.rnd.Uint64()
	ec.rndMu.Unlock()
	// NumShards is a power of two, so an odd step reaches every shard.
	start, step := int(r%NumShards), int((r>>32)%NumShards)|1
	for i := 0; i < NumShards; i++ {
		if !fn(ec.store.shards[(start+i*step)%NumShards]) {
			return
		}
	}
}

func (ec *EvictionController) selectRandom() string {
	// A random key of a random shard, without materializing all keys.
	var victim string
	ec.visitShards(func(shard *Shard) bool {
		shard.mu.RLock()
		defer shard.mu.RUnlock()
		for key := range shard.data {
			victim = key
			return false
		}
		return true
	})
	return victim
}

// sampleKeys takes a key from each of up to sampleSize shards.
func (ec *EvictionController) sampleKeys() []candidate {
	return ec.sample(false)
}

// sampleVolatileKeys takes a key with an expiry from each of up to
// sampleSize shards.
func (ec *EvictionController) sampleVolatileKeys() []candidate {
	return ec.sample(true)
}

func candidateOf(key string, entry *Entry) candidate {
	return candidate{
		key:         key,
		lastAccess:  entry.LastAccess(),
		accessCount: uint64(entry.LFUCount()),
		expiresAt:   entry.ExpiresAt.Load(),
	}
}

func (ec *EvictionController) sample(volatile bool) []candidate {
	sampleSize := ec.SampleSize()
	candidates := make([]candidate, 0, sampleSize)
	ec.visitShards(func(shard *Shard) bool {
		shard.mu.RLock()
		if volatile {
			// The shard's keys with an expiry, not a walk of all its keys
			// for one that has one.
			for key := range shard.vol {
				if entry := shard.data[key]; entry != nil {
					candidates = append(candidates, candidateOf(key, entry))
				}
				break
			}
		} else {
			for key, entry := range shard.data {
				candidates = append(candidates, candidateOf(key, entry))
				break
			}
		}
		shard.mu.RUnlock()
		return len(candidates) < sampleSize
	})
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
