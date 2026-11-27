package store

import (
	"math"
	"slices"
	"sort"
)

// batchScanMax is how many pairs SetStringBatch matches by scanning before
// it indexes them in a map.
const batchScanMax = 32

type StringPair struct {
	Key  string
	Data []byte
}

// BatchSetOptions are MSETEX's conditions and expiry for SetStringBatchOpts.
type BatchSetOptions struct {
	// NX sets nothing if any key exists, XX if any key is missing.
	NX, XX bool
	// ExpireAtMS, when not 0, is every key's expiry (unix ms); KeepTTL keeps
	// each key's current one instead.
	ExpireAtMS int64
	KeepTTL    bool
}

// SetStringBatch is MSET (or MSETNX with nx).
func (s *Store) SetStringBatch(pairs []StringPair, nx bool) (bool, error) {
	return s.SetStringBatchOpts(pairs, BatchSetOptions{NX: nx})
}

// SetStringBatchOpts sets every pair at once, or none when a condition
// fails, which it reports as false.
func (s *Store) SetStringBatchOpts(pairs []StringPair, opts BatchSetOptions) (bool, error) {
	nx := opts.NX
	// keys are the distinct keys in first-seen order and vals[i] the last
	// value given for keys[i], built straight in stored form. An MSET names
	// a handful of keys, so they are matched by a scan rather than a map,
	// and the shard indices likewise.
	keys := make([]string, 0, len(pairs))
	vals := make([]Value, 0, len(pairs))
	var index map[string]int
	if len(pairs) > batchScanMax {
		index = make(map[string]int, len(pairs))
	}
	indices := make([]int, 0, min(len(pairs), NumShards))
	for _, pair := range pairs {
		value := NewCompactString(pair.Data, StringEncAuto)
		if _, err := validateStoreWrite(pair.Key, value); err != nil {
			return false, err
		}
		at := -1
		if index != nil {
			if i, ok := index[pair.Key]; ok {
				at = i
			}
		} else {
			for i, k := range keys {
				if k == pair.Key {
					at = i
					break
				}
			}
		}
		if at >= 0 {
			vals[at] = value
			continue
		}
		if index != nil {
			index[pair.Key] = len(keys)
		}
		keys = append(keys, pair.Key)
		vals = append(vals, value)
		if shard := int(s.shardIndex(pair.Key)); !slices.Contains(indices, shard) {
			indices = append(indices, shard)
		}
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
		for i, key := range keys {
			previous := s.shards[s.shardIndex(key)].data[key]
			live := previous != nil && !previous.IsExpired()
			if previous != nil && !live && !s.passiveExpiry.Load() {
				expired = key
				break
			}
			if (nx && live) || (opts.XX && !live) {
				unlock()
				return false, nil
			}
			size := entryMetadataSize(nil) + int64(len(key)) + 16 + vals[i].SizeOf()
			if size > math.MaxInt64-newMem {
				unlock()
				return false, ErrValueTooLarge
			}
			newMem += size
			if previous != nil {
				oldMem += previous.charged()
			}
		}
		if expired != "" {
			unlock()
			s.DeleteIfExpired(expired)
			continue
		}
		delta := newMem - oldMem
		reserved := s.memTracker != nil && !s.clock.loading.Load()
		if reserved && !s.memTracker.TryReserve(delta) {
			tooLarge := !s.memTracker.withinLimit(newMem)
			unlock()
			if tooLarge || !s.evictor.evicts() {
				return false, ErrMemoryLimit
			}
			if !s.mayEvictAgain(&remaining) {
				return false, ErrMemoryLimit
			}
			if !s.evictor.evictOne(pairs[0].Key, false) {
				return false, ErrMemoryLimit
			}
			continue
		}
		actual := int64(0)
		var created map[string]bool
		var replaced map[string]DataType
		notifyNew := s.Notifying(NotifyNew | NotifyOverwritten | NotifyTypeChanged)
		for i, key := range keys {
			shard := s.shards[s.shardIndex(key)]
			entry := NewEntry(vals[i])
			if previous := shard.data[key]; previous != nil && !previous.IsExpired() {
				entry.inheritAccess(previous)
				if opts.KeepTTL {
					entry.ExpiresAt.Store(previous.ExpiresAt.Load())
				}
			}
			if opts.ExpireAtMS != 0 {
				entry.ExpiresAt.Store(opts.ExpireAtMS)
			}
			if notifyNew {
				if previous := shard.data[key]; previous == nil || previous.IsExpired() {
					if created == nil {
						created = make(map[string]bool)
					}
					created[key] = true
				} else {
					if replaced == nil {
						replaced = make(map[string]DataType)
					}
					replaced[key] = previous.Value.Type()
				}
			}
			change, previous := shard.setLocked(key, entry)
			actual += change
			if previous != nil && len(previous.TagList()) > 0 {
				s.tagIndex.RemoveKey(key, previous.TagList())
			}
		}
		if reserved {
			s.trackMemory(actual - delta)
		} else {
			s.trackMemory(actual)
		}
		unlock()
		if s.Notifying(NotifyString | NotifyNew | NotifyGeneric | NotifyOverwritten | NotifyTypeChanged) {
			for _, pair := range pairs {
				// Each pair is a setKey of its own: a key named twice is
				// new the first time and overwritten the second.
				if created[pair.Key] {
					delete(created, pair.Key)
					s.notifyNew(pair.Key)
					if replaced == nil {
						replaced = make(map[string]DataType)
					}
					replaced[pair.Key] = DataTypeString
				} else if old, ok := replaced[pair.Key]; ok {
					s.NotifyOverwrite(pair.Key, old, DataTypeString)
					replaced[pair.Key] = DataTypeString
				}
				// MSETEX reports each key's expiry before its set.
				if opts.ExpireAtMS != 0 {
					s.Notify(NotifyGeneric, "expire", pair.Key)
				}
				s.Notify(NotifyString, "set", pair.Key)
			}
		}
		for i, key := range keys {
			s.published(key, vals[i])
		}
		return true, nil
	}
}
