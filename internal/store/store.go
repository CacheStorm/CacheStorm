package store

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/logger"
)

const (
	// MaxKeySize limits the maximum key length in bytes
	MaxKeySize = 64 * 1024 // 64KB
	// MaxValueSize limits the maximum value size in bytes
	MaxValueSize = 512 * 1024 * 1024 // 512MB
)

var (
	ErrKeyNotFound   = errors.New("key not found")
	ErrKeyExists     = errors.New("key already exists")
	ErrWrongType     = errors.New("WRONGTYPE Operation against a key holding the wrong kind of value")
	ErrMemoryLimit   = errors.New("OOM command not allowed when used memory > 'maxmemory'")
	ErrInvalidKey    = errors.New("ERR invalid key name")
	ErrKeyTooLarge   = fmt.Errorf("ERR string length limit is %d bytes", MaxKeySize)
	ErrValueTooLarge = fmt.Errorf("ERR string length limit is %d bytes", MaxValueSize)
)

type GetResult struct {
	Entry *Entry
	Found bool
}

type SetOptions struct {
	TTL        time.Duration
	ExpireAtMS int64
	NX         bool
	XX         bool
	KeepTTL    bool
	Tags       []string
}

type Store struct {
	order         uint64
	passiveExpiry atomic.Bool
	loading       atomic.Bool
	dbIndex       atomic.Int32
	shards        [NumShards]*Shard
	tagIndex      *TagIndex
	pubsub        *PubSub
	keyNotifier   *KeyNotifier
	versions      map[string]int64
	watchRefs     map[string]int
	watching      atomic.Int64
	versionMu     sync.RWMutex
	memTracker    *MemoryTracker
	evictor       *EvictionController
	expiry        *TimingWheel
	hfe           *hashExpiryIndex
	setOpsMu      sync.RWMutex
	expiryMu      sync.Mutex
	expiryActive  bool
	hooksMu       sync.RWMutex
	hooks         StoreHooks
}

func NewStore() *Store {
	s := &Store{
		order:       storeOrder.Add(1),
		tagIndex:    NewTagIndex(),
		pubsub:      NewPubSub(),
		keyNotifier: NewKeyNotifier(),
		versions:    make(map[string]int64),
		watchRefs:   make(map[string]int),
	}
	s.expiry = NewTimingWheel(s)
	s.hfe = newHashExpiryIndex()
	for i := 0; i < NumShards; i++ {
		s.shards[i] = NewShard()
		s.shards[i].loading = &s.loading
		s.shards[i].hfe = s.hfe
	}
	return s
}

// MemoryLimit returns the configured memory limit (0 when unlimited) and the
// eviction policy.
func (s *Store) MemoryLimit() (int64, EvictionPolicy) {
	if s.memTracker == nil || s.evictor == nil {
		return 0, EvictionNoEviction
	}
	return s.memTracker.Max(), s.evictor.Policy()
}

// SetMemoryLimit changes the memory limit, eviction policy and sample size at
// runtime. Every database shares the tracker and the eviction settings, so
// calling it on any one of them applies to all. The store must have been set
// up with ConfigureMemory.
func (s *Store) SetMemoryLimit(maxMemory int64, policy EvictionPolicy, sampleSize int) bool {
	if s.memTracker == nil || s.evictor == nil {
		return false
	}
	s.evictor.SetPolicy(policy)
	s.evictor.SetSampleSize(sampleSize)
	s.memTracker.SetMax(maxMemory)
	return true
}

// ConfigureMemory sets up memory tracking and eviction. Call after NewStore().
func (s *Store) ConfigureMemory(maxMemory int64, policy EvictionPolicy, warningPct, criticalPct, sampleSize int) {
	s.memTracker = NewMemoryTracker(maxMemory, warningPct, criticalPct)
	s.evictor = NewEvictionController(policy, maxMemory, s, s.memTracker, sampleSize)
	s.evictor.SetOnEvict(func(key string, entry *Entry) {
		s.fireEvict(key, entry.Value)
	})
}

// MemoryPeak returns the highest tracked memory usage, shared by every
// database that shares this store's tracker; 0 without a tracker.
func (s *Store) MemoryPeak() int64 {
	if s.memTracker == nil {
		return 0
	}
	return s.memTracker.Peak()
}

