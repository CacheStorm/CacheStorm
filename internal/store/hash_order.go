package store

import (
	"encoding/binary"
	"iter"
	"strings"
	"sync/atomic"
	"unsafe"
)

// Redis keeps a hash of at most hash-max-listpack-entries fields, none of
// them or their values longer than hash-max-listpack-value bytes, in a
// listpack, which lists fields in the order they were added; fields given an
// expiry move to the front, soonest first. A hash that outgrows the listpack
// becomes a hash table for good, and its order is the table's.
//
// HashValue holds a hash of at most hashSmallMax fields and hashSmallBytes
// bytes packed in one buffer (lp), in the listpack's order while the hash
// would be a listpack, and a bigger one as a map, with order keeping the
// listpack order while it would still be one.
//
// The buffer is copy-on-write: a value or field read from it stays valid
// after the hash changes, because every change builds a new buffer.
var (
	hashListpackEntries atomic.Int64
	hashListpackValue   atomic.Int64
)

// hashSmallMax and hashSmallBytes bound the packed form, whose lookups
// are linear, as a listpack's are, and whose changes copy it.
const (
	hashSmallMax   = 128
	hashSmallBytes = 16 << 10
)

// lpEntry locates one field in the packed buffer: the field is
// lp[f:v-hdr], the value lp[vs:end].
type lpEntry struct {
	start, fs, fe, vs, end int
}

func (v *HashValue) lpAt(off int) lpEntry {
	e := lpEntry{start: off}
	n, k := binary.Uvarint(v.lp[off:])
	e.fs = off + k
	e.fe = e.fs + int(n)
	n, k = binary.Uvarint(v.lp[e.fe:])
	e.vs = e.fe + k
	e.end = e.vs + int(n)
	return e
}

func (v *HashValue) lpField(e lpEntry) string {
	return unsafe.String(&v.lp[e.fs], e.fe-e.fs)
}

func (v *HashValue) lpVal(e lpEntry) []byte { return v.lp[e.vs:e.end:e.end] }

func lpAppend(buf []byte, field string, val []byte) []byte {
	buf = binary.AppendUvarint(buf, uint64(len(field)))
	buf = append(buf, field...)
	buf = binary.AppendUvarint(buf, uint64(len(val)))
	return append(buf, val...)
}

// lpEach calls fn on each entry until it returns false.
func (v *HashValue) lpEach(fn func(e lpEntry) bool) {
	for off := 0; off < len(v.lp); {
		e := v.lpAt(off)
		if !fn(e) {
			return
		}
		off = e.end
	}
}

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

// NewHash returns an empty hash sized for n fields.
func NewHash(n int) *HashValue {
	if n > hashSmallMax {
		v := &HashValue{dict: make(map[string][]byte, n)}
		v.adj.Store(hashBaseSize - hashPackedBase)
		return v
	}
	return &HashValue{}
}

// NewHashFrom returns a hash of the fields, in no particular order.
func NewHashFrom(fields map[string][]byte) *HashValue {
	v := NewHash(len(fields))
	for f, val := range fields {
		v.SetFieldKeepTTL(f, val)
	}
	return v
}

// Big reports whether the hash outgrew Redis' listpack encoding. The caller
// holds the value's lock.
func (v *HashValue) Big() bool { return v.big }

// Len is the number of fields. The caller holds the value's lock.
func (v *HashValue) Len() int {
	if v.dict != nil {
		return len(v.dict)
	}
	return int(v.lpN)
}

// Get returns field's value. The caller holds the value's lock.
func (v *HashValue) Get(field string) ([]byte, bool) {
	if v.dict != nil {
		val, ok := v.dict[field]
		return val, ok
	}
	if e, ok := v.find(field); ok {
		return v.lpVal(e), true
	}
	return nil, false
}

// Field returns field's value, nil when it is not set. The caller holds the
// value's lock.
func (v *HashValue) Field(field string) []byte {
	val, _ := v.Get(field)
	return val
}

// Has reports whether field is set. The caller holds the value's lock.
func (v *HashValue) Has(field string) bool {
	_, ok := v.Get(field)
	return ok
}

func (v *HashValue) find(field string) (lpEntry, bool) {
	var found lpEntry
	ok := false
	v.lpEach(func(e lpEntry) bool {
		if e.fe-e.fs == len(field) && string(v.lp[e.fs:e.fe]) == field {
			found, ok = e, true
			return false
		}
		return true
	})
	return found, ok
}

