package search

import (
	"math"
	"sort"
	"unicode"
)

// Trie is RediSearch's suggestion trie (FT.SUGADD): a radix tree of 16-bit
// runes whose children are kept in order of their best score, searched with
// a Levenshtein automaton for prefixes. It follows RediSearch's trie.c and
// trie_type.c closely, quirks included, so suggestions, their scores and
// their order are Redis'.
type Trie struct {
	root *trieNode
	size int
}

type trieNode struct {
	str      []uint16
	children []*trieNode
	payload  []byte
	score    float32
	maxChild float32
	terminal bool
	deleted  bool
}

// Suggestion is one FT.SUGGET result.
type Suggestion struct {
	Str     []byte
	Payload []byte
	Score   float32
}

const (
	trieMaxString = 256
	trieMaxPrefix = 100
	trieIntMin    = float32(math.MinInt32)
	trieIntMax    = float32(math.MaxInt32)
)

// NewTrie returns an empty trie.
func NewTrie() *Trie { return &Trie{root: &trieNode{}} }

// Size is the number of strings in the trie.
func (t *Trie) Size() int { return t.size }

// utf8Runes decodes s as libnu does, without validation, keeping the low
// 16 bits of each code point and stopping at a NUL; fold folds each code
// point before it is cut to 16 bits.
func utf8Runes(s []byte, fold bool) []uint16 {
	out := make([]uint16, 0, len(s))
	at := func(i int) uint32 {
		if i < len(s) {
			return uint32(s[i])
		}
		return 0
	}
	for i := 0; i < len(s); {
		c := uint32(s[i])
		switch {
		case c < 0x80:
			i++
		case c < 0xE0:
			c = (c&0x1F)<<6 | at(i+1)&0x3F
			i += 2
		case c < 0xF0:
			c = (c&0x0F)<<12 | (at(i+1)&0x3F)<<6 | at(i+2)&0x3F
			i += 3
		default:
			c = (c&0x07)<<18 | (at(i+1)&0x3F)<<12 | (at(i+2)&0x3F)<<6 | at(i+3)&0x3F
			i += 4
		}
		if c == 0 {
			break
		}
		if fold {
			c = foldRune(c)
		}
		out = append(out, uint16(c))
	}
	return out
}

func foldRune(c uint32) uint32 {
	i := sort.Search(len(foldTable), func(i int) bool { return foldTable[i][0] >= c })
	if i < len(foldTable) && foldTable[i][0] == c {
		return foldTable[i][1]
	}
	return c
}

// runesUTF8 encodes 16-bit runes as UTF-8, surrogates included.
func runesUTF8(r []uint16) []byte {
	out := make([]byte, 0, len(r)*3)
	for _, c := range r {
		switch {
		case c < 0x80:
			out = append(out, byte(c))
		case c < 0x800:
			out = append(out, byte(0xC0|c>>6), byte(0x80|c&0x3F))
		default:
			out = append(out, byte(0xE0|c>>12), byte(0x80|c>>6&0x3F), byte(0x80|c&0x3F))
		}
	}
	return out
}

func updateScore(n *trieNode, v float32) {
	if v > n.maxChild {
		n.maxChild = v
	}
}

// Insert adds s, or sets (with incr, adds to) its score; a zero score or
// an empty or over-long string is ignored, as in RediSearch.
func (t *Trie) Insert(s []byte, score float64, incr bool, payload []byte) {
	if len(s) > trieMaxString*2 {
		return
	}
	r := utf8Runes(s, false)
	if len(r) == 0 || len(r) >= trieMaxString {
		return
	}
	t.size += trieAdd(&t.root, r, payload, float32(score), incr)
}

func newTrieNode(str []uint16, score float32, terminal bool, payload []byte) *trieNode {
	n := &trieNode{str: append([]uint16(nil), str...), score: score, maxChild: score, terminal: terminal}
	if len(payload) > 0 {
		n.payload = append([]byte(nil), payload...)
	}
	return n
}