// ResetMemoryPeak restarts peak tracking from the current usage.
func (s *Store) ResetMemoryPeak() {
	if s.memTracker != nil {
		s.memTracker.ResetPeak()
	}
}

// trackMemory applies a shard-size delta to the configured memory tracker so
// pressure checks and eviction observe live usage. Shard Set/Delete/Flush
// return exact byte deltas; without this feed the tracker stays at zero,
// CanAllocate always succeeds, and max_memory is silently unenforced.
func (s *Store) trackMemory(delta int64) {
	if delta != 0 && s.memTracker != nil {
		s.memTracker.Add(delta)
	}
}

// Reaccount charges the memory a key's value gained or lost in place since
// it was last accounted. Set and Delete account whole values; the commands
// that modify a collection in place (RPUSH, HSET, SADD, ZADD, XADD, ...) run
// this afterwards for the keys they wrote. It is O(1): collections keep their
// size up to date as they change.
func (s *Store) Reaccount(keys ...string) {
	for _, key := range keys {
		s.trackMemory(s.shards[s.shardIndex(key)].reaccount(key))
	}
}

// CheckWriteMemory enforces maxmemory before a command that may grow memory,
// as Redis does for commands flagged denyoom: when used memory is over the
// limit it evicts per the policy and refuses with ErrMemoryLimit if that does
// not bring usage back under the limit. Store.Set only sees new values, so
// without this a collection could grow in place without bound.
func (s *Store) CheckWriteMemory() error {
	mt := s.memTracker
	if mt == nil || mt.Max() <= 0 || mt.Usage() <= mt.Max() {
		return nil
	}
	if s.evictor.evicts() {
		if err := s.evictor.CheckAndEvict(); err != nil {
			logger.Warn().Err(err).Msg("eviction attempt failed")
		}
	}
	if mt.Usage() > mt.Max() {
		return ErrMemoryLimit
	}
	return nil
}

func (s *Store) MemoryTracker() *MemoryTracker {
	return s.memTracker
}

func (s *Store) Evictor() *EvictionController {
	return s.evictor
}

// StartExpiry starts the timing wheel that removes expired keys without
// requiring reads. Idempotent while running; a stopped wheel can be started
// again (Stop resets its channel).
func (s *Store) StartExpiry() {
	s.expiryMu.Lock()
	defer s.expiryMu.Unlock()
	if s.expiryActive {
		return
	}
	s.expiryActive = true
	s.expiry.Start()
}

// StopExpiry stops active expiration. Safe to call when not running.
func (s *Store) StopExpiry() {
	s.expiryMu.Lock()
	defer s.expiryMu.Unlock()
	if !s.expiryActive {
		return
	}
	s.expiryActive = false
	s.expiry.Stop()
}

// scheduleExpiry queues a key on the timing wheel so active expiration can
// remove it without a read. No-op when the key has no expiry. Stale schedules
// (key deleted, rescheduled, or PERSISTed) fire as no-ops — expireKey only
// removes entries that are still actually expired.
func (s *Store) scheduleExpiry(key string, expiresAt int64) {
	if expiresAt > 0 && s.expiry != nil {
		s.expiry.Add(key, expiresAt)
	}
}

// DeleteIfExpired removes key only if it is present and past its expiry, with
// the same bookkeeping as Delete. The expiry check and the removal both happen
// under the shard lock so a stale schedule can never delete a rescheduled or
// PERSISTed key. Reports whether the key was removed.
func (s *Store) DeleteIfExpired(key string) bool {
	if s.passiveExpiry.Load() {
		return false
	}
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	shard.mu.Lock()
	entry, exists := shard.data[key]
	if !exists || !entry.IsExpired() {
		shard.mu.Unlock()
		return false
	}
	mem := shard.deleteLocked(key, entry)
	s.fireExpireLocked(key)
	shard.mu.Unlock()

	s.trackMemory(-mem)
	s.tagIndex.RemoveKey(key, entry.TagList())
	s.TouchKey(key)
	s.fireExpire(key, entry.Value)
	return true
}

