package command

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// RL, RATELIMIT, TOKENBUCKET, LEAKYBUCKET and THROTTLE keep each limiter in
// a hash key holding its settings and state, times in unix milliseconds.
// Being keys, limiters are per database, persisted, replicated and removed
// by DEL and FLUSHALL. Each change is recorded as an HSET of the new values
// (DEL and HSET when a limiter is replaced), so replay does not depend on the
// clock and Redis can load the AOF. Their writes are serialized per key by
// the write gate, which makes each read-modify-write atomic.

// recordKind is the shape of one kind of limiter: the fields its hash holds,
// and whether other fields may sit beside them.
type recordKind struct {
	name   string
	fields []string
	extra  bool
}

var (
	rlKind          = recordKind{name: "rate limiter", fields: []string{"max_tokens", "refill_rate", "interval_ms", "tokens", "last_refill"}}
	rateLimitKind   = recordKind{name: "rate limiter", fields: []string{"limit", "window", "window_start"}, extra: true}
	tokenBucketKind = recordKind{name: "token bucket", fields: []string{"max_tokens", "refill_rate", "tokens", "last_refill"}}
	leakyBucketKind = recordKind{name: "leaky bucket", fields: []string{"capacity", "leak_rate", "remaining", "last_leak"}}
	throttleKind    = recordKind{name: "throttle", fields: []string{"interval_ms", "last_call"}}
)

// record is a limiter read from its hash. Reading a number that does not
// parse marks it bad, which callers answer with WRONGTYPE.
type record struct {
	vals map[string]string
	bad  bool
}

func (r *record) int(f string) int64 {
	n, err := strconv.ParseInt(r.vals[f], 10, 64)
	if err != nil {
		r.bad = true
	}
	return n
}

func (r *record) float(f string) float64 {
	n, err := strconv.ParseFloat(r.vals[f], 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		r.bad = true
	}
	return n
}

// read returns the limiter at key, nil when the key is missing, and
// WRONGTYPE when the key holds anything else. With counter set it also
// reads that extra field, empty when absent.
func (k recordKind) read(ctx *Context, key string, counter ...string) (*record, error) {
	entry, ok := ctx.Store.Get(key)
	if !ok {
		return nil, nil
	}
	h, ok := entry.Value.(*store.HashValue)
	if !ok {
		return nil, store.ErrWrongType
	}
	h.RLock()
	defer h.RUnlock()
	if !k.extra && h.Len() != len(k.fields) {
		return nil, store.ErrWrongType
	}
	r := &record{vals: make(map[string]string, len(k.fields)+len(counter))}
	for _, f := range k.fields {
		v, ok := h.Get(f)
		if !ok {
			return nil, store.ErrWrongType
		}
		r.vals[f] = string(v)
	}
	for _, f := range counter {
		if v, ok := h.Get(f); ok {
			r.vals[f] = string(v)
		}
	}
	return r, nil
}

// load is read for a limiter that must exist.
func (k recordKind) load(ctx *Context, key string, counter ...string) (*record, error) {
	r, err := k.read(ctx, key, counter...)
	if err == nil && r == nil {
		err = fmt.Errorf("ERR %s not found", k.name)
	}
	return r, err
}

func pairArgs(key string, pairs []string) [][]byte {
	args := make([][]byte, 0, 1+len(pairs))
	args = append(args, []byte(key))
	for _, p := range pairs {
		args = append(args, []byte(p))
	}
	return args
}

// updateRecord sets fields of the hash at key, recorded as one HSET.
func updateRecord(ctx *Context, key string, pairs ...string) error {
	args := pairArgs(key, pairs)
	if _, err := ctx.Store.ApplyHashPairs(key, false, args[1:]); err != nil {
		return err
	}
	ctx.notify(store.NotifyHash, "hset", key)
	ctx.PropagateAs(ExecutedCommand{Name: "HSET", Args: args, DB: ctx.DB})
	return nil
}

// replaceRecord makes key a new hash of pairs, recorded as DEL and HSET.
func replaceRecord(ctx *Context, key string, pairs ...string) error {
	v, ok := store.NewLoadedHash(pairs)
	if !ok {
		return errors.New("ERR duplicate field")
	}
	if err := ctx.Store.Set(key, v, store.SetOptions{}); err != nil {
		return err
	}
	ctx.notify(store.NotifyHash, "hset", key)
	ctx.PropagateAs(
		ExecutedCommand{Name: "DEL", Args: [][]byte{[]byte(key)}, DB: ctx.DB},
		ExecutedCommand{Name: "HSET", Args: pairArgs(key, pairs), DB: ctx.DB},
	)
	return nil
}