func trieAdd(np **trieNode, str []uint16, payload []byte, score float32, incr bool) int {
	if score == 0 || len(str) == 0 {
		return 0
	}
	n := *np
	offset := 0
	for offset < len(str) && offset < len(n.str) && str[offset] == n.str[offset] {
		offset++
	}
	if offset < len(n.str) {
		n = trieSplit(n, offset)
		if offset == len(str) {
			n.score = score
			n.terminal = true
			n.payload = nil
			if len(payload) > 0 {
				n.payload = append([]byte(nil), payload...)
			}
		} else {
			idx := 0
			if str[offset] > n.children[0].str[0] {
				idx = 1
			}
			trieAddChild(n, str[offset:], payload, score, idx)
			updateScore(n, score)
		}
		*np = n
		return 1
	}
	updateScore(n, score)
	if offset == len(str) {
		existed := n.terminal && !n.deleted
		if incr {
			n.score += score
		} else {
			n.score = score
		}
		if len(payload) > 0 {
			n.payload = append([]byte(nil), payload...)
		}
		n.terminal, n.deleted = true, false
		if existed {
			return 0
		}
		return 1
	}
	scoreIdx := -1
	idx := 0
	for ; idx < len(n.children); idx++ {
		child := n.children[idx]
		if str[offset] == child.str[0] {
			rc := trieAdd(&n.children[idx], str[offset:], payload, score, incr)
			child = n.children[idx]
			if k := len(n.children); k > 1 && (idx > 0 && child.maxChild > n.children[idx-1].maxChild ||
				idx < k-2 && child.maxChild < n.children[idx+1].maxChild) {
				sortTrieChildren(n)
			}
			return rc
		}
		if child.maxChild < score && scoreIdx == -1 {
			scoreIdx = idx
		}
	}
	if scoreIdx != -1 {
		idx = scoreIdx
	}
	trieAddChild(n, str[offset:], payload, score, idx)
	return 1
}

func trieAddChild(n *trieNode, str []uint16, payload []byte, score float32, idx int) {
	child := newTrieNode(str, score, true, payload)
	n.children = append(n.children, nil)
	copy(n.children[idx+1:], n.children[idx:])
	n.children[idx] = child
}

// trieSplit makes n's string end at offset, its rest a single child.
func trieSplit(n *trieNode, offset int) *trieNode {
	child := &trieNode{str: append([]uint16(nil), n.str[offset:]...), children: n.children, payload: n.payload,
		score: n.score, maxChild: n.maxChild, terminal: n.terminal, deleted: n.deleted}
	n.payload = nil
	n.children = []*trieNode{child}
	n.str = n.str[:offset:offset]
	n.score = 0
	n.terminal, n.deleted = false, false
	updateScore(n, child.score)
	return n
}

func runeCmp(a, b []uint16) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if d := int(a[i]) - int(b[i]); d != 0 {
			return d
		}
	}
	return len(a) - len(b)
}

func sortTrieChildren(n *trieNode) {
	sort.SliceStable(n.children, func(i, j int) bool {
		a, b := n.children[i], n.children[j]
		if a.maxChild != b.maxChild {
			return a.maxChild > b.maxChild
		}
		return runeCmp(a.str, b.str) < 0
	})
}

// Delete removes s and reports whether it was there. Like RediSearch, it
// tidies the nodes along the path even when s is missing.
func (t *Trie) Delete(s []byte) int {
	r := utf8Runes(s, false)
	if len(r) > trieMaxString {
		return 0
	}
	var stack []*trieNode
	rc := 0
	n, offset := t.root, 0
	for n != nil && offset < len(r) {
		stack = append(stack, n)
		local := 0
		for offset < len(r) && local < len(n.str) && r[offset] == n.str[local] {
			offset++
			local++
		}
		if offset == len(r) {
			if local == len(n.str) && !n.deleted && n.terminal {
				n.deleted, n.terminal, n.score = true, false, 0
				rc = 1
			}
			break
		}
		if local != len(n.str) {
			break
		}
		var next *trieNode
		for _, c := range n.children {
			if c.str[0] == r[offset] {
				next = c
				break
			}
		}
		n = next
	}
	for i := len(stack) - 1; i >= 0; i-- {
		optimizeTrieChildren(stack[i])
	}
	t.size -= rc
	return rc
}

// optimizeTrieChildren drops a deleted leaf child and merges children
// left with one child into it. As in RediSearch, the pass ends at the first
// child it drops.
func optimizeTrieChildren(n *trieNode) {
	n.maxChild = n.score
	for i := 0; i < len(n.children); i++ {
		c := n.children[i]
		if len(c.children) == 0 && c.deleted {
			n.children = append(n.children[:i], n.children[i+1:]...)
			for _, rest := range n.children[i:] {
				updateScore(n, rest.maxChild)
			}
			break
		}
		if len(c.children) == 1 {
			n.children[i] = mergeTrieChild(c)
		}
		updateScore(n, n.children[i].maxChild)
	}
	sortTrieChildren(n)
}

