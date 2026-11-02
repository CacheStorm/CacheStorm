package search

import (
	"math"
	"sort"
)

// The query iterators of RediSearch 8.2 (index.c and inverted_index.c),
// ported to score documents as Redis does. Scores depend on more than
// what a document matches: a union that skips past a document keeps one
// child's result, an aggregate keeps pointers to its children's records,
// which later reads overwrite, and an intersection sums its children in
// the order of their estimated sizes. Matching stays with Match; these
// iterators only give the scores.

const (
	readOK = iota
	readEOF
	readNotFound
)

const (
	resTerm = 1 << iota
	resVirtual
	resUnion
	resIntersect
	resNumeric
)

type rsResult struct {
	children  []*rsResult
	docID     int64
	weight    float64
	idf       float64
	fieldMask uint64
	freq      uint32
	typ       int
}

func (r *rsResult) reset() {
	r.docID = 0
	r.children = r.children[:0]
}

func (r *rsResult) addChild(c *rsResult) {
	r.children = append(r.children, c)
	r.freq += c.freq
	r.docID = c.docID
	r.fieldMask |= c.fieldMask
}

type rsIter interface {
	read() (int, *rsResult)
	skipTo(id int64) (int, *rsResult)
	lastDocID() int64
	estimate() int
	hasNext() bool
	base() *iterBase
}

type iterBase struct {
	cur     *rsResult
	minID   int64
	invalid bool
	kind    int
}

func (b *iterBase) base() *iterBase { return b }

const (
	kindRead = iota
	kindUnion
	kindIntersect
	kindOther
)

type postingEntry struct {
	id   int64
	mask uint64
	freq uint32
}

// termReader is an IndexReader over a term's (or tag value's) postings,
// returning the entries whose field mask meets the query's.
type termReader struct {
	iterBase
	profType string
	profTerm string
	entries  []postingEntry
	pos      int
	lastID   int64
	mask     uint64
	atEnd    bool
}

func newTermReader(entries []postingEntry, mask uint64, weight, idf float64) *termReader {
	r := &termReader{entries: entries, mask: mask, profType: "TEXT"}
	r.cur = &rsResult{typ: resTerm, weight: weight, idf: idf}
	r.kind = kindRead
	if len(entries) > 0 {
		r.lastID = entries[0].id
	}
	return r
}

func (r *termReader) setEnd() {
	r.atEnd = true
	r.invalid = true
}

func (r *termReader) read() (int, *rsResult) {
	if r.atEnd {
		r.setEnd()
		return readEOF, nil
	}
	for r.pos < len(r.entries) {
		e := r.entries[r.pos]
		r.pos++
		r.lastID = e.id
		if e.mask&r.mask == 0 {
			continue
		}
		r.cur.docID, r.cur.freq, r.cur.fieldMask = e.id, e.freq, e.mask
		return readOK, r.cur
	}
	r.setEnd()
	return readEOF, nil
}

func (r *termReader) skipTo(id int64) (int, *rsResult) {
	if id == 0 {
		return r.read()
	}
	if r.atEnd || len(r.entries) == 0 || id > r.entries[len(r.entries)-1].id {
		r.setEnd()
		return readEOF, nil
	}
	for {
		rc, res := r.read()
		if rc == readEOF {
			return readEOF, nil
		}
		if res.docID < id {
			continue
		}
		if res.docID == id {
			return readOK, res
		}
		return readNotFound, res
	}
}

func (r *termReader) lastDocID() int64 { return r.lastID }
func (r *termReader) estimate() int    { return len(r.entries) }
func (r *termReader) hasNext() bool    { return !r.invalid }

// listIter walks a sorted list of documents with a fixed record: numeric,
// geo, missing-field and vector-range filters, which score nothing.
type listIter struct {
	termReader
}

func newListIter(ids []int64) *listIter {
	entries := make([]postingEntry, len(ids))
	for i, id := range ids {
		entries[i] = postingEntry{id: id, mask: math.MaxUint64}
	}
	l := &listIter{termReader: *newTermReader(entries, math.MaxUint64, 1, 0)}
	l.cur.typ = resNumeric
	return l
}

type emptyIter struct{ iterBase }

func newEmptyIter() *emptyIter {
	e := &emptyIter{}
	e.invalid = true
	e.kind = kindOther
	return e
}

