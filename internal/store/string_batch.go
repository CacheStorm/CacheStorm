package store

import (
	"math"
	"sort"
)

type StringPair struct {
	Key  string
	Data []byte
}

func (s *Store) SetStringBatch(pairs []StringPair, nx bool) (bool, error) {
	keys := make([]string, 0, len(pairs))
	values := make(map[string]*StringValue, len(pairs))
	indices := make([]int, 0, len(pairs))
	seen := make(map[int]bool)
	for _, pair := range pairs {
		value := &StringValue{Data: pair.Data}
		if _, err := validateStoreWrite(pair.Key, value); err != nil {
			return false, err
		}
		if _, exists := values[pair.Key]; !exists {
			keys = append(keys, pair.Key)
			index := int(s.shardIndex(pair.Key))
			if !seen[index] {
				seen[index] = true
				indices = append(indices, index)
			}
		}
		values[pair.Key] = value
	}
	if len(keys) == 0 {
		return false, nil
	}
	sort.Ints(indices)
	unlock := func() {
		for i := len(indices) - 1; i >= 0; i-- {
			s.shards[indices[i]].mu.Unlock()
		}
	}
	remaining := -1
	for {
		for _, index := range indices {
			s.shards[index].mu.Lock()
		}
		newMem, oldMem := int64(0), int64(0)
		expired := ""
		for _, key := range keys {
			previous := s.shards[s.shardIndex(key)].data[key]
			live := previous != nil && !previous.IsExpired()
			if previous != nil && !live && !s.passiveExpiry.Load() {
				expired = key
				break
			}
			if nx && live {
				unlock()
				return false, nil
			}
			size := entryMetadataSize(nil) + int64(len(key)) + 16 + values[key].SizeOf()
			if size > math.MaxInt64-newMem {
				unlock()
				return false, ErrValueTooLarge
			}
			newMem += size
			if previous != nil {
				oldMem += previous.accounted.Load()
			}
		}
		if expired != "" {
			unlock()
			s.DeleteIfExpired(expired)
			continue
		}
		delta := newMem - oldMem
		reserved := s.memTracker != nil && !s.loading.Load()
		if reserved && !s.memTracker.TryReserve(delta) {
			tooLarge := !s.memTracker.withinLimit(newMem)
			unlock()
			if tooLarge || !s.evictor.evicts() {
				return false, ErrMemoryLimit
			}
			if remaining < 0 {
				remaining = int(s.KeyCount()) + 1
			}
			if remaining == 0 {
				return false, ErrMemoryLimit
			}
			remaining--
			if !s.evictor.evictOne() {
				return false, ErrMemoryLimit
			}
			continue
		}
		actual := int64(0)
		var created map[string]bool
		notifyNew := s.Notifying(NotifyNew)
		for _, key := range keys {
			shard := s.shards[s.shardIndex(key)]
			entry := NewEntry(values[key])
			if previous := shard.data[key]; previous != nil && !previous.IsExpired() {
				entry.inheritAccess(previous)
			}
			if notifyNew {
				if previous := shard.data[key]; previous == nil || previous.IsExpired() {
					if created == nil {
						created = make(map[string]bool)
					}
					created[key] = true
				}
			}
			change, previous := shard.setLocked(key, entry)
			actual += change
			if previous != nil && len(previous.TagList()) > 0 {
				s.tagIndex.RemoveKey(key, previous.TagList())
			}
			s.scheduleExpiry(key, 0)
		}
		if reserved {
			s.trackMemory(actual - delta)
		} else {
			s.trackMemory(actual)
		}
		unlock()
		if s.Notifying(NotifyString | NotifyNew) {
			for _, pair := range pairs {
				if created[pair.Key] {
					delete(created, pair.Key)
					s.notifyNew(pair.Key)
				}
				s.Notify(NotifyString, "set", pair.Key)
			}
		}
		for _, key := range keys {
			s.published(key, values[key])
		}
		return true, nil
	}
}
