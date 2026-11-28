package store

import (
	"cmp"
	"errors"
	"fmt"
	"runtime"
	"slices"
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
	// KeepTags keeps the key's tags, for a write that changes its value in
	// place (APPEND, SETRANGE, INCR...) rather than replacing it; Tags is
	// then ignored.
	KeepTags bool
	Tags     []string
	// Replaced, when set, receives the type of the live value the write
	// replaced; it is left alone when the key was absent.
	Replaced *DataType
	// Overwrite marks a write that replaces the key whatever it held, as a
	// *STORE command does with its destination. An expired value it finds
	// is replaced as it is rather than expired first: such a command may
	// have read the destination as one of its sources a moment before, when
	// it was alive, and an expiry recorded ahead of the command made the
	// replicas and the AOF run it without that source. (Redis looks every
	// key up at the command's one time.)
	Overwrite bool
}

type Store struct {
	// generation changes when the store's contents are replaced wholesale
	// (a flush, a replica's full copy), for caches built over its keys.
	generation    atomic.Uint64
	order         uint64
	passiveExpiry atomic.Bool
	clock         expiryClock
	plain         *entryMeta
	dbIndex       atomic.Int32
	shards        [NumShards]*Shard
	tagIndex      *TagIndex
	pubsub        *PubSub
	keyNotifier   *KeyNotifier
	// watch holds the WATCH registrations of the database index this
	// store serves; SWAPDB exchanges it with the other store's, so a watch
	// stays with its database index, as in Redis.
	watch        atomic.Pointer[WatchTable]
	memTracker   *MemoryTracker
	evictor      *EvictionController
	expiry       *activeExpiry
	hfe          *hashExpiryIndex
	setOpsMu     sync.RWMutex
	expiryMu     sync.Mutex
	expiryActive bool
	hooksMu      sync.RWMutex
	hooks        StoreHooks

	// activeExpireOff is DEBUG SET-ACTIVE-EXPIRE 0, shared by every
	// database of a server: the active expiry cycle skips its runs while
	// it is set, and keys and hash fields expire only when read.
	activeExpireOff *atomic.Bool
}

