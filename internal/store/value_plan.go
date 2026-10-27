package store

import "math"

type ValuePlan struct {
	Size          int64
	Apply         func() Value
	unlock        func()
	allowEmptyKey bool
}

func (s *Store) ApplyValuePlan(key string, prepare func(Value) (ValuePlan, error)) error {
	s.Get(key)
	return s.applyValuePlan(key, nil, nil, prepare)
}

func (s *Store) applyValuePlan(key string, options *SetOptions, snapshot *Entry, prepare func(Value) (ValuePlan, error)) error {
	shard := s.shards[s.shardIndex(key)]
	remaining := -1
	for {
		shard.mu.Lock()
		previous := shard.data[key]
		live := previous != nil && !previous.IsExpired()
		if previous != nil && !live && !s.passiveExpiry.Load() {
			shard.mu.Unlock()
			s.DeleteIfExpired(key)
			continue
		}
		var current Value
		var tags []string
		if live {
			current = previous.Value
			tags = previous.Tags
		}
		if options != nil {
			if options.NX && live {
				shard.mu.Unlock()
				return ErrKeyExists
			}
			if options.XX && !live {
				shard.mu.Unlock()
				return ErrKeyNotFound
			}
			tags = options.Tags
		}
		if snapshot != nil {
			tags = snapshot.Tags
		}
		plan, err := prepare(current)
		unlock := func() {
			if plan.unlock != nil {
				plan.unlock()
			}
			shard.mu.Unlock()
		}
		if err != nil {
			unlock()
			return err
		}
		if plan.Apply == nil {
			unlock()
			return nil
		}
		if key == "" && snapshot == nil && !plan.allowEmptyKey {
			unlock()
			return ErrInvalidKey
		}
		if len(key) > MaxKeySize {
			unlock()
			return ErrKeyTooLarge
		}
		base := entryMetadataSize(tags) + int64(len(key)) + 16
		if plan.Size < 0 || plan.Size > math.MaxInt64-base {
			unlock()
			return ErrValueTooLarge
		}
		newMem := base + plan.Size
		oldMem := int64(0)
		if previous != nil {
			oldMem = previous.accounted.Load()
		}
		delta := newMem - oldMem
		reserved := s.memTracker != nil && !s.loading.Load()
		if reserved && !s.memTracker.TryReserve(delta) {
			tooLarge := !s.memTracker.withinLimit(newMem)
			unlock()
			if tooLarge || s.evictor == nil || s.evictor.policy == EvictionNoEviction {
				return ErrMemoryLimit
			}
			if remaining < 0 {
				remaining = int(s.KeyCount()) + 1
			}
			if remaining == 0 {
				return ErrMemoryLimit
			}
			remaining--
			if !s.evictor.evictOne() {
				return ErrMemoryLimit
			}
			continue
		}
		value := plan.Apply()
		var entry *Entry
		if snapshot != nil {
			entry = snapshot.rehome()
			entry.Value = value
		} else if options != nil {
			entry = NewEntry(value)
			entry.Tags = tags
			if options.KeepTTL {
				if live {
					entry.ExpiresAt.Store(previous.ExpiresAt.Load())
				}
			} else if options.ExpireAtMS > 0 {
				entry.SetExpiresAt(options.ExpireAtMS)
			} else if options.TTL > 0 {
				entry.SetTTL(options.TTL)
			}
		} else if live {
			if value == current {
				entry = previous
			} else {
				entry = previous.rehome()
				entry.Value = value
			}
		} else {
			entry = NewEntry(value)
		}
		actual, _ := shard.setLocked(key, entry)
		if reserved {
			s.trackMemory(actual - delta)
		} else {
			s.trackMemory(actual)
		}
		if snapshot != nil || options != nil || !live {
			if snapshot != nil {
				s.tagIndex.RemoveKey(key, nil)
			} else if previous != nil && len(previous.Tags) > 0 {
				s.tagIndex.RemoveKey(key, previous.Tags)
			}
			if len(entry.Tags) > 0 {
				s.tagIndex.AddTags(key, entry.Tags)
			}
		}
		if options != nil || snapshot != nil {
			s.scheduleExpiry(key, entry.ExpiresAt.Load())
		}
		unlock()
		s.published(key, value)
		return nil
	}
}
