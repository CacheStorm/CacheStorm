package store

import (
	"math"
	"sync/atomic"
)

// A list's OBJECT ENCODING depends on how Redis would hold it: one listpack
// until list-max-listpack-size is exceeded, then a quicklist of listpack
// nodes, which turns back into a listpack only when a deletion leaves a
// single node at half the limit. Whether that node is single depends on the
// order elements arrived and left, so listShape replays Redis' quicklist
// bookkeeping (node counts and listpack bytes, with Redis' fill rules,
// splits and merges) beside the elements.

var listMaxListpackSize atomic.Int64

func init() { listMaxListpackSize.Store(-2) }

// SetListMaxListpackSize sets list-max-listpack-size.
func SetListMaxListpackSize(n int64) {
	listMaxListpackSize.Store(max(min(n, math.MaxInt32), math.MinInt32))
}

func listFill() int { return int(listMaxListpackSize.Load()) }

const (
	qlSizeSafetyLimit      = 8192
	qlSizeEstimateOverhead = 8
	qlFillMax              = 1<<15 - 1
	lpEmptyBytes           = 7
)

var qlOptimizationLevel = [...]int{4096, 8192, 16384, 32768, 65536}

func qlNodeLimit(fill int) (size, count int) {
	if fill >= 0 {
		return math.MaxInt, max(fill, 1)
	}
	return qlOptimizationLevel[min(-fill-1, len(qlOptimizationLevel)-1)], math.MaxInt
}

func qlExceeds(fill, sz, count int) bool {
	szLimit, countLimit := qlNodeLimit(fill)
	if szLimit != math.MaxInt {
		return sz > szLimit
	}
	return sz > qlSizeSafetyLimit || count > countLimit
}

func qlLarge(sz, fill int) bool {
	if fill >= 0 {
		return sz > qlSizeSafetyLimit
	}
	limit, _ := qlNodeLimit(fill)
	return sz > limit
}

// lpEntrySize is how many bytes el takes in a listpack.
func lpEntrySize(el []byte) int {
	var enc int
	if v, ok := lpStringToInt64(el); ok {
		switch {
		case v >= 0 && v <= 127:
			enc = 1
		case v >= -4096 && v <= 4095:
			enc = 2
		case v >= -32768 && v <= 32767:
			enc = 3
		case v >= -8388608 && v <= 8388607:
			enc = 4
		case v >= math.MinInt32 && v <= math.MaxInt32:
			enc = 5
		default:
			enc = 9
		}
	} else {
		switch n := len(el); {
		case n < 64:
			enc = 1 + n
		case n < 4096:
			enc = 2 + n
		default:
			enc = 5 + n
		}
	}
	switch {
	case enc <= 127:
		return enc + 1
	case enc < 16383:
		return enc + 2
	case enc < 2097151:
		return enc + 3
	case enc < 268435455:
		return enc + 4
	}
	return enc + 5
}

// lpStringToInt64 is listpack's test for an element stored as an integer.
func lpStringToInt64(s []byte) (int64, bool) {
	if len(s) == 0 || len(s) >= 21 {
		return 0, false
	}
	if len(s) == 1 && s[0] == '0' {
		return 0, true
	}
	p := s
	neg := p[0] == '-'
	if neg {
		p = p[1:]
		if len(p) == 0 {
			return 0, false
		}
	}
	if p[0] < '1' || p[0] > '9' {
		return 0, false
	}
	var v uint64
	for _, c := range p {
		if c < '0' || c > '9' {
			return 0, false
		}
		if v > math.MaxUint64/10 {
			return 0, false
		}
		v *= 10
		if v > math.MaxUint64-uint64(c-'0') {
			return 0, false
		}
		v += uint64(c - '0')
	}
	if neg {
		if v > 1<<63 {
			return 0, false
		}
		return -int64(v), true
	}
	if v > math.MaxInt64 {
		return 0, false
	}
	return int64(v), true
}

func lpBytesOf(elems [][]byte) int {
	n := lpEmptyBytes
	for _, el := range elems {
		n += lpEntrySize(el)
	}
	return n
}

type qlNode struct {
	count, sz int
	plain     bool
}

type listShape struct {
	// count is the number of elements the shape describes; a list whose
	// Elements were replaced wholesale no longer matches it.
	count   int
	quick   bool
	lpBytes int
	fill    int
	nodes   []qlNode
}

func newListShape() *listShape { return &listShape{lpBytes: lpEmptyBytes} }

