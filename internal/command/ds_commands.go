package command

import (
	"fmt"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterDataStructuresCommands(router *Router) {
	router.Register(&CommandDef{Name: "PQ.CREATE", Handler: cmdPQCREATE})
	router.Register(&CommandDef{Name: "PQ.PUSH", Handler: cmdPQPUSH})
	router.Register(&CommandDef{Name: "PQ.POP", Handler: cmdPQPOP})
	router.Register(&CommandDef{Name: "PQ.PEEK", Handler: cmdPQPEEK})
	router.Register(&CommandDef{Name: "PQ.LEN", Handler: cmdPQLEN})
	router.Register(&CommandDef{Name: "PQ.CLEAR", Handler: cmdPQCLEAR})
	router.Register(&CommandDef{Name: "PQ.GETALL", Handler: cmdPQGETALL})

	router.Register(&CommandDef{Name: "LRU.CREATE", Handler: cmdLRUCREATE})
	router.Register(&CommandDef{Name: "LRU.GET", Handler: cmdLRUGET})
	router.Register(&CommandDef{Name: "LRU.SET", Handler: cmdLRUSET})
	router.Register(&CommandDef{Name: "LRU.DEL", Handler: cmdLRUDEL})
	router.Register(&CommandDef{Name: "LRU.CLEAR", Handler: cmdLRUCLEAR})
	router.Register(&CommandDef{Name: "LRU.KEYS", Handler: cmdLRUKEYS})
	router.Register(&CommandDef{Name: "LRU.STATS", Handler: cmdLRUSTATS})

	router.Register(&CommandDef{Name: "TOKENBUCKET.CREATE", Handler: cmdTOKENBUCKETCREATE})
	router.Register(&CommandDef{Name: "TOKENBUCKET.CONSUME", Handler: cmdTOKENBUCKETCONSUME})
	router.Register(&CommandDef{Name: "TOKENBUCKET.AVAILABLE", Handler: cmdTOKENBUCKETAVAILABLE})
	router.Register(&CommandDef{Name: "TOKENBUCKET.RESET", Handler: cmdTOKENBUCKETRESET})
	router.Register(&CommandDef{Name: "TOKENBUCKET.DELETE", Handler: cmdTOKENBUCKETDELETE})

	router.Register(&CommandDef{Name: "LEAKYBUCKET.CREATE", Handler: cmdLEAKYBUCKETCREATE})
	router.Register(&CommandDef{Name: "LEAKYBUCKET.ADD", Handler: cmdLEAKYBUCKETADD})
	router.Register(&CommandDef{Name: "LEAKYBUCKET.AVAILABLE", Handler: cmdLEAKYBUCKETAVAILABLE})
	router.Register(&CommandDef{Name: "LEAKYBUCKET.DELETE", Handler: cmdLEAKYBUCKETDELETE})

	router.Register(&CommandDef{Name: "SLIDINGWINDOW.CREATE", Handler: cmdSLIDINGWINDOWCREATE})
	router.Register(&CommandDef{Name: "SLIDINGWINDOW.INCR", Handler: cmdSLIDINGWINDOWINCR})
	router.Register(&CommandDef{Name: "SLIDINGWINDOW.COUNT", Handler: cmdSLIDINGWINDOWCOUNT})
	router.Register(&CommandDef{Name: "SLIDINGWINDOW.RESET", Handler: cmdSLIDINGWINDOWRESET})
	router.Register(&CommandDef{Name: "SLIDINGWINDOW.DELETE", Handler: cmdSLIDINGWINDOWDELETE})

	router.Register(&CommandDef{Name: "DEBOUNCE.SET", Handler: cmdDEBOUNCESET})
	router.Register(&CommandDef{Name: "DEBOUNCE.GET", Handler: cmdDEBOUNCEGET})
	router.Register(&CommandDef{Name: "DEBOUNCE.CALL", Handler: cmdDEBOUNCECALL})
	router.Register(&CommandDef{Name: "DEBOUNCE.DELETE", Handler: cmdDEBOUNCEDELETE})

	router.Register(&CommandDef{Name: "THROTTLE.SET", Handler: cmdTHROTTLESET})
	router.Register(&CommandDef{Name: "THROTTLE.CALL", Handler: cmdTHROTTLECALL})
	router.Register(&CommandDef{Name: "THROTTLE.RESET", Handler: cmdTHROTTLERESET})
	router.Register(&CommandDef{Name: "THROTTLE.DELETE", Handler: cmdTHROTTLEDELETE})
}

var (
	priorityQueues          = make(map[string]*store.PriorityQueue)
	priorityQueuesMu        sync.RWMutex
	lruCaches               = make(map[string]*store.LRUCache)
	lruCachesMu             sync.RWMutex
	tokenBuckets            = make(map[string]*store.TokenBucket)
	tokenBucketsMu          sync.RWMutex
	leakyBuckets            = make(map[string]*store.LeakyBucket)
	leakyBucketsMu          sync.RWMutex
	slidingWindowCounters   = make(map[string]*store.SlidingWindowCounter)
	slidingWindowCountersMu sync.RWMutex
	debounceTimers          = make(map[string]int64)
	debounceValues          = make(map[string]string)
	debounceDelay           = make(map[string]int64)
	debounceMu              sync.RWMutex
	throttleLastCall        = make(map[string]int64)
	throttleInterval        = make(map[string]int64)
	throttleMu              sync.RWMutex
)

func cmdPQCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	priorityQueuesMu.Lock()
	priorityQueues[name] = store.NewPriorityQueue()
	priorityQueuesMu.Unlock()

	return ctx.WriteOK()
}

