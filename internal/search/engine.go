package search

import (
	"math"
	"sort"
)

// scoreEngine builds RediSearch's iterators for a query over a cache and
// scores the documents its root reads.
type scoreEngine struct {
	idx   *Index
	c     *docCache
	stats *Stats
	view  *engineView
	rest  int64
	// profile wraps each iterator in a counting one, for FT.PROFILE.
	profile bool
}

func (e *scoreEngine) wrap(it rsIter) rsIter {
	if it == nil || !e.profile {
		return it
	}
	return &profIter{rsIter: it}
}

// engineView is a cache's documents by id and its NUMERIC fields' documents
// in id order, kept until the next write.
type engineView struct {
	byID  map[int64]*Document
	nums  map[int][]*Document
	maxID int64
}

func (c *docCache) engineView(idx *Index) *engineView {
	if c.engine != nil {
		return c.engine
	}
	v := &engineView{byID: make(map[int64]*Document, len(c.docs)), nums: make(map[int][]*Document)}
	all := make([]*Document, 0, len(c.docs))
	for _, d := range c.docs {
		v.byID[d.id] = d
		v.maxID = max(v.maxID, d.id)
		all = append(all, d)
	}
	SortByID(all)
	for i := range idx.Fields {
		if idx.Fields[i].Type != FieldNumeric {
			continue
		}
		for _, d := range all {
			if len(d.field(i).nums) > 0 {
				v.nums[i] = append(v.nums[i], d)
			}
		}
	}
	c.engine = v
	return v
}

// needsEngine reports whether a query has a union or intersection whose
// children score: only then do RediSearch's iterators score a document
// differently from the sum of what it matches.
func needsEngine(n *qnode) bool {
	if n == nil {
		return false
	}
	switch n.typ {
	case qUnion, qPhrase, qOptional:
		if len(n.children) > 1 || n.typ == qOptional {
			return true
		}
	case qTag:
		if len(n.children) > 1 {
			return true
		}
	}
	for _, c := range n.children {
		if needsEngine(c) {
			return true
		}
	}
	return false
}

func newScoreEngine(idx *Index, c *docCache, stats *Stats) *scoreEngine {
	return &scoreEngine{idx: idx, c: c, stats: stats, view: c.engineView(idx), rest: -c.minID}
}

// docID numbers documents from 1 as Redis does: those a rebuild read
// (ids -n to -1) first, then the ones written since (ids 1, 2, ...).
func (e *scoreEngine) docID(d *Document) int64 {
	if d.id < 0 {
		return d.id + e.rest + 1
	}
	return d.id + e.rest
}

func (e *scoreEngine) doc(id int64) *Document {
	raw := id - e.rest
	if id <= e.rest {
		raw--
	}
	return e.view.byID[raw]
}

func (e *scoreEngine) maxID() int64 {
	if e.view.maxID > 0 {
		return e.view.maxID + e.rest
	}
	return e.rest
}

