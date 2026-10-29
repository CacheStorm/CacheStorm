package store

import (
	"math"
	"sync/atomic"
	"time"
)

var storeOrder atomic.Uint64

type TransferOptions struct {
	Copy    bool
	Replace bool
}

func lockTransferShards(src *Store, srcKey string, dst *Store, dstKey string) func() {
	a, b := src.shards[src.shardIndex(srcKey)], dst.shards[dst.shardIndex(dstKey)]
	if a == b {
		a.mu.Lock()
		return a.mu.Unlock
	}
	if src.order > dst.order || (src == dst && src.shardIndex(srcKey) > dst.shardIndex(dstKey)) {
		a, b = b, a
	}
	a.mu.Lock()
	b.mu.Lock()
	return func() { b.mu.Unlock(); a.mu.Unlock() }
}

func (s *Store) Transfer(srcKey string, dst *Store, dstKey string, options TransferOptions) (bool, error) {
	lookup := s.Peek
	if options.Copy {
		lookup = s.Get
	}
	if _, ok := lookup(srcKey); !ok {
		return false, ErrKeyNotFound
	}
	if len(dstKey) > MaxKeySize {
		return false, ErrKeyTooLarge
	}
	if s == dst && srcKey == dstKey {
		return false, nil
	}
	srcShard, dstShard := s.shards[s.shardIndex(srcKey)], dst.shards[dst.shardIndex(dstKey)]
	remaining := -1
	for {
		unlock := lockTransferShards(s, srcKey, dst, dstKey)
		source := srcShard.data[srcKey]
		if source == nil {
			unlock()
			return false, ErrKeyNotFound
		}
		if source.IsExpired() {
			unlock()
			s.DeleteIfExpired(srcKey)
			return false, ErrKeyNotFound
		}
		previous := dstShard.data[dstKey]
		live := previous != nil && !previous.IsExpired()
		if previous != nil && !live && !dst.passiveExpiry.Load() {
			unlock()
			dst.DeleteIfExpired(dstKey)
			continue
		}
		if live && !options.Replace {
			unlock()
			return false, nil
		}
		base := entryMetadataSize(source.TagList()) + int64(len(dstKey)) + 16
		size := source.Value.SizeOf()
		if size < 0 || size > math.MaxInt64-base {
			unlock()
			return false, ErrValueTooLarge
		}
		newMem := base + size
		oldMem := int64(0)
		if previous != nil {
			oldMem = previous.accounted.Load()
		}
		shared := s.memTracker == dst.memTracker
		delta := newMem - oldMem
		if shared && !options.Copy {
			delta -= source.accounted.Load()
		}
		reserved := dst.memTracker != nil && !dst.loading.Load()
		if reserved && !dst.memTracker.TryReserve(delta) {
			minimum := newMem
			tooLarge := false
			if options.Copy && shared {
				sourceMem := source.accounted.Load()
				if sourceMem > math.MaxInt64-minimum {
					tooLarge = true
				} else {
					minimum += sourceMem
				}
			}
			tooLarge = tooLarge || !dst.memTracker.withinLimit(minimum)
			unlock()
			if tooLarge || !dst.evictor.evicts() {
				return false, ErrMemoryLimit
			}
			if remaining < 0 {
				remaining = int(dst.KeyCount()) + 1
			}
			if remaining == 0 {
				return false, ErrMemoryLimit
			}
			remaining--
			if !dst.evictor.evictOne() {
				return false, ErrMemoryLimit
			}
			continue
		}
		entry := source.rehome()
		// Like Redis' dbAdd, a key that arrives under a new name starts its
		// access history afresh.
		entry.access.Store(newAccess(time.Now().Unix()))
		entry.SetTags(append([]string(nil), source.TagList()...))
		if options.Copy {
			entry = NewEntry(source.Value.Clone())
			entry.ExpiresAt.Store(source.ExpiresAt.Load())
			entry.SetTags(append([]string(nil), source.TagList()...))
		}
		dstDelta, _ := dstShard.setLocked(dstKey, entry)
		srcDelta := int64(0)
		if !options.Copy {
			srcDelta = -srcShard.deleteLocked(srcKey, source)
			s.tagIndex.RemoveKey(srcKey, source.TagList())
		}
		if reserved {
			actual := dstDelta
			if shared {
				actual += srcDelta
			} else {
				s.trackMemory(srcDelta)
			}
			dst.trackMemory(actual - delta)
		} else {
			dst.trackMemory(dstDelta)
			s.trackMemory(srcDelta)
		}
		dst.tagIndex.RemoveKey(dstKey, nil)
		if len(entry.TagList()) > 0 {
			dst.tagIndex.AddTags(dstKey, entry.TagList())
		}
		dst.scheduleExpiry(dstKey, entry.ExpiresAt.Load())
		unlock()
		dst.notifyNew(dstKey)
		if !options.Copy {
			s.TouchKey(srcKey)
			s.keyNotifier.NotifyKey(srcKey)
		}
		dst.published(dstKey, entry.Value)
		if emptyCollection(entry.Value) {
			dst.keyNotifier.NotifyKey(dstKey)
		}
		return true, nil
	}
}
