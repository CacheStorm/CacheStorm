package store

import "math/rand/v2"

// ShardLen returns the number of entries in shard idx, expired ones
// included.
func (s *Store) ShardLen(idx int) int {
	sh := s.shards[idx]
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	return len(sh.data)
}

// EachInShard calls fn for every unexpired entry of shard idx under the
// shard's read lock, so fn must not call back into the store.
func (s *Store) EachInShard(idx int, fn func(key string, e *Entry)) {
	sh := s.shards[idx]
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	for k, e := range sh.data {
		if !e.IsExpired() {
			fn(k, e)
		}
	}
}

// randomKeyWindow is how many consecutive keys of a shard RandomKey draws
// one from, as Redis' dictGetFairRandomKey samples a few keys.
const randomKeyWindow = 16

// RandomKey returns a random unexpired key, or false when there is none. It
// starts at a random shard and a random point of its map and picks one of
// the next few keys, so it costs the same whatever the size of the keyspace.
func (s *Store) RandomKey() (string, bool) {
	start := rand.IntN(NumShards)
	for i := 0; i < NumShards; i++ {
		sh := s.shards[(start+i)&ShardMask]
		sh.mu.RLock()
		var window [randomKeyWindow]string
		n := 0
		for k, e := range sh.data {
			if e.IsExpired() {
				continue
			}
			window[n] = k
			n++
			if n == len(window) {
				break
			}
		}
		sh.mu.RUnlock()
		if n > 0 {
			return window[rand.IntN(n)], true
		}
	}
	return "", false
}
