package store

import "math"

func (s *Store) AddSetMembers(key string, members []string) (int, error) {
	added := 0
	err := s.ApplyValuePlan(key, func(value Value) (ValuePlan, error) {
		set := &SetValue{}
		if value != nil {
			var ok bool
			set, ok = value.(*SetValue)
			if !ok {
				return ValuePlan{}, ErrWrongType
			}
		}
		set.SizeOf()
		set.Lock()
		fresh := make(map[string]struct{}, len(members))
		for _, member := range members {
			if _, exists := set.Members[member]; !exists {
				fresh[member] = struct{}{}
			}
		}
		added = len(fresh)
		if added == 0 {
			set.Unlock()
			return ValuePlan{}, nil
		}
		size := set.SizeOf()
		for member := range fresh {
			n := setMemberSize(member)
			if n > math.MaxInt64-size {
				set.Unlock()
				return ValuePlan{}, ErrValueTooLarge
			}
			size += n
		}
		return ValuePlan{Size: size, Apply: func() Value {
			for member := range fresh {
				set.AddMember(member)
			}
			return set
		}, unlock: set.Unlock, allowEmptyKey: value != nil}, nil
	})
	if err != nil {
		return 0, err
	}
	return added, nil
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
			_, exists := src.Members[member]
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
		if _, exists := src.Members[member]; !exists {
			unlock()
			return false, nil
		}
		if !live {
			if dstKey == "" {
				unlock()
				return false, ErrInvalidKey
			}
			if len(dstKey) > MaxKeySize {
				unlock()
				return false, ErrKeyTooLarge
			}
		}
		_, duplicate := dst.Members[member]
		srcEmpty := len(src.Members) == 1
		srcSize := int64(0)
		if !srcEmpty {
			srcSize = entryMetadataSize(source.Tags) + int64(len(srcKey)) + 16 + src.SizeOf() - setMemberSize(member)
		}
		dstSize := dst.SizeOf()
		if !duplicate {
			n := setMemberSize(member)
			if n > math.MaxInt64-dstSize {
				unlock()
				return false, ErrValueTooLarge
			}
			dstSize += n
		}
		dstBase := entryMetadataSize(nil) + int64(len(dstKey)) + 16
		if live {
			dstBase = entryMetadataSize(previous.Tags) + int64(len(dstKey)) + 16
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
			s.tagIndex.RemoveKey(srcKey, source.Tags)
		} else {
			d, _ := srcShard.setLocked(srcKey, source)
			actual += d
		}
		if !live {
			s.tagIndex.RemoveKey(dstKey, nil)
			s.scheduleExpiry(dstKey, 0)
		}
		if reserved {
			s.trackMemory(actual - delta)
		} else {
			s.trackMemory(actual)
		}
		unlock()
		s.TouchKey(srcKey)
		s.keyNotifier.NotifyKey(srcKey)
		if !duplicate {
			s.published(dstKey, dst)
		}
		return true, nil
	}
}
