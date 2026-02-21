package store

import (
	"container/heap"
	"sync"
	"time"
)

type PriorityQueue struct {
	items []*PriorityItem
	mu    sync.RWMutex
}

type PriorityItem struct {
	Value    string
	Priority int64
	Index    int
}

func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{
		items: make([]*PriorityItem, 0),
	}
	heap.Init(pq)
	return pq
}

func (pq *PriorityQueue) Len() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return len(pq.items)
}

func (pq *PriorityQueue) Less(i, j int) bool {
	return pq.items[i].Priority < pq.items[j].Priority
}

func (pq *PriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].Index = i
	pq.items[j].Index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(pq.items)
	item := x.(*PriorityItem)
	item.Index = n
	pq.items = append(pq.items, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.Index = -1
	pq.items = old[0 : n-1]
	return item
}

func (pq *PriorityQueue) PushItem(value string, priority int64) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	heap.Push(pq, &PriorityItem{
		Value:    value,
		Priority: priority,
	})
}

func (pq *PriorityQueue) PopItem() (string, int64, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.items) == 0 {
		return "", 0, false
	}

	item := heap.Pop(pq).(*PriorityItem)
	return item.Value, item.Priority, true
}

func (pq *PriorityQueue) Peek() (string, int64, bool) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	if len(pq.items) == 0 {
		return "", 0, false
	}

	return pq.items[0].Value, pq.items[0].Priority, true
}

func (pq *PriorityQueue) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.items = make([]*PriorityItem, 0)
}

func (pq *PriorityQueue) GetAll() []PriorityItem {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	result := make([]PriorityItem, len(pq.items))
	for i, item := range pq.items {
		result[i] = *item
	}
	return result
}

type LRUCache struct {
	Items    map[string]*LRUNode
	Head     *LRUNode
	Tail     *LRUNode
	Capacity int
	Size     int
	mu       sync.RWMutex
}

type LRUNode struct {
	Key   string
	Value string
	Prev  *LRUNode
	Next  *LRUNode
}

func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		Items:    make(map[string]*LRUNode),
		Capacity: capacity,
	}
}

func (lru *LRUCache) Get(key string) (string, bool) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if node, exists := lru.Items[key]; exists {
		lru.moveToFront(node)
		return node.Value, true
	}
	return "", false
}

func (lru *LRUCache) Set(key, value string) bool {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if node, exists := lru.Items[key]; exists {
		node.Value = value
		lru.moveToFront(node)
		return false
	}

	node := &LRUNode{Key: key, Value: value}
	lru.Items[key] = node
	lru.addToFront(node)
	lru.Size++

	if lru.Size > lru.Capacity {
		lru.removeLRU()
	}
	return true
}

func (lru *LRUCache) Delete(key string) bool {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if node, exists := lru.Items[key]; exists {
		lru.removeNode(node)
		delete(lru.Items, key)
		lru.Size--
		return true
	}
	return false
}

func (lru *LRUCache) moveToFront(node *LRUNode) {
	lru.removeNode(node)
	lru.addToFront(node)
}

func (lru *LRUCache) addToFront(node *LRUNode) {
	node.Prev = nil
	node.Next = lru.Head

	if lru.Head != nil {
		lru.Head.Prev = node
	}
	lru.Head = node

	if lru.Tail == nil {
		lru.Tail = node
	}
}

func (lru *LRUCache) removeNode(node *LRUNode) {
	if node.Prev != nil {
		node.Prev.Next = node.Next
	} else {
		lru.Head = node.Next
	}

	if node.Next != nil {
		node.Next.Prev = node.Prev
	} else {
		lru.Tail = node.Prev
	}
}

func (lru *LRUCache) removeLRU() {
	if lru.Tail == nil {
		return
	}

	delete(lru.Items, lru.Tail.Key)
	lru.removeNode(lru.Tail)
	lru.Size--
}

func (lru *LRUCache) Clear() {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	lru.Items = make(map[string]*LRUNode)
	lru.Head = nil
	lru.Tail = nil
	lru.Size = 0
}

func (lru *LRUCache) Keys() []string {
	lru.mu.RLock()
	defer lru.mu.RUnlock()

	keys := make([]string, 0, len(lru.Items))
	current := lru.Head
	for current != nil {
		keys = append(keys, current.Key)
		current = current.Next
	}
	return keys
}

func (lru *LRUCache) Stats() map[string]interface{} {
	lru.mu.RLock()
	defer lru.mu.RUnlock()

	return map[string]interface{}{
		"size":     lru.Size,
		"capacity": lru.Capacity,
		"usage":    float64(lru.Size) / float64(lru.Capacity) * 100,
	}
}

