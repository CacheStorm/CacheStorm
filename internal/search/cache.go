package search

import (
	"strings"
	"sync"
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
	dirty   map[string]struct{}
	stats   *Stats
	// The postings: documents by term stem, by token (for prefixes) and by
	// tag value of each TAG field.
	stems  map[string]docSet
	tokens map[string]docSet
	tags   map[string]map[string]docSet
}

type docSet map[*Document]struct{}

func (c *docCache) reset() {
	c.docs = make(map[string]*Document)
	c.stats = NewStats(nil)
	c.stems = make(map[string]docSet)
	c.tokens = make(map[string]docSet)
	c.tags = make(map[string]map[string]docSet)
}

func addPosting(m map[string]docSet, k string, d *Document) {
	set := m[k]
	if set == nil {
		set = make(docSet)
		m[k] = set
	}
	set[d] = struct{}{}
}

func dropPosting(m map[string]docSet, k string, d *Document) {
	if set := m[k]; set != nil {
		delete(set, d)
		if len(set) == 0 {
			delete(m, k)
		}
	}
}

// index adds (add) or removes a document and its postings.
func (c *docCache) index(d *Document, add bool) {
	if add {
		c.docs[d.Key] = d
		c.stats.add(d, 1)
	} else {
		delete(c.docs, d.Key)
		c.stats.add(d, -1)
	}
	for st := range d.terms {
		if add {
			addPosting(c.stems, st, d)
		} else {
			dropPosting(c.stems, st, d)
		}
	}
	for _, toks := range d.tokens {
		for _, t := range toks {
			if add {
				addPosting(c.tokens, t, d)
			} else {
				dropPosting(c.tokens, t, d)
			}
		}
	}
	for field, tags := range d.tags {
		m := c.tags[field]
		if m == nil {
			m = make(map[string]docSet)
			c.tags[field] = m
		}
		for _, t := range tags {
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
		c = &docCache{dirty: make(map[string]struct{})}
		c.reset()
		idx.cacheSet.caches[key] = c
	}
	return c
}

func (idx *Index) cache(st *store.Store) *docCache { return idx.lookup(st, true) }

// maxDirty bounds the keys marked between two queries: past it the cache
// is rebuilt on the next query instead of growing without limit.
const maxDirty = 1 << 16

// Touch marks key as written in st. It creates nothing: a store no query
// has read yet has no cache to keep current.
func (idx *Index) Touch(st *store.Store, key string) {
	if !idx.Covers(key) {
		return
	}
	c := idx.lookup(st, false)
	if c == nil {
		return
	}
	c.dirtyMu.Lock()
	if c.built {
		if len(c.dirty) >= maxDirty {
			c.built = false
			clear(c.dirty)
		} else {
			c.dirty[key] = struct{}{}
		}
	}
	c.dirtyMu.Unlock()
}

// Documents returns the index's documents in st and their corpus
// statistics. The caller must not change them.
func (idx *Index) Documents(st *store.Store) ([]*Document, *Stats) {
	c := idx.cache(st)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncLocked(idx, st)
	docs := make([]*Document, 0, len(c.docs))
	for _, d := range c.docs {
		docs = append(docs, d)
	}
	return docs, c.stats
}

// syncLocked brings the cache up to date with st.
func (c *docCache) syncLocked(idx *Index, st *store.Store) {
	c.dirtyMu.Lock()
	built := c.built
	dirty := c.dirty
	c.dirty = make(map[string]struct{})
	c.built = true
	c.dirtyMu.Unlock()
	if gen := st.Generation(); !built || c.generation != gen {
		c.reset()
		st.ForEachLive(func(key string, entry *store.Entry) error { //nolint:errcheck // the callback never fails
			if idx.Covers(key) {
				if doc, ok := idx.Load(key, entry); ok {
					c.index(doc, true)
				}
			}
			return nil
		})
		c.generation = gen
	}
	for key := range dirty {
		c.refreshLocked(idx, st, key)
	}
}

func (c *docCache) refreshLocked(idx *Index, st *store.Store, key string) *Document {
	if old, ok := c.docs[key]; ok {
		c.index(old, false)
	}
	entry, ok := st.Peek(key)
	if !ok {
		return nil
	}
	doc, ok := idx.Load(key, entry)
	if !ok {
		return nil
	}
	c.index(doc, true)
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

// Hit is a document a query matched and its score.
type Hit struct {
	Doc   *Document
	Score float64
}

// Search runs a query over the index's documents in st: the postings give
// the candidates, which are then matched and scored.
func (idx *Index) Search(st *store.Store, n Node, opts QueryOptions) []Hit {
	c := idx.cache(st)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncLocked(idx, st)
	var hits []Hit
	match := func(d *Document) {
		if ok, score := idx.Match(n, d, opts, c.stats); ok {
			hits = append(hits, Hit{d, score})
		}
	}
	if cand, all := c.candidates(idx, n); all {
		for _, d := range c.docs {
			match(d)
		}
	} else {
		for d := range cand {
			match(d)
		}
	}
	return hits
}

// candidates returns a superset of the documents n can match; all is true
// when the postings cannot narrow it.
func (c *docCache) candidates(idx *Index, n Node) (docSet, bool) {
	switch n := n.(type) {
	case emptyNode:
		return docSet{}, false
	case *termNode:
		if n.phrase != nil {
			var out docSet
			for _, w := range n.phrase {
				out = intersect(out, c.stems[Stem(w)], out == nil)
			}
			return out, false
		}
		if n.prefix {
			out := make(docSet)
			for t, set := range c.tokens {
				if strings.HasPrefix(t, n.text) {
					for d := range set {
						out[d] = struct{}{}
					}
				}
			}
			return out, false
		}
		out := make(docSet)
		for _, k := range []string{n.stem, Stem(n.text)} {
			for d := range c.stems[k] {
				out[d] = struct{}{}
			}
		}
		return out, false
	case *tagNode:
		f := idx.Field(n.field)
		if f == nil {
			return docSet{}, false
		}
		out := make(docSet)
		for _, want := range n.values {
			prefix := strings.HasSuffix(want, "*")
			want = strings.TrimSuffix(want, "*")
			if !f.CaseSensitive {
				want = strings.ToLower(want)
			}
			for t, set := range c.tags[f.Name] {
				if t == want || prefix && strings.HasPrefix(t, want) {
					for d := range set {
						out[d] = struct{}{}
					}
				}
			}
		}
		return out, false
	case *andNode:
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
	case *orNode:
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
		out := make(docSet, len(b))
		for d := range b {
			out[d] = struct{}{}
		}
		return out
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