// autoListShape is the shape of a list built whole, as SORT ... STORE and
// loading build one: a quicklist filled from the tail, kept as a listpack
// if it fits one node.
func autoListShape(elems [][]byte) *listShape {
	if len(elems) == 0 {
		return newListShape()
	}
	s := &listShape{quick: true, fill: qlClampFill(listFill())}
	for _, el := range elems {
		s.push(el, false)
	}
	s.toListpack(false)
	return s
}

func qlClampFill(fill int) int { return max(min(fill, qlFillMax), -5) }

func (s *listShape) clone() *listShape {
	c := *s
	c.nodes = append([]qlNode(nil), s.nodes...)
	return &c
}

// grow converts a listpack that addBytes and addLen more would overflow.
func (s *listShape) grow(addBytes, addLen int) {
	if s.quick {
		return
	}
	fill := listFill()
	if !qlExceeds(fill, s.lpBytes+addBytes, s.count+addLen) {
		return
	}
	s.quick, s.fill, s.nodes = true, qlClampFill(fill), nil
	if s.count > 0 {
		s.nodes = append(s.nodes, qlNode{count: s.count, sz: s.lpBytes})
	}
}

// toListpack turns a quicklist of one packed node within the limit (half
// of it when shrinking) back into a listpack.
func (s *listShape) toListpack(shrinking bool) {
	if !s.quick || len(s.nodes) != 1 || s.nodes[0].plain {
		return
	}
	szLimit, countLimit := qlNodeLimit(listFill())
	if shrinking {
		szLimit, countLimit = szLimit/2, countLimit/2
	}
	if s.nodes[0].sz > szLimit || s.count > countLimit {
		return
	}
	s.quick, s.lpBytes, s.nodes = false, s.nodes[0].sz, nil
}

func (s *listShape) allowInsert(i, sz int) bool {
	if i < 0 || i >= len(s.nodes) {
		return false
	}
	n := s.nodes[i]
	if n.plain || qlLarge(sz, s.fill) {
		return false
	}
	return !qlExceeds(s.fill, n.sz+sz+qlSizeEstimateOverhead, n.count+1)
}

func (s *listShape) allowMerge(a, b int) bool {
	if a < 0 || b < 0 || a >= len(s.nodes) || b >= len(s.nodes) {
		return false
	}
	x, y := s.nodes[a], s.nodes[b]
	if x.plain || y.plain {
		return false
	}
	return !qlExceeds(s.fill, x.sz+y.sz-lpEmptyBytes, x.count+y.count)
}

func (s *listShape) insertNode(i int, n qlNode) {
	s.nodes = append(s.nodes, qlNode{})
	copy(s.nodes[i+1:], s.nodes[i:])
	s.nodes[i] = n
}

func (s *listShape) deleteNode(i int) {
	s.nodes = append(s.nodes[:i], s.nodes[i+1:]...)
}

func newNode(el []byte, fill int) qlNode {
	if qlLarge(len(el), fill) {
		return qlNode{count: 1, sz: len(el), plain: true}
	}
	return qlNode{count: 1, sz: lpEmptyBytes + lpEntrySize(el)}
}

func (s *listShape) push(el []byte, head bool) {
	s.count++
	if !s.quick {
		s.lpBytes += lpEntrySize(el)
		return
	}
	i := len(s.nodes) - 1
	if head {
		i = 0
	}
	if !qlLarge(len(el), s.fill) && s.allowInsert(i, len(el)) {
		s.nodes[i].count++
		s.nodes[i].sz += lpEntrySize(el)
		return
	}
	if head {
		s.insertNode(0, newNode(el, s.fill))
	} else {
		s.nodes = append(s.nodes, newNode(el, s.fill))
	}
}

// locate returns the node holding element i and i's offset in it.
func (s *listShape) locate(i int) (int, int) {
	for ni, n := range s.nodes {
		if i < n.count {
			return ni, i
		}
		i -= n.count
	}
	return -1, -1
}

// removeOne drops el, the element at offset off of node ni.
func (s *listShape) removeOne(ni int, el []byte) {
	n := &s.nodes[ni]
	n.count--
	if n.plain || n.count == 0 {
		s.deleteNode(ni)
		return
	}
	n.sz -= lpEntrySize(el)
}

func (s *listShape) popFront(els [][]byte) {
	for _, el := range els {
		s.count--
		if !s.quick {
			s.lpBytes -= lpEntrySize(el)
			continue
		}
		s.removeOne(0, el)
	}
}

func (s *listShape) popBack(els [][]byte) {
	for _, el := range els {
		s.count--
		if !s.quick {
			s.lpBytes -= lpEntrySize(el)
			continue
		}
		s.removeOne(len(s.nodes)-1, el)
	}
}