// reapExpired removes entry from shard if it is still the entry stored under
// key, announcing the expiry under the shard lock. It reports false when the
// key was replaced or removed in the meantime.
func (s *Store) reapExpired(shard *Shard, key string, entry *Entry) (int64, bool) {
	shard.mu.Lock()
	defer shard.mu.Unlock()
	cur, ok := shard.data[key]
	if !ok {
		return 0, true
	}
	if cur != entry {
		return 0, false
	}
	mem := shard.deleteLocked(key, cur)
	s.fireExpireLocked(key)
	return mem, true
}

func (s *Store) KeyNotifier() *KeyNotifier {
	return s.keyNotifier
}

// versionStamp numbers every modification of a watched key. It is shared by
// all stores, so a stamp is never reused: a key that is deleted and recreated
// can never return to the version a WATCH recorded.
var versionStamp atomic.Int64

// GetVersion returns the modification stamp of a watched key: 0 until the key
// is modified while watched.
func (s *Store) GetVersion(key string) int64 {
	s.versionMu.RLock()
	defer s.versionMu.RUnlock()
	return s.versions[key]
}

// WatchKey registers a WATCH on key and returns its current version. Only
// watched keys carry a version, so the map is bounded by the open watches.
func (s *Store) WatchKey(key string) int64 {
	s.versionMu.Lock()
	defer s.versionMu.Unlock()
	if s.watchRefs[key] == 0 {
		s.watching.Add(1)
	}
	s.watchRefs[key]++
	return s.versions[key]
}

// UnwatchKey drops one WATCH registration taken by WatchKey.
func (s *Store) UnwatchKey(key string) {
	s.versionMu.Lock()
	defer s.versionMu.Unlock()
	n, ok := s.watchRefs[key]
	if !ok {
		return
	}
	if n <= 1 {
		delete(s.watchRefs, key)
		delete(s.versions, key)
		s.watching.Add(-1)
		return
	}
	s.watchRefs[key] = n - 1
}

// HasWatchedKeys reports whether any client watches a key of this store.
func (s *Store) HasWatchedKeys() bool {
	return s.watching.Load() > 0
}

// touchLocked bumps key's version if it is watched. Caller holds versionMu.
func (s *Store) touchLocked(key string) {
	if s.watchRefs[key] > 0 {
		s.versions[key] = versionStamp.Add(1)
	}
}

// TouchKey records a modification of key, aborting every transaction that
// watches it.
func (s *Store) TouchKey(key string) {
	if s.watching.Load() == 0 {
		return
	}
	s.versionMu.Lock()
	s.touchLocked(key)
	s.versionMu.Unlock()
}

// TouchKeys is TouchKey for several keys.
func (s *Store) TouchKeys(keys []string) {
	if s.watching.Load() == 0 || len(keys) == 0 {
		return
	}
	s.versionMu.Lock()
	for _, k := range keys {
		s.touchLocked(k)
	}
	s.versionMu.Unlock()
}

// TouchAllWatched bumps every watched key of this store, the effect a
// database swap has on watchers.
func (s *Store) TouchAllWatched() {
	if s.watching.Load() == 0 {
		return
	}
	s.versionMu.Lock()
	for k := range s.watchRefs {
		s.versions[k] = versionStamp.Add(1)
	}
	s.versionMu.Unlock()
}

// TouchWatchedPresent bumps the watched keys of this store that exist in it
// or in other, as Redis' touchAllWatchedKeysInDb does on SWAPDB: a watched
// key missing from both databases is not changed by the swap.
func (s *Store) TouchWatchedPresent(other *Store) {
	if s.watching.Load() == 0 {
		return
	}
	s.versionMu.RLock()
	keys := make([]string, 0, len(s.watchRefs))
	for k := range s.watchRefs {
		keys = append(keys, k)
	}
	s.versionMu.RUnlock()
	present := keys[:0]
	for _, k := range keys {
		if s.Exists(k) || other.Exists(k) {
			present = append(present, k)
		}
	}
	s.TouchKeys(present)
}

// IncrementVersion records a modification of key (see TouchKey).
func (s *Store) IncrementVersion(key string) {
	s.TouchKey(key)
}

// DeleteVersion records the removal of key. A deletion is a modification, so
// a watcher of the key must see its version change.
func (s *Store) DeleteVersion(key string) {
	s.TouchKey(key)
}

