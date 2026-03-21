package store

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	// MaxKeySize limits the maximum key length in bytes
	MaxKeySize = 64 * 1024 // 64KB
	// MaxValueSize limits the maximum value size in bytes
	MaxValueSize = 512 * 1024 * 1024 // 512MB
)

var (
	ErrKeyNotFound   = errors.New("key not found")
	ErrKeyExists    = errors.New("key already exists")
	ErrWrongType    = errors.New("WRONGTYPE Operation against a key holding the wrong kind of value")
	ErrMemoryLimit  = errors.New("OOM command not allowed when used memory > 'maxmemory'")
	ErrKeyTooLarge  = fmt.Errorf("ERR string length limit is %d bytes", MaxKeySize)
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
	namespaceMgr *NamespaceManager
	pubsub       *PubSub
	keyNotifier  *KeyNotifier
	versions     map[string]int64
	versionMu    sync.RWMutex
	memTracker   *MemoryTracker
	evictor      *EvictionController
}

func NewStore() *Store {
	s := &Store{
		tagIndex:    NewTagIndex(),
		pubsub:      NewPubSub(),
		keyNotifier: NewKeyNotifier(),
		versions:    make(map[string]int64),
	}
	for i := 0; i < NumShards; i++ {
		s.shards[i] = NewShard()
	}
	return s
}

// ConfigureMemory sets up memory tracking and eviction. Call after NewStore().
func (s *Store) ConfigureMemory(maxMemory int64, policy EvictionPolicy, warningPct, criticalPct, sampleSize int) {
	s.memTracker = NewMemoryTracker(maxMemory, warningPct, criticalPct)
	s.evictor = NewEvictionController(policy, maxMemory, s, s.memTracker, sampleSize)
}

func (s *Store) MemoryTracker() *MemoryTracker {
	return s.memTracker
}

func (s *Store) Evictor() *EvictionController {
	return s.evictor
}

func (s *Store) KeyNotifier() *KeyNotifier {
	return s.keyNotifier
}

func NewStoreWithNamespaces() *Store {
	s := &Store{
		tagIndex:     NewTagIndex(),
		namespaceMgr: NewNamespaceManagerNoCycle(),
		pubsub:       NewPubSub(),
		keyNotifier:  NewKeyNotifier(),
		versions:     make(map[string]int64),
	}
	for i := 0; i < NumShards; i++ {
		s.shards[i] = NewShard()
	}
	return s
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
		return nil, false
	}

	if entry.IsExpired() {
		shard.Delete(key)
		s.DeleteVersion(key) // Clean up version to prevent memory leak
		return nil, false
	}

	entry.Touch()
	return entry, true
}

func (s *Store) Set(key string, value Value, opts SetOptions) error {
	// Validate key size
	if len(key) > MaxKeySize {
		return ErrKeyTooLarge
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
				s.evictor.CheckAndEvict()
			}
			if !s.memTracker.CanAllocate(valueSize) {
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

	entry := NewEntry(value)
	if opts.TTL > 0 {
		entry.SetTTL(opts.TTL)
	}
	if len(opts.Tags) > 0 {
		entry.Tags = opts.Tags
		s.tagIndex.AddTags(key, opts.Tags)
	}

	shard.Set(key, entry)
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
	shard.Set(key, entry)
	s.IncrementVersion(key)
}

func (s *Store) Delete(key string) bool {
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists {
		return false
	}

	s.tagIndex.RemoveKey(key, entry.Tags)
	_, deleted := shard.Delete(key)
	if deleted {
		s.IncrementVersion(key)
		s.DeleteVersion(key) // Clean up version to prevent memory leak
	}
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
		shard.Delete(key)
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
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists || entry.IsExpired() {
		return false
	}

	entry.SetTTL(ttl)
	return true
}

func (s *Store) SetExpiresAt(key string, expiresAt int64) bool {
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists || entry.IsExpired() {
		return false
	}

	entry.SetExpiresAt(expiresAt)
	return true
}

func (s *Store) Persist(key string) bool {
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists || entry.IsExpired() {
		return false
	}

	entry.ExpiresAt = 0
	return true
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
	for i := 0; i < NumShards; i++ {
		s.shards[i].Flush()
	}
	// Clear version map to prevent memory leak
	s.versionMu.Lock()
	s.versions = make(map[string]int64)
	s.versionMu.Unlock()
}

func (s *Store) GetShard(key string) *Shard {
	return s.shards[s.shardIndex(key)]
}

func (s *Store) GetTagIndex() *TagIndex {
	return s.tagIndex
}

func (s *Store) GetNamespaceManager() *NamespaceManager {
	return s.namespaceMgr
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
