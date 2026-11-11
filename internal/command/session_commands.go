package command

import (
	"errors"
	"strconv"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A SESSION is a hash key that expires with the session. Its data fields sit
// beside one marker field, sessionMarker, holding the creation time in
// milliseconds: it tells a session from another hash and keeps a session
// with no data in existence, as a hash with no field is no key. Being a key,
// a session is per database, persisted with its absolute deadline,
// replicated, and removed by DEL, FLUSHALL and its expiry; changes are
// recorded as HSET, HDEL, DEL and PEXPIREAT, which Redis can load.

const sessionMarker = "__session_created"

var (
	errNoSession  = errors.New("ERR session not found")
	errNotSession = errors.New("WRONGTYPE Operation against a key holding a value that is not a session")
)

// readSession returns the session hash at key, nil when there is none.
func readSession(ctx *Context, key string) (*store.HashValue, *store.Entry, error) {
	entry, ok := ctx.Store.Get(key)
	if !ok {
		return nil, nil, nil
	}
	h, ok := entry.Value.(*store.HashValue)
	if !ok {
		return nil, nil, errNotSession
	}
	h.RLock()
	_, marked := h.Get(sessionMarker)
	h.RUnlock()
	if !marked {
		return nil, nil, errNotSession
	}
	return h, entry, nil
}

func parseSessionTTL(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 || n > int64(100*365*24*time.Hour/time.Millisecond) {
		return 0, errors.New("ERR invalid expire time, must be a positive number of milliseconds")
	}
	return n, nil
}

// writeEmptySession stores an empty session at key until when (Unix ms),
// recorded as DEL, HSET and PEXPIREAT.
func writeEmptySession(ctx *Context, key string, created, when int64) error {
	createdStr := strconv.FormatInt(created, 10)
	h, _ := store.NewLoadedHash([]string{sessionMarker, createdStr})
	if err := ctx.Store.Set(key, h, store.SetOptions{ExpireAtMS: when}); err != nil {
		return err
	}
	ctx.notify(store.NotifyHash, "hset", key)
	ctx.PropagateAs(
		ExecutedCommand{Name: "DEL", Args: [][]byte{[]byte(key)}, DB: ctx.DB},
		ExecutedCommand{Name: "HSET", Args: [][]byte{[]byte(key), []byte(sessionMarker), []byte(createdStr)}, DB: ctx.DB},
		ExecutedCommand{Name: "PEXPIREAT", Args: [][]byte{[]byte(key), strconv.AppendInt(nil, when, 10)}, DB: ctx.DB},
	)
	return nil
}

// SESSION.CREATE id ttl_ms creates or replaces the session, empty.
func cmdSESSIONCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	ttl, err := parseSessionTTL(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	if _, _, err := readSession(ctx, key); err != nil {
		return ctx.WriteError(err)
	}
	now := time.Now().UnixMilli()
	if err := writeEmptySession(ctx, key, now, now+ttl); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(key),
		resp.BulkString("ttl"), resp.IntegerValue(ttl),
	})
}

