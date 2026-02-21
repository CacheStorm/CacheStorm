package store

import (
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

	if elapsed >= entry.Interval {
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

func (dl *DistributedLock) TryLock(key, holder, token string, ttl time.Duration) bool {
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

	return false
}

func (dl *DistributedLock) Lock(key, holder, token string, ttl time.Duration, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for {
		if dl.TryLock(key, holder, token, ttl) {
			return true
		}

		if time.Now().After(deadline) {
			return false
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func (dl *DistributedLock) Unlock(key, holder, token string) bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	lock, exists := dl.Locks[key]
	if !exists {
		return false
	}

	if lock.Holder == holder && lock.Token == token {
		delete(dl.Locks, key)
		return true
	}

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
	numStr := ""
	neg := n < 0
	if neg {
		n = -n
	}

	for n > 0 {
		numStr = string(rune('0'+n%10)) + numStr
		n /= 10
	}

	if numStr == "" {
		numStr = "0"
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
		time.Sleep(100 * time.Microsecond)
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