func NewStore() *Store {
	s := &Store{
		order:       storeOrder.Add(1),
		tagIndex:    NewTagIndex(),
		pubsub:      NewPubSub(),
		keyNotifier: NewKeyNotifier(),
	}
	s.watch.Store(newWatchTable())
	s.plain = &entryMeta{clock: &s.clock}
	s.expiry = newActiveExpiry(s)
	s.activeExpireOff = new(atomic.Bool)
	s.hfe = newHashExpiryIndex()
	for i := 0; i < NumShards; i++ {
		s.shards[i] = NewShard()
		s.shards[i].plain = s.plain
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

// MemoryPeakTime returns the unix second MemoryPeak was reached; 0 without
// a tracker.
func (s *Store) MemoryPeakTime() int64 {
	if s.memTracker == nil {
		return 0
	}
	return s.memTracker.PeakTime()
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
		s.trackMemory(reaccountIn(s.shards[s.shardIndex(key)], key))
	}
}

// ReaccountKey is Reaccount of one key given as bytes, which it does not
// copy into a string.
func (s *Store) ReaccountKey(key []byte) {
	s.trackMemory(reaccountIn(s.shards[fnv32a(key)&ShardMask], key))
}

// CheckWriteMemory enforces maxmemory before a command that may grow memory,
// as Redis does for commands flagged denyoom: when used memory is over the
// limit it evicts per the policy and refuses with ErrMemoryLimit if that does
// not bring usage back under the limit. Store.Set only sees new values, so
// without this a collection could grow in place without bound.
func (s *Store) CheckWriteMemory() error {
	return s.CheckWriteMemoryFor("")
}

// CheckWriteMemoryFor is CheckWriteMemory before a write of the key
// writing, which an eviction to make room may take as its victim.
func (s *Store) CheckWriteMemoryFor(writing string) error {
	mt := s.memTracker
	if mt == nil || mt.Max() <= 0 || mt.Usage() <= mt.Limit() {
		return nil
	}
	if s.evictor.evicts() {
		if err := s.evictor.CheckAndEvictFor(writing); err != nil {
			logger.Warn().Err(err).Msg("eviction attempt failed")
		}
	}
	if mt.Usage() > mt.Limit() {
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

// StartExpiry starts active expiry, which removes expired keys without
// requiring reads. Idempotent while running; a stopped cycle can be started
// again.
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

// WatchTable holds the keys clients WATCH in one database and a version
// for each, bumped whenever the key is modified. Only watched keys carry a
// version, so the maps are bounded by the open watches.
type WatchTable struct {
	mu       sync.RWMutex
	versions map[string]int64
	refs     map[string]int
	watching atomic.Int64
}

func newWatchTable() *WatchTable {
	return &WatchTable{versions: make(map[string]int64), refs: make(map[string]int)}
}

// Version returns the modification stamp of a watched key: 0 until the key
// is modified while watched.
func (w *WatchTable) Version(key string) int64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.versions[key]
}

func (w *WatchTable) watch(key string) int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.refs[key] == 0 {
		w.watching.Add(1)
	}
	w.refs[key]++
	return w.versions[key]
}

// Unwatch drops one registration taken by Store.Watch or Databases.Watch.
func (w *WatchTable) Unwatch(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, ok := w.refs[key]
	if !ok {
		return
	}
	if n <= 1 {
		delete(w.refs, key)
		delete(w.versions, key)
		w.watching.Add(-1)
		return
	}
	w.refs[key] = n - 1
}

// touchLocked bumps key's version if it is watched. Caller holds mu.
func (w *WatchTable) touchLocked(key string) {
	if w.refs[key] > 0 {
		w.versions[key] = versionStamp.Add(1)
	}
}

func (w *WatchTable) touch(keys ...string) {
	if w.watching.Load() == 0 || len(keys) == 0 {
		return
	}
	w.mu.Lock()
	for _, k := range keys {
		w.touchLocked(k)
	}
	w.mu.Unlock()
}

// touchIf bumps every watched key for which present reports true, or every
// watched key when present is nil. present runs without the table's lock.
func (w *WatchTable) touchIf(present func(string) bool) {
	if w.watching.Load() == 0 {
		return
	}
	if present == nil {
		w.mu.Lock()
		for k := range w.refs {
			w.touchLocked(k)
		}
		w.mu.Unlock()
		return
	}
	w.mu.RLock()
	keys := make([]string, 0, len(w.refs))
	for k := range w.refs {
		keys = append(keys, k)
	}
	w.mu.RUnlock()
	hit := keys[:0]
	for _, k := range keys {
		if present(k) {
			hit = append(hit, k)
		}
	}
	w.touch(hit...)
}

// GetVersion returns the modification stamp of a watched key: 0 until the key
// is modified while watched.
func (s *Store) GetVersion(key string) int64 {
	return s.watch.Load().Version(key)
}

// WatchKey registers a WATCH on key and returns its current version.
func (s *Store) WatchKey(key string) int64 {
	_, v := s.Watch(key)
	return v
}

// Watch registers a WATCH on key in the table of this store's database and
// returns the table, to check the version in and release the watch from
// later, and the key's current version. A connection that resolves its
// database by index while SWAPDB may run uses Databases.Watch.
func (s *Store) Watch(key string) (*WatchTable, int64) {
	for {
		w := s.watch.Load()
		v := w.watch(key)
		if s.watch.Load() == w {
			return w, v
		}
		w.Unwatch(key)
	}
}

// WatchedKeys reports how many distinct keys are watched in this database.
func (s *Store) WatchedKeys() int64 {
	return s.watch.Load().watching.Load()
}

// UnwatchKey drops one WATCH registration taken by WatchKey.
func (s *Store) UnwatchKey(key string) {
	s.watch.Load().Unwatch(key)
}

// HasWatchedKeys reports whether any client watches a key of this store.
func (s *Store) HasWatchedKeys() bool {
	return s.watch.Load().watching.Load() > 0
}

// TouchKey records a modification of key, aborting every transaction that
// watches it.
func (s *Store) TouchKey(key string) {
	s.watch.Load().touch(key)
}

// TouchKeys is TouchKey for several keys.
func (s *Store) TouchKeys(keys []string) {
	s.watch.Load().touch(keys...)
}

// TouchAllWatched bumps every watched key of this store, the effect a
// replacement of its contents has on watchers.
func (s *Store) TouchAllWatched() {
	s.watch.Load().touchIf(nil)
}

// TouchWatchedPresent bumps the watched keys of this store that exist in it
// or in other, as Redis' touchAllWatchedKeysInDb does on SWAPDB: a watched
// key missing from both databases is not changed by the swap.
func (s *Store) TouchWatchedPresent(other *Store) {
	s.watch.Load().touchIf(func(k string) bool { return s.Exists(k) || other.Exists(k) })
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

func fnv32a[K string | []byte](s K) uint32 {
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
	return s.get(key, false, true)
}

// GetBytes is Get of a key given as bytes, as commands receive it: a live
// key is found without copying it to a string. Expired keys and hashes with
// field TTLs take Get's path.
func (s *Store) GetBytes(key []byte) (*Entry, bool) {
	shard := s.shards[fnv32a(key)&ShardMask]
	shard.mu.RLock()
	entry, ok := shard.data[string(key)]
	shard.mu.RUnlock()
	if !ok {
		GlobalMetrics.RecordMiss()
		return nil, false
	}
	if entry.IsExpired() {
		return s.get(string(key), false, true)
	}
	if h, isHash := entry.Value.(*HashValue); isHash && h.minExpire.Load() != 0 {
		return s.get(string(key), false, true)
	}
	entry.Touch()
	GlobalMetrics.RecordHit()
	return entry, true
}

// Peek is Get without recording an access, for the commands Redis looks keys
// up for with LOOKUP_NOTOUCH (TYPE, TTL and its kin, OBJECT).
func (s *Store) Peek(key string) (*Entry, bool) {
	return s.get(key, false, false)
}

// PeekStored returns the stored entry of key even when it is past its
// expiry, without removing it or recording an access, for DEBUG
// DIGEST-VALUE, which Redis lets look at logically expired keys.
func (s *Store) PeekStored(key string) (*Entry, bool) {
	return s.shards[s.shardIndex(key)].Get(key)
}

// Stored reports whether key is stored, past its expiry or not, without
// removing it.
func (s *Store) Stored(key string) bool {
	_, ok := s.PeekStored(key)
	return ok
}

// Live reports whether key is stored and not past its expiry, without
// removing it when it is.
func (s *Store) Live(key string) bool {
	e, ok := s.PeekStored(key)
	return ok && !e.IsExpired()
}

// GetForWrite is Get for a caller that modifies the value in place. On a
// replica, where Get hides expired hash fields behind a copy, it returns the
// stored hash itself; the caller treats fields with HashValue.FieldLive.
func (s *Store) GetForWrite(key string) (*Entry, bool) {
	return s.get(key, true, true)
}

func (s *Store) get(key string, write, touch bool) (*Entry, bool) {
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
			return s.get(key, write, touch)
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
			return s.get(key, write, touch)
		}
		if !live {
			GlobalMetrics.RecordMiss()
			return nil, false
		}
		entry = settled
	}

	if touch {
		entry.Touch()
	}
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
		if sv, ok := value.(*StringValue); ok {
			return ValuePlan{Size: size, op: (*installString)(sv)}, nil
		}
		return ValuePlan{Size: size, Apply: func() Value { return value }}, nil
	})
}

