package store

import (
	"math"
	"slices"
)

func (s *Store) AddSetMembers(key string, members []string) (int, error) {
	added := 0
	err := s.ApplyValuePlan(key, func(value Value) (ValuePlan, error) {
		var set *SetValue
		if value == nil {
			set = &SetValue{}
		} else {
			var ok bool
			set, ok = value.(*SetValue)
			if !ok {
				return ValuePlan{}, ErrWrongType
			}
		}
		set.SizeOf()
		set.Lock()
		if len(members) > 0 {
			set.PrepareAdd(members[0], len(members))
		}
		fresh := freshMembers(set, members)
		added = len(fresh)
		if added == 0 {
			set.Unlock()
			return ValuePlan{}, nil
		}
		size := set.SizeOf()
		for _, member := range fresh {
			n := set.memberCost(member)
			if n > math.MaxInt64-size {
				set.Unlock()
				return ValuePlan{}, ErrValueTooLarge
			}
			size += n
		}
		return ValuePlan{Size: size, Apply: func() Value {
			for _, member := range fresh {
				set.AddMember(member)
			}
			return set
		}, unlock: set.Unlock}, nil
	})
	if err != nil {
		return 0, err
	}
	return added, nil
}

// freshMembers returns the members the set does not hold yet, each once.
// The common single-member call allocates nothing, and short lists are
// deduplicated without a map. The caller holds the set's lock.
func freshMembers(set *SetValue, members []string) []string {
	if len(members) == 1 {
		if set.Has(members[0]) {
			return nil
		}
		return members
	}
	var seen map[string]struct{}
	if len(members) > 16 {
		seen = make(map[string]struct{}, len(members))
	}
	fresh := make([]string, 0, len(members))
	for _, member := range members {
		if set.Has(member) {
			continue
		}
		if seen != nil {
			if _, dup := seen[member]; dup {
				continue
			}
			seen[member] = struct{}{}
		} else if slices.Contains(fresh, member) {
			continue
		}
		fresh = append(fresh, member)
	}
	return fresh
}

func (s *Store) moveSetMember(srcKey, dstKey, member string) (bool, error) {
	remaining := -1
	for {
		s.setOpsMu.Lock()
		unlockShards := lockTransferShards(s, srcKey, s, dstKey)
		unlockEntries := func() { unlockShards(); s.setOpsMu.Unlock() }
		srcShard, dstShard := s.shards[s.shardIndex(srcKey)], s.shards[s.shardIndex(dstKey)]
		source := srcShard.data[srcKey]
		if source == nil {
			unlockEntries()
			return false, nil
		}
		if source.IsExpired() {
			unlockEntries()
			if !s.passiveExpiry.Load() {
				s.DeleteIfExpired(srcKey)
			}
			return false, nil
		}
		src, ok := source.Value.(*SetValue)
		if !ok {
			unlockEntries()
			return false, ErrWrongType
		}
		previous := dstShard.data[dstKey]
		live := previous != nil && !previous.IsExpired()
		if previous != nil && !live && !s.passiveExpiry.Load() {
			unlockEntries()
			s.DeleteIfExpired(dstKey)
			continue
		}
		dst := &SetValue{}
		if live {
			dst, ok = previous.Value.(*SetValue)
			if !ok {
				unlockEntries()
				return false, ErrWrongType
			}
		}
		src.SizeOf()
		dst.SizeOf()
		if srcKey == dstKey {
			src.RLock()
			exists := src.Has(member)
			src.RUnlock()
			unlockEntries()
			return exists, nil
		}
		if src == dst {
			unlockEntries()
			return false, ErrWrongType
		}
		first, second := src, dst
		if dstKey < srcKey {
			first, second = second, first
		}
		first.Lock()
		second.Lock()
		unlock := func() { second.Unlock(); first.Unlock(); unlockEntries() }
		if src.dead || dst.dead {
			unlock()
			continue
		}
		if exists := src.Has(member); !exists {
			unlock()
			return false, nil
		}
		if !live {
			if len(dstKey) > MaxKeySize {
				unlock()
				return false, ErrKeyTooLarge
			}
		}
		duplicate := dst.Has(member)
		srcEmpty := src.Card() == 1
		srcSize := int64(0)
		if !srcEmpty {
			srcSize = entryMetadataSize(source.TagList()) + int64(len(srcKey)) + 16 + src.SizeOf() - src.memberCost(member)
		}
		dstSize := dst.SizeOf()
		if !duplicate {
			n := dst.memberCost(member)
			if n > math.MaxInt64-dstSize {
				unlock()
				return false, ErrValueTooLarge
			}
			dstSize += n
		}
		dstBase := entryMetadataSize(nil) + int64(len(dstKey)) + 16
		if live {
			dstBase = entryMetadataSize(previous.TagList()) + int64(len(dstKey)) + 16
		}
		if dstSize > math.MaxInt64-dstBase || srcSize > math.MaxInt64-dstBase-dstSize {
			unlock()
			return false, ErrValueTooLarge
		}
		newMem := srcSize + dstBase + dstSize
		oldMem := source.accounted.Load()
		if previous != nil {
			oldMem += previous.accounted.Load()
		}
		delta := newMem - oldMem
		reserved := s.memTracker != nil && !s.loading.Load()
		if reserved && !s.memTracker.TryReserve(delta) {
			tooLarge := !s.memTracker.withinLimit(newMem)
			unlock()
			if tooLarge || !s.evictor.evicts() {
				return false, ErrMemoryLimit
			}
			if !s.mayEvictAgain(&remaining) {
				return false, ErrMemoryLimit
			}
			if !s.evictor.evictOne() {
				return false, ErrMemoryLimit
			}
			continue
		}
		dst.AddMember(member)
		src.RemoveMember(member)
		entry := previous
		if !live {
			entry = NewEntry(dst)
		}
		actual, _ := dstShard.setLocked(dstKey, entry)
		if srcEmpty {
			src.markDeadLocked()
			actual -= srcShard.deleteLocked(srcKey, source)
			s.tagIndex.RemoveKey(srcKey, source.TagList())
		} else {
			d, _ := srcShard.setLocked(srcKey, source)
			actual += d
		}
		if !live {
			s.tagIndex.RemoveKey(dstKey, nil)
		}
		if reserved {
			s.trackMemory(actual - delta)
		} else {
			s.trackMemory(actual)
		}
		unlock()
		s.Notify(NotifySet, "srem", srcKey)
		if srcEmpty {
			s.Notify(NotifyGeneric, "del", srcKey)
		}
		if !live {
			s.notifyNew(dstKey)
		}
		if !duplicate {
			s.Notify(NotifySet, "sadd", dstKey)
		}
		s.TouchKey(srcKey)
		s.keyNotifier.NotifyKey(srcKey)
		if !duplicate {
			s.published(dstKey, dst)
		}
		return true, nil
	}
}
