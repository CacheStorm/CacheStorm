package search

import (
	"sort"
	"sync"
	"sync/atomic"
	"weak"

	"github.com/cachestorm/cachestorm/internal/store"
)

// docCache holds an index's documents read from one store, so a query reads
// only the keys written since the last one. Writes mark keys dirty; a
// wholesale change of the store (its generation) rebuilds the cache, and a
// document whose key changed in any other way is caught when a query
// returns it (Fresh).
type docCache struct {
	mu         sync.Mutex
	generation uint64
	docs       map[string]*Document
	// dirty has its own lock, taken last: Touch is called from store hooks
	// that hold a shard lock, while a query holds mu and reads the store.
	dirtyMu sync.Mutex
	built   bool
	// dirty holds the keys written since the last query, each with the
	// sequence number of its last write: the document id it takes.
	dirty map[string]int64
	seq   int64
	// minID is the lowest document id the last rebuild gave.
	minID int64
	// engine holds what the score engine reads, rebuilt after writes.
	engine *engineView
	stats  *Stats
	// The postings: documents by index term, and by tag value of each TAG
	// field (by schema position). A document frequency is a posting's
	// length.
	terms map[string][]*Document
	tags  map[int]map[string][]*Document
	// bytes estimates the memory the cache holds, for INFO search.
	bytes atomic.Int64
}

type docSet map[*Document]struct{}

func (c *docCache) reset() {
	c.bytes.Store(0)
	c.docs = make(map[string]*Document)
	c.terms = make(map[string][]*Document)
	c.tags = make(map[int]map[string][]*Document)
	c.stats = &Stats{
		termDocs: func(term string) int { return len(c.terms[term]) },
		tagDocs:  func(pos int, tag string) int { return len(c.tags[pos][tag]) },
	}
}

func addPosting(m map[string][]*Document, k string, d *Document) {
	m[k] = append(m[k], d)
}

func dropPosting(m map[string][]*Document, k string, d *Document) {
	list := m[k]
	for i, x := range list {
		if x == d {
			list[i] = list[len(list)-1]
			list[len(list)-1] = nil
			list = list[:len(list)-1]
			break
		}
	}
	if len(list) == 0 {
		delete(m, k)
	} else {
		m[k] = list
	}
}

// index adds (add) or removes a document and its postings.
func (c *docCache) index(idx *Index, d *Document, add bool) {
	c.engine = nil
	if add {
		c.docs[d.Key] = d
		c.stats.add(d, 1)
		c.bytes.Add(d.size())
	} else {
		delete(c.docs, d.Key)
		c.stats.add(d, -1)
		c.bytes.Add(-d.size())
	}
	for t := range d.terms {
		if add {
			addPosting(c.terms, t, d)
		} else {
			dropPosting(c.terms, t, d)
		}
	}
	for i := range idx.Fields {
		if idx.Fields[i].Type != FieldTag {
			continue
		}
		m := c.tags[i]
		if m == nil {
			m = make(map[string][]*Document)
			c.tags[i] = m
		}
		seen := make(map[string]bool)
		for _, t := range d.field(i).strs {
			if seen[t] {
				continue
			}
			seen[t] = true
			if add {
				addPosting(m, t, d)
			} else {
				dropPosting(m, t, d)
			}
		}
	}
}

// cacheSet holds an index's caches by store. The stores are held weakly:
// a store nothing else uses (a replica's discarded copy, a test's) takes
// its cache with it.
type cacheSet struct {
	mu     sync.Mutex
	caches map[weak.Pointer[store.Store]]*docCache
}

