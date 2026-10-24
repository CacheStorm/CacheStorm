package store

import "sync/atomic"

// Byte estimates for the collection types. SizeOf of a collection is its base
// plus the sum of its elements' sizes; the per-element figures are what
// SizeOf has always charged.
const (
	listBaseSize      = 24
	listElemOverhead  = 24
	hashBaseSize      = 48
	hashFieldOverhead = 80
	setBaseSize       = 48
	setMemberOverhead = 48
	zsetBaseSize      = 48
	zsetMemberOverhd  = 16 + 80
	streamBaseSize    = 48
	streamEntryOverhd = 32
	streamPairOverhd  = 80
)

func listElemSize(el []byte) int64 { return int64(len(el)) + listElemOverhead }

func hashFieldSize(field string, val []byte) int64 {
	return int64(len(field)) + int64(len(val)) + hashFieldOverhead
}

func setMemberSize(member string) int64 { return int64(len(member)) + setMemberOverhead }

func zsetMemberSize(member string) int64 { return int64(len(member)) + zsetMemberOverhd }

func streamEntrySize(e *StreamEntry) int64 {
	size := int64(len(e.ID)) + streamEntryOverhd
	if e.Pairs != nil {
		for _, p := range e.Pairs {
			size += int64(len(p.Name)) + int64(len(p.Value)) + streamPairOverhd
		}
		return size
	}
	for k, val := range e.Fields {
		size += int64(len(k)) + int64(len(val)) + streamPairOverhd
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
