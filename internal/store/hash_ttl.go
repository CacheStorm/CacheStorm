package store

import (
	"container/heap"
	"sync"
	"time"
)

// hashFieldTTLOverhead is what a field's expiry adds to the hash's size: its
// entry in the expiry map and in the expiry heap.
const hashFieldTTLOverhead = 40

// MaxHashFieldExpireMS is the latest absolute field expiry Redis 7.4 accepts
// (EB_EXPIRE_TIME_MAX >> 2, in unix milliseconds).
const MaxHashFieldExpireMS = (int64(1)<<48 - 1) >> 2

// hashFieldTTLs holds the expiry times of the fields of a hash that have one.
// The heap orders them by time; an entry whose time no longer matches the map
// is stale and skipped when it reaches the top.
type hashFieldTTLs struct {
	at   map[string]int64
	heap fieldTTLHeap
}

type fieldTTLItem struct {
	at    int64
	field string
}

type fieldTTLHeap []fieldTTLItem

func (h fieldTTLHeap) Len() int           { return len(h) }
func (h fieldTTLHeap) Less(i, j int) bool { return h[i].at < h[j].at }
func (h fieldTTLHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *fieldTTLHeap) Push(x any)        { *h = append(*h, x.(fieldTTLItem)) }
func (h *fieldTTLHeap) Pop() any {
	old := *h
	n := len(old) - 1
	it := old[n]
	*h = old[:n]
	return it
}

func (t *hashFieldTTLs) valid(it fieldTTLItem) bool {
	at, ok := t.at[it.field]
	return ok && at == it.at
}

// settle drops stale entries from the top of the heap, compacts a heap that
// is mostly stale, and returns the earliest expiry (0 when none).
func (t *hashFieldTTLs) settle() int64 {
	if len(t.heap) > 2*len(t.at)+32 {
		t.heap = t.heap[:0]
		for f, at := range t.at {
			t.heap = append(t.heap, fieldTTLItem{at: at, field: f})
		}
		heap.Init(&t.heap)
	}
	for len(t.heap) > 0 && !t.valid(t.heap[0]) {
		heap.Pop(&t.heap)
	}
	if len(t.heap) == 0 {
		return 0
	}
	return t.heap[0].at
}

// FieldExpireAt returns field's expiry in unix milliseconds, 0 when it has
// none. The caller holds the value's lock.
func (v *HashValue) FieldExpireAt(field string) int64 {
	if v.ttl == nil {
		return 0
	}
	return v.ttl.at[field]
}

// SetFieldExpireAt gives the existing field an expiry (unix milliseconds,
// > 0). The caller holds the value's write lock.
func (v *HashValue) SetFieldExpireAt(field string, at int64) {
	if _, ok := v.Fields[field]; !ok || at <= 0 {
		return
	}
	if v.ttl == nil {
		v.ttl = &hashFieldTTLs{at: make(map[string]int64)}
	}
	old, had := v.ttl.at[field]
	if had && old == at {
		return
	}
	if !had {
		v.size.add(hashFieldTTLOverhead)
	}
	v.ttl.at[field] = at
	heap.Push(&v.ttl.heap, fieldTTLItem{at: at, field: field})
	v.minExpire.Store(v.ttl.settle())
}

// PersistField removes field's expiry and reports whether it had one. The
// caller holds the value's write lock.
func (v *HashValue) PersistField(field string) bool {
	if v.ttl == nil {
		return false
	}
	if _, ok := v.ttl.at[field]; !ok {
		return false
	}
	delete(v.ttl.at, field)
	v.size.add(-hashFieldTTLOverhead)
	v.minExpire.Store(v.ttl.settle())
	return true
}

// HasFieldTTLs reports whether any field has an expiry.
func (v *HashValue) HasFieldTTLs() bool { return v.minExpire.Load() != 0 }

// NextFieldExpiry returns the earliest field expiry (unix ms), 0 when none.
func (v *HashValue) NextFieldExpiry() int64 { return v.minExpire.Load() }

// FieldTTLsUsed reports whether a field of the hash was ever given an
// expiry, which is what makes Redis report the listpackex encoding. The
// caller holds the value's lock.
func (v *HashValue) FieldTTLsUsed() bool { return v.ttl != nil }

