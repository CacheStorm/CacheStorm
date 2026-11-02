package search

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type qtype uint8

const (
	qPhrase qtype = iota
	qUnion
	qToken
	qPrefix
	qNot
	qOptional
	qNumeric
	qTag
	qGeo
	qFuzzy
	qWildcard
	qVector
	qWildcardQuery
	qMissing
)

// allFields is the field mask of a node no modifier restricts.
const allFields = ^uint64(0)

// Node is a parsed query: a tree of RediSearch query nodes. A nil Node
// matches nothing.
type Node = *qnode

type qnode struct {
	typ      qtype
	children []*qnode
	mask     uint64
	weight   float64
	verbatim bool
	// exact marks a quoted phrase, whose words must follow each other.
	exact bool
	// str is a token's term, an affix's or a fuzzy term's text.
	str      string
	expanded bool
	// prefix and suffix tell an affix's kind: both for an infix.
	prefix, suffix bool
	dist           int
	// field is a numeric, geo or tag node's field name.
	field          string
	lo, hi         float64
	loIncl, hiIncl bool
	lon, lat       float64
	radius         float64
	unit           string
	// terms are an affix's or a fuzzy term's index terms, in order, which
	// a query fills in before matching.
	terms []string
	// A vector query: KNN (k nearest, the root of a query, whose child is
	// the filter) or a range; str is the blob parameter's name, blob its
	// value, yield the field the distance is returned as.
	knn      bool
	k        int
	byScore  bool
	blob     string
	yield    string
	vecAttrs [][2]string
}

func newNode(t qtype) *qnode { return &qnode{typ: t, mask: allFields, weight: 1} }

func tokenNode(s string) *qnode {
	n := newNode(qToken)
	n.str = s
	return n
}

func (n *qnode) add(children ...*qnode) *qnode {
	n.children = append(n.children, children...)
	return n
}

// setMask restricts n and its children to the fields of mask.
func (n *qnode) setMask(mask uint64) {
	if n == nil {
		return
	}
	n.mask &= mask
	for _, c := range n.children {
		c.setMask(mask)
	}
}

// textFieldBit is a TEXT field's bit in field masks; 0 for any other.
func (idx *Index) textFieldBit(name string) uint64 {
	f := idx.Field(name)
	if f == nil || f.Type != FieldText || f.NoIndex {
		return 0
	}
	return fieldBit(f.pos)
}

// expand adds stems to the terms of n, as RediSearch's stemming query
// expander does: a term becomes a union of itself, "+stem" and the stem.
func (idx *Index) expand(pn **qnode) {
	n := *pn
	if n == nil || n.verbatim || n.typ == qPhrase && n.exact || n.typ == qTag {
		return
	}
	if n.mask != allFields && !idx.stemmable(n.mask) {
		return
	}
	if n.typ != qToken {
		for i := range n.children {
			idx.expand(&n.children[i])
		}
		return
	}
	if n.str == "" || idx.Language != "english" {
		return
	}
	st := snowballEnglish(n.str)
	u := n
	if n.typ != qUnion {
		u = newNode(qUnion)
		u.mask = n.mask
		u.add(n)
		*pn = u
	}
	mask := n.mask
	if mask != allFields {
		for i := range idx.Fields {
			if idx.Fields[i].NoStem {
				mask &^= fieldBit(i)
			}
		}
	}
	plus := tokenNode("+" + st)
	plus.expanded, plus.mask = true, mask
	u.add(plus)
	if st != n.str {
		word := tokenNode(st)
		word.expanded, word.mask = true, mask
		u.add(word)
	}
}

// stemmable reports whether mask holds a TEXT field that stems.
func (idx *Index) stemmable(mask uint64) bool {
	for i := range idx.Fields {
		f := &idx.Fields[i]
		if mask&fieldBit(i) != 0 && f.Type == FieldText && !f.NoStem {
			return true
		}
	}
	return false
}

// Explain renders n as FT.EXPLAIN does.
func (idx *Index) Explain(n Node) string {
	if n == nil {
		return "<empty>\n"
	}
	var b strings.Builder
	idx.dump(&b, n, 0)
	return b.String()
}

func pad(b *strings.Builder, depth int) { b.WriteString(strings.Repeat("  ", depth)) }