// lpSplice returns a new buffer: lp with lp[from:to] replaced by ins.
func (v *HashValue) lpSplice(from, to int, ins []byte) {
	buf := make([]byte, 0, len(v.lp)-(to-from)+len(ins))
	buf = append(buf, v.lp[:from]...)
	buf = append(buf, ins...)
	v.lp = append(buf, v.lp[to:]...)
}

// Range calls fn for every field, in Redis' order while the hash is small,
// until fn returns false. The caller holds the value's lock.
func (v *HashValue) Range(fn func(field string, val []byte) bool) {
	if v.dict == nil {
		v.lpEach(func(e lpEntry) bool { return fn(v.lpField(e), v.lpVal(e)) })
		return
	}
	if !v.big && len(v.order) == len(v.dict) {
		for _, f := range v.order {
			if !fn(f, v.dict[f]) {
				return
			}
		}
		return
	}
	for f, val := range v.dict {
		if !fn(f, val) {
			return
		}
	}
}

// All yields the fields and values as Range does.
func (v *HashValue) All() iter.Seq2[string, []byte] {
	return func(yield func(string, []byte) bool) { v.Range(yield) }
}

// Dict returns the map of a hash held as one, nil for a packed hash. The
// caller holds the value's lock and does not change the map.
func (v *HashValue) Dict() map[string][]byte { return v.dict }

// Fields returns a copy of the fields and their values.
func (v *HashValue) Fields() map[string][]byte {
	m := make(map[string][]byte, v.Len())
	v.Range(func(f string, val []byte) bool {
		m[f] = val
		return true
	})
	return m
}

// put stores val under field, a new field going last, and returns the old
// value. The caller holds the value's write lock.
func (v *HashValue) put(field string, val []byte) ([]byte, bool) {
	if v.dict != nil {
		old, ok := v.dict[field]
		v.dict[field] = val
		if !ok {
			v.orderAppendLocked(field)
		}
		return old, ok
	}
	if e, ok := v.find(field); ok {
		old := v.lpVal(e)
		// A new buffer, not an overwrite: readers may still hold slices
		// of the old one after they release the lock.
		buf := make([]byte, 0, len(v.lp)-(e.end-e.start)+uvarintLen(len(field))+len(field)+uvarintLen(len(val))+len(val))
		buf = append(buf, v.lp[:e.start]...)
		buf = lpAppend(buf, field, val)
		v.lp = append(buf, v.lp[e.end:]...)
		if len(v.lp) > hashSmallBytes {
			v.toDict()
		}
		return old, true
	}
	// An exact-size copy, as Redis reallocates a listpack: doubling
	// would leave up to half of every small hash unused.
	buf := make([]byte, len(v.lp), len(v.lp)+2*binary.MaxVarintLen32+len(field)+len(val))
	copy(buf, v.lp)
	v.lp = lpAppend(buf, field, val)
	v.lpN++
	v.adj.Add(hashPackedOverhead - hashFieldOverhead)
	if v.lpN > hashSmallMax || len(v.lp) > hashSmallBytes {
		v.toDict()
	}
	v.noteCountLocked()
	return nil, false
}

func (v *HashValue) del(field string) ([]byte, bool) {
	if v.dict != nil {
		old, ok := v.dict[field]
		if ok {
			delete(v.dict, field)
			v.orderRemoveLocked(field)
		}
		return old, ok
	}
	e, ok := v.find(field)
	if !ok {
		return nil, false
	}
	old := v.lpVal(e)
	v.lpSplice(e.start, e.end, nil)
	v.lpN--
	v.adj.Add(hashFieldOverhead - hashPackedOverhead)
	return old, true
}

func (v *HashValue) toDict() {
	v.dict = make(map[string][]byte, v.lpN*2)
	if !v.big {
		v.order = make([]string, 0, v.lpN*2)
	}
	v.lpEach(func(e lpEntry) bool {
		f := string(v.lp[e.fs:e.fe])
		v.dict[f] = v.lpVal(e)
		if !v.big {
			v.order = append(v.order, f)
		}
		return true
	})
	v.lp, v.lpN = nil, 0
	v.adj.Store(hashBaseSize - hashPackedBase)
}

