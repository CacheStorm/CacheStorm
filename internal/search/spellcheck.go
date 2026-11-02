package search

import (
	"sort"
	"strings"

	"github.com/cachestorm/cachestorm/internal/store"
)

// The spell-check dictionaries (FT.DICTADD) belong to the server, not to an
// index, as in RediSearch; FLUSHALL clears them.

func (m *IndexManager) dictsLocked() map[string]map[string]bool {
	if m.dicts == nil {
		m.dicts = make(map[string]map[string]bool)
	}
	return m.dicts
}

// DictAdd adds terms to a dictionary and returns how many were new. Like
// RediSearch's trie, it ignores empty and over-long terms.
func (m *IndexManager) DictAdd(name string, terms []string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	dicts := m.dictsLocked()
	d := dicts[name]
	if d == nil {
		d = make(map[string]bool)
		dicts[name] = d
	}
	added := 0
	for _, t := range terms {
		if !dictStorable(t) || d[t] {
			continue
		}
		d[t] = true
		added++
	}
	return added
}

func dictStorable(t string) bool {
	if len(t) > trieMaxString*2 {
		return false
	}
	n := len(utf8Runes([]byte(t), false))
	return n > 0 && n < trieMaxString
}

// DictDel deletes terms from a dictionary and returns how many it held;
// an emptied dictionary goes.
func (m *IndexManager) DictDel(name string, terms []string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := m.dictsLocked()[name]
	if d == nil {
		return 0
	}
	n := 0
	for _, t := range terms {
		if d[t] {
			delete(d, t)
			n++
		}
	}
	if len(d) == 0 {
		delete(m.dicts, name)
	}
	return n
}

// DictDump returns a dictionary's terms in RediSearch's trie order.
func (m *IndexManager) DictDump(name string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return sortedTerms(m.dicts[name])
}

func sortedTerms(d map[string]bool) []string {
	out := make([]string, 0, len(d))
	for t := range d {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// DictExists reports whether a dictionary has terms.
func (m *IndexManager) DictExists(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.dicts[name]) > 0
}

// SpellTerm is one query term FT.SPELLCHECK reports: Found when the index
// holds it (shown with FULLSCOREINFO), else its suggestions, best first,
// with their scores.
type SpellTerm struct {
	Term        string
	Found       bool
	Suggestions []string
	Scores      []float64
}

// SpellCheck checks the query's terms as RediSearch does: a term the index
// lacks and no EXCLUDE dictionary holds gets the index terms (stems and
// synonym groups included) and INCLUDE dictionary terms within distance
// edits, scored by how many documents hold them, divided by the number of
// documents unless full.
func (m *IndexManager) SpellCheck(idx *Index, st *store.Store, n Node, distance int, include, exclude []string, full bool) ([]SpellTerm, int) {
	m.mu.RLock()
	includeDicts := make([][]string, 0, len(include))
	for _, name := range include {
		includeDicts = append(includeDicts, sortedTerms(m.dicts[name]))
	}
	excludeDicts := make([]map[string]bool, 0, len(exclude))
	for _, name := range exclude {
		excludeDicts = append(excludeDicts, lowerSet(m.dicts[name]))
	}
	m.mu.RUnlock()

	c := idx.cache(st)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncLocked(idx, st)
	total := len(c.docs)
	terms := make([]string, 0, len(c.terms))
	for t := range c.terms {
		if t != "" {
			terms = append(terms, t)
		}
	}
	sort.Strings(terms)
	divisor := float64(total)
	if full || total == 0 {
		divisor = 1
	}
	var out []SpellTerm
	var walk func(q *qnode)
	walk = func(q *qnode) {
		if q == nil {
			return
		}
		if q.typ == qToken && !q.expanded && q.str != "" {
			if r, ok := c.spellTerm(q, terms, includeDicts, excludeDicts, distance, divisor, full); ok {
				out = append(out, r)
			}
		}
		for _, ch := range q.children {
			walk(ch)
		}
	}
	walk(n)
	return out, total
}

func lowerSet(d map[string]bool) map[string]bool {
	out := make(map[string]bool, len(d))
	for t := range d {
		out[strings.ToLower(t)] = true
	}
	return out
}

func (c *docCache) spellTerm(q *qnode, terms []string, include [][]string, exclude []map[string]bool, distance int, divisor float64, full bool) (SpellTerm, bool) {
	term := strings.ToLower(q.str)
	if _, ok := c.terms[term]; ok {
		return SpellTerm{Term: q.str, Found: true}, full
	}
	for _, d := range exclude {
		if d[term] {
			return SpellTerm{}, false
		}
	}
	sugg := NewTrie()
	// exists is RediSearch's case-insensitive lookup of a suggestion, with
	// the score it holds.
	exists := func(s string) (float32, bool) {
		var score float32
		found := false
		sugg.Each(func(b []byte, sc float32, _ []byte) {
			if !found && strings.EqualFold(string(b), s) {
				score, found = sc, true
			}
		})
		return score, found
	}
	add := func(s string, score float64, incr bool) {
		cur, ok := exists(s)
		if score == 0 {
			score = -1
		}
		if !incr {
			if !ok {
				sugg.Insert([]byte(s), score, false, nil)
			}
			return
		}
		if ok && score == -1 {
			return
		}
		if !ok || cur == -1 {
			incr = false
		}
		sugg.Insert([]byte(s), score, incr, nil)
	}
	score := func(s string) float64 {
		docs, ok := c.terms[s]
		if !ok {
			return 0
		}
		for _, d := range docs {
			if d.terms[s].mask&q.mask != 0 {
				return float64(len(docs))
			}
		}
		return -1
	}
	filter := newDFAFilter(utf8Runes([]byte(term), false), distance, false)
	filter.lower = true
	near := func(t string) bool { return filter.matchWord(utf8Runes([]byte(t), false)) }
	for _, t := range terms {
		if near(t) {
			if s := score(t); s != -1 {
				add(t, s, true)
			}
		}
	}
	for _, d := range include {
		for _, t := range d {
			if near(t) {
				if s := score(t); s != -1 {
					add(t, s, false)
				}
			}
		}
	}
	r := SpellTerm{Term: q.str}
	type entry struct {
		s     string
		score float32
	}
	var found []entry
	sugg.Each(func(b []byte, sc float32, _ []byte) { found = append(found, entry{string(b), sc}) })
	sort.SliceStable(found, func(i, j int) bool { return found[i].score > found[j].score })
	for _, e := range found {
		r.Suggestions = append(r.Suggestions, e.s)
		if e.score == -1 {
			r.Scores = append(r.Scores, 0)
		} else {
			r.Scores = append(r.Scores, float64(e.score)/divisor)
		}
	}
	return r, true
}

// DictCommands are the FT.DICTADD commands that rebuild the dictionaries.
func (m *IndexManager) dictCommandsLocked() [][]string {
	var out [][]string
	names := make([]string, 0, len(m.dicts))
	for name := range m.dicts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out = append(out, append([]string{"FT.DICTADD", name}, sortedTerms(m.dicts[name])...))
	}
	return out
}