func (idx *Index) dump(b *strings.Builder, n *qnode, depth int) {
	pad(b, depth)
	if n.mask == 0 {
		b.WriteString("@NULL:")
	}
	if n.mask != 0 && n.mask != allFields && n.typ != qNumeric && n.typ != qGeo {
		b.WriteString("@")
		k := 0
		for i := 0; i < 64; i++ {
			if n.mask&(1<<uint(i)) == 0 {
				continue
			}
			if k > 0 {
				b.WriteString("|")
			}
			name := "n/a"
			for j := range idx.Fields {
				if fieldBit(j) == 1<<uint(i) && idx.Fields[j].Type == FieldText {
					name = idx.Fields[j].Name
					break
				}
			}
			b.WriteString(name)
			k++
		}
		b.WriteString(":")
	}
	children := func() {
		for _, c := range n.children {
			idx.dump(b, c, depth+1)
		}
		pad(b, depth)
		b.WriteString("}")
	}
	switch n.typ {
	case qPhrase:
		if n.exact {
			b.WriteString("EXACT {\n")
		} else {
			b.WriteString("INTERSECT {\n")
		}
		children()
	case qToken:
		if n.str == "" {
			b.WriteString(`""`)
		} else {
			b.WriteString(n.str)
		}
		if n.expanded {
			b.WriteString("(expanded)")
		}
		if n.weight != 1 {
			fmt.Fprintf(b, " => {$weight: %s;}", formatG(n.weight))
		}
		b.WriteString("\n")
		return
	case qPrefix:
		kind := "SUFFIX"
		switch {
		case n.prefix && n.suffix:
			kind = "INFIX"
		case n.prefix:
			kind = "PREFIX"
		}
		b.WriteString(kind + "{")
		if n.suffix {
			b.WriteString("*")
		}
		b.WriteString(n.str)
		if n.prefix {
			b.WriteString("*")
		}
		b.WriteString("}")
	case qNot:
		b.WriteString("NOT{\n")
		children()
	case qOptional:
		b.WriteString("OPTIONAL{\n")
		children()
	case qNumeric:
		fmt.Fprintf(b, "NUMERIC {%s %s @%s %s %s}", cfmtF(n.lo), lessSign(n.loIncl), n.field, lessSign(n.hiIncl), cfmtF(n.hi))
	case qUnion:
		b.WriteString("UNION {\n")
		children()
	case qTag:
		b.WriteString("TAG:@" + n.field + " {\n")
		children()
	case qGeo:
		fmt.Fprintf(b, "GEO %s:{%s,%s --> %s %s}", n.field, cfmtF(n.lon), cfmtF(n.lat), cfmtF(n.radius), n.unit)
	case qWildcard:
		b.WriteString("<WILDCARD>")
	case qWildcardQuery:
		b.WriteString("WILDCARD{" + n.str + "}")
	case qMissing:
		b.WriteString("ISMISSING{" + n.field + "}")
	case qVector:
		b.WriteString("VECTOR {")
		if len(n.children) > 0 {
			b.WriteString("\n")
			for _, c := range n.children {
				idx.dump(b, c, depth+1)
			}
			pad(b, depth)
			b.WriteString("} => {")
		}
		if n.knn {
			fmt.Fprintf(b, "K=%d nearest vectors to `$%s` ", n.k, n.str)
		} else {
			fmt.Fprintf(b, "Vectors that are within %s distance radius from `$%s` ", formatG(n.radius), n.str)
		}
		b.WriteString("in vector index associated with field @" + n.field)
		for _, a := range n.vecAttrs {
			b.WriteString(", " + a[0] + " = " + a[1])
		}
		if n.yield != "" {
			b.WriteString(", yields distance as `" + n.yield + "`")
		}
		b.WriteString("}")
	case qFuzzy:
		b.WriteString("FUZZY{" + n.str + "}")
	}
	if n.weight != 1 {
		fmt.Fprintf(b, " => { $weight: %s; }", formatG(n.weight))
	}
	b.WriteString("\n")
}

func lessSign(incl bool) string {
	if incl {
		return "<="
	}
	return "<"
}

func formatG(f float64) string { return strconv.FormatFloat(f, 'g', 6, 64) }

// cfmtF formats like C's %f.
func cfmtF(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		return "nan"
	}
	return strconv.FormatFloat(f, 'f', 6, 64)
}

// fillTerms gives each affix and fuzzy node in n the index terms it
// covers: the first ones in order, up to RediSearch's limit.
func (c *docCache) fillTerms(n *qnode) {
	if n == nil {
		return
	}
	switch n.typ {
	case qPrefix, qFuzzy, qWildcardQuery:
		if n.terms != nil {
			return
		}
		n.terms = []string{}
		s := strings.ToLower(n.str)
		if n.typ == qPrefix && len(n.str) < minPrefixLength {
			return
		}
		for t := range c.terms {
			if n.typ == qFuzzy && levenshteinWithin(s, t, n.dist) || n.typ == qPrefix && affixMatch(n, s, t) ||
				n.typ == qWildcardQuery && wildcardMatch(s, t) {
				n.terms = append(n.terms, t)
			}
		}
		sort.Strings(n.terms)
		if len(n.terms) > maxPrefixExpansions {
			n.terms = n.terms[:maxPrefixExpansions]
		}
	case qTag:
		return
	}
	for _, ch := range n.children {
		c.fillTerms(ch)
	}
}

