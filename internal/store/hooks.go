package store

// StoreHooks carries the store's lifecycle event callbacks. The composition
// root (internal/server) wires them to the plugin Manager's dispatchers so
// registered hook consumers observe eviction, expiry, and tag invalidation;
// the store itself never imports the plugin layer. Set Hooks before the
// store starts serving traffic.
type StoreHooks struct {
	OnEvictLocked   func(s *Store, key string)
	OnEvict         func(key string, value interface{})
	OnExpire        func(key string, value interface{})
	OnTagInvalidate func(tag string, keys []string)

	// OnExpireLocked is called while the shard lock of the expired key is
	// still held, so whatever it records is ordered before any later write to
	// the key. It must not call back into the store. Replication uses it to
	// stream the expiry as a DEL in the order the master applied it.
	OnExpireLocked func(s *Store, key string)
	// OnEvictKey is called after a key was evicted, with the store it was
	// evicted from.
	OnEvictKey func(s *Store, key string)
	// ExpiryGuard, when set, wraps every active expiration: it is called
	// before the key is checked and removed, and the function it returns is
	// called afterwards.
	ExpiryGuard func(s *Store, key string) func()
	// OnHashFieldsExpiredLocked is called, with the shard lock of key held,
	// when fields of the hash at key expired and were removed. Replication
	// and the AOF record them as HDEL, as Redis propagates field expiry.
	OnHashFieldsExpiredLocked func(s *Store, key string, fields []string)
	// EvictGuard, when set, is asked before a key is evicted for a write of
	// writing (empty when the write names no key), before the command ran
	// (before) or halfway through it. It returns what to call once the key
	// is gone, or false when the key may not be evicted now, and another one
	// is picked: another command is in the middle of a write of it, or the
	// command evicting it already used it, and the eviction's DEL would be
	// recorded before a write the master made first, so a replica and the
	// AOF would keep the key the master evicted.
	EvictGuard func(s *Store, victim, writing string, before bool) (release func(), ok bool)
}

func (s *Store) fireEvictLocked(key string) {
	s.hooksMu.RLock()
	h := s.hooks.OnEvictLocked
	s.hooksMu.RUnlock()
	if h != nil {
		h(s, key)
	}
}

// SetHooks installs the event callbacks. Call before the store serves
// traffic; reads on the hot paths take the read lock.
func (s *Store) SetHooks(h StoreHooks) {
	s.hooksMu.Lock()
	s.hooks = h
	s.hooksMu.Unlock()
	s.tagIndex.SetOnInvalidate(s.fireTagInvalidate)
}

func (s *Store) fireEvict(key string, value interface{}) {
	s.hooksMu.RLock()
	h := s.hooks.OnEvict
	s.hooksMu.RUnlock()
	if h != nil {
		h(key, value)
	}
}

func (s *Store) fireExpire(key string, value interface{}) {
	s.Notify(NotifyExpired, "expired", key)
	s.hooksMu.RLock()
	h := s.hooks.OnExpire
	s.hooksMu.RUnlock()
	if h != nil {
		h(key, value)
	}
}

func (s *Store) fireTagInvalidate(tag string, keys []string) {
	s.hooksMu.RLock()
	h := s.hooks.OnTagInvalidate
	s.hooksMu.RUnlock()
	if h != nil {
		h(tag, keys)
	}
}

func (s *Store) fireExpireLocked(key string) {
	s.hooksMu.RLock()
	h := s.hooks.OnExpireLocked
	s.hooksMu.RUnlock()
	if h != nil {
		h(s, key)
	}
}

func (s *Store) fireEvictKey(key string) {
	s.hooksMu.RLock()
	h := s.hooks.OnEvictKey
	s.hooksMu.RUnlock()
	if h != nil {
		h(s, key)
	}
}

func (s *Store) expiryGuard(key string) func() {
	s.hooksMu.RLock()
	h := s.hooks.ExpiryGuard
	s.hooksMu.RUnlock()
	if h == nil {
		return func() {}
	}
	return h(s, key)
}

func (s *Store) evictGuard(victim, writing string, before bool) (func(), bool) {
	s.hooksMu.RLock()
	h := s.hooks.EvictGuard
	s.hooksMu.RUnlock()
	if h == nil {
		return func() {}, true
	}
	return h(s, victim, writing, before)
}

func (s *Store) fireHashFieldsExpiredLocked(key string, fields []string) {
	s.hooksMu.RLock()
	h := s.hooks.OnHashFieldsExpiredLocked
	s.hooksMu.RUnlock()
	if h != nil {
		h(s, key, fields)
	}
}
