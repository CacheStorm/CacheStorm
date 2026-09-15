package store

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/logger"
)

const (
	// MaxKeySize limits the maximum key length in bytes
	MaxKeySize = 64 * 1024 // 64KB
	// MaxValueSize limits the maximum value size in bytes
	MaxValueSize = 512 * 1024 * 1024 // 512MB
)

var (
	ErrKeyNotFound   = errors.New("key not found")
	ErrKeyExists     = errors.New("key already exists")
	ErrWrongType     = errors.New("WRONGTYPE Operation against a key holding the wrong kind of value")
	ErrMemoryLimit   = errors.New("OOM command not allowed when used memory > 'maxmemory'")
	ErrInvalidKey    = errors.New("ERR invalid key name")
	ErrKeyTooLarge   = fmt.Errorf("ERR string length limit is %d bytes", MaxKeySize)
	ErrValueTooLarge = fmt.Errorf("ERR string length limit is %d bytes", MaxValueSize)
)

type GetResult struct {
	Entry *Entry
	Found bool
}

type SetOptions struct {
	TTL     time.Duration
	NX      bool
	XX      bool
	KeepTTL bool
	Tags    []string
}

type Store struct {
	shards       [NumShards]*Shard
	tagIndex     *TagIndex
	pubsub       *PubSub
	keyNotifier  *KeyNotifier
	versions     map[string]int64
	versionMu    sync.RWMutex
	memTracker   *MemoryTracker
	evictor      *EvictionController
	expiry       *TimingWheel
	expiryMu     sync.Mutex
	expiryActive bool
	hooksMu      sync.RWMutex
	hooks        StoreHooks
}

func NewStore() *Store {
	s := &Store{
		tagIndex:    NewTagIndex(),
		pubsub:      NewPubSub(),
		keyNotifier: NewKeyNotifier(),
		versions:    make(map[string]int64),
	}
	s.expiry = NewTimingWheel(s)
	for i := 0; i < NumShards; i++ {
		s.shards[i] = NewShard()
	}
	return s
}

// ConfigureMemory sets up memory tracking and eviction. Call after NewStore().
func (s *Store) ConfigureMemory(maxMemory int64, policy EvictionPolicy, warningPct, criticalPct, sampleSize int) {
	s.memTracker = NewMemoryTracker(maxMemory, warningPct, criticalPct)
	s.evictor = NewEvictionController(policy, maxMemory, s, s.memTracker, sampleSize)
	s.evictor.SetOnEvict(func(key string, entry *Entry) {
		s.fireEvict(key, entry.Value)
	})
}

// trackMemory applies a shard-size delta to the configured memory tracker so
// pressure checks and eviction observe live usage. Shard Set/Delete/Flush
// return exact byte deltas; without this feed the tracker stays at zero,
// CanAllocate always succeeds, and max_memory is silently unenforced.
func (s *Store) trackMemory(delta int64) {
	if delta != 0 && s.memTracker != nil {
		s.memTracker.Add(delta)
	}
}

func (s *Store) MemoryTracker() *MemoryTracker {
	return s.memTracker
}

func (s *Store) Evictor() *EvictionController {
	return s.evictor
}

// StartExpiry starts the timing wheel that removes expired keys without
// requiring reads. Idempotent while running; a stopped wheel can be started
// again (Stop resets its channel).
func (s *Store) StartExpiry() {
	s.expiryMu.Lock()
	defer s.expiryMu.Unlock()
	if s.expiryActive {
		return
	}
	s.expiryActive = true
	s.expiry.Start()
}

// StopExpiry stops active expiration. Safe to call when not running.
func (s *Store) StopExpiry() {
	s.expiryMu.Lock()
	defer s.expiryMu.Unlock()
	if !s.expiryActive {
		return
	}
	s.expiryActive = false
	s.expiry.Stop()
}

// scheduleExpiry queues a key on the timing wheel so active expiration can
// remove it without a read. No-op when the key has no expiry. Stale schedules
// (key deleted, rescheduled, or PERSISTed) fire as no-ops — expireKey only
// removes entries that are still actually expired.
func (s *Store) scheduleExpiry(key string, expiresAt int64) {
	if expiresAt > 0 && s.expiry != nil {
		s.expiry.Add(key, expiresAt)
	}
}

// DeleteIfExpired removes key only if it is present and past its expiry, with
// the same bookkeeping as Delete. The expiry check and the removal both happen
// under the shard lock so a stale schedule can never delete a rescheduled or
// PERSISTed key. Reports whether the key was removed.
func (s *Store) DeleteIfExpired(key string) bool {
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	shard.mu.Lock()
	entry, exists := shard.data[key]
	if !exists || !entry.IsExpired() {
		shard.mu.Unlock()
		return false
	}
	keyOverhead := int64(len(key)) + 16
	mem := entry.MemoryUsage() + keyOverhead
	shard.memUsage -= mem
	shard.keyCount--
	delete(shard.data, key)
	shard.mu.Unlock()

	s.trackMemory(-mem)
	s.tagIndex.RemoveKey(key, entry.Tags)
	s.IncrementVersion(key)
	s.DeleteVersion(key)
	s.fireExpire(key, entry.Value)
	return true
}