type TokenBucket struct {
	Tokens     float64
	MaxTokens  float64
	RefillRate float64
	LastRefill int64
	mu         sync.Mutex
}

func NewTokenBucket(maxTokens, refillRate float64) *TokenBucket {
	return &TokenBucket{
		Tokens:     maxTokens,
		MaxTokens:  maxTokens,
		RefillRate: refillRate,
		LastRefill: time.Now().UnixNano(),
	}
}

func (tb *TokenBucket) Consume(tokens float64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.Tokens >= tokens {
		tb.Tokens -= tokens
		return true
	}
	return false
}

func (tb *TokenBucket) refill() {
	now := time.Now().UnixNano()
	elapsed := float64(now-tb.LastRefill) / 1e9
	tb.Tokens += elapsed * tb.RefillRate

	if tb.Tokens > tb.MaxTokens {
		tb.Tokens = tb.MaxTokens
	}

	tb.LastRefill = now
}

func (tb *TokenBucket) Available() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return tb.Tokens
}

func (tb *TokenBucket) Reset() {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.Tokens = tb.MaxTokens
	tb.LastRefill = time.Now().UnixNano()
}

type LeakyBucket struct {
	Capacity  int64
	Remaining int64
	LeakRate  int64
	LastLeak  int64
	mu        sync.Mutex
}

func NewLeakyBucket(capacity, leakRate int64) *LeakyBucket {
	return &LeakyBucket{
		Capacity:  capacity,
		Remaining: capacity,
		LeakRate:  leakRate,
		LastLeak:  time.Now().UnixNano(),
	}
}

func (lb *LeakyBucket) Add(amount int64) bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.leak()

	if lb.Remaining >= amount {
		lb.Remaining -= amount
		return true
	}
	return false
}

func (lb *LeakyBucket) leak() {
	now := time.Now().UnixNano()
	elapsed := (now - lb.LastLeak) / 1e9
	leaked := elapsed * lb.LeakRate

	lb.Remaining += leaked
	if lb.Remaining > lb.Capacity {
		lb.Remaining = lb.Capacity
	}

	lb.LastLeak = now
}

func (lb *LeakyBucket) Available() int64 {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.leak()
	return lb.Remaining
}

type SlidingWindowCounter struct {
	Windows    map[int64]int64
	WindowSize int64
	Limit      int64
	mu         sync.RWMutex
}

func NewSlidingWindowCounter(windowSizeMs, limit int64) *SlidingWindowCounter {
	return &SlidingWindowCounter{
		Windows:    make(map[int64]int64),
		WindowSize: windowSizeMs,
		Limit:      limit,
	}
}

func (swc *SlidingWindowCounter) Increment(key string) (int64, bool) {
	swc.mu.Lock()
	defer swc.mu.Unlock()

	now := time.Now().UnixMilli()
	window := now / swc.WindowSize

	count := swc.countWindows(window)
	if count >= swc.Limit {
		return count, false
	}

	swc.Windows[window]++
	swc.cleanup(window)

	return swc.Windows[window], true
}

func (swc *SlidingWindowCounter) countWindows(current int64) int64 {
	var total int64
	for w, count := range swc.Windows {
		if w >= current-1 {
			total += count
		}
	}
	return total
}

func (swc *SlidingWindowCounter) cleanup(current int64) {
	for w := range swc.Windows {
		if w < current-1 {
			delete(swc.Windows, w)
		}
	}
}

func (swc *SlidingWindowCounter) Count() int64 {
	swc.mu.RLock()
	defer swc.mu.RUnlock()

	now := time.Now().UnixMilli()
	window := now / swc.WindowSize
	return swc.countWindows(window)
}

func (swc *SlidingWindowCounter) Reset() {
	swc.mu.Lock()
	defer swc.mu.Unlock()
	swc.Windows = make(map[int64]int64)
}

var (
	priorityQueues          = make(map[string]*PriorityQueue)
	priorityQueuesMu        sync.RWMutex
	lruCaches               = make(map[string]*LRUCache)
	lruCachesMu             sync.RWMutex
	tokenBuckets            = make(map[string]*TokenBucket)
	tokenBucketsMu          sync.RWMutex
	leakyBuckets            = make(map[string]*LeakyBucket)
	leakyBucketsMu          sync.RWMutex
	slidingWindowCounters   = make(map[string]*SlidingWindowCounter)
	slidingWindowCountersMu sync.RWMutex
)