func mergeTrieChild(n *trieNode) *trieNode {
	if n.terminal || len(n.children) != 1 {
		return n
	}
	ch := n.children[0]
	str := make([]uint16, 0, len(n.str)+len(ch.str))
	str = append(append(str, n.str...), ch.str...)
	return &trieNode{str: str, children: ch.children, payload: ch.payload, score: ch.score,
		maxChild: ch.maxChild, terminal: ch.terminal, deleted: ch.deleted}
}

// Each calls fn for every string in the trie, in the order RediSearch saves
// them.
func (t *Trie) Each(fn func(s []byte, score float32, payload []byte)) {
	it := newTrieIter(t.root, nil)
	for {
		r, n, ok := it.next(nil)
		if !ok {
			return
		}
		fn(runesUTF8(r), n.score, n.payload)
	}
}

// Clone copies the trie.
func (t *Trie) Clone() *Trie {
	var cp func(n *trieNode) *trieNode
	cp = func(n *trieNode) *trieNode {
		c := *n
		c.str = append([]uint16(nil), n.str...)
		c.payload = append([]byte(nil), n.payload...)
		if n.payload == nil {
			c.payload = nil
		}
		c.children = make([]*trieNode, len(n.children))
		for i, ch := range n.children {
			c.children[i] = cp(ch)
		}
		return &c
	}
	return &Trie{root: cp(t.root), size: t.size}
}

// Bytes estimates the trie's memory.
func (t *Trie) Bytes() int64 {
	var walk func(n *trieNode) int64
	walk = func(n *trieNode) int64 {
		b := int64(64 + 2*len(n.str) + 8*len(n.children) + len(n.payload))
		for _, c := range n.children {
			b += walk(c)
		}
		return b
	}
	return walk(t.root)
}

const (
	iterSelf = iota
	iterChildren
	iterMatch
)

const (
	stepStop = iota
	stepCont
	stepMatch
)

type trieFrame struct {
	n        *trieNode
	strOff   int
	childOff int
	state    int
}

type trieIter struct {
	stack    []trieFrame
	buf      []uint16
	filter   *dfaFilter
	minScore float32
}

func newTrieIter(root *trieNode, f *dfaFilter) *trieIter {
	it := &trieIter{filter: f, minScore: trieIntMin}
	it.push(root)
	return it
}

func (it *trieIter) push(n *trieNode) {
	if len(it.stack) < trieMaxString-1 {
		it.stack = append(it.stack, trieFrame{n: n})
	}
}

func (it *trieIter) pop() {
	if len(it.stack) == 0 {
		return
	}
	cur := it.stack[len(it.stack)-1]
	if it.filter != nil {
		it.filter.pop(cur.strOff)
	}
	it.buf = it.buf[:len(it.buf)-cur.strOff]
	it.stack = it.stack[:len(it.stack)-1]
}

func (it *trieIter) step(dist *int) int {
	if len(it.stack) == 0 {
		return stepStop
	}
	cur := &it.stack[len(it.stack)-1]
	switch cur.state {
	case iterMatch:
		it.pop()
		return stepCont
	case iterSelf:
		if cur.strOff < len(cur.n.str) {
			b := cur.n.str[cur.strOff]
			matched := false
			if it.filter != nil {
				var stop bool
				stop, matched = it.filter.step(b, dist)
				if stop {
					if matched {
						cur.state = iterMatch
						return stepMatch
					}
					it.pop()
					return stepCont
				}
			}
			it.buf = append(it.buf, b)
			cur.strOff++
			if it.filter == nil && cur.strOff == len(cur.n.str) && cur.n.terminal && !cur.n.deleted {
				matched = true
			}
			if matched {
				return stepMatch
			}
			return stepCont
		}
		cur.state = iterChildren
	}
	if cur.childOff < len(cur.n.children) {
		ch := cur.n.children[cur.childOff]
		cur.childOff++
		if ch.maxChild >= it.minScore || ch.score >= it.minScore {
			it.push(ch)
		}
	} else {
		it.pop()
	}
	return stepCont
}

// next returns the next string the iterator matches and its node.
func (it *trieIter) next(dist *int) ([]uint16, *trieNode, bool) {
	for {
		rc := it.step(dist)
		if rc == stepStop {
			return nil, nil, false
		}
		if rc == stepMatch {
			cur := it.stack[len(it.stack)-1]
			if cur.n.terminal && len(cur.n.str) == cur.strOff && !cur.n.deleted {
				return it.buf, cur.n, true
			}
		}
	}
}

