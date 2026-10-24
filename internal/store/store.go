package store

import (
	"errors"
	"fmt"
	"math"
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
	TTL     time.Duration
	NX      bool
	XX      bool
	KeepTTL bool
	Tags    []string
}

type Store struct {
	shards       [NumShards]*Shard
	tagIndex     *TagIndex
	pubsub       *PubSub
	keyNotifier  *KeyNotifier
	versions     map[string]int64
	watchRefs    map[string]int
	watching     atomic.Int64
	versionMu    sync.RWMutex
	memTracker   *MemoryTracker
	evictor      *EvictionController
	expiry       *TimingWheel
	setOpsMu     sync.RWMutex
	expiryMu     sync.Mutex
	expiryActive bool
	hooksMu      sync.RWMutex
	hooks        StoreHooks
}

func NewStore() *Store {
	s := &Store{
		tagIndex:    NewTagIndex(),
		pubsub:      NewPubSub(),
		keyNotifier: NewKeyNotifier(),
		versions:    make(map[string]int64),
		watchRefs:   make(map[string]int),
	}
	s.expiry = NewTimingWheel(s)
	for i := 0; i < NumShards; i++ {
		s.shards[i] = NewShard()
	}
	return s
}

// ConfigureMemory sets up memory tracking and eviction. Call after NewStore().
func (s *Store) ConfigureMemory(maxMemory int64, policy EvictionPolicy, warningPct, criticalPct, sampleSize int) {
	s.memTracker = NewMemoryTracker(maxMemory, warningPct, criticalPct)
	s.evictor = NewEvictionController(policy, maxMemory, s, s.memTracker, sampleSize)
	s.evictor.SetOnEvict(func(key string, entry *Entry) {
		s.fireEvict(key, entry.Value)
	})
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
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	shard.mu.Lock()
	entry, exists := shard.data[key]
	if !exists || !entry.IsExpired() {
		shard.mu.Unlock()
		return false
	}
	keyOverhead := int64(len(key)) + 16
	mem := entry.MemoryUsage() + keyOverhead
	shard.memUsage -= mem
	shard.keyCount--
	delete(shard.data, key)
	shard.mu.Unlock()

	s.trackMemory(-mem)
	s.tagIndex.RemoveKey(key, entry.Tags)
	s.TouchKey(key)
	s.fireExpire(key, entry.Value)
	return true
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
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists {
		GlobalMetrics.RecordMiss()
		return nil, false
	}

	if entry.IsExpired() {
		mem, _ := shard.Delete(key)
		s.trackMemory(-mem)
		s.DeleteVersion(key) // Clean up version to prevent memory leak
		s.tagIndex.RemoveKey(key, entry.Tags)
		s.fireExpire(key, entry.Value)
		// A lookup of an expired key counts as both a miss and an expiration
		// (lazy expiry here is the only expiry path that observes reads).
		GlobalMetrics.RecordMiss()
		GlobalMetrics.RecordExpiration()
		return nil, false
	}

	entry.Touch()
	GlobalMetrics.RecordHit()
	return entry, true
}

// admit validates a write of value under key and enforces maxmemory,
// evicting first when needed. It takes no shard lock of its own key.
func (s *Store) admit(key string, value Value) error {
	// Validate key
	if len(key) == 0 {
		return ErrInvalidKey
	}
	if len(key) > MaxKeySize {
		return ErrKeyTooLarge
	}

	// Validate value size
	if value.SizeOf() > MaxValueSize {
		return ErrValueTooLarge
	}

	// Check memory limit and try eviction if needed
	if s.memTracker != nil && s.memTracker.Max() > 0 {
		valueSize := value.SizeOf()
		if !s.memTracker.CanAllocate(valueSize) {
			// Try eviction before rejecting
			if s.evictor != nil {
				if err := s.evictor.CheckAndEvict(); err != nil {
					logger.Warn().Err(err).Msg("eviction attempt failed")
				}
			}
			if !s.memTracker.CanAllocate(valueSize) {
				logger.Warn().
					Int64("value_size", valueSize).
					Int64("usage", s.memTracker.Usage()).
					Int64("max", s.memTracker.Max()).
					Msg("OOM: rejecting write, memory limit exceeded")
				return ErrMemoryLimit
			}
		}
	}
	return nil
}

