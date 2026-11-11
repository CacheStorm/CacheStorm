package command

import (
	"sort"

	"github.com/cachestorm/cachestorm/internal/store"
)

// A marked hash is a hash key that an extension namespace recognises by a
// marker field it always holds, such as __service or __doc. Its other fields
// are the namespace's records, so it is persisted, replicated and readable
// with HGETALL like any hash, and changes are recorded as HSET, HDEL and DEL.

// readMarkedHash returns the hash at key, nil when the key is missing, and
// WRONGTYPE when it holds anything but a hash carrying marker.
func readMarkedHash(ctx *Context, key, marker string) (*store.HashValue, error) {
	entry, ok := ctx.Store.Get(key)
	if !ok {
		return nil, nil
	}
	h, ok := entry.Value.(*store.HashValue)
	if !ok {
		return nil, store.ErrWrongType
	}
	h.RLock()
	_, marked := h.Get(marker)
	h.RUnlock()
	if !marked {
		return nil, store.ErrWrongType
	}
	return h, nil
}

// markedFields copies the fields of a marked hash other than its marker.
func markedFields(h *store.HashValue, marker string) map[string][]byte {
	h.RLock()
	fields := h.Fields()
	h.RUnlock()
	delete(fields, marker)
	return fields
}

// setMarkedFields sets pairs on the hash at key with its marker, creating
// the key when missing; it is recorded as one HSET.
func setMarkedFields(ctx *Context, key, marker string, pairs ...string) error {
	return updateRecord(ctx, key, append([]string{marker, "1"}, pairs...)...)
}

// deleteMarkedFields removes fields from the marked hash at key, and the key
// itself once only the marker is left. It reports how many it removed and
// records HDEL, or DEL when the key went.
func deleteMarkedFields(ctx *Context, key, marker string, fields ...string) int {
	h, err := lockHash(ctx, key)
	if err != nil || h == nil {
		ctx.PropagateAs()
		return 0
	}
	removed := 0
	var args [][]byte
	for _, f := range fields {
		if _, ok := h.Get(f); ok && f != marker {
			h.DeleteField(f)
			removed++
			args = append(args, []byte(f))
		}
	}
	_, marked := h.Get(marker)
	bare := marked && h.Len() == 1
	h.Unlock()
	switch {
	case removed == 0:
		ctx.PropagateAs()
	case bare:
		ctx.Store.Delete(key)
		ctx.notify(store.NotifyGeneric, "del", key)
		ctx.propagateDelete(key)
	default:
		ctx.notify(store.NotifyHash, "hdel", key)
		ctx.PropagateAs(ExecutedCommand{Name: "HDEL", Args: append([][]byte{[]byte(key)}, args...), DB: ctx.DB})
	}
	return removed
}

// markedKeys returns the keys of the selected database holding a hash
// carrying marker, sorted.
func markedKeys(ctx *Context, marker string) []string {
	var keys []string
	_ = ctx.Store.ForEachLive(func(key string, e *store.Entry) error { //nolint:errcheck // the callback never fails
		if h, ok := e.Value.(*store.HashValue); ok {
			h.RLock()
			_, marked := h.Get(marker)
			h.RUnlock()
			if marked {
				keys = append(keys, key)
			}
		}
		return nil
	})
	sort.Strings(keys)
	return keys
}
