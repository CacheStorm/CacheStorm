package store

import "encoding/binary"

// A list keeps its elements packed in chunks of at most listChunkBytes, as
// Redis' quicklist keeps listpacks. An entry is its length as an uvarint,
// the bytes, and the length again written backwards, so a chunk can be
// walked from either end.
//
// Bytes once written to a chunk's buffer are never written again: popping
// moves the chunk's live window, pushing appends past the highest byte ever
// written or prepends before the lowest, and any other change builds a new
// buffer. An element handed out as a slice of a buffer therefore stays
// valid after the list changes, as the elements of the slice it replaces
// did.
const listChunkBytes = 4096

type listChunk struct {
	buf []byte
	// lo and hi bound the live entries; wlo and whi the bytes ever written.
	lo, hi   int
	wlo, whi int
	n        int
}

func uvarintLen(x int) int {
	n := 1
	for x >= 0x80 {
		x >>= 7
		n++
	}
	return n
}

func listEntrySize(n int) int { return 2*uvarintLen(n) + n }

// putListEntry writes an entry for el at buf[off:], which has room for it.
func putListEntry(buf []byte, off int, el []byte) {
	off += binary.PutUvarint(buf[off:], uint64(len(el)))
	off += copy(buf[off:], el)
	x := uint64(len(el))
	k := uvarintLen(len(el))
	for i := k - 1; i >= 0; i-- {
		b := byte(x & 0x7f)
		if i != 0 {
			b |= 0x80
		}
		buf[off+i] = b
		x >>= 7
	}
}

// entryAt returns the bounds of the entry starting at off: its bytes
// buf[ds:de] and its end.
func (c *listChunk) entryAt(off int) (ds, de, end int) {
	n, k := binary.Uvarint(c.buf[off:])
	ds = off + k
	de = ds + int(n)
	return ds, de, de + k
}

// entryBefore returns the bounds of the entry ending at end: its start and
// its bytes buf[ds:de].
func (c *listChunk) entryBefore(end int) (start, ds, de int) {
	var x uint64
	shift, k := 0, 0
	for i := end - 1; ; i-- {
		b := c.buf[i]
		x |= uint64(b&0x7f) << shift
		shift += 7
		k++
		if b&0x80 == 0 {
			break
		}
	}
	de = end - k
	ds = de - int(x)
	return ds - k, ds, de
}

func (c *listChunk) elem(ds, de int) []byte { return c.buf[ds:de:de] }

func (c *listChunk) live() int { return c.hi - c.lo }

// newListChunk returns a chunk with room for need bytes, placed at the
// front of its buffer for appending or at the back for prepending.
func newListChunk(need int, front bool) *listChunk {
	size := need
	if size < listChunkBytes {
		size = max(min(2*need, listChunkBytes), 32)
	}
	c := &listChunk{buf: make([]byte, size)}
	if front {
		c.lo, c.hi, c.wlo, c.whi = size, size, size, size
	}
	return c
}

// grown is c in a new buffer with room for need more bytes after its live
// entries (or before them, with front).
func (c *listChunk) grown(need int, front bool) *listChunk {
	// A quarter more than needed: amortised constant copying, with less
	// slack than doubling would leave in every small list.
	live := c.live()
	size := max(min((live+need)*5/4+16, listChunkBytes), live+need)
	nc := &listChunk{buf: make([]byte, size), n: c.n}
	if front {
		nc.lo = size - live
	}
	nc.hi = nc.lo + live
	copy(nc.buf[nc.lo:], c.buf[c.lo:c.hi])
	nc.wlo, nc.whi = nc.lo, nc.hi
	return nc
}

// listChunks is the element storage of a ListValue.
type listChunks struct {
	chunks []*listChunk
	count  int
}

func (l *listChunks) pushBack(el []byte) {
	need := listEntrySize(len(el))
	var c *listChunk
	if k := len(l.chunks); k > 0 {
		c = l.chunks[k-1]
		if c.live()+need > listChunkBytes && c.n > 0 {
			c = nil
		} else if c.hi != c.whi || c.hi+need > len(c.buf) {
			c = c.grown(need, false)
			l.chunks[k-1] = c
		}
	}
	if c == nil {
		c = newListChunk(need, false)
		l.chunks = append(l.chunks, c)
	}
	putListEntry(c.buf, c.hi, el)
	c.hi += need
	c.whi = c.hi
	c.n++
	l.count++
}

func (l *listChunks) pushFront(el []byte) {
	need := listEntrySize(len(el))
	var c *listChunk
	if len(l.chunks) > 0 {
		c = l.chunks[0]
		if c.live()+need > listChunkBytes && c.n > 0 {
			c = nil
		} else if c.lo != c.wlo || c.lo < need {
			c = c.grown(need, true)
			l.chunks[0] = c
		}
	}
	if c == nil {
		c = newListChunk(need, true)
		l.chunks = append(l.chunks, nil)
		copy(l.chunks[1:], l.chunks)
		l.chunks[0] = c
	}
	c.lo -= need
	putListEntry(c.buf, c.lo, el)
	c.wlo = c.lo
	c.n++
	l.count++
}

