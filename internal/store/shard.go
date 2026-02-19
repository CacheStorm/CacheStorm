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

func (s *Shard) Set(key string, entry *Entry) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldMem := int64(0)
	if old, exists := s.data[key]; exists {
		oldMem = old.MemoryUsage()
		s.memUsage -= oldMem
	} else {
		s.keyCount++
	}

	newMem := entry.MemoryUsage()
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

	mem := entry.MemoryUsage()
	s.memUsage -= mem
	s.keyCount--
	delete(s.data, key)

	return mem, true
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
