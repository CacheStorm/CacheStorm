package store

import "sync/atomic"

// Redis keeps a hash of at most hash-max-listpack-entries fields, none of
// them or their values longer than hash-max-listpack-value bytes, in a
// listpack, which lists fields in the order they were added; fields given an
// expiry move to the front, soonest first. A hash that outgrows the listpack
// becomes a hash table for good, and its order is the table's. HashValue
// keeps the listpack order in order while the hash would be a listpack.
var (
	hashListpackEntries atomic.Int64
	hashListpackValue   atomic.Int64
)

func init() {
	hashListpackEntries.Store(512)
	hashListpackValue.Store(64)
}

// SetHashListpackLimits sets hash-max-listpack-entries and
// hash-max-listpack-value.
func SetHashListpackLimits(entries, value int64) {
	hashListpackEntries.Store(entries)
	hashListpackValue.Store(value)
}

// Big reports whether the hash outgrew Redis' listpack encoding. The caller
// holds the value's lock.
func (v *HashValue) Big() bool { return v.big }

// Range calls fn for every field, in Redis' order while the hash is small,
// until fn returns false. The caller holds the value's lock.
func (v *HashValue) Range(fn func(field string, val []byte) bool) {
	if !v.big && len(v.order) == len(v.Fields) {
		for _, f := range v.order {
			if !fn(f, v.Fields[f]) {
				return
			}
		}
		return
	}
	for f, val := range v.Fields {
		if !fn(f, val) {
			return
		}
	}
}

// SetOrder records fields, every field of the hash, as its order, for code
// that filled Fields directly. The caller holds the value's write lock.
func (v *HashValue) SetOrder(fields []string) {
	v.order = append([]string(nil), fields...)
	if int64(len(v.order)) > hashListpackEntries.Load() {
		v.convertLocked()
		return
	}
	limit := hashListpackValue.Load()
	for _, f := range v.order {
		if int64(len(f)) > limit || int64(len(v.Fields[f])) > limit {
			v.convertLocked()
			return
		}
	}
}

func (v *HashValue) convertLocked() {
	v.big = true
	v.order = nil
}

// noteLengthsLocked converts the hash when field or val is too long for the
// listpack.
func (v *HashValue) noteLengthsLocked(field string, val []byte) {
	if limit := hashListpackValue.Load(); !v.big && (int64(len(field)) > limit || int64(len(val)) > limit) {
		v.convertLocked()
	}
}

func (v *HashValue) orderAppendLocked(field string) {
	if v.big {
		return
	}
	v.order = append(v.order, field)
	if int64(len(v.order)) > hashListpackEntries.Load() {
		v.convertLocked()
	}
}

func (v *HashValue) orderRemoveLocked(field string) {
	if v.big {
		return
	}
	for i, f := range v.order {
		if f == field {
			v.order = append(v.order[:i:i], v.order[i+1:]...)
			return
		}
	}
}

// orderPlaceLocked moves field to where the listpack keeps a field expiring
// at at: before the first field that expires no sooner or never, or last
// when at is 0.
func (v *HashValue) orderPlaceLocked(field string, at int64) {
	if v.big {
		return
	}
	v.orderRemoveLocked(field)
	if at == 0 {
		v.order = append(v.order, field)
		return
	}
	i := 0
	for ; i < len(v.order); i++ {
		if t := v.FieldExpireAt(v.order[i]); t == 0 || t >= at {
			break
		}
	}
	v.order = append(v.order, "")
	copy(v.order[i+1:], v.order[i:])
	v.order[i] = field
}
