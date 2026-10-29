package store

import "sync/atomic"

// Redis keeps a sorted set of at most zset-max-listpack-entries members, none
// longer than zset-max-listpack-value bytes, in a listpack, and converts one
// that outgrows it to a skiplist for good; a set built as a command's result
// is a listpack when it fits.
var (
	zsetListpackEntries atomic.Int64
	zsetListpackValue   atomic.Int64
)

func init() {
	zsetListpackEntries.Store(128)
	zsetListpackValue.Store(64)
}

// SetZsetListpackLimits sets zset-max-listpack-entries and
// zset-max-listpack-value.
func SetZsetListpackLimits(entries, value int64) {
	zsetListpackEntries.Store(entries)
	zsetListpackValue.Store(value)
}

// ZsetListpackLimits returns zset-max-listpack-entries and
// zset-max-listpack-value.
func ZsetListpackLimits() (entries, value int64) {
	return zsetListpackEntries.Load(), zsetListpackValue.Load()
}

// Big reports whether the sorted set outgrew Redis' listpack encoding. The
// caller holds the value's lock.
func (v *SortedSetValue) Big() bool { return v.big }

// Skiplist reports whether Redis holds the set as a skiplist, which walks a
// lexicographic range by rank instead of in order. The caller holds the
// value's lock.
func (v *SortedSetValue) Skiplist() bool {
	return v.big || int64(len(v.Members)) > zsetListpackEntries.Load()
}

// NoteAdd is ZADD's zsetTypeCreate and zsetTypeMaybeConvert: a command adding
// n members makes the set a skiplist when they could not all fit a listpack,
// whatever it ends up adding, and so does a member of maxLen bytes, which
// ZADD counts only when it creates the set. The caller holds the value's
// write lock.
func (v *SortedSetValue) NoteAdd(n, maxLen int) {
	v.noteAddLocked(n, maxLen)
}

func (v *SortedSetValue) noteAddLocked(n, maxLen int) {
	if int64(n) > zsetListpackEntries.Load() || int64(maxLen) > zsetListpackValue.Load() {
		v.big = true
	}
}