// lookup returns st's cache, creating it when create is set.
func (idx *Index) lookup(st *store.Store, create bool) *docCache {
	idx.cacheSet.mu.Lock()
	defer idx.cacheSet.mu.Unlock()
	if idx.cacheSet.caches == nil {
		if !create {
			return nil
		}
		idx.cacheSet.caches = make(map[weak.Pointer[store.Store]]*docCache)
	}
	key := weak.Make(st)
	c := idx.cacheSet.caches[key]
	if c == nil && create {
		for k := range idx.cacheSet.caches {
			if k.Value() == nil {
				delete(idx.cacheSet.caches, k)
			}
		}
		c = &docCache{dirty: make(map[string]int64)}
		c.reset()
		idx.cacheSet.caches[key] = c
	}
	return c
}

func (idx *Index) cache(st *store.Store) *docCache { return idx.lookup(st, true) }

// rebuildAll makes every cache read its store again on its next query.
func (cs *cacheSet) rebuildAll() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for _, c := range cs.caches {
		c.dirtyMu.Lock()
		c.built = false
		c.dirtyMu.Unlock()
	}
}

// maxDirty bounds the keys marked between two queries: past it the cache
// is rebuilt on the next query instead of growing without limit.
const maxDirty = 1 << 16

// Touch marks key as written in st. It creates nothing: a store no
// command of the index has read yet has no cache to keep current.
func (idx *Index) Touch(st *store.Store, key string) {
	if !idx.Covers(key) {
		return
	}
	c := idx.lookup(st, false)
	if c == nil {
		return
	}
	c.dirtyMu.Lock()
	if len(c.dirty) >= maxDirty {
		c.built = false
		clear(c.dirty)
	} else {
		c.seq++
		c.dirty[key] = c.seq
	}
	c.dirtyMu.Unlock()
}

// Open makes the index keep track of st from now on, indexing what it
// holds: FT.CREATE calls it, so documents written later are numbered in
// the order they are written.
func (idx *Index) Open(st *store.Store) {
	c := idx.cache(st)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncLocked(idx, st)
}

// Documents returns the index's documents in st, in document id order,
// and their corpus statistics. The caller must not change them.
func (idx *Index) Documents(st *store.Store) ([]*Document, *Stats) {
	c := idx.cache(st)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncLocked(idx, st)
	docs := make([]*Document, 0, len(c.docs))
	for _, d := range c.docs {
		docs = append(docs, d)
	}
	SortByID(docs)
	return docs, c.stats
}

// SortByID orders documents by id.
func SortByID(docs []*Document) {
	sort.Slice(docs, func(i, j int) bool { return docs[i].id < docs[j].id })
}

// Before reports whether a was indexed before b.
func (d *Document) Before(b *Document) bool { return d.id < b.id }

// syncLocked brings the cache up to date with st.
func (c *docCache) syncLocked(idx *Index, st *store.Store) {
	c.dirtyMu.Lock()
	built := c.built
	dirty := c.dirty
	c.dirty = make(map[string]int64)
	c.built = true
	c.dirtyMu.Unlock()
	c.stats.score = idx.Score
	if gen := st.Generation(); !built || c.generation != gen {
		// A rebuild numbers the documents written since the last query
		// by their writes, after the others.
		c.reset()
		c.stats.score = idx.Score
		var rest []*Document
		st.ForEachLive(func(key string, entry *store.Entry) error { //nolint:errcheck // the callback never fails
			if idx.Covers(key) {
				if doc, ok := idx.Load(key, entry, false); ok {
					if seq, ok := dirty[key]; ok {
						doc.id = seq
					} else {
						rest = append(rest, doc)
					}
					c.index(idx, doc, true)
				}
			}
			return nil
		})
		sort.Slice(rest, func(i, j int) bool { return rest[i].Key < rest[j].Key })
		for i, d := range rest {
			d.id = int64(i - len(rest))
		}
		c.minID = int64(-len(rest))
		c.generation = gen
		return
	}
	keys := make([]string, 0, len(dirty))
	for key := range dirty {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return dirty[keys[i]] < dirty[keys[j]] })
	for _, key := range keys {
		if doc := c.refreshLocked(idx, st, key); doc != nil {
			doc.id = dirty[key]
		}
	}
}