func (s *Store) shardIndex(key string) uint32 {
	return fnv32a(key) & ShardMask
}

func fnv32a(s string) uint32 {
	const (
		offset32 = uint32(2166136261)
		prime32  = uint32(16777619)
	)
	h := offset32
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime32
	}
	return h
}

func (s *Store) Get(key string) (*Entry, bool) {
	return s.get(key, false)
}

// GetForWrite is Get for a caller that modifies the value in place. On a
// replica, where Get hides expired hash fields behind a copy, it returns the
// stored hash itself; the caller treats fields with HashValue.FieldLive.
func (s *Store) GetForWrite(key string) (*Entry, bool) {
	return s.get(key, true)
}

func (s *Store) get(key string, write bool) (*Entry, bool) {
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists {
		GlobalMetrics.RecordMiss()
		return nil, false
	}

	if entry.IsExpired() {
		if s.passiveExpiry.Load() {
			GlobalMetrics.RecordMiss()
			return nil, false
		}
		mem, reaped := s.reapExpired(shard, key, entry)
		if !reaped {
			return s.get(key, write)
		}
		s.trackMemory(-mem)
		s.DeleteVersion(key) // Clean up version to prevent memory leak
		s.tagIndex.RemoveKey(key, entry.TagList())
		s.fireExpire(key, entry.Value)
		// A lookup of an expired key counts as both a miss and an expiration
		// (lazy expiry here is the only expiry path that observes reads).
		GlobalMetrics.RecordMiss()
		GlobalMetrics.RecordExpiration()
		return nil, false
	}

	if h, ok := entry.Value.(*HashValue); ok && h.minExpire.Load() != 0 {
		settled, live, retry := s.settleHashFields(shard, key, entry, h, write)
		if retry {
			return s.get(key, write)
		}
		if !live {
			GlobalMetrics.RecordMiss()
			return nil, false
		}
		entry = settled
	}

	entry.Touch()
	GlobalMetrics.RecordHit()
	return entry, true
}

func (s *Store) ExpiryDeadline(key string) (int64, bool) {
	entry, ok := s.shards[s.shardIndex(key)].Get(key)
	if !ok {
		return 0, false
	}
	return entry.ExpiresAt.Load(), true
}

func validateStoreWrite(key string, value Value) (int64, error) {
	if key == "" {
		return 0, ErrInvalidKey
	}
	if len(key) > MaxKeySize {
		return 0, ErrKeyTooLarge
	}
	size := value.SizeOf()
	if size < 0 || size > MaxValueSize {
		return 0, ErrValueTooLarge
	}
	return size, nil
}

func (s *Store) Set(key string, value Value, opts SetOptions) error {
	size, err := validateStoreWrite(key, value)
	if err != nil {
		return err
	}
	opts.Tags = append([]string(nil), opts.Tags...)
	return s.applyValuePlan(key, &opts, nil, func(Value) (ValuePlan, error) {
		return ValuePlan{Size: size, Apply: func() Value { return value }}, nil
	})
}

// published records that value was just installed under key: the version a
// WATCH observes moves, and a blocked client is woken unless the value is an
// empty collection (see emptyCollection).
func (s *Store) published(key string, value Value) {
	s.IncrementVersion(key)
	if !emptyCollection(value) {
		s.keyNotifier.NotifyKey(key)
	}
}

// GetOrCreate returns the live value stored at key, or installs create() there
// when the key is absent (an expired entry counts as absent), reporting
// whether it created the value. Lookup and installation happen under the
// shard lock, so two concurrent creators can never both install a value and
// have the second overwrite the first's writes. Creation has the same
// validation, maxmemory enforcement, accounting and WATCH/notification effects
// as Set. The existing value is returned whatever its type: the caller checks.
func (s *Store) GetOrCreate(key string, create func() Value) (Value, bool, error) {
	if entry, ok := s.Get(key); ok {
		return entry.Value, false, nil
	}
	value := create()
	size, err := validateStoreWrite(key, value)
	if err != nil {
		return nil, false, err
	}
	var result Value
	created := false
	shard := s.shards[s.shardIndex(key)]
	err = s.applyValuePlan(key, nil, nil, func(current Value) (ValuePlan, error) {
		if current != nil {
			shard.data[key].Touch()
			GlobalMetrics.RecordHit()
			result = current
			created = false
			return ValuePlan{}, nil
		}
		result = value
		created = true
		return ValuePlan{Size: size, Apply: func() Value { return value }}, nil
	})
	if err != nil {
		return nil, false, err
	}
	return result, created, nil
}

