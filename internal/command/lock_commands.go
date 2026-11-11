package command

import (
	"errors"
	"strconv"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A LOCK is a hash key with the fields holder, token and renewals that
// expires when the lock does, the way a Redis lock is a key set with NX and
// PX. Being a key, it is per database, saved in the RDB and the AOF,
// replicated, and released by DEL, FLUSHDB and FLUSHALL; HGETALL and PTTL
// read it. A change is recorded as HSET and PEXPIREAT with the absolute
// deadline, so a restart or a replica keeps the deadline the lock was given.

var errNotALock = errors.New("WRONGTYPE Operation against a key holding a value that is not a lock")

type lockState struct {
	holder, token string
	renewals      int64
	expiresAt     int64
}

// readLock returns the lock at key, or nil when it is free.
func readLock(ctx *Context, key string) (*lockState, error) {
	entry, ok := ctx.Store.Get(key)
	if !ok {
		return nil, nil
	}
	h, ok := entry.Value.(*store.HashValue)
	if !ok {
		return nil, errNotALock
	}
	h.RLock()
	defer h.RUnlock()
	holder, ok1 := h.Get("holder")
	token, ok2 := h.Get("token")
	renewals, ok3 := h.Get("renewals")
	if !ok1 || !ok2 || !ok3 || h.Len() != 3 {
		return nil, errNotALock
	}
	n, err := strconv.ParseInt(string(renewals), 10, 64)
	if err != nil {
		return nil, errNotALock
	}
	return &lockState{holder: string(holder), token: string(token), renewals: n, expiresAt: entry.ExpiresAt.Load()}, nil
}

// writeLock stores the lock at key until the deadline when, in milliseconds.
func writeLock(ctx *Context, key string, l *lockState) error {
	renewals := strconv.FormatInt(l.renewals, 10)
	h, _ := store.NewLoadedHash([]string{"holder", l.holder, "token", l.token, "renewals", renewals})
	if err := ctx.Store.Set(key, h, store.SetOptions{ExpireAtMS: l.expiresAt}); err != nil {
		return err
	}
	ctx.notify(store.NotifyHash, "hset", key)
	ctx.PropagateAs(
		ExecutedCommand{Name: "HSET", Args: [][]byte{[]byte(key), []byte("holder"), []byte(l.holder), []byte("token"), []byte(l.token), []byte("renewals"), []byte(renewals)}, DB: ctx.DB},
		ExecutedCommand{Name: "PEXPIREAT", Args: [][]byte{[]byte(key), strconv.AppendInt(nil, l.expiresAt, 10)}, DB: ctx.DB},
	)
	return nil
}

func parseLockMillis(s string) (time.Duration, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 || n > int64(100*365*24*time.Hour/time.Millisecond) {
		return 0, errors.New("ERR invalid expire time, must be a positive number of milliseconds")
	}
	return time.Duration(n) * time.Millisecond, nil
}

// tryLock takes the free lock, or renews it for its own holder and token.
func tryLock(ctx *Context, key, holder, token string, ttl time.Duration) (bool, error) {
	l, err := readLock(ctx, key)
	if err != nil {
		return false, err
	}
	switch {
	case l == nil:
		l = &lockState{holder: holder, token: token}
	case l.holder == holder && l.token == token:
		l.renewals++
	default:
		return false, nil
	}
	l.expiresAt = time.Now().Add(ttl).UnixMilli()
	return true, writeLock(ctx, key, l)
}

func lockReply(ctx *Context, ok bool, err error) error {
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ok {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(1)
}

// LOCK.TRY key holder token ttl_ms
func cmdLOCKTRY(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	ttl, err := parseLockMillis(ctx.ArgString(3))
	if err != nil {
		return ctx.WriteError(err)
	}
	ok, err := tryLock(ctx, ctx.ArgString(0), ctx.ArgString(1), ctx.ArgString(2), ttl)
	return lockReply(ctx, ok, err)
}

// LOCK.ACQUIRE key holder token ttl_ms timeout_ms waits up to timeout_ms for
// the lock; 0 does not wait. Inside MULTI, a script or a replay it does not
// wait, as Redis' blocking commands do not.
func cmdLOCKACQUIRE(ctx *Context) error {
	if ctx.ArgCount() != 5 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	ttl, err := parseLockMillis(ctx.ArgString(3))
	if err != nil {
		return ctx.WriteError(err)
	}
	timeoutMs, err := strconv.ParseInt(ctx.ArgString(4), 10, 64)
	if err != nil || timeoutMs < 0 {
		return ctx.WriteError(errors.New("ERR timeout is not an integer or out of range"))
	}
	key, holder, token := ctx.ArgString(0), ctx.ArgString(1), ctx.ArgString(2)
	ok, err := tryLock(ctx, key, holder, token, ttl)
	if err != nil || ok || timeoutMs == 0 || ctx.InExec {
		return lockReply(ctx, ok, err)
	}
	// A release or an expiry does not signal the key, so the wait is
	// sliced: it wakes on a write to the key or every lockPoll.
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	for !ok && err == nil {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		if ctx.Done != nil {
			select {
			case <-ctx.Done:
				return lockReply(ctx, false, nil)
			default:
			}
		}
		ctx.waitForStreamKeys([]string{key}, min(remaining, lockPoll), func(k string) bool {
			l, err := readLock(ctx, k)
			return err != nil || l == nil
		})
		ok, err = tryLock(ctx, key, holder, token, ttl)
	}
	return lockReply(ctx, ok, err)
}

const lockPoll = 10 * time.Millisecond

// LOCK.RELEASE key holder token
func cmdLOCKRELEASE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	l, err := readLock(ctx, key)
	if err != nil || l == nil || l.holder != ctx.ArgString(1) || l.token != ctx.ArgString(2) {
		return lockReply(ctx, false, err)
	}
	ctx.Store.Delete(key)
	ctx.notify(store.NotifyGeneric, "del", key)
	ctx.propagateDelete(key)
	return ctx.WriteInteger(1)
}

// LOCK.RENEW key holder token ttl_ms
func cmdLOCKRENEW(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	ttl, err := parseLockMillis(ctx.ArgString(3))
	if err != nil {
		return ctx.WriteError(err)
	}
	key := ctx.ArgString(0)
	l, err := readLock(ctx, key)
	if err != nil || l == nil || l.holder != ctx.ArgString(1) || l.token != ctx.ArgString(2) {
		return lockReply(ctx, false, err)
	}
	l.renewals++
	l.expiresAt = time.Now().Add(ttl).UnixMilli()
	return lockReply(ctx, true, writeLock(ctx, key, l))
}

// LOCK.INFO key
func cmdLOCKINFO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	l, err := readLock(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if l == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("holder"),
		resp.BulkString(l.holder),
		resp.BulkString("expires_at"),
		resp.IntegerValue(l.expiresAt / 1000),
		resp.BulkString("ttl_ms"),
		resp.IntegerValue(max(l.expiresAt-time.Now().UnixMilli(), 0)),
		resp.BulkString("renewals"),
		resp.IntegerValue(l.renewals),
	})
}