// Search returns up to num suggestions for the prefix q within maxDist
// edits, best first, with RediSearch's scores; ok is false for a prefix
// RediSearch refuses.
func (t *Trie) Search(q []byte, num, maxDist int, trim bool) ([]Suggestion, bool) {
	if len(q) > trieMaxPrefix*2 {
		return nil, false
	}
	runes := utf8Runes(q, true)
	if len(runes) >= trieMaxPrefix {
		return nil, false
	}
	h := &suggestHeap{size: num}
	it := newTrieIter(t.root, newDFAFilter(runes, maxDist, true))
	dist := maxDist + 1
	for {
		r, n, ok := it.next(&dist)
		if !ok {
			break
		}
		score := n.score
		if len(r) > 0 && len(r) == len(runes) && sameLeadingBytes(runes, r, len(r)) {
			score = trieIntMax
		}
		if maxDist > 0 {
			score = float32(float64(score) * math.Exp(float64(-(2 * dist))))
		}
		diff := len(r) - len(q)
		if diff < 0 {
			diff = -diff
		}
		score = float32(float64(score) / math.Sqrt(float64(1+diff)))
		e := &Suggestion{Score: score}
		if len(h.items) < h.size {
			e.Str, e.Payload = runesUTF8(r), n.payload
			h.offer(e)
			if len(h.items) == h.size {
				it.minScore = h.items[0].Score
			}
		} else if e.Score > it.minScore {
			h.poll()
			e.Str, e.Payload = runesUTF8(r), n.payload
			h.offer(e)
			if top := h.items[0].Score; top > it.minScore {
				it.minScore = top
			}
		}
	}
	out := make([]Suggestion, len(h.items))
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = *h.poll()
	}
	if trim {
		var maxScore float32
		for i, s := range out {
			if maxScore != 0 && float64(s.Score) < float64(maxScore)/10.0 {
				out = out[:i]
				break
			}
			maxScore = max(maxScore, s.Score)
		}
	}
	return out, true
}

// sameLeadingBytes is RediSearch's exact-match test, a memcmp of n bytes of
// the two rune arrays: it compares about half the runes.
func sameLeadingBytes(a, b []uint16, n int) bool {
	for i := 0; i < n; i++ {
		x, y := a[i/2], b[i/2]
		if i%2 == 1 {
			x, y = x>>8, y>>8
		}
		if byte(x) != byte(y) {
			return false
		}
	}
	return true
}

// suggestHeap is RediSearch's binary heap with its comparator, the lowest
// score on top; ties fall as its sift steps leave them.
type suggestHeap struct {
	items []*Suggestion
	size  int
}

func suggestCmp(a, b *Suggestion) int {
	switch {
	case a.Score < b.Score:
		return 1
	case a.Score > b.Score:
		return -1
	}
	return 0
}

func (h *suggestHeap) offer(e *Suggestion) {
	h.items = append(h.items, e)
	for i := len(h.items) - 1; i != 0; {
		p := (i - 1) / 2
		if suggestCmp(h.items[i], h.items[p]) < 0 {
			return
		}
		h.items[i], h.items[p] = h.items[p], h.items[i]
		i = p
	}
}

func (h *suggestHeap) poll() *Suggestion {
	top := h.items[0]
	last := len(h.items) - 1
	h.items[0] = h.items[last]
	h.items = h.items[:last]
	if len(h.items) > 1 {
		for i := 0; ; {
			l, r := 2*i+1, 2*i+2
			var c int
			switch {
			case r >= len(h.items):
				if l >= len(h.items) {
					return top
				}
				c = l
			case suggestCmp(h.items[l], h.items[r]) < 0:
				c = r
			default:
				c = l
			}
			if suggestCmp(h.items[i], h.items[c]) < 0 {
				h.items[i], h.items[c] = h.items[c], h.items[i]
				i = c
			} else {
				return top
			}
		}
	}
	return top
}

// The Levenshtein automaton of RediSearch's levenshtein.c: sparse state
// vectors, made into a DFA whose nodes are shared by equal states.
type svEntry struct{ idx, val int }

type dfaNode struct {
	v        []svEntry
	edges    []dfaEdge
	fallback *dfaNode
	distance int
	match    bool
}

type dfaEdge struct {
	n *dfaNode
	r uint16
}

type dfaFilter struct {
	s          []uint16
	cache      []*dfaNode
	stack      []*dfaNode
	dists      []int
	max        int
	prefixMode bool
	// lower makes the filter lower-case the trie's runes, as RediSearch's
	// term trie filter does, instead of case folding them.
	lower bool
}