// removeAt drops the elements at the ascending indexes idx of elems.
func (s *listShape) removeAt(elems [][]byte, idx []int) {
	if !s.quick {
		for _, i := range idx {
			s.lpBytes -= lpEntrySize(elems[i])
		}
		s.count -= len(idx)
		return
	}
	kept := s.nodes[:0]
	start, k := 0, 0
	for _, n := range s.nodes {
		end := start + n.count
		for ; k < len(idx) && idx[k] < end; k++ {
			n.count--
			n.sz -= lpEntrySize(elems[idx[k]])
		}
		start = end
		if n.count > 0 {
			kept = append(kept, n)
		}
	}
	s.nodes = kept
	s.count -= len(idx)
}

// split divides node ni at offset off as _quicklistSplitNode does: with
// after, ni keeps [0, off] and the returned node holds the rest; without,
// ni keeps [off, count) and the returned node holds [0, off). elems are
// the node's elements.
func (s *listShape) split(ni, off int, after bool, elems [][]byte) qlNode {
	cut := off
	if after {
		cut = off + 1
	}
	left := qlNode{count: cut, sz: lpBytesOf(elems[:cut])}
	right := qlNode{count: len(elems) - cut, sz: lpBytesOf(elems[cut:])}
	if after {
		s.nodes[ni] = left
		return right
	}
	s.nodes[ni] = right
	return left
}

func (s *listShape) merge(a, b int) int {
	s.nodes[a].count += s.nodes[b].count
	s.nodes[a].sz += s.nodes[b].sz - lpEmptyBytes
	s.deleteNode(b)
	return a
}

// mergeAround is _quicklistMergeNodes: it merges the pairs before and
// after center, then center with its neighbours. It returns the index of
// the node center ended up in.
func (s *listShape) mergeAround(c int) int {
	if s.allowMerge(c-1, c-2) {
		s.merge(c-2, c-1)
		c--
	}
	if s.allowMerge(c+1, c+2) {
		s.merge(c+1, c+2)
	}
	if s.allowMerge(c, c-1) {
		c = s.merge(c-1, c)
	}
	if s.allowMerge(c, c+1) {
		s.merge(c, c+1)
	}
	return c
}

// insert adds el before or after element i of elems, the elements before
// the insertion, as _quicklistInsert does.
func (s *listShape) insert(elems [][]byte, i int, el []byte, after bool) {
	s.count++
	if !s.quick {
		s.lpBytes += lpEntrySize(el)
		return
	}
	ni, off := s.locate(i)
	n := s.nodes[ni]
	sz := len(el)
	full := !s.allowInsert(ni, sz)
	atTail := after && off == n.count-1
	atHead := !after && off == 0
	availNext := atTail && s.allowInsert(ni+1, sz)
	availPrev := atHead && s.allowInsert(ni-1, sz)
	pos := ni
	if after {
		pos = ni + 1
	}
	nodeElems := func() [][]byte {
		start := i - off
		return elems[start : start+n.count]
	}
	if qlLarge(sz, s.fill) {
		if n.plain || atTail || atHead {
			s.insertNode(pos, newNode(el, s.fill))
			return
		}
		rest := s.split(ni, off, after, nodeElems())
		if after {
			s.insertNode(ni+1, newNode(el, s.fill))
			s.insertNode(ni+2, rest)
		} else {
			s.insertNode(ni, newNode(el, s.fill))
			s.insertNode(ni, rest)
		}
		return
	}
	entry := lpEntrySize(el)
	switch {
	case !full:
		s.nodes[ni].count++
		s.nodes[ni].sz += entry
	case atTail && availNext:
		s.nodes[ni+1].count++
		s.nodes[ni+1].sz += entry
	case atHead && availPrev:
		s.nodes[ni-1].count++
		s.nodes[ni-1].sz += entry
	case atTail || atHead:
		s.insertNode(pos, newNode(el, s.fill))
	default:
		rest := s.split(ni, off, after, nodeElems())
		rest.count++
		rest.sz += entry
		center := ni
		if after {
			s.insertNode(ni+1, rest)
		} else {
			s.insertNode(ni, rest)
			center = ni + 1
		}
		s.mergeAround(center)
	}
}