// installString is the plan op installing a string made beforehand. A
// pointer converts to planOp without allocating, where the Apply closure
// was allocated for every SET.
type installString StringValue

func (o *installString) apply() Value { return (*StringValue)(o) }
func (*installString) release()       {}

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
	err = s.applyValuePlan(key, nil, nil, func(current Value) (ValuePlan, error) {
		if current != nil {
			// The empty plan records the access.
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
		return v.Len() == 0
	case *SortedSetValue:
		v.RLock()
		defer v.RUnlock()
		return v.Card() == 0
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
	if entry.IsExpired() && !s.passiveExpiry.Load() {
		// As in Redis, an expired key is expired first: DEL and UNLINK do
		// not count it, and its expiry is what is announced and recorded. A
		// replica (passive expiry) deletes what its master tells it to.
		if entry, exists = s.get(key, true, false); !exists {
			return false
		}
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

// parallelDeleteMin is the batch size from which DeleteBatch spreads the
// shards over several goroutines. Invalidating a large tag deletes tens of
// thousands of keys while every other command waits; the shards are
// independent, so the work divides across cores.
const parallelDeleteMin = 512

// deleteBatchParallel is DeleteBatch for a large batch: the keys are bucketed
// by shard, and each worker deletes the keys of a range of shards and drops
// them from the tag index. Everything here runs while every other command
// waits, so the bucketing is parallel too: each worker hashes a slice of
// the keys and counts them per shard, and after a prefix sum places its
// keys at its own offsets.
func (s *Store) deleteBatchParallel(keys []string, dropped *droppedTags) ([]string, int) {
	workers := min(runtime.GOMAXPROCS(0), 8)
	if per := len(keys) / (parallelDeleteMin / 2); workers > per {
		workers = per
	}
	workers = max(workers, 1)

	n := len(keys)
	idx := make([]uint8, n)
	counts := make([][NumShards]int, workers)
	chunk := (n + workers - 1) / workers
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		lo, hi := min(w*chunk, n), min((w+1)*chunk, n)
		wg.Add(1)
		go func(c *[NumShards]int) {
			defer wg.Done()
			for i := lo; i < hi; i++ {
				sh := s.shardIndex(keys[i])
				idx[i] = uint8(sh)
				c[sh]++
			}
		}(&counts[w])
	}
	wg.Wait()
	var start [NumShards + 1]int
	offsets := make([][NumShards]int, workers)
	pos := 0
	for sh := 0; sh < NumShards; sh++ {
		start[sh] = pos
		for w := 0; w < workers; w++ {
			offsets[w][sh] = pos
			pos += counts[w][sh]
		}
	}
	start[NumShards] = pos
	byShard := make([]string, n)
	for w := 0; w < workers; w++ {
		lo, hi := min(w*chunk, n), min((w+1)*chunk, n)
		wg.Add(1)
		go func(off *[NumShards]int) {
			defer wg.Done()
			for i := lo; i < hi; i++ {
				sh := idx[i]
				byShard[off[sh]] = keys[i]
				off[sh]++
			}
		}(&offsets[w])
	}
	wg.Wait()

	type result struct {
		gone    []string
		expired []string
		freed   int64
		_       [24]byte // keep workers' results on separate cache lines
	}
	results := make([]result, workers)
	for w := 0; w < workers; w++ {
		lo, hi := w*NumShards/workers, (w+1)*NumShards/workers
		wg.Add(1)
		go func(r *result) {
			defer wg.Done()
			r.gone = make([]string, 0, start[hi]-start[lo])
			// The keys to drop from each tag, so a tag's lock is taken once
			// per worker rather than once per key: the keys of a large tag
			// all share it. The tags the keys were invalidated through are
			// gone from the index already.
			var untag map[string][]string
			for sh := lo; sh < hi; sh++ {
				shardKeys := byShard[start[sh]:start[sh+1]]
				if len(shardKeys) == 0 {
					continue
				}
				shard := s.shards[sh]
				shard.mu.Lock()
				for _, key := range shardKeys {
					entry, exists := shard.data[key]
					if !exists {
						continue
					}
					r.freed += shard.deleteLocked(key, entry)
					if s.expiredNow(entry) {
						r.expired = append(r.expired, key)
					} else {
						r.gone = append(r.gone, key)
					}
					for _, tag := range entry.TagList() {
						if dropped.has(tag) {
							continue
						}
						if untag == nil {
							untag = map[string][]string{}
						}
						untag[tag] = append(untag[tag], key)
					}
				}
				shard.mu.Unlock()
			}
			for tag, tagKeys := range untag {
				s.tagIndex.RemoveTagsBatch(tagKeys, []string{tag})
			}
			s.TouchKeys(r.gone)
			s.TouchKeys(r.expired)
		}(&results[w])
	}
	wg.Wait()

	total, live, freed := 0, 0, int64(0)
	for i := range results {
		total += len(results[i].gone) + len(results[i].expired)
		live += len(results[i].gone)
		freed += results[i].freed
	}
	gone := make([]string, 0, total)
	for i := range results {
		gone = append(gone, results[i].gone...)
	}
	for i := range results {
		gone = append(gone, results[i].expired...)
	}
	s.trackMemory(-freed)
	return gone, live
}

// droppedTags are the tags an INVALIDATE removed from the index before
// deleting their keys: removing the keys from them again finds nothing.
type droppedTags struct {
	list []string
	set  map[string]struct{}
}

func newDroppedTags(tags []string) *droppedTags {
	if len(tags) == 0 {
		return nil
	}
	d := &droppedTags{list: tags}
	if len(tags) > 8 {
		d.set = make(map[string]struct{}, len(tags))
		for _, t := range tags {
			d.set[t] = struct{}{}
		}
	}
	return d
}

func (d *droppedTags) has(tag string) bool {
	if d == nil {
		return false
	}
	if d.set != nil {
		_, ok := d.set[tag]
		return ok
	}
	for _, t := range d.list {
		if t == tag {
			return true
		}
	}
	return false
}

// remaining is tags without the dropped ones; it allocates only when some
// tags remain besides dropped ones.
func (d *droppedTags) remaining(tags []string) []string {
	if d == nil {
		return tags
	}
	keep := 0
	for _, t := range tags {
		if !d.has(t) {
			keep++
		}
	}
	if keep == len(tags) {
		return tags
	}
	if keep == 0 {
		return nil
	}
	out := make([]string, 0, keep)
	for _, t := range tags {
		if !d.has(t) {
			out = append(out, t)
		}
	}
	return out
}

// shardKey is a key with its shard and whether it was deleted, or with the
// tags it had once deleted.
type shardKey struct {
	idx  uint32
	gone bool
	key  string
	tags []string
}

func (s *Store) DeleteBatch(keys []string) int {
	_, live := s.deleteBatchKeys(keys, nil)
	return live
}

// DeleteBatchKeys deletes keys and returns those that were stored, expired
// ones included.
func (s *Store) DeleteBatchKeys(keys []string) []string {
	gone, _ := s.deleteBatchKeys(keys, nil)
	return gone
}

// DeleteInvalidated is DeleteBatchKeys for INVALIDATE: dropped are the tags
// the keys were invalidated through, which the index no longer has, so the
// keys are not looked for in them again. The first live of the keys it
// returns were alive; the rest had expired without being removed yet, which
// INVALIDATE does not count, as DEL does not.
func (s *Store) DeleteInvalidated(keys, dropped []string) (gone []string, live int) {
	return s.deleteBatchKeys(keys, newDroppedTags(dropped))
}

// expiredNow reports whether entry has expired and the store removes such
// keys itself (a replica waits for its master).
func (s *Store) expiredNow(entry *Entry) bool {
	return entry.ExpiresAt.Load() != 0 && !s.passiveExpiry.Load() && entry.IsExpired()
}

func (s *Store) deleteBatchKeys(keys []string, dropped *droppedTags) ([]string, int) {
	if len(keys) == 0 {
		return nil, 0
	}
	if len(keys) >= parallelDeleteMin {
		return s.deleteBatchParallel(keys, dropped)
	}

	// Group the keys by shard with one sorted slice rather than a map of
	// slices: INVALIDATE deletes a tag's few keys this way on every call.
	var small, smallUntag [16]shardKey
	ops := small[:0]
	if len(keys) > len(small) {
		ops = make([]shardKey, 0, len(keys))
	}
	for _, key := range keys {
		ops = append(ops, shardKey{idx: s.shardIndex(key), key: key})
	}
	slices.SortFunc(ops, func(a, b shardKey) int { return cmp.Compare(a.idx, b.idx) })

	var gone, expired []string
	freed := int64(0)
	untag := smallUntag[:0]
	for i := 0; i < len(ops); {
		idx := ops[i].idx
		shard := s.shards[idx]
		shard.mu.Lock()
		for ; i < len(ops) && ops[i].idx == idx; i++ {
			key := ops[i].key
			entry, exists := shard.data[key]
			if !exists {
				continue
			}
			freed += shard.deleteLocked(key, entry)
			ops[i].gone = true
			if tags := dropped.remaining(entry.TagList()); len(tags) > 0 {
				untag = append(untag, shardKey{key: key, tags: tags})
			}
			if s.expiredNow(entry) {
				expired = append(expired, key)
			} else {
				gone = append(gone, key)
			}
		}
		shard.mu.Unlock()
	}
	// Like Store.Delete, bump the watched keys after deleting them, and take
	// the version lock only when some key is watched: it is global.
	if w := s.watch.Load(); w.watching.Load() != 0 {
		w.mu.Lock()
		for i := range ops {
			if ops[i].gone {
				w.touchLocked(ops[i].key)
			}
		}
		w.mu.Unlock()
	}

	// Mirror Store.Delete's index bookkeeping, outside the shard/version
	// locks to preserve their ordering with the tag-index locks.
	for _, u := range untag {
		s.tagIndex.RemoveKey(u.key, u.tags)
	}

	s.trackMemory(-freed)

	live := len(gone)
	return append(gone, expired...), live
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
	entry, exists := s.Peek(key)
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
	entry, exists := s.Peek(key)
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
	return true
}

func (s *Store) SetExpiresAt(key string, expiresAt int64) bool {
	if !s.shards[s.shardIndex(key)].updateExpiry(key, expiresAt) {
		return false
	}
	s.TouchKey(key)
	return true
}

// Persist removes the key's expiry. It reports true only when the key exists
// and had an expiry to remove, which is what PERSIST replies.
func (s *Store) Persist(key string) bool {
	if entry, ok := s.shards[s.shardIndex(key)].Get(key); ok && !entry.IsExpired() {
		entry.Touch()
	}
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

// Generation identifies the store's contents: it changes when they are
// replaced wholesale rather than key by key.
func (s *Store) Generation() uint64 { return s.generation.Load() }

func (s *Store) Flush() {
	defer s.generation.Add(1)
	keyCount := s.KeyCount()
	s.watch.Load().touchIf(func(k string) bool { return s.shards[s.shardIndex(k)].Exists(k) })
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

// AppendLiveKeys appends to dst every unexpired key that keep accepts,
// calling keep under each shard's read lock, and returns dst. keep must not
// take store locks.
func (s *Store) AppendLiveKeys(dst []string, keep func(key string, e *Entry) bool) []string {
	for i := 0; i < NumShards; i++ {
		sh := s.shards[i]
		sh.mu.RLock()
		for k, v := range sh.data {
			if !v.IsExpired() && keep(k, v) {
				dst = append(dst, k)
			}
		}
		sh.mu.RUnlock()
	}
	return dst
}

// ForEachLive calls fn for every unexpired entry, shard by shard. Each shard's
// entries are collected under its read lock and fn runs after the lock is
// released, so fn may take other locks. It stops at the first error.
func (s *Store) ForEachLive(fn func(key string, e *Entry) error) error {
	var batch []keyEntry
	for i := 0; i < NumShards; i++ {
		var err error
		if batch, err = s.forEachLiveInShard(i, batch[:0], fn); err != nil {
			return err
		}
	}
	return nil
}

// ForEachLiveInShard is ForEachLive over shard i (0 <= i < NumShards) alone.
// Different shards may be walked concurrently.
func (s *Store) ForEachLiveInShard(i int, fn func(key string, e *Entry) error) error {
	_, err := s.forEachLiveInShard(i, nil, fn)
	return err
}

type keyEntry struct {
	key   string
	entry *Entry
}

func (s *Store) forEachLiveInShard(i int, batch []keyEntry, fn func(key string, e *Entry) error) ([]keyEntry, error) {
	sh := s.shards[i]
	sh.mu.RLock()
	if batch == nil {
		batch = make([]keyEntry, 0, len(sh.data))
	}
	for k, v := range sh.data {
		batch = append(batch, keyEntry{k, v})
	}
	sh.mu.RUnlock()
	for _, p := range batch {
		if p.entry.IsExpired() {
			continue
		}
		if err := fn(p.key, p.entry); err != nil {
			return batch, err
		}
	}
	return batch, nil
}
