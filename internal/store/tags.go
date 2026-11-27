package store

// UpdateTags replaces key's tags with what fn returns for the current ones,
// under the shard lock, keeps the tag index in step and charges the change
// in memory. fn gets a copy it may modify. It reports false for a missing or
// expired key.
func (s *Store) UpdateTags(key string, fn func(tags []string) []string) bool {
	shard := s.shards[s.shardIndex(key)]
	shard.mu.Lock()
	entry, ok := shard.data[key]
	if !ok || entry.IsExpired() {
		shard.mu.Unlock()
		return false
	}
	old := entry.TagList()
	next := fn(append([]string(nil), old...))
	entry.SetTags(next)
	shard.mu.Unlock()

	if removed := tagsMissingFrom(old, next); len(removed) > 0 {
		s.tagIndex.RemoveTags(key, removed)
	}
	if added := tagsMissingFrom(next, old); len(added) > 0 {
		s.tagIndex.AddTags(key, added)
	}
	s.Reaccount(key)
	return true
}

// tagsMissingFrom returns the tags of a that b does not have.
func tagsMissingFrom(a, b []string) []string {
	if len(a) == 0 {
		return nil
	}
	if len(b) == 0 {
		return a
	}
	var out []string
	if len(a)*len(b) > 64 {
		in := make(map[string]struct{}, len(b))
		for _, u := range b {
			in[u] = struct{}{}
		}
		for _, t := range a {
			if _, ok := in[t]; !ok {
				out = append(out, t)
			}
		}
		return out
	}
	for _, t := range a {
		found := false
		for _, u := range b {
			if t == u {
				found = true
				break
			}
		}
		if !found {
			out = append(out, t)
		}
	}
	return out
}

// AddKeyTags gives key the tags of tags it lacks and reports how many that
// was; ok is false for a missing or expired key.
func (s *Store) AddKeyTags(key string, tags []string) (added int, ok bool) {
	return s.changeKeyTags(key, tags, true)
}

// RemoveKeyTags takes tags off key and reports how many it had; ok is
// false for a missing or expired key.
func (s *Store) RemoveKeyTags(key string, tags []string) (removed int, ok bool) {
	return s.changeKeyTags(key, tags, false)
}

func (s *Store) changeKeyTags(key string, tags []string, add bool) (int, bool) {
	shard := s.shards[s.shardIndex(key)]
	shard.mu.Lock()
	entry, ok := shard.data[key]
	if !ok {
		shard.mu.Unlock()
		return 0, false
	}
	if entry.IsExpired() && !s.passiveExpiry.Load() {
		// The key is expired first, its DEL recorded, as any write to it
		// does. A replica (passive expiry) applies what its master changed.
		shard.mu.Unlock()
		s.get(key, true, false)
		return 0, false
	}
	old := entry.TagList()
	var changed []string
	var next []string
	if add {
		next = append(make([]string, 0, len(old)+len(tags)), old...)
		for _, t := range tags {
			if !hasTag(next, t) {
				next = append(next, t)
				changed = append(changed, t)
			}
		}
	} else {
		next = make([]string, 0, len(old))
		for _, t := range old {
			if hasTag(tags, t) {
				changed = append(changed, t)
			} else {
				next = append(next, t)
			}
		}
	}
	if len(changed) == 0 {
		shard.mu.Unlock()
		return 0, true
	}
	entry.SetTags(next)
	shard.mu.Unlock()
	if add {
		s.tagIndex.AddTags(key, changed)
	} else {
		s.tagIndex.RemoveTags(key, changed)
	}
	s.Reaccount(key)
	return len(changed), true
}

func hasTag(tags []string, t string) bool {
	for _, u := range tags {
		if u == t {
			return true
		}
	}
	return false
}
