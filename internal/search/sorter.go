package search

import (
	"bytes"
)

// sortItem is a result in RediSearch's sorter: its row and document id.
// Grouped rows have no document; the first to reach the sorter keeps the
// id of the last document the grouper read, the others 0, and a result
// the sorter drops lends its id to the next.
type sortItem struct {
	row   *Row
	docID int64
}

// SortRowsTop orders rows by keys as RediSearch's sorter does, keeping
// the best keep in a min-max heap ordered by cmpByFields, ties going by
// document id, and returns them best first.
func SortRowsTop(rows []*Row, keys []SortKey, keep int) []*Row {
	if keep <= 0 || len(rows) == 0 {
		return rows[:0]
	}
	cmp := func(a, b *sortItem) int { return cmpByFields(a, b, keys) }
	h := &minMaxHeap{cmp: cmp, data: make([]*sortItem, 1, min(keep, len(rows))+1)}
	pooled := &sortItem{}
	for i, r := range rows {
		pooled.row = r
		switch {
		case r.docID != 0:
			pooled.docID = r.docID
		case i == 0:
			pooled.docID = 1
		}
		if h.count() < keep {
			h.insert(pooled)
			pooled = &sortItem{}
			continue
		}
		if cmp(pooled, h.data[1]) > 0 {
			pooled = h.exchangeMin(pooled)
		}
	}
	out := make([]*Row, 0, h.count())
	for h.count() > 0 {
		out = append(out, h.popMax().row)
	}
	return out
}

// cmpByFields is RediSearch's: a row lacking a key loses either way, then
// values compare as RSValue_Cmp does, reversed for ascending keys, and
// ties go to the lower document id ascending, the higher descending.
func cmpByFields(a, b *sortItem, keys []SortKey) int {
	asc := false
	for i, k := range keys {
		if i >= 8 {
			break
		}
		asc = !k.Desc
		_, okA := a.row.Get(k.Field)
		_, okB := b.row.Get(k.Field)
		if !okA || !okB {
			if okA {
				return 1
			}
			if okB {
				return -1
			}
			continue
		}
		if rc := compareCellsQ(a.row.cell(k.Field), b.row.cell(k.Field)); rc != 0 {
			if asc {
				return -rc
			}
			return rc
		}
	}
	rc := 1
	if a.docID < b.docID {
		rc = -1
	}
	if asc {
		return -rc
	}
	return rc
}

// compareCellsQ is RSValue_Cmp with an error status: numbers numerically,
// strings by their bytes, lists by their first items (or their lengths),
// a null below anything else, and a string against a number as a number,
// equal when it is not one.
func compareCellsQ(a, b cell) int {
	switch {
	case a.null && b.null:
		return 0
	case a.null:
		return -1
	case b.null:
		return 1
	case a.isArr && b.isArr:
		if len(a.arr) > 0 && len(b.arr) > 0 {
			return compareCellsQ(a.arr[0], b.arr[0])
		}
		return len(a.arr) - len(b.arr)
	case a.isArr || b.isArr:
		return 0
	case a.num && b.num:
		return cmpFloat(a.f, b.f)
	case a.num || b.num:
		x, okA := a.number()
		y, okB := b.number()
		if !okA || !okB {
			return 0
		}
		return cmpFloat(x, y)
	}
	return bytes.Compare([]byte(a.v), []byte(b.v))
}

func cmpFloat(x, y float64) int {
	switch {
	case x > y:
		return 1
	case x < y:
		return -1
	}
	return 0
}

// minMaxHeap is RediSearch's min-max heap, 1-based.
type minMaxHeap struct {
	data []*sortItem
	cmp  func(a, b *sortItem) int
}

