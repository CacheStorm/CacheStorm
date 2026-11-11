package command

import (
	"errors"
	"math"
	"strconv"

	"github.com/cachestorm/cachestorm/internal/store"
)

// A DEDUP set is a hash key with one field per seen id, each expiring on its
// own (hash field expiration), so an id counts as seen until its TTL passes
// and is then removed by the server. Being a key, a set is per database,
// persisted and replicated; DEDUP.ADD is recorded as HSET and HPEXPIREAT
// with the absolute deadline, DEDUP.CLEAR as DEL.

const dedupDefaultTTL = 3600000

var errDedupTTL = errors.New("ERR ttl must be a positive integer (milliseconds)")

// DEDUP.ADD key id [ttl_ms] marks id seen for ttl_ms (an hour by default):
// 1 when it was not seen yet, 0 when it was.
func cmdDEDUPADD(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key, id := ctx.ArgString(0), ctx.ArgString(1)
	ttl := int64(dedupDefaultTTL)
	if ctx.ArgCount() == 3 {
		var err error
		if ttl, err = strconv.ParseInt(ctx.ArgString(2), 10, 64); err != nil || ttl < 1 || ttl > math.MaxInt64/2 {
			return ctx.WriteError(errDedupTTL)
		}
	}
	at := ctx.nowMS() + ttl
	added, err := ctx.Store.ApplyHashSetEx(key, [][]byte{[]byte(id), []byte("1")}, store.HashSetExOptions{FNX: true, ExpireAt: at})
	if err != nil {
		return ctx.WriteError(err)
	}
	if !added {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	ctx.notify(store.NotifyHash, "hset", key)
	ctx.notify(store.NotifyHash, "hexpire", key)
	ctx.PropagateAs(
		ExecutedCommand{Name: "HSET", Args: hfeArgs(key, id, "1"), DB: ctx.DB},
		ExecutedCommand{Name: "HPEXPIREAT", Args: hfeArgs(key, strconv.FormatInt(at, 10), "FIELDS", "1", id), DB: ctx.DB},
	)
	return ctx.WriteInteger(1)
}

// DEDUP.CHECK key id is 1 while id is seen.
func cmdDEDUPCHECK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entry, ok := ctx.Store.Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteInteger(0)
	}
	h, ok := entry.Value.(*store.HashValue)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}
	h.RLock()
	live := h.FieldLive(ctx.ArgString(1), ctx.Store.HashFieldClock())
	h.RUnlock()
	return ctx.WriteInteger(boolInt(live))
}

// DEDUP.EXPIRE key replies 0: expired ids are removed by the server.
func cmdDEDUPEXPIRE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if _, err := lockHashRead(ctx, ctx.ArgString(0)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(0)
}

// DEDUP.CLEAR key forgets every id and replies how many were seen.
func cmdDEDUPCLEAR(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	h, err := lockHashRead(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if h == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	h.RLock()
	now := ctx.Store.HashFieldClock()
	n := 0
	for f := range h.Fields() {
		if h.FieldLive(f, now) {
			n++
		}
	}
	h.RUnlock()
	ctx.Store.Delete(key)
	ctx.notify(store.NotifyGeneric, "del", key)
	ctx.propagateDelete(key)
	return ctx.WriteInteger(int64(n))
}

// lockHashRead returns the hash at key, nil when missing, WRONGTYPE for
// another type.
func lockHashRead(ctx *Context, key string) (*store.HashValue, error) {
	entry, ok := ctx.Store.Get(key)
	if !ok {
		return nil, nil
	}
	h, ok := entry.Value.(*store.HashValue)
	if !ok {
		return nil, store.ErrWrongType
	}
	return h, nil
}