func (e *emptyIter) read() (int, *rsResult)        { return readEOF, nil }
func (e *emptyIter) skipTo(int64) (int, *rsResult) { return readEOF, nil }
func (e *emptyIter) lastDocID() int64              { return 0 }
func (e *emptyIter) estimate() int                 { return 0 }
func (e *emptyIter) hasNext() bool                 { return false }

type unionIter struct {
	iterBase
	queryType string
	orig      []rsIter
	its       []rsIter
	heap      []rsIter
	minDocID  int64
	quick     bool
	expected  int
	weight    float64
}

func newUnionIter(its []rsIter, quick bool, weight float64) *unionIter {
	u := &unionIter{its: append([]rsIter(nil), its...), orig: its, quick: quick, weight: weight, queryType: "UNION"}
	u.kind = kindUnion
	u.cur = &rsResult{typ: resUnion, weight: weight}
	for _, it := range its {
		it.base().minID = 0
		u.expected += it.estimate()
	}
	if len(its) > unionHeapMin() {
		for _, it := range u.its {
			u.heapOffer(it)
		}
	}
	return u
}

func (u *unionIter) estimate() int    { return u.expected }
func (u *unionIter) lastDocID() int64 { return u.minDocID }
func (u *unionIter) hasNext() bool    { return !u.invalid }

func (u *unionIter) remove(i int) int {
	u.its = append(u.its[:i], u.its[i+1:]...)
	return i - 1
}

func (u *unionIter) read() (int, *rsResult) {
	if u.heap != nil {
		return u.readHeap()
	}
	if len(u.its) == 0 || !hasNext(u) {
		u.invalid = true
		return readEOF, nil
	}
	u.cur.reset()
	for {
		var minDocID int64 = math.MaxInt64
		var minIt rsIter
		active := 0
		for i := 0; i < len(u.its); i++ {
			it := u.its[i]
			res := it.base().cur
			rc := readOK
			for it.base().minID <= u.minDocID && rc != readEOF {
				rc = readNotFound
				for rc == readNotFound {
					var r *rsResult
					rc, r = it.read()
					if r != nil {
						res = r
						it.base().minID = r.docID
					}
				}
			}
			if rc == readEOF {
				i = u.remove(i)
				continue
			}
			active++
			if rc == readOK && res.docID <= minDocID {
				minDocID, minIt = res.docID, it
			}
		}
		if minIt != nil {
			u.skipTo(minIt.base().minID)
			u.minDocID = minIt.base().minID
			return readOK, u.cur
		}
		if active == 0 {
			break
		}
	}
	u.invalid = true
	return readEOF, nil
}

func (u *unionIter) skipTo(id int64) (int, *rsResult) {
	if id == 0 {
		return u.read()
	}
	if u.heap != nil {
		return u.skipToHeap(id)
	}
	if !hasNext(u) {
		return readEOF, nil
	}
	u.cur.reset()
	u.cur.weight = u.weight
	active, found := 0, 0
	var minDocID int64 = math.MaxUint32
	var minResult *rsResult
	for i := 0; i < len(u.its); i++ {
		it := u.its[i]
		var res *rsResult
		var rc int
		if it.base().minID < id {
			if rc, res = it.skipTo(id); rc == readEOF {
				i = u.remove(i)
				continue
			}
			if res != nil {
				it.base().minID = res.docID
			}
		} else {
			rc = readNotFound
			if it.base().minID == id {
				rc = readOK
			}
			res = it.base().cur
		}
		if it.base().minID != 0 {
			if it.base().minID < minDocID || minResult == nil {
				minResult, minDocID = res, it.base().minID
			}
		}
		if rc == readOK {
			if res == nil {
				res = it.base().cur
			}
			u.cur.addChild(res)
			u.minDocID = it.base().minID
			found++
		}
		active++
		if found > 0 && u.quick {
			break
		}
	}
	if active == 0 {
		u.invalid = true
		return readEOF, nil
	}
	if found > 0 {
		return readOK, u.cur
	}
	if minResult != nil {
		u.cur.addChild(minResult)
	}
	u.minDocID = minDocID
	return readNotFound, u.cur
}

// The heap variants serve unions of more than 20 children, as RediSearch's
// do, with its binary heap on each child's last id.
func heapCmp(a, b rsIter) int {
	switch x, y := a.base().minID, b.base().minID; {
	case x < y:
		return 1
	case x > y:
		return -1
	}
	return 0
}