func (e *scoreEngine) entries(docs []*Document, term string, tag bool) []postingEntry {
	out := make([]postingEntry, 0, len(docs))
	for _, d := range docs {
		if tag {
			out = append(out, postingEntry{id: e.docID(d), mask: math.MaxUint64, freq: 1})
			continue
		}
		tf := d.terms[term]
		out = append(out, postingEntry{id: e.docID(d), mask: tf.mask, freq: tf.freq})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

// reader opens a term's reader, nil when no document holds the term in
// the fields of mask, as Redis_OpenReader returns none.
func (e *scoreEngine) reader(term string, mask uint64, weight float64) rsIter {
	docs := e.c.terms[term]
	if len(docs) == 0 {
		return nil
	}
	var union uint64
	for _, d := range docs {
		union |= d.terms[term].mask
	}
	if union&mask == 0 {
		return nil
	}
	r := newTermReader(e.entries(docs, term, false), mask, weight, e.stats.idf(len(docs)))
	r.profTerm = term
	return e.wrap(r)
}

func (e *scoreEngine) tagReader(f *Field, value string, weight float64) rsIter {
	docs := e.c.tags[f.pos][value]
	if len(docs) == 0 {
		return nil
	}
	r := newTermReader(e.entries(docs, value, true), math.MaxUint64, weight, e.stats.idf(len(docs)))
	r.profType, r.profTerm = "TAG", value
	return e.wrap(r)
}

func (e *scoreEngine) list(n *qnode) rsIter {
	var ids []int64
	for _, d := range e.view.byID {
		if ok, _ := e.idx.Match(n, d, QueryOptions{}, e.stats); ok {
			ids = append(ids, e.docID(d))
		}
	}
	if len(ids) == 0 {
		return nil
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	l := newListIter(ids)
	l.profType = map[qtype]string{qGeo: "GEO", qMissing: "MISSING", qVector: "METRIC - VECTOR DISTANCE"}[n.typ]
	return e.wrap(l)
}

func (e *scoreEngine) build(n *qnode, mask uint64) rsIter {
	if n == nil {
		return nil
	}
	mask &= n.mask
	switch n.typ {
	case qToken:
		return e.reader(n.str, mask, n.weight)
	case qUnion:
		var its []rsIter
		for _, c := range n.children {
			if it := e.build(c, mask); it != nil {
				its = append(its, it)
			}
		}
		switch len(its) {
		case 0:
			return nil
		case 1:
			return its[0]
		}
		return e.wrap(newUnionIter(its, false, n.weight))
	case qPhrase:
		if len(n.children) == 1 {
			return e.build(n.children[0], mask)
		}
		its := make([]rsIter, len(n.children))
		for i, c := range n.children {
			its[i] = e.build(c, mask)
		}
		var relevant func(int64) bool
		if n.exact {
			relevant = func(id int64) bool { d := e.doc(id); return d != nil && e.idx.exactPhrase(n, d) }
		}
		return e.wrap(newIntersectIter(its, n.exact, mask, n.weight, relevant))
	case qPrefix, qFuzzy, qWildcardQuery:
		var its []rsIter
		for _, t := range n.terms {
			if it := e.reader(t, mask, 1); it != nil {
				its = append(its, it)
			}
		}
		if len(its) == 0 {
			return nil
		}
		u := newUnionIter(its, true, n.weight)
		u.queryType = map[qtype]string{qPrefix: "PREFIX", qFuzzy: "FUZZY", qWildcardQuery: "WILDCARD"}[n.typ] + " - " + n.str
		return e.wrap(u)
	case qNot:
		var child rsIter
		if len(n.children) > 0 {
			child = e.build(n.children[0], mask)
		}
		if child == nil {
			child = e.wrap(newEmptyIter())
		}
		return e.wrap(newNotIter(child, e.maxID(), n.weight))
	case qOptional:
		child := e.build(n.children[0], mask)
		if child == nil {
			child = e.wrap(newEmptyIter())
		}
		return e.wrap(newOptionalIter(child, e.maxID(), n.weight))
	case qWildcard:
		return e.wrap(newWildcardIter(e.maxID(), len(e.c.docs)))
	case qTag:
		return e.tag(n)
	case qNumeric:
		return e.numeric(n)
	}
	return e.list(n)
}

// numeric reads a NUMERIC filter as RediSearch reads a range tree of one
// leaf: every document with the field, skipping those out of range, so
// its estimate is the field's document count.
func (e *scoreEngine) numeric(n *qnode) rsIter {
	f := e.idx.Field(n.field)
	if f == nil || f.Type != FieldNumeric {
		return nil
	}
	docs := e.view.nums[f.pos]
	entries := make([]postingEntry, len(docs))
	lo, hi := math.Inf(1), math.Inf(-1)
	for i, d := range docs {
		entries[i].id = e.docID(d)
		for _, v := range d.field(f.pos).nums {
			lo, hi = min(lo, v), max(hi, v)
			if (v > n.lo || v == n.lo && n.loIncl) && (v < n.hi || v == n.hi && n.hiIncl) {
				entries[i].mask = 1
			}
		}
	}
	// The range's leaf is read when its values overlap the filter's.
	if len(docs) == 0 || hi < n.lo || lo > n.hi {
		return nil
	}
	r := newTermReader(entries, 1, 1, 0)
	r.cur.typ = resNumeric
	r.profType, r.profTerm = "NUMERIC", numericRangeTerm(lo, hi)
	return e.wrap(r)
}

func (e *scoreEngine) tag(n *qnode) rsIter {
	f := e.idx.Field(n.field)
	if f == nil || f.Type != FieldTag {
		return nil
	}
	one := func(c *qnode) rsIter {
		want, affix := tagChild(f, c)
		if affix == nil {
			return e.tagReader(f, want, n.weight)
		}
		if len(want) < minPrefixLength() {
			return nil
		}
		var values []string
		for v := range e.c.tags[f.pos] {
			if affixMatch(affix, want, v) {
				values = append(values, v)
			}
		}
		sort.Strings(values)
		var its []rsIter
		for _, v := range values {
			if it := e.tagReader(f, v, n.weight); it != nil {
				its = append(its, it)
			}
		}
		if len(its) == 0 {
			return nil
		}
		u := newUnionIter(its, true, n.weight)
		u.queryType = "PREFIX - " + want
		return e.wrap(u)
	}
	if len(n.children) == 1 {
		return one(n.children[0])
	}
	var its []rsIter
	for _, c := range n.children {
		if it := one(c); it != nil {
			its = append(its, it)
		}
	}
	if len(its) == 0 {
		return nil
	}
	u := newUnionIter(its, false, n.weight)
	u.queryType = "TAG"
	return e.wrap(u)
}

// scores reads the root to its end and scores each document it returns.
func (e *scoreEngine) scores(root *qnode) map[*Document]float64 {
	it := e.build(root, allFields)
	out := make(map[*Document]float64)
	if it == nil {
		return out
	}
	for {
		rc, r := it.read()
		if rc == readEOF {
			return out
		}
		if rc != readOK || r == nil {
			continue
		}
		d := e.doc(r.docID)
		if d == nil {
			continue
		}
		if _, seen := out[d]; !seen {
			out[d] = e.stats.score * e.bm25(r, d)
		}
	}
}

func (e *scoreEngine) bm25(r *rsResult, d *Document) float64 {
	switch {
	case r.typ == resTerm:
		return e.term(r.weight, r.idf, float64(r.freq), d)
	case r.typ&(resUnion|resIntersect) != 0:
		ret := 0.0
		for _, c := range r.children {
			ret += e.bm25(c, d)
		}
		return ret * r.weight
	case r.typ == resVirtual && r.freq != 0 && r.weight != 0:
		return e.term(r.weight, 1, 1, d)
	}
	return 0
}

func (e *scoreEngine) term(weight, idf, f float64, d *Document) float64 {
	const k1, b = float32(1.2), float32(0.75)
	norm := float64(1-b) + float64(b*float32(d.length))/e.stats.avgLen()
	return weight * idf * f * float64(k1+1) / (f + float64(k1)*norm)
}