// SetOrder puts the fields, every field of the hash, in that order, for
// code that loaded them in another. The caller holds the value's write lock.
func (v *HashValue) SetOrder(fields []string) {
	if v.dict == nil {
		buf := make([]byte, 0, len(v.lp))
		n := 0
		for _, f := range fields {
			if e, ok := v.find(f); ok {
				buf = append(buf, v.lp[e.start:e.end]...)
				n++
			}
		}
		if n == int(v.lpN) {
			v.lp = buf
		}
	} else if !v.big {
		v.order = append([]string(nil), fields...)
	}
	if int64(len(fields)) > hashListpackEntries.Load() {
		v.convertLocked()
		return
	}
	limit := hashListpackValue.Load()
	for _, f := range fields {
		val, _ := v.Get(f)
		if int64(len(f)) > limit || int64(len(val)) > limit {
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

func (v *HashValue) noteCountLocked() {
	if !v.big && int64(v.Len()) > hashListpackEntries.Load() {
		v.convertLocked()
	}
}

func (v *HashValue) orderAppendLocked(field string) {
	if v.big {
		return
	}
	v.order = append(v.order, field)
	v.noteCountLocked()
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
	if v.dict == nil {
		e, ok := v.find(field)
		if !ok {
			return
		}
		entry := v.lp[e.start:e.end]
		rest := make([]byte, 0, len(v.lp))
		rest = append(rest, v.lp[:e.start]...)
		rest = append(rest, v.lp[e.end:]...)
		v.lp = rest
		pos := len(v.lp)
		if at != 0 {
			v.lpEach(func(o lpEntry) bool {
				if t := v.FieldExpireAt(v.lpField(o)); t == 0 || t >= at {
					pos = o.start
					return false
				}
				return true
			})
		}
		buf := make([]byte, 0, len(v.lp)+len(entry))
		buf = append(buf, v.lp[:pos]...)
		buf = append(buf, entry...)
		v.lp = append(buf, v.lp[pos:]...)
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

// NewLoadedHash builds a hash from fields and values loaded in order, items
// alternating them, in one pass. It copies them, so they may share the
// loader's input. Setting them one at a time searched and copied the packed
// buffer for each field, quadratic in the fields, which made the small
// hashes of a snapshot slower to load than everything else in it. It
// reports false when a field repeats.
func NewLoadedHash(items []string) (*HashValue, bool) {
	n := len(items) / 2
	if !uniqueFields(items) {
		return nil, false
	}
	limit := hashListpackValue.Load()
	tooLong := false
	var size int64
	packed := 0
	for i := 0; i+1 < len(items); i += 2 {
		f, val := items[i], items[i+1]
		size += int64(len(f)) + int64(len(val)) + hashFieldOverhead
		packed += uvarintLen(len(f)) + len(f) + uvarintLen(len(val)) + len(val)
		if int64(len(f)) > limit || int64(len(val)) > limit {
			tooLong = true
		}
	}
	v := &HashValue{}
	if n <= hashSmallMax && packed <= hashSmallBytes {
		buf := make([]byte, 0, packed)
		for i := 0; i+1 < len(items); i += 2 {
			buf = binary.AppendUvarint(buf, uint64(len(items[i])))
			buf = append(buf, items[i]...)
			buf = binary.AppendUvarint(buf, uint64(len(items[i+1])))
			buf = append(buf, items[i+1]...)
		}
		v.lp, v.lpN = buf, int32(n)
		v.adj.Store(int64(n) * (hashPackedOverhead - hashFieldOverhead))
	} else {
		v.dict = make(map[string][]byte, n)
		v.order = make([]string, 0, n)
		for i := 0; i+1 < len(items); i += 2 {
			// The loader hands over strings sharing its input's bytes.
			f := strings.Clone(items[i])
			v.dict[f] = []byte(items[i+1])
			v.order = append(v.order, f)
		}
		v.adj.Store(hashBaseSize - hashPackedBase)
	}
	v.size.set(size)
	if tooLong || int64(n) > hashListpackEntries.Load() {
		v.convertLocked()
	}
	return v, true
}

// uniqueFields reports whether no field of items, which alternates fields
// and values, repeats.
func uniqueFields(items []string) bool {
	n := len(items) / 2
	if n <= 16 {
		for i := 0; i < len(items); i += 2 {
			for j := i + 2; j < len(items); j += 2 {
				if items[i] == items[j] {
					return false
				}
			}
		}
		return true
	}
	seen := make(map[string]struct{}, n)
	for i := 0; i < len(items); i += 2 {
		if _, ok := seen[items[i]]; ok {
			return false
		}
		seen[items[i]] = struct{}{}
	}
	return true
}