func (l *listChunks) popFront() []byte {
	c := l.chunks[0]
	ds, de, end := c.entryAt(c.lo)
	el := c.elem(ds, de)
	c.lo = end
	c.n--
	l.count--
	if c.n == 0 {
		l.chunks[0] = nil
		l.chunks = l.chunks[1:]
	}
	return el
}

func (l *listChunks) popBack() []byte {
	k := len(l.chunks) - 1
	c := l.chunks[k]
	start, ds, de := c.entryBefore(c.hi)
	el := c.elem(ds, de)
	c.hi = start
	c.n--
	l.count--
	if c.n == 0 {
		l.chunks[k] = nil
		l.chunks = l.chunks[:k]
	}
	return el
}

// locate returns the chunk holding element i and i's offset in it.
func (l *listChunks) locate(i int) (int, int) {
	if i < l.count/2 {
		for ci, c := range l.chunks {
			if i < c.n {
				return ci, i
			}
			i -= c.n
		}
	}
	j := l.count - 1 - i
	for ci := len(l.chunks) - 1; ci >= 0; ci-- {
		c := l.chunks[ci]
		if j < c.n {
			return ci, c.n - 1 - j
		}
		j -= c.n
	}
	return -1, -1
}

// entry returns the bounds of the off-th entry of chunk c.
func (c *listChunk) entry(off int) (start, ds, de, end int) {
	if off < c.n/2 {
		start = c.lo
		for {
			ds, de, end = c.entryAt(start)
			if off == 0 {
				return start, ds, de, end
			}
			start = end
			off--
		}
	}
	end = c.hi
	for j := c.n - 1; ; j-- {
		start, ds, de = c.entryBefore(end)
		if j == off {
			return start, ds, de, end
		}
		end = start
	}
}

func (l *listChunks) at(i int) []byte {
	ci, off := l.locate(i)
	c := l.chunks[ci]
	_, ds, de, _ := c.entry(off)
	return c.elem(ds, de)
}

// each calls fn on elements from index from on, or backwards with
// reverse, until fn returns false.
func (l *listChunks) each(from int, reverse bool, fn func(i int, el []byte) bool) {
	if from < 0 || from >= l.count {
		return
	}
	ci, off := l.locate(from)
	i := from
	if reverse {
		for ; ci >= 0; ci-- {
			c := l.chunks[ci]
			_, _, _, end := c.entry(off)
			for k := off; k >= 0; k-- {
				start, ds, de := c.entryBefore(end)
				if !fn(i, c.elem(ds, de)) {
					return
				}
				i--
				end = start
			}
			if ci > 0 {
				off = l.chunks[ci-1].n - 1
			}
		}
		return
	}
	for ; ci < len(l.chunks); ci++ {
		c := l.chunks[ci]
		start, _, _, _ := c.entry(off)
		for k := off; k < c.n; k++ {
			ds, de, end := c.entryAt(start)
			if !fn(i, c.elem(ds, de)) {
				return
			}
			i++
			start = end
		}
		off = 0
	}
}

// all returns the elements as slices of the chunks.
func (l *listChunks) all() [][]byte {
	out := make([][]byte, 0, l.count)
	l.each(0, false, func(_ int, el []byte) bool {
		out = append(out, el)
		return true
	})
	return out
}

// rebuild packs elems as the whole list.
func (l *listChunks) rebuild(elems [][]byte) {
	var fresh listChunks
	for _, el := range elems {
		fresh.pushBack(el)
	}
	*l = fresh
}

// setAt replaces the off-th element of chunk ci in a new buffer for the
// chunk.
func (l *listChunks) setAt(ci, off int, el []byte) {
	c := l.chunks[ci]
	start, _, _, end := c.entry(off)
	need := listEntrySize(len(el))
	nc := &listChunk{buf: make([]byte, c.live()-(end-start)+need), n: c.n}
	w := copy(nc.buf, c.buf[c.lo:start])
	putListEntry(nc.buf, w, el)
	w += need
	w += copy(nc.buf[w:], c.buf[end:c.hi])
	nc.hi, nc.whi = w, w
	l.chunks[ci] = nc
}

func (l *listChunks) clone() listChunks {
	c := listChunks{count: l.count, chunks: make([]*listChunk, len(l.chunks))}
	for i, ch := range l.chunks {
		buf := append([]byte(nil), ch.buf[ch.lo:ch.hi]...)
		c.chunks[i] = &listChunk{buf: buf, hi: len(buf), whi: len(buf), n: ch.n}
	}
	return c
}

// trim keeps elements [start, stop], valid indexes with start <= stop.
func (l *listChunks) trim(start, stop int) {
	for i := l.count - 1; i > stop; i-- {
		l.popBack()
	}
	for i := 0; i < start; i++ {
		l.popFront()
	}
}
