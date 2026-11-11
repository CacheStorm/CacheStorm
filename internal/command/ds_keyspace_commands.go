package command

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// PQ, LRU, SLIDINGWINDOW and DEBOUNCE keep each object in a key: a priority
// queue is a sorted set scored by priority whose members are a sequence
// number, a colon and the value, so equal priorities pop first in first
// out; an LRU cache, a sliding window and a debounce are hashes marked by a
// field naming their kind. Being keys, they are per database, persisted and
// replicated; changes are recorded as ZADD, ZREM, HSET, HDEL and DEL.

const (
	lruMarker      = "__lru"
	slidingMarker  = "__slidingwindow"
	debounceMarker = "__debounce"
	maxPQPriority  = 1 << 53
	pqSeqWidth     = 20
)

// --- PQ ---

// pqSeq numbers queued items: nanoseconds since the epoch, raised past the
// last number handed out, so numbers grow across restarts.
var (
	pqSeqMu   sync.Mutex
	pqSeqLast int64
)

func nextPQSeq(ctx *Context) string {
	pqSeqMu.Lock()
	n := max(ctx.nowMS()*1_000_000, pqSeqLast+1)
	pqSeqLast = n
	pqSeqMu.Unlock()
	s := strconv.FormatInt(n, 10)
	return strings.Repeat("0", max(pqSeqWidth-len(s), 0)) + s
}

func pqValue(member string) (string, error) {
	if len(member) <= pqSeqWidth || member[pqSeqWidth] != ':' {
		return "", store.ErrWrongType
	}
	return member[pqSeqWidth+1:], nil
}

func readPQ(ctx *Context, name string) (*store.SortedSetValue, error) {
	return readTrie(ctx, name)
}

func pqItem(e store.SortedEntry) ([]*resp.Value, error) {
	v, err := pqValue(e.Member)
	if err != nil {
		return nil, err
	}
	return []*resp.Value{resp.BulkString(v), resp.IntegerValue(int64(e.Score))}, nil
}

// PQ.CREATE name empties the queue at name.
func cmdPQCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return pqClear(ctx)
}