// refreshLocked reads key again; the document read takes the next id.
func (c *docCache) refreshLocked(idx *Index, st *store.Store, key string) *Document {
	if old, ok := c.docs[key]; ok {
		c.index(idx, old, false)
	}
	entry, ok := st.Peek(key)
	if !ok {
		return nil
	}
	doc, ok := idx.Load(key, entry, false)
	if !ok {
		return nil
	}
	c.dirtyMu.Lock()
	c.seq++
	doc.id = c.seq
	c.dirtyMu.Unlock()
	c.index(idx, doc, true)
	return doc
}

// Fresh returns doc as its key holds it now: doc itself when the key is
// unchanged, a reloaded document when it changed, nil when it is gone.
func (idx *Index) Fresh(st *store.Store, doc *Document) *Document {
	entry, ok := st.Peek(doc.Key)
	if ok && entry == doc.entry && entry.Value == doc.value && !hasFieldTTLs(entry.Value) {
		return doc
	}
	c := idx.cache(st)
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.refreshLocked(idx, st, doc.Key)
}

func hasFieldTTLs(v store.Value) bool {
	h, ok := v.(*store.HashValue)
	return ok && h.HasFieldTTLs()
}

// Hit is a document a query matched, its score, and the fields a vector
// query yields for it (a distance, by name).
type Hit struct {
	Doc      *Document
	Score    float64
	Distance float64
	Yields   [][2]string
	// Terms are the query terms the document matched, when asked for.
	Terms []string
}

// Search runs a query over the index's documents in st: the postings give
// the candidates, which are then matched and scored. A KNN query keeps the
// k documents nearest its vector among those its filter matches.
func (idx *Index) Search(st *store.Store, n Node, opts QueryOptions) ([]Hit, error) {
	if n == nil {
		return nil, nil
	}
	qs := make(map[*qnode][]float64)
	if err := idx.checkVectors(n, qs); err != nil {
		return nil, err
	}
	c := idx.cache(st)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncLocked(idx, st)
	root := n
	var knn *qnode
	if n.typ == qVector && n.knn {
		knn, root = n, nil
		if len(n.children) > 0 {
			root = n.children[0]
		}
	}
	c.fillTerms(root)
	c.stats.vectors = qs
	defer func() { c.stats.vectors = nil }()
	var hits []Hit
	match := func(d *Document) {
		if root == nil {
			hits = append(hits, Hit{Doc: d})
			return
		}
		if ok, score := idx.Match(root, d, opts, c.stats); ok {
			h := Hit{Doc: d, Score: score}
			h.Yields = idx.rangeYields(root, d, qs, nil)
			hits = append(hits, h)
		}
	}
	if cand, all := c.candidates(idx, root); all || root == nil {
		for _, d := range c.docs {
			match(d)
		}
	} else {
		for d := range cand {
			match(d)
		}
	}
	if opts.MatchedTerms && len(hits) > 0 {
		terms := newScoreEngine(idx, c, c.stats).matchedTerms(root)
		for i := range hits {
			hits[i].Terms = terms[hits[i].Doc]
		}
	}
	if needsEngine(root) && len(hits) > 0 {
		scores := newScoreEngine(idx, c, c.stats).scores(root)
		for i := range hits {
			if s, ok := scores[hits[i].Doc]; ok {
				hits[i].Score = s
			}
		}
	}
	if knn != nil {
		if knn.byScore {
			for i := range hits {
				hits[i].Score = 0
			}
		}
		hits = idx.knn(knn, qs[knn], hits)
	}
	return hits, nil
}

// NearestFirst reports whether a query's results come nearest first: a
// KNN query without a filter.
func NearestFirst(n Node) bool { return n != nil && n.typ == qVector && n.knn && n.byScore }