// collection is a value that is removed from the keyspace when emptied.
type collection interface {
	Lock()
	Unlock()
	emptyLocked() bool
	markDeadLocked()
}

// DeleteIfEmpty removes key only while it still holds exactly v and v is
// empty, then marks v dead, with Delete's bookkeeping. The commands that
// empty a collection call it after releasing the value's lock: a plain Delete
// there raced a writer that had already looked up v, deleting the elements it
// added in between or leaving it to write into a value no longer stored.
// Writers that find v dead look the key up again. Reports whether the key
// was removed.
func (s *Store) DeleteIfEmpty(key string, v Value) bool {
	c, ok := v.(collection)
	if !ok {
		return false
	}
	shard := s.shards[s.shardIndex(key)]
	shard.mu.Lock()
	entry, exists := shard.data[key]
	if !exists || entry.Value != v {
		shard.mu.Unlock()
		return false
	}
	c.Lock()
	if !c.emptyLocked() {
		c.Unlock()
		shard.mu.Unlock()
		return false
	}
	c.markDeadLocked()
	c.Unlock()
	mem := shard.deleteLocked(key, entry)
	shard.mu.Unlock()

	s.trackMemory(-mem)
	s.tagIndex.RemoveKey(key, entry.TagList())
	s.TouchKey(key)
	return true
}

// emptyCollection reports a list or sorted set stored before its first
// element: the push commands create the value empty and then add to it. A
// blocked pop woken for the empty value finds nothing, and the wakeup the
// push sends once the element is in would then reach the next waiter in
// line instead, so creation does not notify; the push does.
func emptyCollection(value Value) bool {
	switch v := value.(type) {
	case *ListValue:
		v.RLock()
		defer v.RUnlock()
		return len(v.Elements) == 0
	case *SortedSetValue:
		v.RLock()
		defer v.RUnlock()
		return len(v.Members) == 0
	}
	return false
}

func (s *Store) SetEntry(key string, entry *Entry) error {
	if len(key) > MaxKeySize {
		return ErrKeyTooLarge
	}
	if entry == nil || entry.Value == nil {
		return ErrWrongType
	}
	entry = entry.rehome()
	entry.SetTags(append([]string(nil), entry.TagList()...))
	size := entry.Value.SizeOf()
	return s.applyValuePlan(key, nil, entry, func(Value) (ValuePlan, error) {
		return ValuePlan{Size: size, Apply: func() Value { return entry.Value }}, nil
	})
}

func (s *Store) Delete(key string) bool {
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists {
		return false
	}

	s.tagIndex.RemoveKey(key, entry.TagList())
	mem, deleted := shard.Delete(key)
	if deleted {
		s.trackMemory(-mem)
		s.TouchKey(key)
	}
	return deleted
}

func (s *Store) evictObserved(key string, expected *Entry) bool {
	shard := s.shards[s.shardIndex(key)]
	shard.mu.Lock()
	entry, ok := shard.data[key]
	if !ok || entry != expected {
		shard.mu.Unlock()
		return false
	}
	freed := shard.deleteLocked(key, entry)
	s.fireEvictLocked(key)
	shard.mu.Unlock()
	s.trackMemory(-freed)
	s.tagIndex.RemoveKey(key, entry.TagList())
	s.TouchKey(key)
	return true
}

