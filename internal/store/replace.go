package store

import "fmt"

func (d *Databases) NewStaging() *Databases {
	base := NewStore()
	if tracker := d.DB(0).MemoryTracker(); tracker != nil {
		base.ConfigureMemory(tracker.Max(), EvictionNoEviction, 80, 90, 5)
	}
	return NewDatabases(base, d.Count())
}

func (d *Databases) ReplaceFrom(src *Databases) error {
	if d == src || d.Count() != src.Count() {
		return fmt.Errorf("invalid replacement databases")
	}
	owners := make(map[*Store]bool)
	d.ForEach(func(_ int, st *Store) { owners[st] = true })
	aliased := false
	src.ForEach(func(_ int, st *Store) {
		if owners[st] {
			aliased = true
		}
	})
	if aliased {
		return fmt.Errorf("replacement shares live stores")
	}
	d.mu.Lock()
	type invalidation struct {
		st   *Store
		tag  string
		keys []string
	}
	var invalidations []invalidation
	type notification struct {
		st  *Store
		key string
	}
	var notifications []notification
	for i := range d.slots {
		dst, incoming := d.slots[i].Load(), src.Existing(i)
		if dst == nil && incoming == nil {
			continue
		}
		if dst == nil {
			dst = d.dbLocked(i)
		}
		for _, tag := range dst.tagIndex.Tags() {
			invalidations = append(invalidations, invalidation{dst, tag, dst.tagIndex.GetKeys(tag)})
		}
		var delta, moved int64
		for j, shard := range &dst.shards {
			shard.mu.Lock()
			if dst.keyNotifier.HasWaiters() {
				for key := range shard.data {
					notifications = append(notifications, notification{dst, key})
				}
			}
			delta -= shard.memUsage
			if incoming == nil {
				shard.data = make(map[string]*Entry)
				shard.keyCount, shard.memUsage = 0, 0
			} else {
				from := incoming.shards[j]
				from.mu.Lock()
				shard.data, shard.keyCount, shard.memUsage = from.data, from.keyCount, from.memUsage
				for _, entry := range shard.data {
					entry.loading.Store(&dst.loading)
				}
				from.data = make(map[string]*Entry)
				from.keyCount, from.memUsage = 0, 0
				from.mu.Unlock()
			}
			delta += shard.memUsage
			moved += shard.memUsage
			shard.mu.Unlock()
		}
		dst.trackMemory(delta)
		if incoming != nil {
			incoming.trackMemory(-moved)
		}
		dst.tagIndex.hierarchy.Lock()
		dst.tagIndex.children = make(map[string]map[string]struct{})
		dst.tagIndex.parents = make(map[string]string)
		dst.tagIndex.hierarchy.Unlock()
		for j, shard := range &dst.tagIndex.shards {
			shard.mu.Lock()
			if incoming == nil {
				shard.index = make(map[string]map[string]struct{})
			} else {
				from := incoming.tagIndex.shards[j]
				from.mu.Lock()
				shard.index = from.index
				from.index = make(map[string]map[string]struct{})
				from.mu.Unlock()
			}
			shard.mu.Unlock()
		}
		dst.TouchAllWatched()
	}
	d.mu.Unlock()
	for _, item := range notifications {
		item.st.keyNotifier.NotifyKey(item.key)
	}
	for _, item := range invalidations {
		item.st.fireTagInvalidate(item.tag, item.keys)
	}
	for i := range d.slots {
		if dst := d.slots[i].Load(); dst != nil {
			for _, shard := range &dst.shards {
				for key, entry := range shard.GetAll() {
					dst.scheduleExpiry(key, entry.ExpiresAt.Load())
					if !entry.IsExpired() {
						dst.published(key, entry.Value)
					}
				}
			}
		}
	}
	return nil
}