// LOCK.ISLOCKED key
func cmdLOCKISLOCKED(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	l, err := readLock(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if l == nil {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(1)
}

// LOCKX is LOCK with the holder as its own token: the same hash key, so
// LOCK.INFO and LOCKX.STATUS read the same lock.

// LOCKX.ACQUIRE key holder ttl_ms takes the free lock, or renews it for its
// holder.
func cmdLOCKXACQUIRE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	ttl, err := parseLockMillis(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	holder := ctx.ArgString(1)
	ok, err := tryLock(ctx, ctx.ArgString(0), holder, holder, ttl)
	return lockReply(ctx, ok, err)
}

// LOCKX.RELEASE key holder
func cmdLOCKXRELEASE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key, holder := ctx.ArgString(0), ctx.ArgString(1)
	l, err := readLock(ctx, key)
	if err != nil || l == nil || l.holder != holder || l.token != holder {
		return lockReply(ctx, false, err)
	}
	ctx.Store.Delete(key)
	ctx.notify(store.NotifyGeneric, "del", key)
	ctx.propagateDelete(key)
	return ctx.WriteInteger(1)
}

// LOCKX.EXTEND key holder ttl_ms gives the holder's lock a new deadline.
func cmdLOCKXEXTEND(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	ttl, err := parseLockMillis(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	key, holder := ctx.ArgString(0), ctx.ArgString(1)
	l, err := readLock(ctx, key)
	if err != nil || l == nil || l.holder != holder || l.token != holder {
		return lockReply(ctx, false, err)
	}
	l.renewals++
	l.expiresAt = time.Now().Add(ttl).UnixMilli()
	return lockReply(ctx, true, writeLock(ctx, key, l))
}

// LOCKX.STATUS key replies locked 0, or locked 1 with the holder and the
// deadline in unix milliseconds.
func cmdLOCKXSTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	l, err := readLock(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if l == nil {
		return ctx.WriteArray([]*resp.Value{resp.BulkString("locked"), resp.IntegerValue(0)})
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("locked"), resp.IntegerValue(1),
		resp.BulkString("holder"), resp.BulkString(l.holder),
		resp.BulkString("expires_at"), resp.IntegerValue(l.expiresAt),
	})
}