func cmdPQPUSH(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := ctx.ArgString(1)
	priority := parseInt64(ctx.ArgString(2))

	priorityQueuesMu.RLock()
	pq, exists := priorityQueues[name]
	priorityQueuesMu.RUnlock()

	if !exists {
		priorityQueuesMu.Lock()
		priorityQueues[name] = store.NewPriorityQueue()
		pq = priorityQueues[name]
		priorityQueuesMu.Unlock()
	}

	pq.PushItem(value, priority)

	return ctx.WriteInteger(int64(pq.Len()))
}

func cmdPQPOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	priorityQueuesMu.RLock()
	pq, exists := priorityQueues[name]
	priorityQueuesMu.RUnlock()

	if !exists {
		return ctx.WriteNull()
	}

	value, priority, ok := pq.PopItem()
	if !ok {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(value),
		resp.IntegerValue(priority),
	})
}

func cmdPQPEEK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	priorityQueuesMu.RLock()
	pq, exists := priorityQueues[name]
	priorityQueuesMu.RUnlock()

	if !exists {
		return ctx.WriteNull()
	}

	value, priority, ok := pq.Peek()
	if !ok {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(value),
		resp.IntegerValue(priority),
	})
}

func cmdPQLEN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	priorityQueuesMu.RLock()
	pq, exists := priorityQueues[name]
	priorityQueuesMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	return ctx.WriteInteger(int64(pq.Len()))
}

func cmdPQCLEAR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	priorityQueuesMu.RLock()
	pq, exists := priorityQueues[name]
	priorityQueuesMu.RUnlock()

	if !exists {
		return ctx.WriteOK()
	}

	pq.Clear()
	return ctx.WriteOK()
}

func cmdPQGETALL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	priorityQueuesMu.RLock()
	pq, exists := priorityQueues[name]
	priorityQueuesMu.RUnlock()

	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}

	items := pq.GetAll()

	results := make([]*resp.Value, 0)
	for _, item := range items {
		results = append(results,
			resp.BulkString(item.Value),
			resp.IntegerValue(item.Priority),
		)
	}

	return ctx.WriteArray(results)
}

func cmdLRUCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	capacity := int(parseInt64(ctx.ArgString(1)))

	lruCachesMu.Lock()
	lruCaches[name] = store.NewLRUCache(capacity)
	lruCachesMu.Unlock()

	return ctx.WriteOK()
}

func cmdLRUGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	key := ctx.ArgString(1)

	lruCachesMu.RLock()
	lru, exists := lruCaches[name]
	lruCachesMu.RUnlock()

	if !exists {
		return ctx.WriteNull()
	}

	value, ok := lru.Get(key)
	if !ok {
		return ctx.WriteNull()
	}

	return ctx.WriteBulkString(value)
}

func cmdLRUSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)

	lruCachesMu.RLock()
	lru, exists := lruCaches[name]
	lruCachesMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR LRU cache not found"))
	}

	evicted := lru.Set(key, value)

	if evicted {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdLRUDEL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	key := ctx.ArgString(1)

	lruCachesMu.RLock()
	lru, exists := lruCaches[name]
	lruCachesMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	if lru.Delete(key) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdLRUCLEAR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	lruCachesMu.RLock()
	lru, exists := lruCaches[name]
	lruCachesMu.RUnlock()

	if !exists {
		return ctx.WriteOK()
	}

	lru.Clear()
	return ctx.WriteOK()
}

