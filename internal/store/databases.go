package store

import (
	"errors"
	"math/bits"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultDatabases is the number of logical databases Redis serves by default.
const DefaultDatabases = 16

// ErrDBIndexOutOfRange is the Redis reply for a database index outside the
// configured range.
var ErrDBIndexOutOfRange = errors.New("ERR DB index is out of range")

// Databases holds the numbered logical databases a server serves. Database 0
// is the store the container was built from; the others are created on first
// use as siblings of it: they share its Pub/Sub hub (channels are not scoped
// to a database), its memory tracker (the memory limit is global) and its
// lifecycle hooks, and each gets its own evictor and expiry wheel.
type Databases struct {
	mu       sync.Mutex
	slots    []atomic.Pointer[Store]
	expiring bool
}

// NewDatabases builds a container of n databases with base as database 0.
// n below 1 means DefaultDatabases.
func NewDatabases(base *Store, n int) *Databases {
	if n < 1 {
		n = DefaultDatabases
	}
	d := &Databases{slots: make([]atomic.Pointer[Store], n)}
	d.slots[0].Store(base)
	base.dbIndex.Store(0)
	return d
}

// Count returns the number of databases.
func (d *Databases) Count() int {
	return len(d.slots)
}

// Valid reports whether index names a database.
func (d *Databases) Valid(index int) bool {
	return index >= 0 && index < len(d.slots)
}

// DB returns database index, creating it on first use. It returns nil for an
// index out of range.
func (d *Databases) DB(index int) *Store {
	if !d.Valid(index) {
		return nil
	}
	if s := d.slots[index].Load(); s != nil {
		return s
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dbLocked(index)
}

func (d *Databases) dbLocked(index int) *Store {
	if s := d.slots[index].Load(); s != nil {
		return s
	}
	s := d.slots[0].Load().newSibling()
	s.dbIndex.Store(int32(index))
	if d.expiring {
		s.StartExpiry()
	}
	d.slots[index].Store(s)
	return s
}

// Existing returns database index if it has been created, without creating
// it.
func (d *Databases) Existing(index int) *Store {
	if !d.Valid(index) {
		return nil
	}
	return d.slots[index].Load()
}

// Swap exchanges the contents of databases a and b: a connection that has
// selected a sees b's keys from its next command on. Watchers of either
// database are touched, as Redis does on SWAPDB.
func (d *Databases) Swap(a, b int) error {
	if !d.Valid(a) || !d.Valid(b) {
		return ErrDBIndexOutOfRange
	}
	if a == b {
		return nil
	}
	d.mu.Lock()
	sa, sb := d.dbLocked(a), d.dbLocked(b)
	d.slots[a].Store(sb)
	d.slots[b].Store(sa)
	sb.dbIndex.Store(int32(a))
	sa.dbIndex.Store(int32(b))
	d.mu.Unlock()
	sa.TouchWatchedPresent(sb)
	sb.TouchWatchedPresent(sa)
	return nil
}

// ForEach calls fn for every database created so far, in index order.
func (d *Databases) ForEach(fn func(index int, s *Store)) {
	for i := range d.slots {
		if s := d.slots[i].Load(); s != nil {
			fn(i, s)
		}
	}
}

// KeyCount returns the number of keys over all databases.
func (d *Databases) KeyCount() int64 {
	var n int64
	d.ForEach(func(_ int, s *Store) { n += s.KeyCount() })
	return n
}

// MemUsage returns the memory used by all databases.
func (d *Databases) MemUsage() int64 {
	var n int64
	d.ForEach(func(_ int, s *Store) { n += s.MemUsage() })
	return n
}

// FlushAll empties every database.
func (d *Databases) FlushAll() {
	d.ForEach(func(_ int, s *Store) { s.Flush() })
}

// StartExpiry starts active expiration in every database, including those
// created later.
func (d *Databases) StartExpiry() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.expiring = true
	for i := range d.slots {
		if s := d.slots[i].Load(); s != nil {
			s.StartExpiry()
		}
	}
}

// StopExpiry stops active expiration in every database.
func (d *Databases) StopExpiry() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.expiring = false
	for i := range d.slots {
		if s := d.slots[i].Load(); s != nil {
			s.StopExpiry()
		}
	}
}

// newSibling builds an empty store that shares s's Pub/Sub hub, memory
// tracker and hooks.
func (s *Store) newSibling() *Store {
	n := NewStore()
	if s.SlotIndexed() {
		n.EnableSlotIndex()
	}
	n.passiveExpiry.Store(s.passiveExpiry.Load())
	n.loading.Store(s.loading.Load())
	n.pubsub = s.pubsub
	if s.memTracker != nil {
		n.memTracker = s.memTracker
		if s.evictor != nil {
			n.evictor = newEvictionController(s.evictor.settings, n, s.memTracker)
			n.evictor.SetOnEvict(func(key string, entry *Entry) {
				n.fireEvict(key, entry.Value)
			})
		}
	}
	s.hooksMu.RLock()
	h := s.hooks
	s.hooksMu.RUnlock()
	n.SetHooks(h)
	return n
}

// ExpiryStats returns how many keys carry an expiry and their average
// remaining time to live in milliseconds, for the INFO keyspace section. As
// in Redis, a key past its expiry counts until it is removed. It reads
// per-shard counters, so it costs the same whatever the number of keys.
func (s *Store) ExpiryStats() (expires int64, avgTTLms int64) {
	var sumHi, sumLo uint64
	for _, sh := range &s.shards {
		sh.mu.RLock()
		expires += sh.expiring
		var carry uint64
		sumLo, carry = bits.Add64(sumLo, sh.expSumLo, 0)
		sumHi += sh.expSumHi + carry
		sh.mu.RUnlock()
	}
	if expires > 0 && sumHi < uint64(expires) {
		avgAt, _ := bits.Div64(sumHi, sumLo, uint64(expires))
		if ttl := int64(avgAt) - time.Now().UnixMilli(); ttl > 0 {
			avgTTLms = ttl
		}
	}
	return expires, avgTTLms
}