// deleteRecord is the DELETE of every limiter kind: 1 when the key held one.
func deleteRecord(ctx *Context, k recordKind) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	r, err := k.read(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	ctx.Store.Delete(key)
	ctx.notify(store.NotifyGeneric, "del", key)
	ctx.PropagateAs(ExecutedCommand{Name: "DEL", Args: [][]byte{[]byte(key)}, DB: ctx.DB})
	return ctx.WriteInteger(1)
}

// createRecord replaces the limiter at key, refusing a key of another kind.
func createRecord(ctx *Context, k recordKind, key string, pairs ...string) error {
	if _, err := k.read(ctx, key); err != nil {
		return ctx.WriteError(err)
	}
	if err := replaceRecord(ctx, key, pairs...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func i64(n int64) string   { return strconv.FormatInt(n, 10) }
func f64(n float64) string { return strconv.FormatFloat(n, 'f', -1, 64) }
func errWrongTypeOr(bad bool, err error) error {
	if bad {
		return store.ErrWrongType
	}
	return err
}

func parseNonNegative(arg, name string) (int64, error) {
	n, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ERR %s is not an integer or out of range", name)
	}
	if n < 0 {
		return 0, fmt.Errorf("ERR %s must not be negative", name)
	}
	return n, nil
}

func parseNonNegativeFloat(arg, name string) (float64, error) {
	n, err := strconv.ParseFloat(arg, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("ERR %s is not a valid float", name)
	}
	if n < 0 {
		return 0, fmt.Errorf("ERR %s must not be negative", name)
	}
	return n, nil
}

// --- RL: an integer token bucket refilled by refill_rate every interval.

type rlState struct {
	max, refill, interval, tokens, last int64
}

func loadRL(ctx *Context, key string) (*rlState, error) {
	r, err := rlKind.load(ctx, key)
	if err != nil {
		return nil, err
	}
	s := &rlState{max: r.int("max_tokens"), refill: r.int("refill_rate"), interval: r.int("interval_ms"), tokens: r.int("tokens"), last: r.int("last_refill")}
	if r.bad || s.interval <= 0 {
		return nil, store.ErrWrongType
	}
	return s, nil
}

// refillAt adds the tokens of every whole interval since the last refill,
// and reports whether it did.
func (s *rlState) refillAt(now int64) bool {
	elapsed := now - s.last
	if elapsed < s.interval {
		return false
	}
	periods := elapsed / s.interval
	if s.refill > 0 && periods > (s.max-s.tokens)/s.refill {
		s.tokens = s.max
	} else {
		s.tokens += periods * s.refill
	}
	if s.tokens > s.max {
		s.tokens = s.max
	}
	s.last = now
	return true
}

func cmdRLCREATE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	maxTokens, err := parseNonNegative(ctx.ArgString(1), "maxTokens")
	if err != nil {
		return ctx.WriteError(err)
	}
	refill, err := parseNonNegative(ctx.ArgString(2), "refillRate")
	if err != nil {
		return ctx.WriteError(err)
	}
	interval, err := parseRateLimitDuration(ctx.ArgString(3), "intervalMs")
	if err != nil {
		return ctx.WriteError(err)
	}
	return createRecord(ctx, rlKind, ctx.ArgString(0),
		"max_tokens", i64(maxTokens), "refill_rate", i64(refill), "interval_ms", i64(interval),
		"tokens", i64(maxTokens), "last_refill", i64(ctx.nowMS()))
}

func cmdRLALLOW(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	want, err := parseNonNegative(ctx.ArgString(1), "tokens")
	if err != nil {
		return ctx.WriteError(err)
	}
	s, err := loadRL(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	changed := s.refillAt(ctx.nowMS())
	allowed := s.tokens >= want
	if allowed {
		s.tokens -= want
		changed = changed || want > 0
	}
	if changed {
		if err := updateRecord(ctx, key, "tokens", i64(s.tokens), "last_refill", i64(s.last)); err != nil {
			return ctx.WriteError(err)
		}
	} else {
		ctx.PropagateAs()
	}
	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(utilityBoolToInt(allowed)),
		resp.IntegerValue(s.tokens),
		resp.IntegerValue((s.last + s.interval) / 1000),
	})
}

func cmdRLGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	s, err := loadRL(ctx, ctx.ArgString(0))
	if err != nil {
		if errors.Is(err, store.ErrWrongType) {
			return ctx.WriteError(err)
		}
		return ctx.WriteNull()
	}
	s.refillAt(ctx.nowMS())
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("tokens"), resp.IntegerValue(s.tokens),
		resp.BulkString("max_tokens"), resp.IntegerValue(s.max),
		resp.BulkString("refill_rate"), resp.IntegerValue(s.refill),
		resp.BulkString("interval_ms"), resp.IntegerValue(s.interval),
	})
}

func cmdRLDELETE(ctx *Context) error { return deleteRecord(ctx, rlKind) }

func cmdRLRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	s, err := loadRL(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrWrongType) {
			return ctx.WriteError(err)
		}
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if err := updateRecord(ctx, key, "tokens", i64(s.max), "last_refill", i64(ctx.nowMS())); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(1)
}

// --- RATELIMIT: a fixed window counting the requests of each id. The hash
// holds the counters of the current window only, as fields "c:<id>"; the
// first request of a new window replaces them.

func cmdRATELIMITCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	limit, err := parseNonNegative(ctx.ArgString(1), "limit")
	if err != nil {
		return ctx.WriteError(err)
	}
	window, err := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	if err != nil || window <= 0 {
		return ctx.WriteError(fmt.Errorf("ERR window must be positive"))
	}
	return createRecord(ctx, rateLimitKind, ctx.ArgString(0), "limit", i64(limit), "window", i64(window), "window_start", "0")
}

func cmdRATELIMITCHECK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key, counter := ctx.ArgString(0), "c:"+ctx.ArgString(1)
	r, err := rateLimitKind.load(ctx, key, counter)
	if err != nil {
		return ctx.WriteError(err)
	}
	limit, window, start := r.int("limit"), r.int("window"), r.int("window_start")
	count := int64(0)
	if _, ok := r.vals[counter]; ok {
		count = r.int(counter)
	}
	if r.bad || window <= 0 {
		return ctx.WriteError(store.ErrWrongType)
	}
	now := ctx.nowMS()
	current := now / window * window
	left := window - (now - current)
	if current != start {
		count = 0
		pairs := []string{"limit", i64(limit), "window", i64(window), "window_start", i64(current)}
		if limit > 0 {
			pairs = append(pairs, counter, "1")
		}
		if err := replaceRecord(ctx, key, pairs...); err != nil {
			return ctx.WriteError(err)
		}
	} else if count < limit {
		if err := updateRecord(ctx, key, counter, i64(count+1)); err != nil {
			return ctx.WriteError(err)
		}
	} else {
		ctx.PropagateAs()
	}
	if count >= limit {
		return ctx.WriteArray([]*resp.Value{resp.IntegerValue(0), resp.IntegerValue(0), resp.IntegerValue(left)})
	}
	return ctx.WriteArray([]*resp.Value{resp.IntegerValue(1), resp.IntegerValue(limit - count - 1), resp.IntegerValue(left)})
}

func cmdRATELIMITRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	r, err := rateLimitKind.load(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	return createRecord(ctx, rateLimitKind, key, "limit", r.vals["limit"], "window", r.vals["window"], "window_start", "0")
}

func cmdRATELIMITDELETE(ctx *Context) error { return deleteRecord(ctx, rateLimitKind) }

// --- TOKENBUCKET: a token bucket refilled continuously, refill_rate tokens
// a second.

type tokenBucket struct {
	max, rate, tokens float64
	last              int64
}

func loadTokenBucket(ctx *Context, key string) (*tokenBucket, error) {
	r, err := tokenBucketKind.load(ctx, key)
	if err != nil {
		return nil, err
	}
	b := &tokenBucket{max: r.float("max_tokens"), rate: r.float("refill_rate"), tokens: r.float("tokens"), last: r.int("last_refill")}
	return b, errWrongTypeOr(r.bad, nil)
}

func (b *tokenBucket) refillAt(now int64) {
	if now > b.last {
		b.tokens = math.Min(b.max, b.tokens+float64(now-b.last)/1000*b.rate)
		b.last = now
	}
}

func cmdTOKENBUCKETCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	maxTokens, err := parseNonNegativeFloat(ctx.ArgString(1), "max tokens")
	if err != nil {
		return ctx.WriteError(err)
	}
	rate, err := parseNonNegativeFloat(ctx.ArgString(2), "refill rate")
	if err != nil {
		return ctx.WriteError(err)
	}
	return createRecord(ctx, tokenBucketKind, ctx.ArgString(0),
		"max_tokens", f64(maxTokens), "refill_rate", f64(rate), "tokens", f64(maxTokens), "last_refill", i64(ctx.nowMS()))
}