// FieldExpiries returns a copy of the field expiries, nil when no field has
// one. The caller holds the value's lock.
func (v *HashValue) FieldExpiries() map[string]int64 {
	if v.ttl == nil || len(v.ttl.at) == 0 {
		return nil
	}
	out := make(map[string]int64, len(v.ttl.at))
	for f, at := range v.ttl.at {
		out[f] = at
	}
	return out
}

// HashFieldClock is the time, in unix milliseconds, against which a write
// judges whether a hash field has expired: 0, meaning never, while the store
// is loading or applying its master's stream, which deletes fields itself.
func (s *Store) HashFieldClock() int64 {
	if s.loading.Load() {
		return 0
	}
	return time.Now().UnixMilli()
}

// FieldLive reports whether field is set and, when now > 0, not past its
// expiry. Only a replica keeps expired fields, until the master deletes
// them. The caller holds the value's lock.
func (v *HashValue) FieldLive(field string, now int64) bool {
	_, ok := v.LiveField(field, now)
	return ok
}

// LiveField returns field's value when FieldLive reports it. The caller
// holds the value's lock.
func (v *HashValue) LiveField(field string, now int64) ([]byte, bool) {
	val, ok := v.Fields[field]
	if !ok || v.expiredAt(field, now) {
		return nil, false
	}
	return val, true
}

// expiredAt reports whether field has an expiry at or before now (> 0).
func (v *HashValue) expiredAt(field string, now int64) bool {
	if v.ttl == nil || now <= 0 {
		return false
	}
	at := v.ttl.at[field]
	return at != 0 && at <= now
}

// dropExpiredTTL clears field's expiry when it has passed at now, so a
// write to a field a replica still holds after its expiry creates it
// afresh. The caller holds the value's write lock.
func (v *HashValue) dropExpiredTTL(field string, now int64) {
	if v.expiredAt(field, now) {
		v.clearFieldTTL(field)
	}
}

func (v *HashValue) hasLiveField(now int64) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	for f := range v.Fields {
		if at := v.FieldExpireAt(f); at == 0 || at > now {
			return true
		}
	}
	return false
}

func (v *HashValue) fieldsDue(now int64) bool {
	m := v.minExpire.Load()
	return m != 0 && m <= now
}

// ExpireDueFields deletes the fields whose expiry is at or before now, at
// most limit of them when limit > 0, and returns their names. The caller
// holds the value's write lock.
func (v *HashValue) ExpireDueFields(now int64, limit int) []string {
	if v.ttl == nil {
		return nil
	}
	var expired []string
	for len(v.ttl.heap) > 0 && v.ttl.heap[0].at <= now {
		if limit > 0 && len(expired) >= limit {
			break
		}
		it := heap.Pop(&v.ttl.heap).(fieldTTLItem)
		if !v.ttl.valid(it) {
			continue
		}
		v.DeleteField(it.field)
		expired = append(expired, it.field)
	}
	v.minExpire.Store(v.ttl.settle())
	return expired
}

// clearFieldTTL drops field's expiry, if any, when the field is overwritten
// or deleted. The caller holds the value's write lock.
func (v *HashValue) clearFieldTTL(field string) {
	if v.ttl == nil {
		return
	}
	if _, ok := v.ttl.at[field]; ok {
		delete(v.ttl.at, field)
		v.size.add(-hashFieldTTLOverhead)
		v.minExpire.Store(v.ttl.settle())
	}
}

func (v *HashValue) ttlSizeLocked() int64 {
	if v.ttl == nil {
		return 0
	}
	return int64(len(v.ttl.at)) * hashFieldTTLOverhead
}

func (v *HashValue) cloneTTLsLocked(dst *HashValue) {
	if v.ttl == nil {
		return
	}
	dst.ttl = &hashFieldTTLs{at: make(map[string]int64, len(v.ttl.at))}
	for f, at := range v.ttl.at {
		dst.ttl.at[f] = at
		dst.ttl.heap = append(dst.ttl.heap, fieldTTLItem{at: at, field: f})
	}
	heap.Init(&dst.ttl.heap)
	dst.minExpire.Store(dst.ttl.settle())
}

