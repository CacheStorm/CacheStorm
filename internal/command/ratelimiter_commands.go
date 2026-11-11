package command

import (
	"errors"
	"strconv"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A RATELIMITER is a hash key: the fields limit, window (milliseconds),
// strategy and next, and one field per slot, "0" to limit-1, holding the
// time in milliseconds of a request. The slots are a ring of the last limit
// requests, so a request is allowed when the slot it would overwrite, the
// limit-th most recent request, is empty or out of the window: at most limit
// requests in any window, as a sliding log allows. A request writes one slot
// and next, recorded as that HSET, so the AOF grows by a few bytes per
// request whatever the limit, and Redis can load it. Being a key, a limiter
// is per database, persisted, replicated and removed by DEL and FLUSHALL.

const maxRateLimit = 100_000

var (
	errNoRateLimiter  = errors.New("ERR rate limiter not found")
	errNotRateLimiter = errors.New("WRONGTYPE Operation against a key holding a value that is not a rate limiter")
)

type rateLimiter struct {
	limit, window, next int64
	strategy            string
	h                   *store.HashValue
}

func readRateLimiter(ctx *Context, key string) (*rateLimiter, error) {
	entry, ok := ctx.Store.Get(key)
	if !ok {
		return nil, errNoRateLimiter
	}
	h, ok := entry.Value.(*store.HashValue)
	if !ok {
		return nil, errNotRateLimiter
	}
	h.RLock()
	defer h.RUnlock()
	field := func(name string) (int64, bool) {
		v, ok := h.Get(name)
		if !ok {
			return 0, false
		}
		n, err := strconv.ParseInt(string(v), 10, 64)
		return n, err == nil
	}
	limit, ok1 := field("limit")
	window, ok2 := field("window")
	next, ok3 := field("next")
	strategy, ok4 := h.Get("strategy")
	if !ok1 || !ok2 || !ok3 || !ok4 || limit < 1 || limit > maxRateLimit || window < 1 || next < 0 || next >= limit {
		return nil, errNotRateLimiter
	}
	return &rateLimiter{limit: limit, window: window, next: next, strategy: string(strategy), h: h}, nil
}

// slot returns the request time in slot i, or 0 when it is empty.
func (rl *rateLimiter) slot(i int64) int64 {
	rl.h.RLock()
	defer rl.h.RUnlock()
	v, ok := rl.h.Get(strconv.FormatInt(i, 10))
	if !ok {
		return 0
	}
	n, _ := strconv.ParseInt(string(v), 10, 64) //nolint:errcheck // a damaged slot counts as empty
	return n
}

// inWindow counts the requests in the window ending at now.
func (rl *rateLimiter) inWindow(now int64) int64 {
	rl.h.RLock()
	defer rl.h.RUnlock()
	n := int64(0)
	cutoff := now - rl.window
	rl.h.Range(func(f string, v []byte) bool {
		if f == "" || f[0] < '0' || f[0] > '9' {
			return true
		}
		if ts, err := strconv.ParseInt(string(v), 10, 64); err == nil && ts > cutoff {
			n++
		}
		return true
	})
	return n
}

// take records a request at now when a slot is free; otherwise it returns
// how many milliseconds remain until one is.
func (rl *rateLimiter) take(ctx *Context, key string, now int64) (bool, int64, error) {
	if oldest := rl.slot(rl.next); oldest != 0 && oldest > now-rl.window {
		return false, max(oldest+rl.window-now, 1), nil
	}
	args := [][]byte{
		[]byte(key),
		strconv.AppendInt(nil, rl.next, 10), strconv.AppendInt(nil, now, 10),
		[]byte("next"), strconv.AppendInt(nil, (rl.next+1)%rl.limit, 10),
	}
	if _, err := ctx.Store.ApplyHashPairs(key, false, args[1:]); err != nil {
		return false, 0, err
	}
	ctx.notify(store.NotifyHash, "hset", key)
	ctx.PropagateAs(ExecutedCommand{Name: "HSET", Args: args, DB: ctx.DB})
	return true, 0, nil
}

// RATELIMITER.CREATE name limit window_ms [strategy] creates or replaces the
// limiter with no request recorded.
func cmdRATELIMITERCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount() > 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	limit, ok := parseCircuitInt(ctx.ArgString(1), 1)
	if !ok || limit > maxRateLimit {
		return ctx.WriteError(errors.New("ERR limit must be an integer from 1 to 100000"))
	}
	window, ok := parseCircuitInt(ctx.ArgString(2), 1)
	if !ok {
		return ctx.WriteError(errors.New("ERR window must be a positive integer"))
	}
	strategy := "sliding"
	if ctx.ArgCount() == 4 {
		strategy = ctx.ArgString(3)
	}
	if _, err := readRateLimiter(ctx, key); err != nil && !errors.Is(err, errNoRateLimiter) {
		return ctx.WriteError(err)
	}
	if err := resetRateLimiter(ctx, key, limit, window, strategy); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// resetRateLimiter writes an empty limiter at key, recorded as DEL and HSET.
func resetRateLimiter(ctx *Context, key string, limit, window int64, strategy string) error {
	fields := []string{"limit", strconv.FormatInt(limit, 10), "window", strconv.FormatInt(window, 10), "strategy", strategy, "next", "0"}
	h, _ := store.NewLoadedHash(fields)
	if err := ctx.Store.Set(key, h, store.SetOptions{}); err != nil {
		return err
	}
	ctx.notify(store.NotifyHash, "hset", key)
	args := [][]byte{[]byte(key)}
	for _, f := range fields {
		args = append(args, []byte(f))
	}
	ctx.PropagateAs(
		ExecutedCommand{Name: "DEL", Args: [][]byte{[]byte(key)}, DB: ctx.DB},
		ExecutedCommand{Name: "HSET", Args: args, DB: ctx.DB},
	)
	return nil
}

// RATELIMITER.TRY name replies [1, remaining, window] when the request is
// allowed and [0, 0, window] when it is not.
func cmdRATELIMITERTRY(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	rl, err := readRateLimiter(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	now := time.Now().UnixMilli()
	allowed, _, err := rl.take(ctx, key, now)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !allowed {
		ctx.PropagateAs()
		return ctx.WriteArray([]*resp.Value{resp.IntegerValue(0), resp.IntegerValue(0), resp.IntegerValue(rl.window)})
	}
	if after, err := readRateLimiter(ctx, key); err == nil {
		rl = after
	}
	return ctx.WriteArray([]*resp.Value{resp.IntegerValue(1), resp.IntegerValue(rl.limit - rl.inWindow(now)), resp.IntegerValue(rl.window)})
}

// RATELIMITER.WAIT name timeout_ms waits for a free slot, takes it and
// replies with the milliseconds it waited, or null on timeout. Inside MULTI
// or a script it does not wait.
func cmdRATELIMITERWAIT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	timeout, err := parseResilienceTimeout(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	start := time.Now()
	deadline := start.Add(timeout)
	for {
		rl, err := readRateLimiter(ctx, key)
		if err != nil {
			return ctx.WriteError(err)
		}
		now := time.Now()
		allowed, waitMs, err := rl.take(ctx, key, now.UnixMilli())
		if err != nil {
			return ctx.WriteError(err)
		}
		if allowed {
			return ctx.WriteInteger(now.Sub(start).Milliseconds())
		}
		left := deadline.Sub(now)
		if ctx.InExec || left <= 0 {
			ctx.PropagateAs()
			return ctx.WriteNull()
		}
		if ctx.Done != nil {
			select {
			case <-ctx.Done:
				ctx.PropagateAs()
				return ctx.WriteNull()
			default:
			}
		}
		// A RESET, CREATE or DELETE writes the key and wakes the wait
		// early; otherwise it lasts until the oldest request leaves the
		// window.
		ctx.waitForStreamKeys([]string{key}, min(time.Duration(waitMs)*time.Millisecond, left), func(string) bool { return false })
	}
}

// RATELIMITER.RESET name forgets the recorded requests.
func cmdRATELIMITERRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	rl, err := readRateLimiter(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := resetRateLimiter(ctx, key, rl.limit, rl.window, rl.strategy); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// RATELIMITER.STATUS name
func cmdRATELIMITERSTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	rl, err := readRateLimiter(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(key),
		resp.BulkString("limit"), resp.IntegerValue(rl.limit),
		resp.BulkString("current"), resp.IntegerValue(rl.inWindow(time.Now().UnixMilli())),
		resp.BulkString("strategy"), resp.BulkString(rl.strategy),
	})
}

// RATELIMITER.DELETE name replies 1 when it removed a limiter, 0 otherwise.
func cmdRATELIMITERDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	if _, err := readRateLimiter(ctx, key); err != nil {
		if errors.Is(err, errNoRateLimiter) {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
		return ctx.WriteError(err)
	}
	ctx.Store.Delete(key)
	ctx.notify(store.NotifyGeneric, "del", key)
	ctx.propagateDelete(key)
	return ctx.WriteInteger(1)
}
