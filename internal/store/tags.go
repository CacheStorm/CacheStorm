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
	var out []string
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
