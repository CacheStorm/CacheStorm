package persistence

import (
	"bufio"
	"encoding/binary"
	"sort"
	"strconv"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

// rdbTypeHashTTL is the snapshot type of a hash whose fields have expiries:
// a length, then for every field its name, its value and its expiry as a
// little-endian int64 of unix milliseconds (0 for none). A hash without
// field expiries keeps type 3, so older snapshots load unchanged.
const rdbTypeHashTTL = 24

// hashSnapshot is a hash's fields that are not expired, with their expiries
// (0 for none), in field order.
type hashSnapshot struct {
	fields []string
	values [][]byte
	ttls   []int64
	anyTTL bool
}

// hashFieldOrder returns the fields of h in the order they are saved: Redis'
// listpack order for a small hash, sorted for a hash table, whose own order
// is arbitrary. The caller holds h's lock.
func hashFieldOrder(h *store.HashValue) []string {
	fields := make([]string, 0, len(h.Fields))
	h.Range(func(f string, _ []byte) bool {
		fields = append(fields, f)
		return true
	})
	if h.Big() {
		sort.Strings(fields)
	}
	return fields
}

func snapshotHash(h *store.HashValue) hashSnapshot {
	now := time.Now().UnixMilli()
	h.RLock()
	defer h.RUnlock()
	var snap hashSnapshot
	snap.fields = make([]string, 0, len(h.Fields))
	for _, f := range hashFieldOrder(h) {
		if at := h.FieldExpireAt(f); at == 0 || at > now {
			snap.fields = append(snap.fields, f)
		}
	}
	snap.values = make([][]byte, len(snap.fields))
	snap.ttls = make([]int64, len(snap.fields))
	for i, f := range snap.fields {
		snap.values[i] = h.Fields[f]
		snap.ttls[i] = h.FieldExpireAt(f)
		if snap.ttls[i] != 0 {
			snap.anyTTL = true
		}
	}
	return snap
}

// hashLive reports whether v is not a hash whose every field has expired.
func hashLive(v store.Value) bool {
	h, ok := v.(*store.HashValue)
	if !ok || !h.HasFieldTTLs() {
		return true
	}
	return len(snapshotHash(h).fields) > 0
}

// newHashWithTTLs builds a hash from decoded fields, dropping those whose
// expiry has passed. It returns nil when none is left.
func newHashWithTTLs(fields []string, values [][]byte, ttls []int64) *store.HashValue {
	now := time.Now().UnixMilli()
	h := &store.HashValue{Fields: make(map[string][]byte, len(fields))}
	order := make([]string, 0, len(fields))
	for i, f := range fields {
		if ttls[i] != 0 && ttls[i] <= now {
			continue
		}
		h.Fields[f] = values[i]
		order = append(order, f)
	}
	if len(h.Fields) == 0 {
		return nil
	}
	for i, f := range fields {
		if ttls[i] > now {
			h.SetFieldExpireAt(f, ttls[i])
		}
	}
	// The fields keep the order they were saved in.
	h.SetOrder(order)
	return h
}

// writeHashTTLs emits HPEXPIREAT commands that restore the field expiries of
// snap, one per expiry time.
func (rw *AOFRewriter) writeHashTTLs(w *bufio.Writer, key string, snap hashSnapshot) error {
	if !snap.anyTTL {
		return nil
	}
	byTime := make(map[int64][]string)
	var times []int64
	for i, at := range snap.ttls {
		if at == 0 {
			continue
		}
		if _, ok := byTime[at]; !ok {
			times = append(times, at)
		}
		byTime[at] = append(byTime[at], snap.fields[i])
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	for _, at := range times {
		fields := byTime[at]
		for start := 0; start < len(fields); start += rewriteItemsPerCommand {
			end := start + rewriteItemsPerCommand
			if end > len(fields) {
				end = len(fields)
			}
			args := []string{key, strconv.FormatInt(at, 10), "FIELDS", strconv.Itoa(end - start)}
			args = append(args, fields[start:end]...)
			if err := rw.writeCommand(w, "HPEXPIREAT", args...); err != nil {
				return err
			}
		}
	}
	return nil
}

// Redis 7.4's RDB types for a hash with field expiries: a hash table with a
// per-field TTL (RDB_TYPE_HASH_METADATA) and a listpack of field, value,
// expiry triplets (RDB_TYPE_HASH_LISTPACK_EX). Both start with the hash's
// earliest expiry.
const (
	dumpTypeHashMetadata   = 24
	dumpTypeHashListpackEx = 25
)

// appendDumpHashTTL encodes snap as RDB_TYPE_HASH_METADATA: the minimum
// expiry, the field count, then per field its expiry as a length (expiry -
// minimum + 1, 0 for none), its name and its value.
func appendDumpHashTTL(out []byte, snap hashSnapshot) []byte {
	var minAt int64
	for _, at := range snap.ttls {
		if at != 0 && (minAt == 0 || at < minAt) {
			minAt = at
		}
	}
	out = append(out, dumpTypeHashMetadata)
	out = binary.LittleEndian.AppendUint64(out, uint64(minAt))
	out = appendDumpLength(out, uint64(len(snap.fields)))
	for i, f := range snap.fields {
		ttl := uint64(0)
		if snap.ttls[i] != 0 {
			ttl = uint64(snap.ttls[i]-minAt) + 1
		}
		out = appendDumpLength(out, ttl)
		out = appendDumpLength(out, uint64(len(f)))
		out = append(out, f...)
		out = appendDumpLength(out, uint64(len(snap.values[i])))
		out = append(out, snap.values[i]...)
	}
	return out
}

// hashWithTTLs decodes RDB_TYPE_HASH_METADATA and RDB_TYPE_HASH_LISTPACK_EX.
// Fields already expired are dropped; a hash left empty is refused.
func (d *dumpDecoder) hashWithTTLs(typ byte) store.Value {
	b := d.take(8)
	if len(b) != 8 {
		d.err = ErrDumpFormat
		return nil
	}
	minAt := int64(binary.LittleEndian.Uint64(b))
	var fields []string
	var values [][]byte
	var ttls []int64
	seen := make(map[string]bool)
	add := func(f, v string, at int64) {
		if seen[f] || at < 0 || at > store.MaxHashFieldExpireMS {
			d.err = ErrDumpFormat
			return
		}
		seen[f] = true
		fields = append(fields, f)
		values = append(values, []byte(v))
		ttls = append(ttls, at)
	}
	if typ == dumpTypeHashMetadata {
		n := d.count()
		for i := 0; i < n && d.err == nil; i++ {
			ttl, enc := d.length()
			if enc {
				d.err = ErrDumpFormat
				break
			}
			f := d.str()
			v := d.str()
			at := int64(0)
			if ttl != 0 {
				if ttl > uint64(store.MaxHashFieldExpireMS) {
					d.err = ErrDumpFormat
					break
				}
				at = int64(ttl) + minAt - 1
			}
			add(f, v, at)
		}
		if n == 0 {
			d.err = ErrDumpFormat
		}
	} else {
		items, err := decodeDumpListpack([]byte(d.str()))
		if err != nil || len(items) == 0 || len(items)%3 != 0 {
			d.err = ErrDumpFormat
			return nil
		}
		for i := 0; i < len(items) && d.err == nil; i += 3 {
			at, err := strconv.ParseInt(items[i+2], 10, 64)
			if err != nil {
				d.err = ErrDumpFormat
				break
			}
			add(items[i], items[i+1], at)
		}
	}
	if d.err != nil {
		return nil
	}
	h := newHashWithTTLs(fields, values, ttls)
	if h == nil {
		d.err = ErrDumpFormat
		return nil
	}
	return h
}
