package store

import (
	"runtime"
	"strconv"
	"sync"
	"time"
)

type RateLimiter struct {
	Requests map[string]*RateLimitEntry
	mu       sync.RWMutex
}

type RateLimitEntry struct {
	Tokens     int
	MaxTokens  int
	RefillRate int
	LastRefill time.Time
	Interval   time.Duration
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		Requests: make(map[string]*RateLimitEntry),
	}
}

func (rl *RateLimiter) Create(key string, maxTokens, refillRate int, interval time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.Requests[key] = &RateLimitEntry{
		Tokens:     maxTokens,
		MaxTokens:  maxTokens,
		RefillRate: refillRate,
		LastRefill: time.Now(),
		Interval:   interval,
	}
}

func (rl *RateLimiter) Allow(key string, tokens int) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.Requests[key]
	if !exists {
		return false, 0, time.Time{}
	}

	now := time.Now()
	elapsed := now.Sub(entry.LastRefill)

	// The refill arithmetic divides by the interval, so a limiter created
	// with a non-positive interval cannot be refilled at all. Skip the refill
	// rather than divide by zero: the command layer refuses such a
	// configuration up front, but the store must not panic if one is ever
	// installed here directly.
	if entry.Interval > 0 && elapsed >= entry.Interval {
		tokensToAdd := int(elapsed/entry.Interval) * entry.RefillRate
		entry.Tokens += tokensToAdd
		if entry.Tokens > entry.MaxTokens {
			entry.Tokens = entry.MaxTokens
		}
		entry.LastRefill = now
	}

	if entry.Tokens >= tokens {
		entry.Tokens -= tokens
		return true, entry.Tokens, entry.LastRefill.Add(entry.Interval)
	}

	return false, entry.Tokens, entry.LastRefill.Add(entry.Interval)
}

func (rl *RateLimiter) Get(key string) (int, int, int, time.Duration, bool) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	entry, exists := rl.Requests[key]
	if !exists {
		return 0, 0, 0, 0, false
	}

	return entry.Tokens, entry.MaxTokens, entry.RefillRate, entry.Interval, true
}

func (rl *RateLimiter) Delete(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if _, exists := rl.Requests[key]; !exists {
		return false
	}

	delete(rl.Requests, key)
	return true
}

func (rl *RateLimiter) Reset(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.Requests[key]
	if !exists {
		return false
	}

	entry.Tokens = entry.MaxTokens
	entry.LastRefill = time.Now()
	return true
}

type DistributedLock struct {
	Locks     map[string]*LockEntry
	mu        sync.RWMutex
	waitQueue map[string][]chan struct{}
}

type LockEntry struct {
	Holder    string
	Token     string
	ExpiresAt time.Time
	Renewals  int
}

func NewDistributedLock() *DistributedLock {
	return &DistributedLock{
		Locks:     make(map[string]*LockEntry),
		waitQueue: make(map[string][]chan struct{}),
	}
}

// acquire takes the lock for holder when it is free, or renews it when the
// same holder/token already holds it. It reports whether the lock was
// acquired.
//
// When the lock is held by someone else AND ch is non-nil, ch is registered as
// a waiter for key before returning false, so that a later release can hand
// the lock over.
//
// The acquisition attempt and that registration happen under a single hold of
// dl.mu. That is what makes the handoff safe: Unlock collects the waiters to
// signal under the same lock, so a caller that has just lost the race is
// either already back on the queue — and will be signalled — or has not lost
// the race at all. Splitting the two into separate critical sections left a
// window in which a release could slip past a waiter that had not registered
// yet, stranding it until its timeout even though the lock was free.
func (dl *DistributedLock) acquire(key, holder, token string, ttl time.Duration, ch chan struct{}) bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	lock, exists := dl.Locks[key]
	if !exists || lock.ExpiresAt.Before(time.Now()) {
		dl.Locks[key] = &LockEntry{
			Holder:    holder,
			Token:     token,
			ExpiresAt: time.Now().Add(ttl),
			Renewals:  0,
		}
		return true
	}

	if lock.Holder == holder && lock.Token == token {
		lock.ExpiresAt = time.Now().Add(ttl)
		lock.Renewals++
		return true
	}

	if ch != nil {
		dl.addWaiterLocked(key, ch)
	}
	return false
}

// addWaiterLocked registers ch as a waiter for key. Registering the same
// channel twice is a no-op, so a waiter that lost the race and re-registers
// does not appear twice in the queue.
//
// The caller must hold dl.mu.
func (dl *DistributedLock) addWaiterLocked(key string, ch chan struct{}) {
	waiters := dl.waitQueue[key]
	for _, w := range waiters {
		if w == ch {
			return
		}
	}
	dl.waitQueue[key] = append(waiters, ch)
}

