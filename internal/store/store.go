package store

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrKeyNotFound = errors.New("key not found")
	ErrKeyExists   = errors.New("key already exists")
	ErrWrongType   = errors.New("WRONGTYPE Operation against a key holding the wrong kind of value")
	ErrMemoryLimit = errors.New("OOM command not allowed when used memory > 'maxmemory'")
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
	mu           sync.RWMutex
}

func NewStore() *Store {
	s := &Store{
		tagIndex: NewTagIndex(),
	}
	for i := 0; i < NumShards; i++ {
		s.shards[i] = NewShard()
	}
	return s
}

func NewStoreWithNamespaces() *Store {
	s := &Store{
		tagIndex:     NewTagIndex(),
		namespaceMgr: NewNamespaceManager(),
	}
	for i := 0; i < NumShards; i++ {
		s.shards[i] = NewShard()
	}
	return s
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
		return nil, false
	}

	entry.Touch()
	return entry, true
}

func (s *Store) Set(key string, value Value, opts SetOptions) error {
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
	return nil
}

func (s *Store) SetEntry(key string, entry *Entry) {
	idx := s.shardIndex(key)
	shard := s.shards[idx]
	shard.Set(key, entry)
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