// liveView returns a copy of the hash without the fields expired at now, for
// a replica, which hides expired fields but leaves their removal to the
// master's HDEL.
func (v *HashValue) liveView(now int64) *HashValue {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := &HashValue{Fields: make(map[string][]byte, len(v.Fields))}
	for f, val := range v.Fields {
		if at := v.FieldExpireAt(f); at != 0 && at <= now {
			continue
		}
		out.Fields[f] = val
	}
	if v.ttl != nil {
		out.ttl = &hashFieldTTLs{at: make(map[string]int64)}
		for f, at := range v.ttl.at {
			if at > now {
				out.ttl.at[f] = at
				out.ttl.heap = append(out.ttl.heap, fieldTTLItem{at: at, field: f})
			}
		}
		heap.Init(&out.ttl.heap)
		out.minExpire.Store(out.ttl.settle())
	}
	return out
}

// hashExpiryIndex orders the keys whose hashes have field expiries by the
// earliest one, so active expiry visits only hashes with due fields. A key is
// listed at most once per scheduled time; an entry whose time no longer
// matches sched is stale and dropped when popped.
type hashExpiryIndex struct {
	mu    sync.Mutex
	sched map[string]int64
	heap  hashExpiryHeap
}

type hashExpiryItem struct {
	at  int64
	key string
}

type hashExpiryHeap []hashExpiryItem

func (h hashExpiryHeap) Len() int           { return len(h) }
func (h hashExpiryHeap) Less(i, j int) bool { return h[i].at < h[j].at }
func (h hashExpiryHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *hashExpiryHeap) Push(x any)        { *h = append(*h, x.(hashExpiryItem)) }
func (h *hashExpiryHeap) Pop() any {
	old := *h
	n := len(old) - 1
	it := old[n]
	*h = old[:n]
	return it
}

func newHashExpiryIndex() *hashExpiryIndex {
	return &hashExpiryIndex{sched: make(map[string]int64)}
}

// schedule makes key due no later than at.
func (ix *hashExpiryIndex) schedule(key string, at int64) {
	if ix == nil || at <= 0 {
		return
	}
	ix.mu.Lock()
	if cur, ok := ix.sched[key]; !ok || at < cur {
		ix.sched[key] = at
		heap.Push(&ix.heap, hashExpiryItem{at: at, key: key})
	}
	ix.mu.Unlock()
}

// popDue removes and returns up to max keys due at now.
func (ix *hashExpiryIndex) popDue(now int64, max int) []string {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	var keys []string
	for len(ix.heap) > 0 && ix.heap[0].at <= now && len(keys) < max {
		it := heap.Pop(&ix.heap).(hashExpiryItem)
		if cur, ok := ix.sched[it.key]; !ok || cur != it.at {
			continue
		}
		delete(ix.sched, it.key)
		keys = append(keys, it.key)
	}
	return keys
}

func (ix *hashExpiryIndex) reset() {
	ix.mu.Lock()
	ix.sched = make(map[string]int64)
	ix.heap = nil
	ix.mu.Unlock()
}

// Len returns the number of keys scheduled for field expiry.
func (ix *hashExpiryIndex) Len() int {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	return len(ix.sched)
}

// ScheduleHashFieldExpiry queues key for active field expiry after a command
// gave fields of the hash v an expiry.
func (s *Store) ScheduleHashFieldExpiry(key string, v *HashValue) {
	s.hfe.schedule(key, v.NextFieldExpiry())
}

// HashFieldExpiryDeletes reports whether a field expiry in the past deletes
// the field, as on a master. While loading and on a replica the field keeps
// the expiry instead: the master's HDEL removes it.
func (s *Store) HashFieldExpiryDeletes() bool {
	return !s.loading.Load() && !s.passiveExpiry.Load()
}

// reapHashFieldsLocked deletes the due fields of h, the value of entry under
// key, at most limit of them when limit > 0. The caller holds the shard's
// write lock. It announces the expired fields, removes the key when the hash
// is left empty, and returns the fields, whether the key was removed and the
// memory delta, for the caller to apply once the shard is unlocked
// (finishHashReap).
func (s *Store) reapHashFieldsLocked(shard *Shard, key string, entry *Entry, h *HashValue, now int64, limit int) ([]string, bool, int64) {
	h.Lock()
	fields := h.ExpireDueFields(now, limit)
	if len(fields) == 0 {
		h.Unlock()
		return nil, false, 0
	}
	empty := len(h.Fields) == 0
	if empty {
		h.markDeadLocked()
	}
	next := h.minExpire.Load()
	h.Unlock()
	s.fireHashFieldsExpiredLocked(key, fields)
	if empty {
		return fields, true, -shard.deleteLocked(key, entry)
	}
	cur := footprint(key, entry)
	delta := cur - entry.accounted.Swap(cur)
	shard.memUsage += delta
	if next != 0 {
		s.hfe.schedule(key, next)
	}
	return fields, false, delta
}