func (s *Store) DeleteBatch(keys []string) int {
	if len(keys) == 0 {
		return 0
	}

	shardOps := make(map[*Shard][]string)
	for _, key := range keys {
		idx := s.shardIndex(key)
		shard := s.shards[idx]
		shardOps[shard] = append(shardOps[shard], key)
	}

	deleted := 0
	freed := int64(0)
	var untag []struct {
		key  string
		tags []string
	}
	s.versionMu.Lock()
	for shard, shardKeys := range shardOps {
		shard.mu.Lock()
		for _, key := range shardKeys {
			entry, exists := shard.data[key]
			if !exists {
				continue
			}
			freed += shard.deleteLocked(key, entry)
			s.touchLocked(key)
			if len(entry.TagList()) > 0 {
				untag = append(untag, struct {
					key  string
					tags []string
				}{key, entry.TagList()})
			}
			deleted++
		}
		shard.mu.Unlock()
	}
	s.versionMu.Unlock()

	// Mirror Store.Delete's index bookkeeping, outside the shard/version
	// locks to preserve their ordering with the tag-index locks.
	for _, u := range untag {
		s.tagIndex.RemoveKey(u.key, u.tags)
	}

	s.trackMemory(-freed)

	return deleted
}

func (s *Store) Exists(key string) bool {
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists {
		return false
	}

	if entry.IsExpired() {
		if s.passiveExpiry.Load() {
			return false
		}
		mem, reaped := s.reapExpired(shard, key, entry)
		if !reaped {
			return s.Exists(key)
		}
		s.trackMemory(-mem)
		s.DeleteVersion(key) // Clean up version to prevent memory leak
		// Drop the tag-index mappings too. This is a removal path like every
		// other one in Store (Get's lazy expiry, Delete, DeleteIfExpired,
		// DeleteBatch); omitting it left a stale tag -> key entry pointing at
		// a key that no longer existed, so TAGKEYS reported a phantom key and
		// a later INVALIDATE <tag> CASCADE deleted a live key that had been
		// recreated WITHOUT that tag (Set only reconciles the tags of the
		// entry currently in the shard, which is by then the new untagged one).
		s.tagIndex.RemoveKey(key, entry.TagList())
		// Announce the expiry like every other reaping path. DeleteIfExpired
		// and Get's inline lazy expiry both call fireExpire, and the store's
		// event contract wires OnExpire to the plugin dispatchers in
		// internal/server. Without it, a key reaped by EXISTS was removed
		// silently, so a registered plugin never learned it had expired.
		s.fireExpire(key, entry.Value)
		return false
	}

	if h, ok := entry.Value.(*HashValue); ok && h.minExpire.Load() != 0 {
		_, live, retry := s.settleHashFields(shard, key, entry, h, false)
		if retry {
			return s.Exists(key)
		}
		return live
	}

	return true
}

func (s *Store) Type(key string) DataType {
	entry, exists := s.Get(key)
	if !exists {
		return DataType(0)
	}
	return entry.Value.Type()
}

func (s *Store) TTL(key string) time.Duration {
	entry, exists := s.Get(key)
	if !exists {
		return -2
	}
	return entry.TTL()
}

func (s *Store) TTLMillis(key string) int64 {
	entry, exists := s.Get(key)
	if !exists {
		return -2
	}
	return entry.TTLMillis()
}

func (s *Store) SetTTL(key string, ttl time.Duration) bool {
	expiresAt := time.Now().Add(ttl).UnixMilli()
	if !s.shards[s.shardIndex(key)].updateExpiry(key, expiresAt) {
		return false
	}
	s.TouchKey(key)
	s.scheduleExpiry(key, expiresAt)
	return true
}

func (s *Store) SetExpiresAt(key string, expiresAt int64) bool {
	if !s.shards[s.shardIndex(key)].updateExpiry(key, expiresAt) {
		return false
	}
	s.TouchKey(key)
	s.scheduleExpiry(key, expiresAt)
	return true
}

// Persist removes the key's expiry. It reports true only when the key exists
// and had an expiry to remove, which is what PERSIST replies.
func (s *Store) Persist(key string) bool {
	// ExpiresAt 0 clears the expiry; any stale wheel schedule for this key
	// later fires as a no-op (expireKey only removes actually-expired keys).
	if !s.shards[s.shardIndex(key)].clearExpiry(key) {
		return false
	}
	s.TouchKey(key)
	return true
}

func (s *Store) GetTTL(key string) time.Duration {
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists {
		return -2 * time.Second
	}

	if entry.IsExpired() {
		return -2 * time.Second
	}

	if entry.ExpiresAt.Load() == 0 {
		return -1 * time.Second
	}

	remaining := expiryDuration(entry.ExpiresAt.Load() - time.Now().UnixMilli())
	if remaining < 0 {
		return -2 * time.Second
	}

	return remaining
}