func cmdTOKENBUCKETCONSUME(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	want, err := parseNonNegativeFloat(ctx.ArgString(1), "tokens")
	if err != nil {
		return ctx.WriteError(err)
	}
	b, err := loadTokenBucket(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	b.refillAt(ctx.nowMS())
	allowed := b.tokens >= want
	if allowed {
		b.tokens -= want
	}
	if err := updateRecord(ctx, key, "tokens", f64(b.tokens), "last_refill", i64(b.last)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(utilityBoolToInt(allowed))
}

func cmdTOKENBUCKETAVAILABLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	b, err := loadTokenBucket(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	b.refillAt(ctx.nowMS())
	return ctx.WriteBulkString(fmt.Sprintf("%.2f", b.tokens))
}

func cmdTOKENBUCKETRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	b, err := loadTokenBucket(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, key, "tokens", f64(b.max), "last_refill", i64(ctx.nowMS())); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdTOKENBUCKETDELETE(ctx *Context) error { return deleteRecord(ctx, tokenBucketKind) }

// --- LEAKYBUCKET: capacity units, of which leak_rate come back every whole
// second.

type leakyBucket struct {
	capacity, rate, remaining, last int64
}

func loadLeakyBucket(ctx *Context, key string) (*leakyBucket, error) {
	r, err := leakyBucketKind.load(ctx, key)
	if err != nil {
		return nil, err
	}
	b := &leakyBucket{capacity: r.int("capacity"), rate: r.int("leak_rate"), remaining: r.int("remaining"), last: r.int("last_leak")}
	return b, errWrongTypeOr(r.bad, nil)
}

func (b *leakyBucket) leakAt(now int64) {
	secs := (now - b.last) / 1000
	if secs <= 0 {
		return
	}
	if b.rate > 0 && secs > (b.capacity-b.remaining)/b.rate {
		b.remaining = b.capacity
	} else {
		b.remaining += secs * b.rate
	}
	b.last += secs * 1000
}

func cmdLEAKYBUCKETCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	capacity, err := parseNonNegative(ctx.ArgString(1), "capacity")
	if err != nil {
		return ctx.WriteError(err)
	}
	rate, err := parseNonNegative(ctx.ArgString(2), "leak rate")
	if err != nil {
		return ctx.WriteError(err)
	}
	return createRecord(ctx, leakyBucketKind, ctx.ArgString(0),
		"capacity", i64(capacity), "leak_rate", i64(rate), "remaining", i64(capacity), "last_leak", i64(ctx.nowMS()))
}

func cmdLEAKYBUCKETADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	amount, err := parseNonNegative(ctx.ArgString(1), "amount")
	if err != nil {
		return ctx.WriteError(err)
	}
	b, err := loadLeakyBucket(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	b.leakAt(ctx.nowMS())
	allowed := b.remaining >= amount
	if allowed {
		b.remaining -= amount
	}
	if err := updateRecord(ctx, key, "remaining", i64(b.remaining), "last_leak", i64(b.last)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(utilityBoolToInt(allowed))
}

func cmdLEAKYBUCKETAVAILABLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	b, err := loadLeakyBucket(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	b.leakAt(ctx.nowMS())
	return ctx.WriteInteger(b.remaining)
}

func cmdLEAKYBUCKETDELETE(ctx *Context) error { return deleteRecord(ctx, leakyBucketKind) }

// --- THROTTLE: at most one call per interval.

func cmdTHROTTLESET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	interval, err := parseNonNegative(ctx.ArgString(1), "interval")
	if err != nil {
		return ctx.WriteError(err)
	}
	return createRecord(ctx, throttleKind, ctx.ArgString(0), "interval_ms", i64(interval), "last_call", "0")
}

func cmdTHROTTLECALL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	r, err := throttleKind.load(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	interval, last := r.int("interval_ms"), r.int("last_call")
	if r.bad {
		return ctx.WriteError(store.ErrWrongType)
	}
	now := ctx.nowMS()
	if now-last < interval {
		ctx.PropagateAs()
		return ctx.WriteArray([]*resp.Value{resp.IntegerValue(0), resp.IntegerValue(interval - (now - last))})
	}
	if err := updateRecord(ctx, key, "last_call", i64(now)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{resp.IntegerValue(1), resp.IntegerValue(0)})
}

func cmdTHROTTLERESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	if _, err := throttleKind.load(ctx, key); err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, key, "last_call", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdTHROTTLEDELETE(ctx *Context) error { return deleteRecord(ctx, throttleKind) }
