package store

import "sync/atomic"

// Byte estimates for the collection types. SizeOf of a collection is its base
// plus the sum of its elements' sizes. The figures were measured from the live
// heap these structures take (Go map groups, order slices, skiplist nodes, the
// packed buffers of small hashes, sets and sorted sets), so used_memory and
// maxmemory track what the process really holds: within about a fifth for
// collections of one to two hundred elements. The size counter of a hash, set
// or sorted set counts at the map form's rates and adj corrects it to the form
// the collection is in.
const (
	listBaseSize      = 250
	listElemOverhead  = 4
	hashBaseSize      = 450
	hashFieldOverhead = 88
	setBaseSize       = 337
	setMemberOverhead = 45
	zsetBaseSize      = 930
	zsetMemberOverhd  = 122
	streamBaseSize    = 48
	// The compact forms: a packed hash or listpack set costs its struct
	// and a few bytes per element besides the bytes themselves, an intset
	// eight bytes per member; adj holds the difference from the map form.
	hashPackedBase     = 152
	hashPackedOverhead = 4
	setCompactBase     = 160
	setListpackOverhd  = 3
	setIntsetElem      = 10
	zsetPackedBase     = 128
	zsetPackedOverhd   = 11
	streamEntryOverhd  = 32
	streamPairOverhd   = 80
)

func listElemSize(el []byte) int64 { return int64(len(el)) + listElemOverhead }

func hashFieldSize(field string, val []byte) int64 {
	return int64(len(field)) + int64(len(val)) + hashFieldOverhead
}

func setMemberSize(member string) int64 { return int64(len(member)) + setMemberOverhead }

func zsetMemberSize(member string) int64 { return int64(len(member)) + zsetMemberOverhd }

func streamEntrySize(e *StreamEntry) int64 {
	size := int64(len(e.ID)) + streamEntryOverhd
	for _, p := range e.Pairs {
		size += int64(len(p.Name)) + int64(len(p.Value)) + streamPairOverhd
	}
	return size
}

// sizeCounter is the incrementally maintained payload size of a collection:
// the sum of its elements' sizes, without the base. A value built from a
// struct literal starts unknown and is counted once, on the first SizeOf;
// from then on every mutation adjusts it, so SizeOf is O(1). Mutations made
// while it is unknown need no adjustment because the first count sees them.
// Writers update it under the value's write lock; SizeOf reads it without
// the lock.
type sizeCounter struct {
	bytes atomic.Int64
	known atomic.Bool
}

// add applies a delta made by a mutation. The caller holds the value's write
// lock (or owns a value that is not yet shared).
func (c *sizeCounter) add(delta int64) {
	if delta != 0 && c.known.Load() {
		c.bytes.Add(delta)
	}
}

// set records a full count. The caller holds the value's lock, read or
// write: concurrent readers counting under the read lock store equal values.
func (c *sizeCounter) set(n int64) {
	c.bytes.Store(n)
	c.known.Store(true)
}

// load returns the payload size and whether it is known.
func (c *sizeCounter) load() (int64, bool) {
	if !c.known.Load() {
		return 0, false
	}
	return c.bytes.Load(), true
}

// RecountSize is what SizeOf maintains, counted from scratch for the form
// the hash is in. The caller holds the value's lock.
func (v *HashValue) RecountSize() int64 {
	n := v.ttlSizeLocked()
	per, base := int64(hashFieldOverhead), int64(hashBaseSize)
	if v.dict == nil {
		per, base = hashPackedOverhead, hashPackedBase
	}
	v.Range(func(f string, val []byte) bool {
		n += int64(len(f)) + int64(len(val)) + per
		return true
	})
	return base + n
}

// RecountSize is what SizeOf maintains, counted from scratch for the form
// the set is in. The caller holds the value's lock.
func (v *SetValue) RecountSize() int64 {
	switch {
	case v.dict != nil:
		n := int64(setBaseSize)
		for m := range v.dict {
			n += setMemberSize(m)
		}
		return n
	case v.enc == setEncIntset:
		return setCompactBase + int64(len(v.ints))*setIntsetElem
	}
	n := int64(setCompactBase)
	v.Range(func(m string) bool {
		n += int64(len(m)) + setListpackOverhd
		return true
	})
	return n
}

// RecountSize is what SizeOf maintains, counted from scratch for the form
// the set is in. The caller holds the value's lock.
func (v *SortedSetValue) RecountSize() int64 {
	per, base := int64(zsetMemberOverhd), int64(zsetBaseSize)
	if v.dict == nil {
		per, base = zsetPackedOverhd, zsetPackedBase
	}
	n := base
	for m := range v.All() {
		n += int64(len(m)) + per
	}
	return n
}

// fieldCost is what a field adds to SizeOf in the hash's current form, for
// a write's admission estimate. The caller holds the value's lock.
func (v *HashValue) fieldCost(field string, val []byte) int64 {
	if v.dict == nil {
		return int64(len(field)) + int64(len(val)) + hashPackedOverhead
	}
	return hashFieldSize(field, val)
}

// memberCost is what a member adds to SizeOf in the set's current form.
// The caller holds the value's lock.
func (v *SetValue) memberCost(member string) int64 {
	if v.dict != nil {
		return setMemberSize(member)
	}
	if _, ok := redisSetInt(member); ok && v.enc == setEncIntset {
		return setIntsetElem
	}
	if int64(len(member)) > setMaxListpackValue.Load() || int64(v.Card()) >= min(setMaxListpackEntries.Load(), setLpMax) {
		// The member moves the set to its map form.
		return setMemberSize(member) + setBaseSize - setCompactBase + int64(v.Card())*(setMemberOverhead-setListpackOverhd)
	}
	return int64(len(member)) + setListpackOverhd
}
