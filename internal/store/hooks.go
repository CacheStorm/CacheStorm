package store

// StoreHooks carries the store's lifecycle event callbacks. The composition
// root (internal/server) wires them to the plugin Manager's dispatchers so
// registered hook consumers observe eviction, expiry, and tag invalidation;
// the store itself never imports the plugin layer. Set Hooks before the
// store starts serving traffic.
type StoreHooks struct {
	OnEvict         func(key string, value interface{})
	OnExpire        func(key string, value interface{})
	OnTagInvalidate func(tag string, keys []string)
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
