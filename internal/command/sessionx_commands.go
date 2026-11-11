package command

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A SESSIONX session is a hash key named by its generated id that expires
// with the session. It holds the user and the creation time in the fields
// __sessionx_user and __sessionx_created beside the session's data. Being a
// key, a session is per database, persisted with its absolute deadline and
// replicated; changes are recorded as HSET and PEXPIREAT.

const (
	sessionXUser    = "__sessionx_user"
	sessionXCreated = "__sessionx_created"
)

var (
	errSessionXNotFound = errors.New("ERR session not found")
	errSessionXField    = errors.New("ERR field names starting with __sessionx_ are reserved")
)

func isSessionX(h *store.HashValue) bool {
	_, u := h.Get(sessionXUser)
	_, c := h.Get(sessionXCreated)
	return u && c
}

// readSessionX returns the session at id, nil when there is none.
func readSessionX(ctx *Context, id string) (*store.HashValue, *store.Entry, error) {
	entry, ok := ctx.Store.Get(id)
	if !ok {
		return nil, nil, nil
	}
	h, ok := entry.Value.(*store.HashValue)
	if !ok {
		return nil, nil, store.ErrWrongType
	}
	h.RLock()
	marked := isSessionX(h)
	h.RUnlock()
	if !marked {
		return nil, nil, store.ErrWrongType
	}
	return h, entry, nil
}

// SESSIONX.CREATE user ttl_ms creates a session and replies its id.
func cmdSESSIONXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ttl, err := parseSessionTTL(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(errors.New("ERR ttl must be a positive integer (milliseconds)"))
	}
	id := generateUUID()
	for ctx.Store.Exists(id) {
		id = generateUUID()
	}
	now := ctx.nowMS()
	pairs := []string{sessionXUser, ctx.ArgString(0), sessionXCreated, strconv.FormatInt(now, 10)}
	h, _ := store.NewLoadedHash(pairs)
	if err := ctx.Store.Set(id, h, store.SetOptions{ExpireAtMS: now + ttl}); err != nil {
		return ctx.WriteError(err)
	}
	ctx.notify(store.NotifyHash, "hset", id)
	ctx.PropagateAs(
		ExecutedCommand{Name: "HSET", Args: pairArgs(id, pairs), DB: ctx.DB},
		ExecutedCommand{Name: "PEXPIREAT", Args: [][]byte{[]byte(id), strconv.AppendInt(nil, now+ttl, 10)}, DB: ctx.DB},
	)
	return ctx.WriteBulkString(id)
}

func cmdSESSIONXDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	h, _, err := readSessionX(ctx, id)
	if err != nil {
		return ctx.WriteError(err)
	}
	if h == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	ctx.Store.Delete(id)
	ctx.notify(store.NotifyGeneric, "del", id)
	ctx.propagateDelete(id)
	return ctx.WriteInteger(1)
}

func cmdSESSIONXGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	h, entry, err := readSessionX(ctx, id)
	if err != nil {
		return ctx.WriteError(err)
	}
	if h == nil {
		return ctx.WriteError(errSessionXNotFound)
	}
	h.RLock()
	user, _ := h.Get(sessionXUser)
	created, perr := strconv.ParseInt(string(h.Field(sessionXCreated)), 10, 64)
	fields := h.Fields()
	h.RUnlock()
	if perr != nil {
		return ctx.WriteError(store.ErrWrongType)
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if !strings.HasPrefix(k, "__sessionx_") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	data := make([]*resp.Value, 0, 2*len(keys))
	for _, k := range keys {
		data = append(data, resp.BulkString(k), resp.BulkBytes(fields[k]))
	}
	expiresAt := entry.ExpiresAt.Load()
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(id),
		resp.BulkString("user"), resp.BulkString(string(user)),
		resp.BulkString("created_at"), resp.IntegerValue(created),
		resp.BulkString("expires_at"), resp.IntegerValue(expiresAt),
		resp.BulkString("ttl"), resp.IntegerValue(max(expiresAt-ctx.nowMS(), 0)),
		resp.BulkString("data"), resp.ArrayValue(data),
	})
}

// SESSIONX.SET id field value [field value ...]
func cmdSESSIONXSET(ctx *Context) error {
	if ctx.ArgCount() < 3 || (ctx.ArgCount()-1)%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	for i := 1; i < ctx.ArgCount(); i += 2 {
		if strings.HasPrefix(ctx.ArgString(i), "__sessionx_") {
			return ctx.WriteError(errSessionXField)
		}
	}
	h, _, err := readSessionX(ctx, id)
	if err != nil {
		return ctx.WriteError(err)
	}
	if h == nil {
		return ctx.WriteError(errSessionXNotFound)
	}
	if _, err := ctx.Store.ApplyHashPairs(id, false, ctx.Args[1:]); err != nil {
		return ctx.WriteError(err)
	}
	ctx.notify(store.NotifyHash, "hset", id)
	ctx.PropagateAs(ExecutedCommand{Name: "HSET", Args: ctx.Args, DB: ctx.DB})
	return ctx.WriteOK()
}

// SESSIONX.LIST replies the session ids of the selected database, sorted.
func cmdSESSIONXLIST(ctx *Context) error {
	var ids []string
	_ = ctx.Store.ForEachLive(func(key string, e *store.Entry) error { //nolint:errcheck // the callback never fails
		if h, ok := e.Value.(*store.HashValue); ok {
			h.RLock()
			marked := isSessionX(h)
			h.RUnlock()
			if marked {
				ids = append(ids, key)
			}
		}
		return nil
	})
	sort.Strings(ids)
	out := make([]*resp.Value, len(ids))
	for i, id := range ids {
		out[i] = resp.BulkString(id)
	}
	return ctx.WriteArray(out)
}