func (s *Store) Set(key string, value Value, opts SetOptions) error {
	if err := s.admit(key, value); err != nil {
		return err
	}

	idx := s.shardIndex(key)
	shard := s.shards[idx]

	// Conditional writes must use the same "does this key exist" rule as
	// Store.Get: an expired key does not exist. The old raw shard.Get made a
	// key whose TTL had lapsed but which active expiry had not yet reaped look
	// present, so NX refused an entitled write and XX revived a dead key.
	if opts.NX {
		if _, exists := shard.getLive(key); exists {
			return ErrKeyExists
		}
	}

	if opts.XX {
		if _, exists := shard.getLive(key); !exists {
			return ErrKeyNotFound
		}
	}

	// Reconcile tag membership: the replaced entry's mappings must drop
	// before the new ones are added (RemoveKey-then-AddTags keeps tags
	// shared between old and new).
	var oldTags []string
	if prev, exists := shard.Get(key); exists {
		oldTags = prev.Tags
	}
	if len(oldTags) > 0 {
		s.tagIndex.RemoveKey(key, oldTags)
	}

	entry := NewEntry(value)
	if opts.KeepTTL {
		// Carry the previous expiry instant unchanged: in-place
		// modifications (APPEND, SETRANGE, INCR family) must not reset or
		// drop the key's TTL.
		if prev, exists := shard.Get(key); exists {
			entry.ExpiresAt.Store(prev.ExpiresAt.Load())
		}
	} else if opts.TTL > 0 {
		entry.SetTTL(opts.TTL)
		s.scheduleExpiry(key, entry.ExpiresAt.Load())
	}
	if len(opts.Tags) > 0 {
		entry.Tags = opts.Tags
		s.tagIndex.AddTags(key, opts.Tags)
	}

	s.trackMemory(shard.Set(key, entry))
	s.published(key, value)
	return nil
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
	if err := s.admit(key, value); err != nil {
		return nil, false, err
	}

	shard := s.shards[s.shardIndex(key)]
	shard.mu.Lock()
	if cur, ok := shard.data[key]; ok && !cur.IsExpired() {
		shard.mu.Unlock()
		cur.Touch()
		GlobalMetrics.RecordHit()
		return cur.Value, false, nil
	}
	delta, old := shard.setLocked(key, NewEntry(value))
	shard.mu.Unlock()

	s.trackMemory(delta)
	if old != nil {
		// The replaced entry had expired: reap it like Get's lazy expiry.
		s.tagIndex.RemoveKey(key, old.Tags)
		s.fireExpire(key, old.Value)
	}
	s.published(key, value)
	return value, true, nil
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
	s.tagIndex.RemoveKey(key, entry.Tags)
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

func (s *Store) SetEntry(key string, entry *Entry) {
	// Validate key size
	if len(key) > MaxKeySize {
		return
	}

	idx := s.shardIndex(key)
	shard := s.shards[idx]

	// Reconcile tag membership: a key being replaced (RENAME/COPY over an
	// existing key) must drop its old mappings before the entry's own tags
	// are added. RemoveKey with no tags sweeps every tag, so mappings left by
	// a previous occupant cannot survive the overwrite.
	s.tagIndex.RemoveKey(key, nil)
	if len(entry.Tags) > 0 {
		s.tagIndex.AddTags(key, entry.Tags)
	}

	s.trackMemory(shard.Set(key, entry))
	s.IncrementVersion(key)
	s.scheduleExpiry(key, entry.ExpiresAt.Load())
}

func (s *Store) Delete(key string) bool {
	idx := s.shardIndex(key)
	shard := s.shards[idx]

	entry, exists := shard.Get(key)
	if !exists {
		return false
	}

	s.tagIndex.RemoveKey(key, entry.Tags)
	mem, deleted := shard.Delete(key)
	if deleted {
		s.trackMemory(-mem)
		s.TouchKey(key)
	}
	return deleted
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
			keyOverhead := int64(len(key)) + 16
			mem := entry.MemoryUsage() + keyOverhead
			shard.memUsage -= mem
			freed += mem
			shard.keyCount--
			delete(shard.data, key)
			s.touchLocked(key)
			if len(entry.Tags) > 0 {
				untag = append(untag, struct {
					key  string
					tags []string
				}{key, entry.Tags})
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
		mem, _ := shard.Delete(key)
		s.trackMemory(-mem)
		s.DeleteVersion(key) // Clean up version to prevent memory leak
		// Drop the tag-index mappings too. This is a removal path like every
		// other one in Store (Get's lazy expiry, Delete, DeleteIfExpired,
		// DeleteBatch); omitting it left a stale tag -> key entry pointing at
		// a key that no longer existed, so TAGKEYS reported a phantom key and
		// a later INVALIDATE <tag> CASCADE deleted a live key that had been
		// recreated WITHOUT that tag (Set only reconciles the tags of the
		// entry currently in the shard, which is by then the new untagged one).
		s.tagIndex.RemoveKey(key, entry.Tags)
		// Announce the expiry like every other reaping path. DeleteIfExpired
		// and Get's inline lazy expiry both call fireExpire, and the store's
		// event contract wires OnExpire to the plugin dispatchers in
		// internal/server. Without it, a key reaped by EXISTS was removed
		// silently, so a registered plugin never learned it had expired.
		s.fireExpire(key, entry.Value)
		return false
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

// nowNSPlus returns time.Now().UnixNano() + d, SATURATING at the int64 bounds
// instead of wrapping.
//
//	time.Time.UnixNano is undefined outside 1678..2262, so a TTL longer than
//	the ~236 years of headroom int64 nanoseconds actually have produced a
//	NEGATIVE expiry instant. Entry.IsExpired then reported the key as already
//	expired and it was silently deleted behind a :1 reply -- and the command
//	layer deliberately accepts TTLs up to maxExpireAtSeconds (292 years).
func nowNSPlus(d int64) int64 {
	now := time.Now().UnixNano()
	if d > 0 && now > math.MaxInt64-d {
		return math.MaxInt64
	}
	if d < 0 && now < math.MinInt64-d {
		return math.MinInt64
	}
	return now + d
}

func (s *Store) SetTTL(key string, ttl time.Duration) bool {
	expiresAt := nowNSPlus(int64(ttl))
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

	remaining := time.Duration(entry.ExpiresAt.Load() - time.Now().UnixNano())
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
	for _, tag := range s.tagIndex.Tags() {
		s.tagIndex.invalidate(tag)
	}
	s.trackMemory(-freed)
	logger.Info().Int64("flushed_keys", keyCount).Msg("store flushed")
}

func (s *Store) GetShard(key string) *Shard {
	return s.shards[s.shardIndex(key)]
}

// getLiveEntry reads an entry without counting a client hit or touching
// LRU state, treating an expired entry as absent. For internal operations
// whose reads must not distort hit-rate accounting.
func (s *Store) getLiveEntry(key string) *Entry {
	entry, _ := s.GetShard(key).Get(key)
	if entry == nil {
		return nil
	}
	if expiresAt := entry.ExpiresAt.Load(); expiresAt > 0 && expiresAt <= time.Now().UnixNano() {
		return nil
	}
	return entry
}

// MoveSetMember atomically moves member from the set at srcKey to the set
// at dstKey, creating the destination when missing and deleting an emptied
// source. Both set locks are acquired in deterministic key order, so
// concurrent opposite-direction moves cannot deadlock, and no observer can
// see the member in neither set: the destination is populated before the
// source is drained. A wrong-type source or destination fails before any
// mutation, leaving the member in place.
func (s *Store) MoveSetMember(srcKey, dstKey, member string) (bool, error) {
	// The set-operations lock: exclusive for cross-key moves, shared for
	// multi-key reads (SUNION and friends). This is what makes SMOVE
	// atomic from every observer's perspective: no multi-key reader can
	// interleave inside the move, and the per-set value locks below keep
	// single-key operations mutually exclusive with it.
	s.setOpsMu.Lock()
	defer s.setOpsMu.Unlock()

	for {
		srcEntry := s.getLiveEntry(srcKey)
		if srcEntry == nil {
			return false, nil
		}
		srcSet, ok := srcEntry.Value.(*SetValue)
		if !ok {
			return false, ErrWrongType
		}
		if dstEntry := s.getLiveEntry(dstKey); dstEntry != nil {
			if _, ok := dstEntry.Value.(*SetValue); !ok {
				return false, ErrWrongType
			}
		}

		srcSet.Lock()
		dead := srcSet.dead
		_, exists := srcSet.Members[member]
		srcSet.Unlock()
		if dead {
			continue
		}
		// Same key: nothing to move; report membership like Redis does.
		if !exists || srcKey == dstKey {
			return exists, nil
		}

		// The destination is created before any value lock is taken: the
		// shard lock may only be acquired with no value lock held.
		v, created, err := s.GetOrCreate(dstKey, func() Value {
			return &SetValue{Members: make(map[string]struct{})}
		})
		if err != nil {
			return false, err
		}
		dstSet, ok := v.(*SetValue)
		if !ok {
			return false, ErrWrongType
		}

		// Per-set value locks in deterministic key order so concurrent
		// single-key operations on both sets serialize with the move.
		first, second := srcSet, dstSet
		if dstKey < srcKey {
			first, second = second, first
		}
		first.Lock()
		second.Lock()
		unlock := func() {
			second.Unlock()
			first.Unlock()
		}
		if srcSet.dead || dstSet.dead {
			unlock()
			if created {
				s.DeleteIfEmpty(dstKey, dstSet)
			}
			continue
		}
		if _, exists := srcSet.Members[member]; !exists {
			unlock()
			if created {
				s.DeleteIfEmpty(dstKey, dstSet)
			}
			return false, nil
		}

		// While both locks are held the member may be visible in both sets,
		// never in neither.
		dstSet.Members[member] = struct{}{}
		delete(srcSet.Members, member)
		srcEmptied := len(srcSet.Members) == 0
		unlock()

		if srcEmptied {
			s.DeleteIfEmpty(srcKey, srcSet)
		}
		return true, nil
	}
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