// removeWaiter unregisters ch from key's wait queue. The caller must hold
// dl.mu.
func (dl *DistributedLock) removeWaiter(key string, ch chan struct{}) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	waiters := dl.waitQueue[key]
	for i, w := range waiters {
		if w == ch {
			waiters = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
	if len(waiters) == 0 {
		delete(dl.waitQueue, key)
		return
	}
	dl.waitQueue[key] = waiters
}

func (dl *DistributedLock) TryLock(key, holder, token string, ttl time.Duration) bool {
	return dl.acquire(key, holder, token, ttl, nil)
}

func (dl *DistributedLock) Lock(key, holder, token string, ttl time.Duration, timeout time.Duration) bool {
	if timeout == 0 {
		// No waiting allowed: a single immediate attempt.
		return dl.TryLock(key, holder, token, ttl)
	}

	// The channel this caller waits on. It is registered as a waiter for key
	// by the first acquire below.
	ch := make(chan struct{}, 1)

	// Take the lock if it is free. If it is held, the same call registers ch
	// as a waiter, so a release happening right afterwards cannot slip past
	// this caller unnoticed.
	if dl.acquire(key, holder, token, ttl, ch) {
		return true
	}

	// Leave the queue once we stop waiting, so a later release does not
	// signal a channel nobody reads.
	defer dl.removeWaiter(key, ch)

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ch:
			// A release woke us: the lock may be free now. Take it. If
			// another waiter won the race, acquire puts us back on the
			// queue, so the NEXT release reaches us instead of stranding
			// us until the timeout expires.
			if dl.acquire(key, holder, token, ttl, ch) {
				return true
			}
		case <-timer.C:
			return false
		}
	}
}

func (dl *DistributedLock) Unlock(key, holder, token string) bool {
	dl.mu.Lock()

	lock, exists := dl.Locks[key]
	if !exists {
		dl.mu.Unlock()
		return false
	}

	if lock.Holder == holder && lock.Token == token {
		delete(dl.Locks, key)
		// Wake the waiters so they can re-check whether the lock is now
		// free.
		//
		// The wait queue is deliberately NOT cleared here. A waiter stays
		// registered from the moment it starts waiting until it acquires
		// the lock or gives up, so the queue is the set of callers that
		// must still be told about a release. Clearing it orphaned every
		// waiter that lost the race for the released lock: it was removed
		// from the queue, so no later release ever signalled it, and it
		// reported failure only when its timeout expired — although the
		// lock had been free for that whole time.
		waiters := dl.waitQueue[key]
		dl.mu.Unlock()

		for _, ch := range waiters {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
		return true
	}

	dl.mu.Unlock()
	return false
}

func (dl *DistributedLock) Renew(key, holder, token string, ttl time.Duration) bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	lock, exists := dl.Locks[key]
	if !exists {
		return false
	}

	if lock.Holder == holder && lock.Token == token {
		lock.ExpiresAt = time.Now().Add(ttl)
		lock.Renewals++
		return true
	}

	return false
}

func (dl *DistributedLock) GetHolder(key string) (string, time.Time, bool) {
	dl.mu.RLock()
	defer dl.mu.RUnlock()

	lock, exists := dl.Locks[key]
	if !exists {
		return "", time.Time{}, false
	}

	return lock.Holder, lock.ExpiresAt, true
}

func (dl *DistributedLock) IsLocked(key string) bool {
	dl.mu.RLock()
	defer dl.mu.RUnlock()

	lock, exists := dl.Locks[key]
	if !exists {
		return false
	}

	return lock.ExpiresAt.After(time.Now())
}

type IDGenerator struct {
	Sequences map[string]*Sequence
	mu        sync.RWMutex
}

type Sequence struct {
	Current   int64
	Increment int64
	Prefix    string
	Suffix    string
	Padding   int
}

func NewIDGenerator() *IDGenerator {
	return &IDGenerator{
		Sequences: make(map[string]*Sequence),
	}
}

func (idg *IDGenerator) Create(name string, start, increment int64, prefix, suffix string, padding int) {
	idg.mu.Lock()
	defer idg.mu.Unlock()

	idg.Sequences[name] = &Sequence{
		Current:   start - increment,
		Increment: increment,
		Prefix:    prefix,
		Suffix:    suffix,
		Padding:   padding,
	}
}

func (idg *IDGenerator) Next(name string) (string, int64, bool) {
	idg.mu.Lock()
	defer idg.mu.Unlock()

	seq, exists := idg.Sequences[name]
	if !exists {
		return "", 0, false
	}

	seq.Current += seq.Increment
	id := seq.formatID(seq.Current)
	return id, seq.Current, true
}

func (idg *IDGenerator) NextN(name string, count int) ([]string, int64, bool) {
	idg.mu.Lock()
	defer idg.mu.Unlock()

	seq, exists := idg.Sequences[name]
	if !exists {
		return nil, 0, false
	}

	ids := make([]string, count)
	for i := 0; i < count; i++ {
		seq.Current += seq.Increment
		ids[i] = seq.formatID(seq.Current)
	}

	return ids, seq.Current, true
}