func (s *Store) KeyCount() int64 {
	var count int64
	for i := 0; i < NumShards; i++ {
		count += s.shards[i].Len()
	}
	return count
}

func (s *Store) MemUsage() int64 {
	var usage int64
	for i := 0; i < NumShards; i++ {
		usage += s.shards[i].MemUsage()
	}
	return usage
}

func (s *Store) Keys() []string {
	keys := make([]string, 0)
	for i := 0; i < NumShards; i++ {
		keys = append(keys, s.shards[i].Keys()...)
	}
	return keys
}

func (s *Store) Flush() {
	keyCount := s.KeyCount()
	if s.watching.Load() > 0 {
		s.versionMu.Lock()
		for k := range s.watchRefs {
			if s.shards[s.shardIndex(k)].Exists(k) {
				s.touchLocked(k)
			}
		}
		s.versionMu.Unlock()
	}
	freed := int64(0)
	for i := 0; i < NumShards; i++ {
		freed += s.shards[i].Flush()
	}
	s.hfe.reset()
	for _, tag := range s.tagIndex.Tags() {
		s.tagIndex.invalidate(tag)
	}
	s.trackMemory(-freed)
	logger.Info().Int64("flushed_keys", keyCount).Msg("store flushed")
}

func (s *Store) GetShard(key string) *Shard {
	return s.shards[s.shardIndex(key)]
}

// MoveSetMember atomically moves member from the set at srcKey to the set
// at dstKey, creating the destination when missing and deleting an emptied
// source. Both set locks are acquired in deterministic key order, so
// concurrent opposite-direction moves cannot deadlock, and no observer can
// see the member in neither set: the destination is populated before the
// source is drained. A wrong-type source or destination fails before any
// mutation, leaving the member in place.
func (s *Store) MoveSetMember(srcKey, dstKey, member string) (bool, error) {
	return s.moveSetMember(srcKey, dstKey, member)
}

// SetOpsRLock takes the shared set-operations lock. Multi-key set reads
// (SUNION, SDIFF, SINTER and their STORE variants) hold it so a concurrent
// MoveSetMember — which takes it exclusively — cannot interleave inside
// their snapshots.
func (s *Store) SetOpsRLock() { s.setOpsMu.RLock() }

// SetOpsRUnlock releases the shared set-operations lock.
func (s *Store) SetOpsRUnlock() { s.setOpsMu.RUnlock() }

// SetOpsLock takes the set-operations lock exclusively. It is the write side
// of SetOpsRLock: MoveSetMember uses it so a multi-set read can never observe
// a move half-applied. It is exported so a check can tell whether a reader
// still holds the shared lock: while a reader holds it, this call blocks.
func (s *Store) SetOpsLock() { s.setOpsMu.Lock() }

// SetOpsUnlock releases the set-operations lock taken by SetOpsLock.
func (s *Store) SetOpsUnlock() { s.setOpsMu.Unlock() }

func (s *Store) GetTagIndex() *TagIndex {
	return s.tagIndex
}

func (s *Store) GetPubSub() *PubSub {
	return s.pubsub
}

func (s *Store) GetAll() map[string]*Entry {
	result := make(map[string]*Entry)
	for i := 0; i < NumShards; i++ {
		shardData := s.shards[i].GetAll()
		for k, v := range shardData {
			if !v.IsExpired() {
				result[k] = v
			}
		}
	}
	return result
}

// ForEachLive calls fn for every unexpired entry, shard by shard. Each shard's
// entries are collected under its read lock and fn runs after the lock is
// released, so fn may take other locks. It stops at the first error.
func (s *Store) ForEachLive(fn func(key string, e *Entry) error) error {
	type pair struct {
		key   string
		entry *Entry
	}
	var batch []pair
	for i := 0; i < NumShards; i++ {
		sh := s.shards[i]
		batch = batch[:0]
		sh.mu.RLock()
		for k, v := range sh.data {
			batch = append(batch, pair{k, v})
		}
		sh.mu.RUnlock()
		for _, p := range batch {
			if p.entry.IsExpired() {
				continue
			}
			if err := fn(p.key, p.entry); err != nil {
				return err
			}
		}
	}
	return nil
}
