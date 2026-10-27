package store

func (s *Store) BeginLoading() {
	s.loading.Store(true)
	for _, shard := range &s.shards {
		shard.mu.Lock()
		for _, entry := range shard.data {
			entry.loading.Store(&s.loading)
		}
		shard.mu.Unlock()
	}
}

func (s *Store) EndLoading() {
	s.loading.Store(false)
	if s.passiveExpiry.Load() {
		return
	}
	for _, shard := range &s.shards {
		shard.mu.RLock()
		var expired []string
		for key, entry := range shard.data {
			if entry.IsExpired() {
				expired = append(expired, key)
			}
		}
		shard.mu.RUnlock()
		for _, key := range expired {
			s.DeleteIfExpired(key)
		}
	}
}

func (d *Databases) SetPassiveExpiry(passive bool) {
	d.ForEach(func(_ int, st *Store) { st.passiveExpiry.Store(passive) })
}

func (d *Databases) BeginAuthoritativeApply() {
	d.ForEach(func(_ int, st *Store) { st.loading.Store(true) })
}
func (d *Databases) EndAuthoritativeApply() {
	d.ForEach(func(_ int, st *Store) { st.loading.Store(false) })
}