func affixMatch(n *qnode, s, t string) bool {
	switch {
	case n.prefix && n.suffix:
		return strings.Contains(t, s)
	case n.prefix:
		return strings.HasPrefix(t, s)
	default:
		return strings.HasSuffix(t, s)
	}
}

// wildcardMatch matches a w'...' pattern: * is any run of characters, ?
// any one character.
func wildcardMatch(pattern, s string) bool {
	p, t := []rune(pattern), []rune(s)
	star, mark := -1, 0
	i, j := 0, 0
	for j < len(t) {
		switch {
		case i < len(p) && (p[i] == '?' || p[i] == t[j]):
			i++
			j++
		case i < len(p) && p[i] == '*':
			star, mark = i, j
			i++
		case star >= 0:
			i, mark = star+1, mark+1
			j = mark
		default:
			return false
		}
	}
	for i < len(p) && p[i] == '*' {
		i++
	}
	return i == len(p)
}

// levenshteinWithin reports whether a and b are at most max edits apart,
// counting runes.
func levenshteinWithin(a, b string, limit int) bool {
	ra, rb := []rune(a), []rune(b)
	if d := len(ra) - len(rb); d > limit || -d > limit {
		return false
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		best := cur[0]
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			best = min(best, cur[j])
		}
		if best > limit {
			return false
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)] <= limit
}

// tagValue is the tag a tag query child names: its text, without escapes,
// in lower case unless the field is case sensitive.
func tagValue(f *Field, s string) string {
	s = unescape(s)
	if !f.CaseSensitive {
		s = strings.ToLower(s)
	}
	return s
}

// unescape drops the backslashes that escape punctuation or spaces.
func unescape(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && isPunctOrSpace(s[i+1]) {
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isPunctOrSpace(c byte) bool {
	return isSpace(c) || isPunct(c)
}

// tagChild is the value a tag query child stands for, and whether it is
// an affix to match tags against.
func tagChild(f *Field, c *qnode) (string, *qnode) {
	switch c.typ {
	case qPrefix:
		return tagValue(f, c.str), c
	case qPhrase:
		words := make([]string, len(c.children))
		for i, w := range c.children {
			words[i] = w.str
		}
		return tagValue(f, strings.Join(words, " ")), nil
	}
	return tagValue(f, c.str), nil
}

// matchTag scores a document's tags against a tag node: each child of the
// union scores on its own, and an affix scores its first tag in order.
func (idx *Index) matchTag(n *qnode, doc *Document, stats *Stats) (bool, float64) {
	f := idx.Field(n.field)
	if f == nil || f.Type != FieldTag {
		return false, 0
	}
	tags := doc.field(f.pos).strs
	matched, total := false, 0.0
	for _, c := range n.children {
		want, affix := tagChild(f, c)
		best, found := "", false
		for _, tag := range tags {
			ok := tag == want
			if affix != nil {
				ok = len(want) >= minPrefixLength && affixMatch(affix, want, tag)
			}
			if ok && (!found || tag < best) {
				best, found = tag, true
			}
		}
		if found {
			matched = true
			total += stats.bm25(stats.idf(stats.tagDocs(f.pos, best)), 1, doc)
		}
	}
	return matched, total * n.weight
}

// textStream is a document's indexed words in position order: RediSearch
// numbers the words of all TEXT fields in one run, so a phrase may span
// two fields.
func (idx *Index) textStream(doc *Document) []string {
	var out []string
	for i := range idx.Fields {
		if idx.Fields[i].Type == FieldText {
			out = append(out, doc.field(i).strs...)
		}
	}
	return out
}

func (idx *Index) exactPhrase(n *qnode, doc *Document) bool {
	words := make([]string, len(n.children))
	for i, c := range n.children {
		if c.typ != qToken {
			return false
		}
		words[i] = c.str
	}
	stream := idx.textStream(doc)
	for j := 0; j+len(words) <= len(stream); j++ {
		all := true
		for k, w := range words {
			all = all && stream[j+k] == w
		}
		if all {
			return true
		}
	}
	return false
}
