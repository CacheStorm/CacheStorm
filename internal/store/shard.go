package store

import (
	"sync"
)

const (
	NumShards = 256
	ShardMask = NumShards - 1
)

type Shard struct {
	mu       sync.RWMutex
	data     map[string]*Entry
	keyCount int64
	memUsage int64
}

func NewShard() *Shard {
	return &Shard{
		data: make(map[string]*Entry),
	}
}

func (s *Shard) Get(key string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.data[key]
	return entry, ok
}

// getLive returns the entry for key only when it is present AND not expired.
// Store.Get already treats an expired key as absent (and reaps it); the
// conditional-write paths must agree with that. They used a raw map lookup, so
// a key whose TTL had lapsed but which active expiry had not yet reaped still
// looked present: SETNX and SET ... NX refused a write the caller was entitled
// to make (silently dropping the value), and SET ... XX revived a key that no
// longer existed.
func (s *Shard) getLive(key string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.data[key]
	if !ok || entry.IsExpired() {
		return nil, false
	}
	return entry, true
}

func (s *Shard) Set(key string, entry *Entry) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyOverhead := int64(len(key)) + 16 // key string + map entry overhead

	oldMem := int64(0)
	if old, exists := s.data[key]; exists {
		oldMem = old.MemoryUsage() + keyOverhead
		s.memUsage -= oldMem
	} else {
		s.keyCount++
	}

	newMem := entry.MemoryUsage() + keyOverhead
	s.memUsage += newMem
	s.data[key] = entry

	return newMem - oldMem
}

func (s *Shard) Delete(key string) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.data[key]
	if !exists {
		return 0, false
	}

	keyOverhead := int64(len(key)) + 16
	mem := entry.MemoryUsage() + keyOverhead
	s.memUsage -= mem
	s.keyCount--
	delete(s.data, key)

	return mem, true
}

// updateExpiry sets a key's absolute expiry under the shard lock so active
// expiration cannot race expiry writes (EXPIRE/PEXPIRE/PERSIST). Reports
// whether the key exists and is still live.
func (s *Shard) updateExpiry(key string, expiresAt int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.data[key]
	if !exists || entry.IsExpired() {
		return false
	}
	entry.ExpiresAt = expiresAt
	return true
}

func (s *Shard) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.data[key]
	return ok
}

func (s *Shard) Len() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.keyCount
}

func (s *Shard) MemUsage() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.memUsage
}

func (s *Shard) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

func (s *Shard) Flush() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	freed := s.memUsage
	s.data = make(map[string]*Entry)
	s.keyCount = 0
	s.memUsage = 0
	return freed
}

func (s *Shard) GetAll() map[string]*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]*Entry, len(s.data))
	for k, v := range s.data {
		result[k] = v
	}
	return result
}
