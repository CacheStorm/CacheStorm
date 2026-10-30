package store

import (
	"math/bits"
	"sync"
)

const (
	NumShards = 256
	ShardMask = NumShards - 1
)

type Shard struct {
	plain    *entryMeta
	mu       sync.RWMutex
	data     map[string]*Entry
	keyCount int64
	memUsage int64
	// expiring counts the entries with an expiry and expSum adds up their
	// expiry times (unix ms, 128 bits), for INFO's expires and avg_ttl.
	expiring           int64
	expSumHi, expSumLo uint64
	slots              *slotIndex
	hfe                *hashExpiryIndex
	// scan orders the keys for SCAN while the shard is being scanned.
	scan *keyScanIndex
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
	delta, _ := s.setLocked(key, entry)
	return delta
}

// footprint is what the shard charges for entry under key: the entry's
// memory usage plus the key and map-slot overhead.
func footprint(key string, entry *Entry) int64 {
	return entry.MemoryUsage() + int64(len(key)) + 16
}

// setLocked installs entry under key and returns the memory delta and the
// entry it replaced, if any. The caller holds s.mu.
func (s *Shard) setLocked(key string, entry *Entry) (int64, *Entry) {
	entry.bind(s.plain)
	oldMem := int64(0)
	old, exists := s.data[key]
	if old != entry {
		if exists {
			s.subExpiry(old.ExpiresAt.Load())
		}
		s.addExpiry(entry.ExpiresAt.Load())
	}
	if exists {
		// Credit what was charged for the old entry, not its current size:
		// in-place changes since the last reaccounting were never charged.
		oldMem = old.accounted.Load()
		s.memUsage -= oldMem
	} else {
		s.keyCount++
		if s.slots != nil {
			s.slots.add(key)
		}
		s.noteAddLocked(key)
	}

	newMem := footprint(key, entry)
	entry.accounted.Store(newMem)
	s.memUsage += newMem
	s.data[key] = entry
	if h, ok := entry.Value.(*HashValue); ok {
		s.hfe.schedule(key, h.NextFieldExpiry())
	}

	return newMem - oldMem, old
}

// deleteLocked removes key and returns the bytes freed. The caller holds s.mu
// and has checked that the key is present.
func (s *Shard) deleteLocked(key string, entry *Entry) int64 {
	mem := entry.accounted.Load()
	s.memUsage -= mem
	s.keyCount--
	s.subExpiry(entry.ExpiresAt.Load())
	delete(s.data, key)
	if s.slots != nil {
		s.slots.remove(key)
	}
	return mem
}

// reaccount charges the change in key's footprint since it was last
// accounted, for a value modified in place, and returns the delta. SizeOf
// is O(1) for every collection, so this is cheap enough to run after every
// write. The common no-change case only takes the read lock.
func (s *Shard) reaccount(key string) int64 {
	s.mu.RLock()
	entry, ok := s.data[key]
	same := ok && footprint(key, entry) == entry.accounted.Load()
	s.mu.RUnlock()
	if !ok || same {
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok = s.data[key]
	if !ok {
		return 0
	}
	cur := footprint(key, entry)
	delta := cur - entry.accounted.Swap(cur)
	s.memUsage += delta
	return delta
}

func (s *Shard) Delete(key string) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.data[key]
	if !exists {
		return 0, false
	}

	return s.deleteLocked(key, entry), true
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
	s.subExpiry(entry.ExpiresAt.Load())
	entry.ExpiresAt.Store(expiresAt)
	s.addExpiry(expiresAt)
	return true
}

// clearExpiry removes a live key's expiry and reports whether it had one.
func (s *Shard) clearExpiry(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.data[key]
	if !exists || entry.IsExpired() || entry.ExpiresAt.Load() == 0 {
		return false
	}
	s.subExpiry(entry.ExpiresAt.Load())
	entry.ExpiresAt.Store(0)
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
	if s.slots != nil {
		for key := range s.data {
			s.slots.remove(key)
		}
	}
	s.data = make(map[string]*Entry)
	s.scan = nil
	s.keyCount = 0
	s.memUsage = 0
	s.expiring, s.expSumHi, s.expSumLo = 0, 0, 0
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

func (s *Shard) addExpiry(at int64) {
	if at <= 0 {
		return
	}
	s.expiring++
	var carry uint64
	s.expSumLo, carry = bits.Add64(s.expSumLo, uint64(at), 0)
	s.expSumHi += carry
}

func (s *Shard) subExpiry(at int64) {
	if at <= 0 {
		return
	}
	s.expiring--
	var borrow uint64
	s.expSumLo, borrow = bits.Sub64(s.expSumLo, uint64(at), 0)
	s.expSumHi -= borrow
}