func (u *unionIter) heapOffer(it rsIter) {
	u.heap = append(u.heap, it)
	for i := len(u.heap) - 1; i != 0; {
		p := (i - 1) / 2
		if heapCmp(u.heap[i], u.heap[p]) < 0 {
			return
		}
		u.heap[i], u.heap[p] = u.heap[p], u.heap[i]
		i = p
	}
}

func (u *unionIter) heapPushDown(i int) {
	h := u.heap
	for {
		l, r := 2*i+1, 2*i+2
		var c int
		switch {
		case r >= len(h):
			if l >= len(h) {
				return
			}
			c = l
		case heapCmp(h[l], h[r]) < 0:
			c = r
		default:
			c = l
		}
		if heapCmp(h[i], h[c]) < 0 {
			h[i], h[c] = h[c], h[i]
			i = c
		} else {
			return
		}
	}
}

func (u *unionIter) heapPoll() {
	last := len(u.heap) - 1
	u.heap[0] = u.heap[last]
	u.heap = u.heap[:last]
	if len(u.heap) > 1 {
		u.heapPushDown(0)
	}
}

func (u *unionIter) heapReplace(it rsIter) {
	u.heap[0] = it
	u.heapPushDown(0)
}

func (u *unionIter) heapCollect() {
	root := u.heap[0]
	u.cur.addChild(root.base().cur)
	var walk func(i int)
	walk = func(i int) {
		if i >= len(u.heap) || heapCmp(root, u.heap[i]) != 0 {
			return
		}
		u.cur.addChild(u.heap[i].base().cur)
		walk(2*i + 1)
		walk(2*i + 2)
	}
	walk(1)
	walk(2)
}

func (u *unionIter) readHeap() (int, *rsResult) {
	if !hasNext(u) {
		u.invalid = true
		return readEOF, nil
	}
	u.cur.reset()
	next := u.minDocID + 1
	var it rsIter
	var res *rsResult
	for len(u.heap) > 0 {
		it = u.heap[0]
		res = it.base().cur
		if it.base().minID >= next && it.base().minID != 0 {
			break
		}
		rc, r := it.skipTo(next)
		if rc == readEOF {
			u.heapPoll()
			continue
		}
		res = r
		it.base().minID = r.docID
		u.heapReplace(it)
		if u.quick && it.base().minID == next {
			break
		}
	}
	if len(u.heap) == 0 {
		u.invalid = true
		return readEOF, nil
	}
	u.minDocID = it.base().minID
	if u.quick {
		u.cur.addChild(res)
	} else {
		u.heapCollect()
	}
	return readOK, u.cur
}

func (u *unionIter) skipToHeap(id int64) (int, *rsResult) {
	if !hasNext(u) {
		return readEOF, nil
	}
	u.cur.reset()
	u.cur.weight = u.weight
	var it rsIter
	for len(u.heap) > 0 {
		it = u.heap[0]
		if it.base().minID >= id {
			break
		}
		rc, r := it.skipTo(id)
		if rc == readEOF {
			u.heapPoll()
			continue
		}
		it.base().minID = r.docID
		u.heapReplace(it)
		if u.quick && it.base().minID == id {
			break
		}
	}
	if len(u.heap) == 0 {
		u.invalid = true
		return readEOF, nil
	}
	rc := readNotFound
	if it.base().minID == id {
		rc = readOK
	}
	if u.quick {
		u.cur.addChild(it.base().cur)
	} else {
		u.heapCollect()
	}
	u.minDocID = it.base().minID
	return rc, u.cur
}

func hasNext(it rsIter) bool { return !it.base().invalid || it.hasNext() }

type intersectIter struct {
	iterBase
	its         []rsIter
	docIDs      []int64
	lastDocID_  int64
	lastFoundID int64
	expected    int
	fieldMask   uint64
	relevant    func(id int64) bool
	broken      bool
}

const invalidEstimate = math.MaxUint32

func newIntersectIter(its []rsIter, inOrder bool, fieldMask uint64, weight float64, relevant func(int64) bool) *intersectIter {
	ii := &intersectIter{its: its, docIDs: make([]int64, len(its)), fieldMask: fieldMask, relevant: relevant, expected: invalidEstimate}
	ii.kind = kindIntersect
	ii.cur = &rsResult{typ: resIntersect, weight: weight}
	for _, it := range its {
		if it == nil {
			ii.broken = true
		}
	}
	if !inOrder {
		sort.SliceStable(ii.its, func(a, b int) bool { return iterCmp(ii.its[a], ii.its[b]) < 0 })
	}
	if ii.broken {
		return ii
	}
	for _, it := range ii.its {
		if n := it.estimate(); n < ii.expected {
			ii.expected = n
		}
	}
	return ii
}