func (s *Store) KeyNotifier() *KeyNotifier {
	return s.keyNotifier
}

func (s *Store) GetVersion(key string) int64 {
	s.versionMu.RLock()
	defer s.versionMu.RUnlock()
	return s.versions[key]
}

func (s *Store) IncrementVersion(key string) {
	s.versionMu.Lock()
	defer s.versionMu.Unlock()
	s.versions[key]++
}

func (s *Store) DeleteVersion(key string) {
	s.versionMu.Lock()
	defer s.versionMu.Unlock()
	delete(s.versions, key)
}

func (s *Store) shardIndex(key string) uint32 {
	return fnv32a(key) & ShardMask
}

func fnv32a(s string) uint32 {
	const (
		offset32 = uint32(2166136261)
		prime32  = uint32(16777619)
	)
	h := offset32
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime32
	}
	return h
}

func (s *Store) Get(key string) (*Entry, bool) {
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists {
		GlobalMetrics.RecordMiss()
		return nil, false
	}

	if entry.IsExpired() {
		mem, _ := shard.Delete(key)
		s.trackMemory(-mem)
		s.DeleteVersion(key) // Clean up version to prevent memory leak
		s.fireExpire(key, entry.Value)
		// A lookup of an expired key counts as both a miss and an expiration
		// (lazy expiry here is the only expiry path that observes reads).
		GlobalMetrics.RecordMiss()
		GlobalMetrics.RecordExpiration()
		return nil, false
	}

	entry.Touch()
	GlobalMetrics.RecordHit()
	return entry, true
}

func (s *Store) Set(key string, value Value, opts SetOptions) error {
	// Validate key
	if len(key) == 0 {
		return ErrInvalidKey
	}
	if len(key) > MaxKeySize {
		return ErrKeyTooLarge
	}
	if strings.ContainsRune(key, 0) {
		return ErrInvalidKey
	}

	// Validate value size
	if value.SizeOf() > MaxValueSize {
		return ErrValueTooLarge
	}

	// Check memory limit and try eviction if needed
	if s.memTracker != nil && s.memTracker.Max() > 0 {
		valueSize := value.SizeOf()
		if !s.memTracker.CanAllocate(valueSize) {
			// Try eviction before rejecting
			if s.evictor != nil {
				if err := s.evictor.CheckAndEvict(); err != nil {
					logger.Warn().Err(err).Msg("eviction attempt failed")
				}
			}
			if !s.memTracker.CanAllocate(valueSize) {
				logger.Warn().
					Int64("value_size", valueSize).
					Int64("usage", s.memTracker.Usage()).
					Int64("max", s.memTracker.Max()).
					Msg("OOM: rejecting write, memory limit exceeded")
				return ErrMemoryLimit
			}
		}
	}

	idx := s.shardIndex(key)
	shard := s.shards[idx]

	if opts.NX {
		if _, exists := shard.Get(key); exists {
			return ErrKeyExists
		}
	}

	if opts.XX {
		if _, exists := shard.Get(key); !exists {
			return ErrKeyNotFound
		}
	}

	// Reconcile tag membership: the replaced entry's mappings must drop
	// before the new ones are added (RemoveKey-then-AddTags keeps tags
	// shared between old and new).
	var oldTags []string
	if prev, exists := shard.Get(key); exists {
		oldTags = prev.Tags
	}
	if len(oldTags) > 0 {
		s.tagIndex.RemoveKey(key, oldTags)
	}

	entry := NewEntry(value)
	if opts.TTL > 0 {
		entry.SetTTL(opts.TTL)
		s.scheduleExpiry(key, entry.ExpiresAt)
	}
	if len(opts.Tags) > 0 {
		entry.Tags = opts.Tags
		s.tagIndex.AddTags(key, opts.Tags)
	}

	s.trackMemory(shard.Set(key, entry))
	s.IncrementVersion(key)
	s.keyNotifier.NotifyKey(key)
	return nil
}

func (s *Store) SetEntry(key string, entry *Entry) {
	// Validate key size
	if len(key) > MaxKeySize {
		return
	}

	idx := s.shardIndex(key)
	shard := s.shards[idx]
	s.trackMemory(shard.Set(key, entry))
	s.IncrementVersion(key)
	s.scheduleExpiry(key, entry.ExpiresAt)
}

func (s *Store) Delete(key string) bool {
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists {
		return false
	}

	s.tagIndex.RemoveKey(key, entry.Tags)
	mem, deleted := shard.Delete(key)
	if deleted {
		s.trackMemory(-mem)
		s.IncrementVersion(key)
		s.DeleteVersion(key)
	}
	return deleted
}