// replace swaps element i of elems, the elements before the change, for
// el, as quicklistReplaceEntry does.
func (s *listShape) replace(elems [][]byte, i int, el []byte) {
	old := elems[i]
	if !s.quick {
		s.lpBytes += lpEntrySize(el) - lpEntrySize(old)
		return
	}
	ni, off := s.locate(i)
	n := s.nodes[ni]
	large := qlLarge(len(el), s.fill)
	switch {
	case !n.plain && !large:
		s.nodes[ni].sz += lpEntrySize(el) - lpEntrySize(old)
	case n.plain && large:
		s.nodes[ni].sz = len(el)
	case n.plain:
		s.insert(elems, i, el, true)
		s.count--
		s.deleteNode(ni)
	default:
		start := i - off
		var rest *qlNode
		if off != n.count-1 {
			r := s.split(ni, off, true, elems[start:start+n.count])
			rest = &r
		}
		s.insertNode(ni+1, newNode(el, s.fill))
		if rest != nil {
			s.insertNode(ni+2, *rest)
		}
		if s.nodes[ni].count == 1 {
			s.deleteNode(ni)
			return
		}
		s.nodes[ni].count--
		s.nodes[ni].sz -= lpEntrySize(old)
		s.mergeAround(ni + 1)
	}
}

// Shape helpers on ListValue. The caller holds the value's write lock.

func (v *ListValue) shapeLocked() *listShape {
	if v.shape == nil || v.shape.count != len(v.Elements) {
		v.shape = autoListShape(v.Elements)
	}
	return v.shape
}

// Encoding is the list's OBJECT ENCODING. The caller holds the value's
// lock, at least for reading.
func (v *ListValue) Encoding() string {
	s := v.shape
	if s == nil || s.count != len(v.Elements) {
		s = autoListShape(v.Elements)
	}
	if s.quick {
		return "quicklist"
	}
	return "listpack"
}

// TryGrow converts the list as Redis does before LSET or LINSERT writes el,
// whether or not the write then happens. The caller holds the write lock.
func (v *ListValue) TryGrow(el []byte) {
	v.shapeLocked().grow(len(el), 1)
}

func argBytes(items [][]byte) int {
	n := 0
	for _, el := range items {
		n += len(el)
	}
	return n
}

// movedBytes is what an element popped from a list adds to the conversion
// check when pushed again: Redis pops an integer element as an integer
// object, which the check does not count.
func movedBytes(el []byte) int {
	if _, ok := lpStringToInt64(el); ok {
		return 0
	}
	return len(el)
}

// PushMoved pushes el, an element popped from a list by LMOVE and its kin.
// The caller holds the write lock.
func (v *ListValue) PushMoved(el []byte, front bool) {
	s := v.shapeLocked()
	s.grow(movedBytes(el), 1)
	s.push(el, front)
	v.pushElems(front, el)
}

// Rotate moves one element from one end of the list to an end, as LMOVE
// with the same source and destination does, and returns it. The list must
// not be empty. The caller holds the write lock.
func (v *ListValue) Rotate(fromFront, toFront bool) []byte {
	s := v.shapeLocked()
	el := v.popElems(1, fromFront)[0]
	if fromFront {
		s.popFront([][]byte{el})
	} else {
		s.popBack([][]byte{el})
	}
	s.grow(movedBytes(el), 1)
	s.push(el, toFront)
	v.pushElems(toFront, el)
	s.toListpack(true)
	return el
}

// DeleteAt removes the elements at the ascending indexes idx, as LREM
// does. The caller holds the write lock.
func (v *ListValue) DeleteAt(idx []int) {
	if len(idx) == 0 {
		return
	}
	s := v.shapeLocked()
	s.removeAt(v.Elements, idx)
	kept := make([][]byte, 0, len(v.Elements)-len(idx))
	k := 0
	for i, el := range v.Elements {
		if k < len(idx) && idx[k] == i {
			k++
			continue
		}
		kept = append(kept, el)
	}
	v.Elements, v.backing, v.head = kept, nil, 0
	v.size.set(sumListElems(kept))
	if len(kept) > 0 {
		s.toListpack(true)
	}
}

// InsertAt inserts el before or after element i, as LINSERT does. The
// caller holds the write lock and has called TryGrow.
func (v *ListValue) InsertAt(i int, el []byte, after bool) {
	v.shapeLocked().insert(v.Elements, i, el, after)
	at := i
	if after {
		at = i + 1
	}
	elems := make([][]byte, 0, len(v.Elements)+1)
	elems = append(elems, v.Elements[:at]...)
	elems = append(elems, el)
	elems = append(elems, v.Elements[at:]...)
	v.Elements, v.backing, v.head = elems, nil, 0
	v.size.add(listElemSize(el))
}
