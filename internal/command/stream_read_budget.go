package command

import (
	"strconv"

	"github.com/cachestorm/cachestorm/internal/store"
)

// xreadBudget is Redis 8.10's MAXCOUNT and MAXSIZE for XREAD and
// XREADGROUP. MAXCOUNT caps the entries over all streams. MAXSIZE stops
// before an entry once the reply written so far holds maxSize bytes, at
// least one entry always going out; like Redis it counts the bytes of
// what it has written, where an array whose length is written last (the
// reply's own and the current stream's entries) counts once it is closed.
type xreadBudget struct {
	maxCount, maxSize int64
	entries, bytes    int64
	resp3, claim      bool
}

func (b *xreadBudget) limited() bool { return b != nil && (b.maxCount > 0 || b.maxSize > 0) }

// exhausted reports the reply may hold no more entries.
func (b *xreadBudget) exhausted() bool {
	if b.maxCount > 0 && b.entries >= b.maxCount {
		return true
	}
	return b.maxSize > 0 && b.entries > 0 && b.bytes >= b.maxSize
}

// streamCount is the entries one stream may add, given COUNT.
func (b *xreadBudget) streamCount(count int64) int64 {
	if b.maxCount <= 0 {
		return count
	}
	if rest := b.maxCount - b.entries; count == 0 || rest < count {
		return rest
	}
	return count
}

func (b *xreadBudget) openStream(key string) {
	if !b.resp3 {
		b.bytes += 4
	}
	b.bytes += respBulkLen(len(key))
}

func (b *xreadBudget) closeStream(n int) { b.bytes += respHeaderLen(int64(n)) }

// admit counts the entry it is given, unless the reply is full.
func (b *xreadBudget) admit(it *store.StreamGroupItem) bool {
	if b.maxSize > 0 && b.entries > 0 && b.bytes >= b.maxSize {
		return false
	}
	b.bytes += b.itemLen(it)
	b.entries++
	return true
}

func (b *xreadBudget) itemLen(it *store.StreamGroupItem) int64 {
	elems := int64(2)
	if b.claim {
		elems = 4
	}
	n := respHeaderLen(elems) + respBulkLen(len(it.ID.String()))
	if it.Entry == nil {
		if b.resp3 {
			return n + 3
		}
		return n + 5
	}
	pairs := it.Entry.FieldPairs()
	n += respHeaderLen(int64(2 * len(pairs)))
	for _, p := range pairs {
		n += respBulkLen(len(p.Name)) + respBulkLen(len(p.Value))
	}
	if b.claim {
		n += respIntLen(it.Idle) + respIntLen(it.Deliveries)
	}
	return n
}

func respHeaderLen(n int64) int64 { return respIntLen(n) }

func respBulkLen(n int) int64 { return int64(len(strconv.Itoa(n))) + 5 + int64(n) }

func respIntLen(n int64) int64 { return int64(len(strconv.FormatInt(n, 10))) + 3 }