func (s *Store) DeleteBatch(keys []string) int {
	if len(keys) == 0 {
		return 0
	}

	shardOps := make(map[*Shard][]string)
	for _, key := range keys {
		idx := s.shardIndex(key)
		shard := s.shards[idx]
		shardOps[shard] = append(shardOps[shard], key)
	}

	deleted := 0
	freed := int64(0)
	var untag []struct {
		key  string
		tags []string
	}
	s.versionMu.Lock()
	for shard, shardKeys := range shardOps {
		shard.mu.Lock()
		for _, key := range shardKeys {
			entry, exists := shard.data[key]
			if !exists {
				continue
			}
			keyOverhead := int64(len(key)) + 16
			mem := entry.MemoryUsage() + keyOverhead
			shard.memUsage -= mem
			freed += mem
			shard.keyCount--
			delete(shard.data, key)
			delete(s.versions, key)
			if len(entry.Tags) > 0 {
				untag = append(untag, struct {
					key  string
					tags []string
				}{key, entry.Tags})
			}
			deleted++
		}
		shard.mu.Unlock()
	}
	s.versionMu.Unlock()

	// Mirror Store.Delete's index bookkeeping, outside the shard/version
	// locks to preserve their ordering with the tag-index locks.
	for _, u := range untag {
		s.tagIndex.RemoveKey(u.key, u.tags)
	}

	s.trackMemory(-freed)

	return deleted
}

func (s *Store) Exists(key string) bool {
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists {
		return false
	}

	if entry.IsExpired() {
		mem, _ := shard.Delete(key)
		s.trackMemory(-mem)
		s.DeleteVersion(key) // Clean up version to prevent memory leak
		return false
	}

	return true
}

func (s *Store) Type(key string) DataType {
	entry, exists := s.Get(key)
	if !exists {
		return DataType(0)
	}
	return entry.Value.Type()
}

func (s *Store) TTL(key string) time.Duration {
	entry, exists := s.Get(key)
	if !exists {
		return -2
	}
	return entry.TTL()
}

func (s *Store) SetTTL(key string, ttl time.Duration) bool {
	expiresAt := time.Now().Add(ttl).UnixNano()
	if !s.shards[s.shardIndex(key)].updateExpiry(key, expiresAt) {
		return false
	}
	s.scheduleExpiry(key, expiresAt)
	return true
}

func (s *Store) SetExpiresAt(key string, expiresAt int64) bool {
	if !s.shards[s.shardIndex(key)].updateExpiry(key, expiresAt) {
		return false
	}
	s.scheduleExpiry(key, expiresAt)
	return true
}

func (s *Store) Persist(key string) bool {
	// ExpiresAt 0 clears the expiry; any stale wheel schedule for this key
	// later fires as a no-op (expireKey only removes actually-expired keys).
	return s.shards[s.shardIndex(key)].updateExpiry(key, 0)
}

func (s *Store) GetTTL(key string) time.Duration {
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists {
		return -2 * time.Second
	}

	if entry.IsExpired() {
		return -2 * time.Second
	}

	if entry.ExpiresAt == 0 {
		return -1 * time.Second
	}

	remaining := time.Duration(entry.ExpiresAt - time.Now().UnixNano())
	if remaining < 0 {
		return -2 * time.Second
	}

	return remaining
}

func (s *Store) KeyCount() int64 {
	var count int64
	for i := 0; i < NumShards; i++ {
		count += s.shards[i].Len()
	}
	return count
}

func (s *Store) MemUsage() int64 {
	var usage int64
	for i := 0; i < NumShards; i++ {
		usage += s.shards[i].MemUsage()
	}
	return usage
}

func (s *Store) Keys() []string {
	keys := make([]string, 0)
	for i := 0; i < NumShards; i++ {
		keys = append(keys, s.shards[i].Keys()...)
	}
	return keys
}

func (s *Store) Flush() {
	keyCount := s.KeyCount()
	freed := int64(0)
	for i := 0; i < NumShards; i++ {
		freed += s.shards[i].Flush()
	}
	s.trackMemory(-freed)
	// Clear version map to prevent memory leak
	s.versionMu.Lock()
	s.versions = make(map[string]int64)
	s.versionMu.Unlock()
	logger.Info().Int64("flushed_keys", keyCount).Msg("store flushed")
}

func (s *Store) GetShard(key string) *Shard {
	return s.shards[s.shardIndex(key)]
}

func (s *Store) GetTagIndex() *TagIndex {
	return s.tagIndex
}

func (s *Store) GetPubSub() *PubSub {
	return s.pubsub
}

func (s *Store) GetAll() map[string]*Entry {
	result := make(map[string]*Entry)
	for i := 0; i < NumShards; i++ {
		shardData := s.shards[i].GetAll()
		for k, v := range shardData {
			if !v.IsExpired() {
				result[k] = v
			}
		}
	}
	return result
}