func (s *Store) finishHashReap(key string, entry *Entry, fields []string, deleted bool, delta int64) {
	if len(fields) == 0 {
		return
	}
	s.trackMemory(delta)
	if deleted {
		s.tagIndex.RemoveKey(key, entry.TagList())
	}
	s.TouchKey(key)
	GlobalMetrics.RecordHashFieldsExpired(int64(len(fields)))
	s.Notify(NotifyHash, "hexpired", key)
	if deleted {
		s.Notify(NotifyGeneric, "del", key)
	}
}

// settleHashFields applies lazy field expiry to the hash h stored in entry
// before a read. It returns the entry the reader should use and whether the
// key still exists; retry is set when the key changed under it. A writer on
// a replica gets the stored hash, expired fields included.
func (s *Store) settleHashFields(shard *Shard, key string, entry *Entry, h *HashValue, write bool) (*Entry, bool, bool) {
	now := time.Now().UnixMilli()
	if !h.fieldsDue(now) || s.loading.Load() {
		return entry, true, false
	}
	if s.passiveExpiry.Load() {
		if write {
			return entry, h.hasLiveField(now), false
		}
		view := h.liveView(now)
		if len(view.Fields) == 0 {
			return nil, false, false
		}
		e := entry.rehome()
		e.Value = view
		return e, true, false
	}
	shard.mu.Lock()
	if shard.data[key] != entry {
		shard.mu.Unlock()
		return nil, false, true
	}
	fields, deleted, delta := s.reapHashFieldsLocked(shard, key, entry, h, now, 0)
	shard.mu.Unlock()
	s.finishHashReap(key, entry, fields, deleted, delta)
	if deleted {
		return nil, false, false
	}
	return entry, true, false
}

// hashFieldActiveBudget bounds the fields one active expiry cycle removes.
const hashFieldActiveBudget = 2000

// activeExpireHashFields removes due hash fields without reads: it visits
// the hashes the expiry index lists as due, at most hashFieldActiveBudget
// fields per call.
func (s *Store) activeExpireHashFields(now int64) {
	if s.passiveExpiry.Load() || s.loading.Load() {
		return
	}
	budget := hashFieldActiveBudget
	for budget > 0 {
		keys := s.hfe.popDue(now, 64)
		if len(keys) == 0 {
			return
		}
		for i, key := range keys {
			if budget <= 0 {
				for _, k := range keys[i:] {
					s.hfe.schedule(k, now)
				}
				return
			}
			release := s.expiryGuard(key)
			budget -= s.expireHashFieldsOf(key, now, budget)
			release()
		}
	}
}

func (s *Store) expireHashFieldsOf(key string, now int64, limit int) int {
	shard := s.shards[s.shardIndex(key)]
	shard.mu.Lock()
	entry, ok := shard.data[key]
	if !ok {
		shard.mu.Unlock()
		return 0
	}
	h, ok := entry.Value.(*HashValue)
	if !ok || entry.IsExpired() {
		shard.mu.Unlock()
		return 0
	}
	if !h.fieldsDue(now) {
		if next := h.NextFieldExpiry(); next != 0 {
			s.hfe.schedule(key, next)
		}
		shard.mu.Unlock()
		return 0
	}
	fields, deleted, delta := s.reapHashFieldsLocked(shard, key, entry, h, now, limit)
	shard.mu.Unlock()
	s.finishHashReap(key, entry, fields, deleted, delta)
	return len(fields)
}

// scheduleHashValue lists key in the field expiry index when value is a hash
// with field expiries. The caller may hold the shard lock.
func (s *Store) scheduleHashValue(key string, value Value) {
	if h, ok := value.(*HashValue); ok {
		s.hfe.schedule(key, h.NextFieldExpiry())
	}
}

// HashFieldExpiryScheduled returns the number of keys listed for active
// field expiry.
func (s *Store) HashFieldExpiryScheduled() int { return s.hfe.Len() }