func (f *dfaFilter) stepVec(state []svEntry, c uint16) []svEntry {
	var nv []svEntry
	if len(state) > 0 && state[0].idx == 0 && state[0].val < f.max {
		nv = append(nv, svEntry{0, state[0].val + 1})
	}
	for j, e := range state {
		if e.idx == len(f.s) {
			break
		}
		val := e.val
		if f.s[e.idx] != c {
			val++
		}
		if len(nv) > 0 && nv[len(nv)-1].idx == e.idx {
			val = min(val, nv[len(nv)-1].val+1)
		}
		if j+1 < len(state) && state[j+1].idx == e.idx+1 {
			val = min(val, state[j+1].val+1)
		}
		if val <= f.max {
			nv = append(nv, svEntry{e.idx + 1, val})
		}
	}
	return nv
}

func (f *dfaFilter) cached(v []svEntry) *dfaNode {
	for _, d := range f.cache {
		if len(d.v) == len(v) {
			same := true
			for i := range v {
				if d.v[i] != v[i] {
					same = false
					break
				}
			}
			if same {
				return d
			}
		}
	}
	return nil
}

func (d *dfaNode) edge(r uint16) *dfaNode {
	for _, e := range d.edges {
		if e.r == r {
			return e.n
		}
	}
	return nil
}

func (f *dfaFilter) newNode(v []svEntry) *dfaNode {
	d := &dfaNode{v: v, distance: v[len(v)-1].val}
	f.cache = append(f.cache, d)
	return d
}

func (f *dfaFilter) build(parent *dfaNode) {
	parent.match = len(parent.v) > 0 && parent.v[len(parent.v)-1].idx == len(f.s)
	for _, e := range parent.v {
		if e.idx >= len(f.s) {
			continue
		}
		c := f.s[e.idx]
		if parent.edge(c) != nil {
			continue
		}
		nv := f.stepVec(parent.v, c)
		if len(nv) == 0 {
			continue
		}
		if d := f.cached(nv); d != nil {
			parent.edges = append(parent.edges, dfaEdge{d, c})
			continue
		}
		d := f.newNode(nv)
		parent.edges = append(parent.edges, dfaEdge{d, c})
		f.build(d)
	}
	nv := f.stepVec(parent.v, 1)
	if len(nv) > 0 {
		if d := f.cached(nv); d != nil {
			parent.fallback = d
		} else {
			parent.fallback = f.newNode(nv)
			f.build(parent.fallback)
		}
	}
}

func newDFAFilter(s []uint16, maxDist int, prefixMode bool) *dfaFilter {
	f := &dfaFilter{s: s, max: maxDist, prefixMode: prefixMode}
	start := make([]svEntry, maxDist+1)
	for i := range start {
		start[i] = svEntry{i, i}
	}
	root := &dfaNode{v: start}
	f.cache = append(f.cache, root)
	f.build(root)
	f.stack = []*dfaNode{root}
	f.dists = []int{maxDist + 1}
	return f
}

// step feeds the filter a rune of the trie, folded.
func (f *dfaFilter) step(b uint16, dist *int) (stop, matched bool) {
	dn := f.stack[len(f.stack)-1]
	minDist := f.dists[len(f.dists)-1]
	if dn == nil {
		f.stack = append(f.stack, nil)
		f.dists = append(f.dists, minDist)
		return false, true
	}
	matched = dn.match
	if matched {
		*dist = min(dn.distance, minDist)
	}
	var r uint16
	if f.lower {
		r = uint16(unicode.ToLower(rune(b)))
	} else {
		r = uint16(foldRune(uint32(b)))
	}
	next := dn.edge(r)
	if next == nil {
		next = dn.fallback
	}
	if next != nil {
		if next.match {
			matched = true
			*dist = min(next.distance, minDist)
		}
		f.stack = append(f.stack, next)
		f.dists = append(f.dists, min(next.distance, minDist))
		return false, matched
	}
	if f.prefixMode && matched {
		f.stack = append(f.stack, nil)
		f.dists = append(f.dists, minDist)
		return false, matched
	}
	return true, matched
}

func (f *dfaFilter) pop(n int) {
	f.stack = f.stack[:len(f.stack)-n]
	f.dists = f.dists[:len(f.dists)-n]
}

// matchWord runs a word through the filter as the trie iterator does and
// reports whether the iterator returns it. The state before the word's last
// rune counts too, so a word whose last rune is one edit too many still
// matches, as in RediSearch.
func (f *dfaFilter) matchWord(word []uint16) bool {
	f.stack, f.dists = f.stack[:1], f.dists[:1]
	matched := false
	for _, b := range word {
		var dist int
		stop, m := f.step(b, &dist)
		if stop {
			return false
		}
		matched = m
	}
	return len(word) > 0 && matched
}