func (h *minMaxHeap) count() int         { return len(h.data) - 1 }
func (h *minMaxHeap) gt(x, y int) bool   { return h.cmp(h.data[x], h.data[y]) > 0 }
func (h *minMaxHeap) lt(x, y int) bool   { return h.cmp(h.data[x], h.data[y]) < 0 }
func (h *minMaxHeap) swap(i, j int)      { h.data[i], h.data[j] = h.data[j], h.data[i] }
func mmhIsMin(n int) bool                { return mmhLevel(n)%2 == 0 }
func (h *minMaxHeap) insert(v *sortItem) { h.data = append(h.data, v); h.bubbleUp(h.count()) }

func mmhLevel(n int) int {
	l := 0
	for n > 1 {
		n >>= 1
		l++
	}
	return l
}

func (h *minMaxHeap) bubbleUpMin(i int) {
	for pp := i / 4; pp > 0 && h.lt(i, pp); pp = i / 4 {
		h.swap(i, pp)
		i = pp
	}
}

func (h *minMaxHeap) bubbleUpMax(i int) {
	for pp := i / 4; pp > 0 && h.gt(i, pp); pp = i / 4 {
		h.swap(i, pp)
		i = pp
	}
}

func (h *minMaxHeap) bubbleUp(i int) {
	p := i / 2
	if p <= 0 {
		return
	}
	if mmhIsMin(i) {
		if h.gt(i, p) {
			h.swap(i, p)
			h.bubbleUpMax(p)
		} else {
			h.bubbleUpMin(i)
		}
		return
	}
	if h.lt(i, p) {
		h.swap(i, p)
		h.bubbleUpMin(p)
	} else {
		h.bubbleUpMax(i)
	}
}

func (h *minMaxHeap) best(order func(x, y int) bool, i int) int {
	a, b := 2*i, 2*i+1
	c, d, e, f := 2*a, 2*a+1, 2*b, 2*b+1
	n := h.count()
	from3 := func(x, y, z int) int {
		if order(x, y) {
			if order(x, z) {
				return x
			}
			return z
		}
		if order(y, z) {
			return y
		}
		return z
	}
	switch {
	case f <= n:
		if order(c, d) {
			return from3(c, e, f)
		}
		return from3(d, e, f)
	case e <= n:
		return from3(c, d, e)
	case d <= n:
		return from3(b, c, d)
	case c <= n:
		if order(b, c) {
			return b
		}
		return c
	case b <= n:
		if order(a, b) {
			return a
		}
		return b
	case a <= n:
		return a
	}
	return -1
}

func (h *minMaxHeap) trickleDownMax(i int) {
	for {
		m := h.best(h.gt, i)
		if m < 0 {
			return
		}
		if m <= 2*i+1 {
			if h.gt(m, i) {
				h.swap(i, m)
			}
			return
		}
		if !h.gt(m, i) {
			return
		}
		h.swap(i, m)
		if h.lt(m, m/2) {
			h.swap(m, m/2)
		}
		i = m
	}
}

func (h *minMaxHeap) trickleDownMin(i int) {
	for {
		m := h.best(h.lt, i)
		if m < 0 {
			return
		}
		if m <= 2*i+1 {
			if h.lt(m, i) {
				h.swap(i, m)
			}
			return
		}
		if !h.lt(m, i) {
			return
		}
		h.swap(i, m)
		if h.gt(m, m/2) {
			h.swap(m, m/2)
		}
		i = m
	}
}

func (h *minMaxHeap) exchangeMin(v *sortItem) *sortItem {
	old := h.data[1]
	h.data[1] = v
	h.trickleDownMin(1)
	return old
}

func (h *minMaxHeap) popMax() *sortItem {
	n := h.count()
	switch {
	case n > 2:
		idx := 2
		if h.lt(2, 3) {
			idx = 3
		}
		d := h.data[idx]
		h.data[idx] = h.data[n]
		h.data = h.data[:n]
		h.trickleDownMax(idx)
		return d
	case n == 2:
		d := h.data[2]
		h.data = h.data[:2]
		return d
	}
	d := h.data[1]
	h.data = h.data[:1]
	return d
}