// candidates returns a superset of the documents n can match; all is true
// when the postings cannot narrow it.
func (c *docCache) candidates(idx *Index, n Node) (docSet, bool) {
	union := func(out docSet, list []*Document) {
		for _, d := range list {
			out[d] = struct{}{}
		}
	}
	if n == nil {
		return docSet{}, false
	}
	switch n.typ {
	case qToken:
		out := make(docSet)
		union(out, c.terms[n.str])
		return out, false
	case qPrefix, qFuzzy, qWildcardQuery:
		out := make(docSet)
		for _, t := range n.terms {
			union(out, c.terms[t])
		}
		return out, false
	case qTag:
		f := idx.Field(n.field)
		if f == nil {
			return docSet{}, false
		}
		out := make(docSet)
		for _, ch := range n.children {
			want, affix := tagChild(f, ch)
			if affix == nil {
				union(out, c.tags[f.pos][want])
				continue
			}
			for t, list := range c.tags[f.pos] {
				if affixMatch(affix, want, t) {
					union(out, list)
				}
			}
		}
		return out, false
	case qPhrase:
		var out docSet
		narrowed := false
		for _, child := range n.children {
			set, all := c.candidates(idx, child)
			if all {
				continue
			}
			out, narrowed = intersect(out, set, !narrowed), true
		}
		return out, !narrowed
	case qUnion:
		out := make(docSet)
		for _, child := range n.children {
			set, all := c.candidates(idx, child)
			if all {
				return nil, true
			}
			for d := range set {
				out[d] = struct{}{}
			}
		}
		return out, false
	}
	return nil, true
}

// intersect returns a ∩ b; first means a is not set yet.
func intersect(a, b docSet, first bool) docSet {
	if first {
		return b
	}
	out := make(docSet)
	small, large := a, b
	if len(b) < len(a) {
		small, large = b, a
	}
	for d := range small {
		if _, ok := large[d]; ok {
			out[d] = struct{}{}
		}
	}
	return out
}

// Content returns the key's document as it is now, with its content for a
// reply; nil when the key is gone.
func (idx *Index) Content(st *store.Store, key string) *Document {
	entry, ok := st.Peek(key)
	if !ok {
		return nil
	}
	doc, ok := idx.Load(key, entry, true)
	if !ok {
		return nil
	}
	return doc
}

// Summary returns the number of documents, distinct terms and term
// postings of the index in st, for FT.INFO.
func (idx *Index) Summary(st *store.Store) (docs, terms, records int) {
	c := idx.cache(st)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncLocked(idx, st)
	for _, list := range c.terms {
		records += len(list)
	}
	return len(c.docs), len(c.terms), records
}

// TagValues returns the distinct values of a TAG field in st, sorted.
func (idx *Index) TagValues(st *store.Store, f *Field) []string {
	c := idx.cache(st)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncLocked(idx, st)
	out := make([]string, 0, len(c.tags[f.pos]))
	for t := range c.tags[f.pos] {
		if t != "" {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

// size estimates the memory a cached document and its postings take.
func (d *Document) size() int64 {
	n := int64(128 + len(d.Key))
	for t := range d.terms {
		n += 48 + int64(len(t))
	}
	for _, f := range d.fields {
		n += 96 + int64(len(f.nums))*8
		for _, t := range f.strs {
			n += 16 + int64(len(t))
		}
		for _, r := range f.raw {
			n += 16 + int64(len(r))
		}
	}
	return n
}

// MemoryUsage estimates the memory the index's caches hold.
func (idx *Index) MemoryUsage() int64 {
	idx.cacheSet.mu.Lock()
	defer idx.cacheSet.mu.Unlock()
	var n int64
	for k, c := range idx.cacheSet.caches {
		if k.Value() != nil {
			n += c.bytes.Load()
		}
	}
	return n
}

// Current reports, for each document, whether its key still holds it in
// st: a write or a delete since the query read it makes it stale.
func (idx *Index) Current(st *store.Store, docs []*Document) []bool {
	c := idx.cache(st)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncLocked(idx, st)
	out := make([]bool, len(docs))
	for i, d := range docs {
		out[i] = c.docs[d.Key] == d
	}
	return out
}