func cmdLRUKEYS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	lruCachesMu.RLock()
	lru, exists := lruCaches[name]
	lruCachesMu.RUnlock()

	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}

	keys := lru.Keys()

	results := make([]*resp.Value, len(keys))
	for i, k := range keys {
		results[i] = resp.BulkString(k)
	}

	return ctx.WriteArray(results)
}

func cmdLRUSTATS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	lruCachesMu.RLock()
	lru, exists := lruCaches[name]
	lruCachesMu.RUnlock()

	if !exists {
		return ctx.WriteNull()
	}

	stats := lru.Stats()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("size"),
		resp.IntegerValue(int64(stats["size"].(int))),
		resp.BulkString("capacity"),
		resp.IntegerValue(int64(stats["capacity"].(int))),
		resp.BulkString("usage"),
		resp.BulkString(fmt.Sprintf("%.2f", stats["usage"])),
	})
}

func cmdTOKENBUCKETCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	maxTokens := parseFloatExt(ctx.Arg(1))
	refillRate := parseFloatExt(ctx.Arg(2))

	tokenBucketsMu.Lock()
	tokenBuckets[name] = store.NewTokenBucket(maxTokens, refillRate)
	tokenBucketsMu.Unlock()

	return ctx.WriteOK()
}

func cmdTOKENBUCKETCONSUME(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	tokens := parseFloatExt(ctx.Arg(1))

	tokenBucketsMu.RLock()
	tb, exists := tokenBuckets[name]
	tokenBucketsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR token bucket not found"))
	}

	if tb.Consume(tokens) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdTOKENBUCKETAVAILABLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	tokenBucketsMu.RLock()
	tb, exists := tokenBuckets[name]
	tokenBucketsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR token bucket not found"))
	}

	available := tb.Available()
	return ctx.WriteBulkString(fmt.Sprintf("%.2f", available))
}

func cmdTOKENBUCKETRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	tokenBucketsMu.RLock()
	tb, exists := tokenBuckets[name]
	tokenBucketsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR token bucket not found"))
	}

	tb.Reset()
	return ctx.WriteOK()
}

func cmdTOKENBUCKETDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	tokenBucketsMu.Lock()
	defer tokenBucketsMu.Unlock()

	if _, exists := tokenBuckets[name]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(tokenBuckets, name)
	return ctx.WriteInteger(1)
}

func cmdLEAKYBUCKETCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	capacity := parseInt64(ctx.ArgString(1))
	leakRate := parseInt64(ctx.ArgString(2))

	leakyBucketsMu.Lock()
	leakyBuckets[name] = store.NewLeakyBucket(capacity, leakRate)
	leakyBucketsMu.Unlock()

	return ctx.WriteOK()
}

func cmdLEAKYBUCKETADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	amount := parseInt64(ctx.ArgString(1))

	leakyBucketsMu.RLock()
	lb, exists := leakyBuckets[name]
	leakyBucketsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR leaky bucket not found"))
	}

	if lb.Add(amount) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdLEAKYBUCKETAVAILABLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	leakyBucketsMu.RLock()
	lb, exists := leakyBuckets[name]
	leakyBucketsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR leaky bucket not found"))
	}

	available := lb.Available()
	return ctx.WriteInteger(available)
}

func cmdLEAKYBUCKETDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	leakyBucketsMu.Lock()
	defer leakyBucketsMu.Unlock()

	if _, exists := leakyBuckets[name]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(leakyBuckets, name)
	return ctx.WriteInteger(1)
}

func cmdSLIDINGWINDOWCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	windowSizeMs := parseInt64(ctx.ArgString(1))
	limit := parseInt64(ctx.ArgString(2))

	slidingWindowCountersMu.Lock()
	slidingWindowCounters[name] = store.NewSlidingWindowCounter(windowSizeMs, limit)
	slidingWindowCountersMu.Unlock()

	return ctx.WriteOK()
}

func cmdSLIDINGWINDOWINCR(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	key := ctx.ArgString(1)

	slidingWindowCountersMu.RLock()
	swc, exists := slidingWindowCounters[name]
	slidingWindowCountersMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sliding window counter not found"))
	}

	count, allowed := swc.Increment(key)

	if allowed {
		return ctx.WriteArray([]*resp.Value{
			resp.IntegerValue(1),
			resp.IntegerValue(count),
		})
	}
	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(0),
		resp.IntegerValue(count),
	})
}