func (idg *IDGenerator) Current(name string) (string, int64, bool) {
	idg.mu.RLock()
	defer idg.mu.RUnlock()

	seq, exists := idg.Sequences[name]
	if !exists {
		return "", 0, false
	}

	return seq.formatID(seq.Current), seq.Current, true
}

func (idg *IDGenerator) Set(name string, value int64) bool {
	idg.mu.Lock()
	defer idg.mu.Unlock()

	seq, exists := idg.Sequences[name]
	if !exists {
		return false
	}

	seq.Current = value
	return true
}

func (idg *IDGenerator) Delete(name string) bool {
	idg.mu.Lock()
	defer idg.mu.Unlock()

	if _, exists := idg.Sequences[name]; !exists {
		return false
	}

	delete(idg.Sequences, name)
	return true
}

func (s *Sequence) formatID(n int64) string {
	numStr := formatNumber(n, s.Padding)
	return s.Prefix + numStr + s.Suffix
}

func formatNumber(n int64, padding int) string {
	numStr := strconv.FormatInt(n, 10)
	neg := n < 0
	if neg {
		numStr = numStr[1:]
	}

	for len(numStr) < padding {
		numStr = "0" + numStr
	}

	if neg {
		numStr = "-" + numStr
	}

	return numStr
}

type SnowflakeIDGenerator struct {
	sequence      int64
	lastTimestamp int64
	nodeID        int64
	mu            sync.Mutex
}

const (
	epoch          = int64(1704067200000)
	nodeBits       = uint(10)
	sequenceBits   = uint(12)
	nodeMax        = int64(-1 ^ (-1 << nodeBits))
	sequenceMask   = int64(-1 ^ (-1 << sequenceBits))
	nodeShift      = sequenceBits
	timestampShift = sequenceBits + nodeBits
)

func NewSnowflakeIDGenerator(nodeID int64) *SnowflakeIDGenerator {
	if nodeID < 0 || nodeID > nodeMax {
		nodeID = 0
	}
	return &SnowflakeIDGenerator{
		nodeID: nodeID,
	}
}

// snowflakeGenerators holds one generator per node ID. The sequence counter
// that keeps IDs unique within a single millisecond lives in the generator,
// so a generator must be REUSED for the IDs it mints to be distinct.
var (
	snowflakeMu         sync.Mutex
	snowflakeGenerators = make(map[int64]*SnowflakeIDGenerator)
)

// GetSnowflakeIDGenerator returns the process-wide generator for nodeID,
// creating it on first use.
//
// The generator carries the intra-millisecond sequence counter, so callers
// MUST reuse the returned generator instead of constructing a new one per ID.
// A fresh generator starts its sequence at zero, so every ID it mints within
// one millisecond carries the same sequence and the same millisecond — two
// such IDs are byte-identical, which defeats the whole point of a snowflake
// ID. That is what happened in the SNOWFLAKE.NEXT command, which built a new
// generator on every call.
func GetSnowflakeIDGenerator(nodeID int64) *SnowflakeIDGenerator {
	snowflakeMu.Lock()
	defer snowflakeMu.Unlock()

	// Normalize the key the same way NewSnowflakeIDGenerator normalizes the
	// node: an out-of-range node ID becomes 0. Two different invalid node IDs
	// would otherwise get separate generators that share the same effective
	// node bits, each with its own sequence counter, and would mint duplicate
	// IDs for the same millisecond.
	if nodeID < 0 || nodeID > nodeMax {
		nodeID = 0
	}

	if gen, exists := snowflakeGenerators[nodeID]; exists {
		return gen
	}
	gen := NewSnowflakeIDGenerator(nodeID)
	snowflakeGenerators[nodeID] = gen
	return gen
}

func (s *SnowflakeIDGenerator) Next() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := currentTimestamp()

	if now == s.lastTimestamp {
		s.sequence = (s.sequence + 1) & sequenceMask
		if s.sequence == 0 {
			now = waitNextMillis(s.lastTimestamp)
		}
	} else {
		s.sequence = 0
	}

	s.lastTimestamp = now

	return ((now - epoch) << timestampShift) | (s.nodeID << nodeShift) | s.sequence
}

func (s *SnowflakeIDGenerator) Parse(id int64) map[string]int64 {
	timestamp := (id >> timestampShift) + epoch
	nodeID := (id >> nodeShift) & nodeMax
	sequence := id & sequenceMask

	return map[string]int64{
		"timestamp": timestamp,
		"node_id":   nodeID,
		"sequence":  sequence,
	}
}

func currentTimestamp() int64 {
	return time.Now().UnixMilli()
}

func waitNextMillis(lastTimestamp int64) int64 {
	now := currentTimestamp()
	for now <= lastTimestamp {
		runtime.Gosched() // Yield CPU instead of sleeping — sub-millisecond wait
		now = currentTimestamp()
	}
	return now
}

var (
	GlobalRateLimiter     = NewRateLimiter()
	GlobalDistributedLock = NewDistributedLock()
	GlobalIDGenerator     = NewIDGenerator()
)

func init() {
	GlobalIDGenerator.Create("default", 1, 1, "", "", 0)
}