// PQ.PUSH name value priority queues value and replies the queue length;
// the lowest priority pops first.
func cmdPQPUSH(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	priority, err := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	if err != nil || priority < -maxPQPriority || priority > maxPQPriority {
		return ctx.WriteError(fmt.Errorf("ERR priority must be an integer between %d and %d", int64(-maxPQPriority), int64(maxPQPriority)))
	}
	name := ctx.ArgString(0)
	if _, err := readPQ(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	member := nextPQSeq(ctx) + ":" + ctx.ArgString(1)
	saved := ctx.Writer
	var sink strings.Builder
	ctx.Writer = resp.NewWriter(&sink)
	err = runAs(ctx, "ZADD", cmdZADD, name, strconv.FormatInt(priority, 10), member)
	ctx.Writer = saved
	if err != nil {
		return err
	}
	if reply := sink.String(); strings.HasPrefix(reply, "-") {
		ctx.PropagateAs()
		return ctx.WriteError(errors.New(strings.TrimRight(reply[1:], "\r\n")))
	}
	z, err := readPQ(ctx, name)
	if err != nil || z == nil {
		return ctx.WriteError(errWrongTypeOr(true, err))
	}
	z.RLock()
	n := z.Card()
	z.RUnlock()
	return ctx.WriteInteger(int64(n))
}

func pqFirst(ctx *Context, name string) (*store.SortedEntry, error) {
	z, err := readPQ(ctx, name)
	if err != nil || z == nil {
		return nil, err
	}
	z.RLock()
	first := z.RangeByRank(0, 0, false)
	z.RUnlock()
	if len(first) == 0 {
		return nil, nil
	}
	return &first[0], nil
}

// PQ.POP name removes and replies the first item: value, priority.
func cmdPQPOP(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	e, err := pqFirst(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if e == nil {
		ctx.PropagateAs()
		return ctx.WriteNull()
	}
	item, err := pqItem(*e)
	if err != nil {
		return ctx.WriteError(err)
	}
	saved := ctx.Writer
	var sink strings.Builder
	ctx.Writer = resp.NewWriter(&sink)
	err = runAs(ctx, "ZREM", cmdZREM, name, e.Member)
	ctx.Writer = saved
	if err != nil {
		return err
	}
	return ctx.WriteArray(item)
}

func cmdPQPEEK(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	e, err := pqFirst(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if e == nil {
		return ctx.WriteNull()
	}
	item, err := pqItem(*e)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray(item)
}

func cmdPQLEN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return cmdZCARD(ctx)
}

func pqClear(ctx *Context) error {
	name := ctx.ArgString(0)
	z, err := readPQ(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if z == nil {
		ctx.PropagateAs()
		return ctx.WriteOK()
	}
	ctx.Store.Delete(name)
	ctx.notify(store.NotifyGeneric, "del", name)
	ctx.propagateDelete(name)
	return ctx.WriteOK()
}

func cmdPQCLEAR(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return pqClear(ctx)
}

// PQ.GETALL name replies value, priority pairs in pop order.
func cmdPQGETALL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	z, err := readPQ(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if z == nil {
		return ctx.WriteArray([]*resp.Value{})
	}
	z.RLock()
	all := z.RangeByRank(0, z.Card(), false)
	z.RUnlock()
	out := make([]*resp.Value, 0, 2*len(all))
	for _, e := range all {
		item, err := pqItem(e)
		if err != nil {
			return ctx.WriteError(err)
		}
		out = append(out, item...)
	}
	return ctx.WriteArray(out)
}

// --- LRU ---

// An LRU cache is a hash marked by __lru (its capacity) with __tick, a
// counter of uses, and for each entry v:<key> (the value) and t:<key> (the
// tick of its last use). LRU.GET counts as a use, so it is a write.

type lruCache struct {
	capacity int64
	tick     int64
	values   map[string]string
	ticks    map[string]int64
}

func loadLRU(ctx *Context, name string, must bool) (*lruCache, error) {
	r, err := readMarked(ctx, name, lruMarker)
	if err != nil {
		return nil, err
	}
	if r == nil {
		if must {
			return nil, errors.New("ERR LRU cache not found")
		}
		return nil, nil
	}
	c := &lruCache{values: map[string]string{}, ticks: map[string]int64{}}
	var e1, e2 error
	c.capacity, e1 = strconv.ParseInt(r[lruMarker], 10, 64)
	c.tick, e2 = strconv.ParseInt(r["__tick"], 10, 64)
	if e1 != nil || e2 != nil {
		return nil, store.ErrWrongType
	}
	for f, v := range r {
		switch {
		case strings.HasPrefix(f, "v:"):
			c.values[f[2:]] = v
		case strings.HasPrefix(f, "t:"):
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, store.ErrWrongType
			}
			c.ticks[f[2:]] = n
		}
	}
	return c, nil
}

// order returns the keys most recently used first.
func (c *lruCache) order() []string {
	keys := make([]string, 0, len(c.values))
	for k := range c.values {
		keys = append(keys, k)
	}
	sortByDesc(keys, c.ticks)
	return keys
}

func sortByDesc(keys []string, by map[string]int64) {
	sort.Slice(keys, func(i, j int) bool {
		if by[keys[i]] != by[keys[j]] {
			return by[keys[i]] > by[keys[j]]
		}
		return keys[i] < keys[j]
	})
}

// LRU.CREATE name capacity makes an empty cache, replacing one at name.
func cmdLRUCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	capacity, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	if err := checkSlots("capacity", capacity); err != nil {
		return ctx.WriteError(err)
	}
	if err := createMarked(ctx, ctx.ArgString(0), lruMarker, lruMarker, strconv.Itoa(capacity), "__tick", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// LRU.GET name key replies the value and marks the key used.
func cmdLRUGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, key := ctx.ArgString(0), ctx.ArgString(1)
	c, err := loadLRU(ctx, name, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	if c == nil {
		ctx.PropagateAs()
		return ctx.WriteNull()
	}
	v, ok := c.values[key]
	if !ok {
		ctx.PropagateAs()
		return ctx.WriteNull()
	}
	if err := updateRecord(ctx, name, "__tick", i64(c.tick+1), "t:"+key, i64(c.tick+1)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(v)
}

// LRU.SET name key value stores the entry and replies 1 when it evicted the
// least recently used one.
func cmdLRUSET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, key := ctx.ArgString(0), ctx.ArgString(1)
	c, err := loadLRU(ctx, name, true)
	if err != nil {
		return ctx.WriteError(err)
	}
	tick := c.tick + 1
	if err := updateRecord(ctx, name, "__tick", i64(tick), "v:"+key, ctx.ArgString(2), "t:"+key, i64(tick)); err != nil {
		return ctx.WriteError(err)
	}
	if _, exists := c.values[key]; exists || int64(len(c.values)) < c.capacity {
		return ctx.WriteInteger(0)
	}
	order := c.order()
	victim := order[len(order)-1]
	set, _ := ctx.Propagation()
	deleteMarkedFields(ctx, name, lruMarker, "v:"+victim, "t:"+victim)
	del, _ := ctx.Propagation()
	ctx.PropagateAs(append(set, del...)...)
	return ctx.WriteInteger(1)
}

func cmdLRUDEL(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, key := ctx.ArgString(0), ctx.ArgString(1)
	c, err := loadLRU(ctx, name, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	if c == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(boolInt(deleteMarkedFields(ctx, name, lruMarker, "v:"+key, "t:"+key) > 0))
}

// LRU.CLEAR name empties the cache.
func cmdLRUCLEAR(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	c, err := loadLRU(ctx, name, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	if c == nil || len(c.values) == 0 {
		ctx.PropagateAs()
		return ctx.WriteOK()
	}
	if err := replaceRecord(ctx, name, lruMarker, i64(c.capacity), "__tick", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// LRU.KEYS name replies the keys, most recently used first.
func cmdLRUKEYS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	c, err := loadLRU(ctx, ctx.ArgString(0), false)
	if err != nil {
		return ctx.WriteError(err)
	}
	if c == nil {
		return ctx.WriteArray([]*resp.Value{})
	}
	return ctx.WriteArray(bulkStrings(c.order()))
}

func cmdLRUSTATS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	c, err := loadLRU(ctx, ctx.ArgString(0), false)
	if err != nil {
		return ctx.WriteError(err)
	}
	if c == nil {
		return ctx.WriteNull()
	}
	size := int64(len(c.values))
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("size"), resp.IntegerValue(size),
		resp.BulkString("capacity"), resp.IntegerValue(c.capacity),
		resp.BulkString("usage"), resp.BulkString(fmt.Sprintf("%.2f", float64(size)/float64(c.capacity)*100)),
	})
}

// --- SLIDINGWINDOW ---

// A sliding window counter is a hash marked by __slidingwindow (the window
// in ms) with limit and, for each counted key, k:<key> holding the index of
// its current window and its counts in that window and the one before.
// A key's count is the sum of the two.

type windowCount struct{ window, cur, prev int64 }

func (w windowCount) at(window int64) windowCount {
	switch window {
	case w.window:
		return w
	case w.window + 1:
		return windowCount{window, 0, w.cur}
	default:
		return windowCount{window, 0, 0}
	}
}

func loadSliding(ctx *Context, name string) (size, limit int64, keys map[string]windowCount, err error) {
	r, err := loadMarked(ctx, name, slidingMarker, "sliding window counter")
	if err != nil {
		return 0, 0, nil, err
	}
	var e1, e2 error
	size, e1 = strconv.ParseInt(r[slidingMarker], 10, 64)
	limit, e2 = strconv.ParseInt(r["limit"], 10, 64)
	if e1 != nil || e2 != nil || size < 1 {
		return 0, 0, nil, store.ErrWrongType
	}
	keys = map[string]windowCount{}
	for f, v := range r {
		if k, ok := strings.CutPrefix(f, "k:"); ok {
			var w windowCount
			if _, err := fmt.Sscanf(v, "%d %d %d", &w.window, &w.cur, &w.prev); err != nil {
				return 0, 0, nil, store.ErrWrongType
			}
			keys[k] = w
		}
	}
	return size, limit, keys, nil
}

func cmdSLIDINGWINDOWCREATE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	size, err1 := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	limit, err2 := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	if err1 != nil || size <= 0 {
		return ctx.WriteError(errors.New("ERR window must be positive"))
	}
	if err2 != nil || limit < 0 {
		return ctx.WriteError(errors.New("ERR limit must be a non-negative integer"))
	}
	if err := createMarked(ctx, ctx.ArgString(0), slidingMarker, slidingMarker, i64(size), "limit", i64(limit)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// SLIDINGWINDOW.INCR name key counts one event for key when the key is
// under the limit: [1, count in the current window] or [0, count].
func cmdSLIDINGWINDOWINCR(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, key := ctx.ArgString(0), ctx.ArgString(1)
	size, limit, keys, err := loadSliding(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	w := keys[key].at(ctx.nowMS() / size)
	if count := w.cur + w.prev; count >= limit {
		ctx.PropagateAs()
		return ctx.WriteArray([]*resp.Value{resp.IntegerValue(0), resp.IntegerValue(count)})
	}
	w.cur++
	if err := updateRecord(ctx, name, "k:"+key, fmt.Sprintf("%d %d %d", w.window, w.cur, w.prev)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{resp.IntegerValue(1), resp.IntegerValue(w.cur)})
}

// SLIDINGWINDOW.COUNT name [key] replies the count of key, or of every key.
func cmdSLIDINGWINDOWCOUNT(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	size, _, keys, err := loadSliding(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	window := ctx.nowMS() / size
	var total int64
	for k, w := range keys {
		if ctx.ArgCount() == 2 && k != ctx.ArgString(1) {
			continue
		}
		w = w.at(window)
		total += w.cur + w.prev
	}
	return ctx.WriteInteger(total)
}

func cmdSLIDINGWINDOWRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	size, limit, keys, err := loadSliding(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if len(keys) == 0 {
		ctx.PropagateAs()
		return ctx.WriteOK()
	}
	if err := replaceRecord(ctx, name, slidingMarker, i64(size), "limit", i64(limit)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSLIDINGWINDOWDELETE(ctx *Context) error { return deleteMarked(ctx, slidingMarker) }

// --- DEBOUNCE ---

// A debounce is a hash marked by __debounce (its value) with due, the unix
// ms time after which DEBOUNCE.CALL fires it.

func cmdDEBOUNCESET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	delay, err := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	if err != nil || delay < 0 || delay > math.MaxInt64/2 {
		return ctx.WriteError(errors.New("ERR delay must be a non-negative integer (milliseconds)"))
	}
	if err := createMarked(ctx, ctx.ArgString(0), debounceMarker, debounceMarker, ctx.ArgString(1), "due", i64(ctx.nowMS()+delay)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdDEBOUNCEGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := readMarked(ctx, ctx.ArgString(0), debounceMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(r[debounceMarker])
}

// DEBOUNCE.CALL key fires a due debounce, removing it: [1, value]; before it
// is due it replies [0, ms left], and [0, 0] when there is none.
func cmdDEBOUNCECALL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	r, err := readMarked(ctx, key, debounceMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		ctx.PropagateAs()
		return ctx.WriteArray([]*resp.Value{resp.IntegerValue(0), resp.IntegerValue(0)})
	}
	due, err := strconv.ParseInt(r["due"], 10, 64)
	if err != nil {
		return ctx.WriteError(store.ErrWrongType)
	}
	if now := ctx.nowMS(); now < due {
		ctx.PropagateAs()
		return ctx.WriteArray([]*resp.Value{resp.IntegerValue(0), resp.IntegerValue(due - now)})
	}
	ctx.Store.Delete(key)
	ctx.notify(store.NotifyGeneric, "del", key)
	ctx.propagateDelete(key)
	return ctx.WriteArray([]*resp.Value{resp.IntegerValue(1), resp.BulkString(r[debounceMarker])})
}

func cmdDEBOUNCEDELETE(ctx *Context) error { return deleteMarked(ctx, debounceMarker) }
