package store

import (
	"sync"

	"github.com/cachestorm/cachestorm/internal/hashslot"
)

// slotIndex records the keys of each cluster hash slot. It exists only in
// cluster mode; the shards keep it current on every insert and removal.
type slotIndex struct {
	slots [hashslot.Count]slotKeys
}

type slotKeys struct {
	mu   sync.Mutex
	keys map[string]struct{}
}

func (x *slotIndex) add(key string) {
	s := &x.slots[hashslot.Key(key)]
	s.mu.Lock()
	if s.keys == nil {
		s.keys = make(map[string]struct{})
	}
	s.keys[key] = struct{}{}
	s.mu.Unlock()
}

func (x *slotIndex) remove(key string) {
	s := &x.slots[hashslot.Key(key)]
	s.mu.Lock()
	delete(s.keys, key)
	if len(s.keys) == 0 {
		s.keys = nil
	}
	s.mu.Unlock()
}

func (x *slotIndex) count(slot int) int {
	s := &x.slots[slot]
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.keys)
}

func (x *slotIndex) list(slot, max int) []string {
	s := &x.slots[slot]
	s.mu.Lock()
	defer s.mu.Unlock()
	if max > len(s.keys) {
		max = len(s.keys)
	}
	out := make([]string, 0, max)
	for k := range s.keys {
		if len(out) == max {
			break
		}
		out = append(out, k)
	}
	return out
}

// EnableSlotIndex starts keeping the per-slot key index cluster mode needs,
// indexing the keys already present. It is called once at startup.
func (s *Store) EnableSlotIndex() {
	idx := &slotIndex{}
	for _, shard := range &s.shards {
		shard.mu.Lock()
		if shard.slots == nil {
			shard.slots = idx
			for key := range shard.data {
				idx.add(key)
			}
		}
		shard.mu.Unlock()
	}
}

// SlotIndexed reports whether the store keeps the per-slot key index.
func (s *Store) SlotIndexed() bool {
	s.shards[0].mu.RLock()
	defer s.shards[0].mu.RUnlock()
	return s.shards[0].slots != nil
}

func (s *Store) slotIndex() *slotIndex {
	s.shards[0].mu.RLock()
	defer s.shards[0].mu.RUnlock()
	return s.shards[0].slots
}

// CountKeysInSlot returns the number of keys in hash slot slot, including
// expired keys not yet removed. Without the slot index it scans the keyspace.
func (s *Store) CountKeysInSlot(slot int) int {
	if idx := s.slotIndex(); idx != nil {
		return idx.count(slot)
	}
	n := 0
	for _, key := range s.Keys() {
		if hashslot.Key(key) == slot {
			n++
		}
	}
	return n
}

// KeysInSlot returns up to max keys of hash slot slot.
func (s *Store) KeysInSlot(slot, max int) []string {
	if idx := s.slotIndex(); idx != nil {
		return idx.list(slot, max)
	}
	var out []string
	for _, key := range s.Keys() {
		if len(out) >= max {
			break
		}
		if hashslot.Key(key) == slot {
			out = append(out, key)
		}
	}
	return out
}