// iterCmp is RediSearch's cmpIter: estimates, an intersection's divided by
// its children (in integer arithmetic, so by zero for two or more), the
// difference cut to an int.
func iterCmp(a, b rsIter) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	factor := func(it rsIter) float64 {
		if ii, ok := unwrap(it).(*intersectIter); ok {
			return float64(1 / max(1, len(ii.its)))
		}
		return 1
	}
	return int(float64(a.estimate())*factor(a) - float64(b.estimate())*factor(b))
}

func (ii *intersectIter) estimate() int    { return ii.expected }
func (ii *intersectIter) lastDocID() int64 { return ii.lastFoundID }
func (ii *intersectIter) hasNext() bool    { return !ii.invalid }

func (ii *intersectIter) skipTo(id int64) (int, *rsResult) {
	if id == 0 {
		return ii.read()
	}
	ii.cur.reset()
	nfound := 0
	for i, it := range ii.its {
		if it == nil || !hasNext(it) {
			return readEOF, nil
		}
		res := it.base().cur
		rc := readOK
		if ii.docIDs[i] != id {
			var r *rsResult
			rc, r = it.skipTo(id)
			if rc != readEOF && r != nil {
				res = r
				id = r.docID
				ii.docIDs[i] = id
			}
		}
		if rc == readEOF {
			ii.invalid = true
			return readEOF, nil
		}
		if rc == readOK {
			if res != nil && res.docID == id {
				ii.cur.addChild(res)
			}
			ii.lastDocID_ = id
			nfound++
		} else if ii.docIDs[i] > ii.lastDocID_ {
			ii.lastDocID_ = ii.docIDs[i]
			break
		}
	}
	if nfound == len(ii.its) && (ii.relevant == nil || ii.relevant(ii.cur.docID)) {
		ii.lastFoundID = ii.cur.docID
		ii.lastDocID_++
		return readOK, ii.cur
	}
	if rc, _ := ii.read(); rc == readEOF {
		return readEOF, nil
	}
	return readNotFound, ii.cur
}

func (ii *intersectIter) read() (int, *rsResult) {
	if len(ii.its) == 0 {
		return readEOF, nil
	}
	for {
		nh := 0
		ii.cur.reset()
		for i, it := range ii.its {
			if it == nil {
				ii.invalid = true
				return readEOF, nil
			}
			h := it.base().cur
			rc := readOK
			if ii.docIDs[i] != ii.lastDocID_ || ii.lastDocID_ == 0 {
				var r *rsResult
				if i == 0 && ii.docIDs[i] >= ii.lastDocID_ {
					rc, r = it.read()
				} else {
					rc, r = it.skipTo(ii.lastDocID_)
				}
				if rc == readEOF {
					ii.invalid = true
					return readEOF, nil
				}
				h = r
				ii.docIDs[i] = h.docID
			}
			if ii.docIDs[i] > ii.lastDocID_ {
				ii.lastDocID_ = ii.docIDs[i]
				break
			}
			if rc == readOK {
				nh++
				ii.cur.addChild(h)
			} else {
				ii.lastDocID_++
			}
		}
		if nh == len(ii.its) {
			ii.lastFoundID = ii.cur.docID
			ii.lastDocID_++
			if ii.cur.fieldMask&ii.fieldMask == 0 {
				continue
			}
			if ii.relevant != nil && !ii.relevant(ii.cur.docID) {
				continue
			}
			return readOK, ii.cur
		}
	}
}

type notIter struct {
	iterBase
	child  rsIter
	lastID int64
	maxID  int64
}

func newNotIter(child rsIter, maxID int64, weight float64) *notIter {
	if child == nil {
		child = newEmptyIter()
	}
	n := &notIter{child: child, maxID: maxID}
	n.kind = kindOther
	n.cur = &rsResult{typ: resVirtual, weight: weight, fieldMask: math.MaxUint64}
	return n
}

func (n *notIter) estimate() int    { return int(n.maxID) }
func (n *notIter) lastDocID() int64 { return n.lastID }
func (n *notIter) hasNext() bool    { return n.lastID <= n.maxID }