func cmdSLIDINGWINDOWCOUNT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	slidingWindowCountersMu.RLock()
	swc, exists := slidingWindowCounters[name]
	slidingWindowCountersMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sliding window counter not found"))
	}

	count := swc.Count()
	return ctx.WriteInteger(count)
}

func cmdSLIDINGWINDOWRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	slidingWindowCountersMu.RLock()
	swc, exists := slidingWindowCounters[name]
	slidingWindowCountersMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sliding window counter not found"))
	}

	swc.Reset()
	return ctx.WriteOK()
}

func cmdSLIDINGWINDOWDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	slidingWindowCountersMu.Lock()
	defer slidingWindowCountersMu.Unlock()

	if _, exists := slidingWindowCounters[name]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(slidingWindowCounters, name)
	return ctx.WriteInteger(1)
}

func cmdDEBOUNCESET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	value := ctx.ArgString(1)
	delayMs := parseInt64(ctx.ArgString(2))

	debounceMu.Lock()
	defer debounceMu.Unlock()

	debounceValues[key] = value
	debounceDelay[key] = delayMs
	debounceTimers[key] = time.Now().UnixMilli() + delayMs

	return ctx.WriteOK()
}

func cmdDEBOUNCEGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	debounceMu.RLock()
	defer debounceMu.RUnlock()

	if value, exists := debounceValues[key]; exists {
		return ctx.WriteBulkString(value)
	}
	return ctx.WriteNull()
}

func cmdDEBOUNCECALL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	debounceMu.Lock()
	defer debounceMu.Unlock()

	now := time.Now().UnixMilli()

	if timer, exists := debounceTimers[key]; exists {
		if now >= timer {
			value := debounceValues[key]
			delete(debounceTimers, key)
			delete(debounceValues, key)
			delete(debounceDelay, key)
			return ctx.WriteArray([]*resp.Value{
				resp.IntegerValue(1),
				resp.BulkString(value),
			})
		}
		remaining := timer - now
		return ctx.WriteArray([]*resp.Value{
			resp.IntegerValue(0),
			resp.IntegerValue(remaining),
		})
	}

	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(0),
		resp.IntegerValue(0),
	})
}

func cmdDEBOUNCEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	debounceMu.Lock()
	defer debounceMu.Unlock()

	if _, exists := debounceTimers[key]; exists {
		delete(debounceTimers, key)
		delete(debounceValues, key)
		delete(debounceDelay, key)
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdTHROTTLESET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	intervalMs := parseInt64(ctx.ArgString(1))

	throttleMu.Lock()
	defer throttleMu.Unlock()

	throttleInterval[key] = intervalMs
	throttleLastCall[key] = 0

	return ctx.WriteOK()
}

func cmdTHROTTLECALL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	throttleMu.Lock()
	defer throttleMu.Unlock()

	interval, exists := throttleInterval[key]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR throttle not found"))
	}

	now := time.Now().UnixMilli()
	lastCall := throttleLastCall[key]

	if now-lastCall >= interval {
		throttleLastCall[key] = now
		return ctx.WriteArray([]*resp.Value{
			resp.IntegerValue(1),
			resp.IntegerValue(0),
		})
	}

	remaining := interval - (now - lastCall)
	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(0),
		resp.IntegerValue(remaining),
	})
}

func cmdTHROTTLERESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	throttleMu.Lock()
	defer throttleMu.Unlock()

	if _, exists := throttleInterval[key]; exists {
		throttleLastCall[key] = 0
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR throttle not found"))
}

func cmdTHROTTLEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	throttleMu.Lock()
	defer throttleMu.Unlock()

	if _, exists := throttleInterval[key]; exists {
		delete(throttleInterval, key)
		delete(throttleLastCall, key)
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func parseFloatExt(data []byte) float64 {
	var result float64
	var sign float64 = 1
	var decimal float64 = 1
	inDecimal := false

	i := 0
	if i < len(data) && data[i] == '-' {
		sign = -1
		i++
	} else if i < len(data) && data[i] == '+' {
		i++
	}

	for ; i < len(data); i++ {
		if data[i] >= '0' && data[i] <= '9' {
			digit := float64(data[i] - '0')
			if inDecimal {
				decimal *= 10
				result += digit / decimal
			} else {
				result = result*10 + digit
			}
		} else if data[i] == '.' && !inDecimal {
			inDecimal = true
		} else {
			break
		}
	}

	return sign * result
}