// SESSION.GET id field
func cmdSESSIONGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	h, _, err := readSession(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	field := ctx.ArgString(1)
	if h == nil || field == sessionMarker {
		return ctx.WriteNull()
	}
	h.RLock()
	val, ok := h.Get(field)
	reply := string(val)
	h.RUnlock()
	if !ok {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(reply)
}

// SESSION.SET id field value, recorded as HSET; the session keeps its TTL.
func cmdSESSIONSET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	h, _, err := readSession(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if h == nil {
		return ctx.WriteError(errNoSession)
	}
	if ctx.ArgString(1) == sessionMarker {
		return ctx.WriteError(errors.New("ERR the field name is reserved"))
	}
	if _, err := ctx.Store.ApplyHashPairs(key, false, ctx.Args[1:]); err != nil {
		return ctx.WriteError(err)
	}
	ctx.notify(store.NotifyHash, "hset", key)
	ctx.PropagateAs(ExecutedCommand{Name: "HSET", Args: ctx.Args, DB: ctx.DB})
	return ctx.WriteOK()
}

func cmdSESSIONDEL(ctx *Context) error {
	return cmdSESSIONDELETE(ctx)
}

// SESSION.DELETE id [field] removes the session, or one of its fields.
func cmdSESSIONDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	h, _, err := readSession(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ctx.ArgCount() == 2 {
		field := ctx.ArgString(1)
		if h == nil {
			return ctx.WriteError(errNoSession)
		}
		if field == sessionMarker {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
		locked, err := lockHash(ctx, key)
		if err != nil || locked == nil {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
		had := locked.FieldLive(field, ctx.Store.HashFieldClock())
		if had {
			locked.DeleteField(field)
		}
		locked.Unlock()
		if !had {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
		ctx.notify(store.NotifyHash, "hdel", key)
		ctx.PropagateAs(ExecutedCommand{Name: "HDEL", Args: [][]byte{[]byte(key), []byte(field)}, DB: ctx.DB})
		return ctx.WriteInteger(1)
	}
	if h == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	ctx.Store.Delete(key)
	ctx.notify(store.NotifyGeneric, "del", key)
	ctx.propagateDelete(key)
	return ctx.WriteInteger(1)
}

// SESSION.EXISTS id
func cmdSESSIONEXISTS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	h, _, err := readSession(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if h == nil {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(1)
}

// SESSION.TTL id replies the milliseconds left, -2 for no session.
func cmdSESSIONTTL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	h, entry, err := readSession(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if h == nil {
		return ctx.WriteInteger(-2)
	}
	at := entry.ExpiresAt.Load()
	if at == 0 {
		return ctx.WriteInteger(-1)
	}
	return ctx.WriteInteger(max(at-time.Now().UnixMilli(), 0))
}

// SESSION.REFRESH id ttl_ms gives the session a new deadline, recorded as
// PEXPIREAT.
func cmdSESSIONREFRESH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	ttl, err := parseSessionTTL(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	h, _, err := readSession(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if h == nil {
		return ctx.WriteError(errNoSession)
	}
	when := time.Now().UnixMilli() + ttl
	ctx.Store.SetExpiresAt(key, when)
	ctx.notify(store.NotifyGeneric, "expire", key)
	ctx.propagateExpiry(key, when)
	return ctx.WriteOK()
}

// SESSION.CLEAR id removes the session's data and keeps the session and its
// deadline.
func cmdSESSIONCLEAR(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	h, entry, err := readSession(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if h == nil {
		return ctx.WriteError(errNoSession)
	}
	h.RLock()
	created, _ := strconv.ParseInt(string(h.Field(sessionMarker)), 10, 64) //nolint:errcheck // a damaged time reads as 0
	h.RUnlock()
	if err := writeEmptySession(ctx, key, created, entry.ExpiresAt.Load()); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// SESSION.ALL id replies the session's fields and values.
func cmdSESSIONALL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	h, _, err := readSession(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if h == nil {
		return ctx.WriteNull()
	}
	var out []*resp.Value
	h.RLock()
	h.Range(func(f string, v []byte) bool {
		if f != sessionMarker {
			out = append(out, resp.BulkString(f), resp.BulkBytes(append([]byte(nil), v...)))
		}
		return true
	})
	h.RUnlock()
	return ctx.WriteArray(out)
}

// sessionKeys lists the sessions of the selected database, a walk over all
// its keys like KEYS.
func sessionKeys(ctx *Context) []string {
	var ids []string
	_ = ctx.Store.ForEachLive(func(key string, e *store.Entry) error { //nolint:errcheck // the callback never fails
		if h, ok := e.Value.(*store.HashValue); ok {
			h.RLock()
			_, marked := h.Get(sessionMarker)
			h.RUnlock()
			if marked {
				ids = append(ids, key)
			}
		}
		return nil
	})
	return ids
}

// SESSION.LIST replies the sessions of the selected database.
func cmdSESSIONLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	ids := sessionKeys(ctx)
	out := make([]*resp.Value, len(ids))
	for i, id := range ids {
		out[i] = resp.BulkString(id)
	}
	return ctx.WriteArray(out)
}

// SESSION.COUNT replies how many sessions the selected database holds.
func cmdSESSIONCOUNT(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return ctx.WriteInteger(int64(len(sessionKeys(ctx))))
}

// SESSION.CLEANUP replies 0: an expired session is removed by its key's
// expiry.
func cmdSESSIONCLEANUP(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return ctx.WriteInteger(0)
}