func (n *notIter) skipTo(id int64) (int, *rsResult) {
	if id > n.maxID {
		n.invalid = true
		return readEOF, nil
	}
	var childID int64
	if n.lastID != 0 {
		childID = n.child.lastDocID()
	}
	if childID > id || !hasNext(n.child) {
		n.cur.docID, n.lastID = id, id
		return readOK, n.cur
	}
	if childID == id {
		n.cur.docID, n.lastID = id, id
		return readNotFound, n.cur
	}
	if rc, r := n.child.skipTo(id); rc == readOK {
		return readNotFound, r
	}
	n.cur.docID, n.lastID = id, id
	return readOK, n.cur
}

func (n *notIter) read() (int, *rsResult) {
	if n.lastID > n.maxID {
		n.invalid = true
		return readEOF, nil
	}
	cr := n.child.base().cur
	if cr == nil || cr.docID == 0 {
		_, r := n.child.read()
		if r != nil {
			cr = r
		}
	}
	n.cur.docID++
	if cr != nil && cr.docID <= n.cur.docID && hasNext(n.child) {
		for cr.docID == n.cur.docID {
			n.cur.docID++
			rc, r := n.child.read()
			if rc == readEOF {
				break
			}
			cr = r
		}
	}
	if n.cur.docID > n.maxID {
		n.invalid = true
		return readEOF, nil
	}
	n.lastID = n.cur.docID
	return readOK, n.cur
}

type optionalIter struct {
	iterBase
	child  rsIter
	virt   *rsResult
	lastID int64
	maxID  int64
	nextID int64
	weight float64
}

func newOptionalIter(child rsIter, maxID int64, weight float64) *optionalIter {
	if child == nil {
		child = newEmptyIter()
	}
	o := &optionalIter{child: child, maxID: maxID, weight: weight}
	o.kind = kindOther
	o.virt = &rsResult{typ: resVirtual, weight: weight, freq: 1, fieldMask: math.MaxUint64}
	o.cur = o.virt
	return o
}

func (o *optionalIter) estimate() int    { return int(o.maxID) }
func (o *optionalIter) lastDocID() int64 { return o.lastID }
func (o *optionalIter) hasNext() bool    { return o.lastID <= o.maxID }

func (o *optionalIter) skipTo(id int64) (int, *rsResult) {
	o.lastID = id
	if o.lastID > o.maxID {
		o.invalid = true
		return readEOF, nil
	}
	if id == 0 {
		return o.read()
	}
	found := false
	if id == o.nextID {
		found = true
		o.cur = o.child.base().cur
	} else if id > o.nextID {
		rc, r := o.child.skipTo(id)
		if r != nil {
			o.cur = r
		}
		if rc == readOK {
			found = true
		}
		if o.cur != nil {
			o.nextID = o.cur.docID
		}
	}
	if found {
		o.cur.weight = o.weight
	} else {
		o.virt.docID = id
		o.virt.weight = 0
		o.cur = o.virt
	}
	return readOK, o.cur
}

func (o *optionalIter) read() (int, *rsResult) {
	if o.lastID >= o.maxID {
		return readEOF, nil
	}
	o.lastID++
	if o.lastID > o.nextID {
		rc, r := o.child.read()
		if rc == readEOF {
			o.nextID = o.maxID + 1
		} else {
			o.cur = r
			o.nextID = r.docID
		}
	}
	if o.lastID != o.nextID {
		o.cur = o.virt
		o.cur.weight = 0
	} else {
		o.cur = o.child.base().cur
		o.cur.weight = o.weight
	}
	o.cur.docID = o.lastID
	return readOK, o.cur
}

type wildcardIter struct {
	iterBase
	top, current int64
	numDocs      int
}

func newWildcardIter(top int64, numDocs int) *wildcardIter {
	w := &wildcardIter{top: top, numDocs: numDocs}
	w.kind = kindOther
	w.cur = &rsResult{typ: resVirtual, weight: 1, freq: 1, fieldMask: math.MaxUint64}
	return w
}

func (w *wildcardIter) estimate() int    { return w.numDocs }
func (w *wildcardIter) lastDocID() int64 { return w.current }
func (w *wildcardIter) hasNext() bool    { return w.current <= w.top }

func (w *wildcardIter) read() (int, *rsResult) {
	w.current++
	w.cur.docID = w.current
	if w.current > w.top {
		return readEOF, nil
	}
	return readOK, w.cur
}

func (w *wildcardIter) skipTo(id int64) (int, *rsResult) {
	if w.current > w.top {
		return readEOF, nil
	}
	if id == 0 {
		return w.read()
	}
	w.current = id
	w.cur.docID = id
	return readOK, w.cur
}
