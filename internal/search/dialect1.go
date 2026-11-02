package search

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// qtoken is a lexer token as RediSearch's lexer fills it in: one struct
// that each token updates, so a punctuation token keeps the text of the
// word before it, which syntax errors quote.
type qtoken struct {
	s   string
	pos int
	num float64
}

type lexer struct {
	q         string
	pos       int
	tok       qtoken
	stopwords map[string]bool
}

func isPunct(c byte) bool {
	return c > ' ' && c < 0x7f && (c < '0' || c > '9') && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z')
}

func isSpace(c byte) bool { return c == ' ' || c >= '\t' && c <= '\r' }

func isCntrl(c byte) bool { return c < ' ' || c == 0x7f }

// termEnd is the end of a term starting at i: letters, digits, '_', other
// bytes, and escaped punctuation or spaces.
func (l *lexer) termEnd(i int) int {
	j := i
	for j < len(l.q) {
		c := l.q[j]
		switch {
		case c == '\\':
			if j+1 < len(l.q) && (isPunct(l.q[j+1]) || isSpace(l.q[j+1])) {
				j += 2
				continue
			}
			return j
		case c == '_' || !isPunct(c) && !isCntrl(c) && !isSpace(c):
			j++
		default:
			return j
		}
	}
	return j
}

func digitsEnd(q string, i int) int {
	for i < len(q) && q[i] >= '0' && q[i] <= '9' {
		i++
	}
	return i
}

// numberEnd is the end of a number at i, or i: -?d+(.d+)?([eE]-?d+)?
func numberEnd(q string, i int) int {
	j := i
	if j < len(q) && q[j] == '-' {
		j++
	}
	k := digitsEnd(q, j)
	if k == j {
		return i
	}
	j = k
	if j+1 < len(q) && q[j] == '.' {
		if k := digitsEnd(q, j+1); k > j+1 {
			j = k
		}
	}
	if j < len(q) && (q[j] == 'e' || q[j] == 'E') {
		e := j + 1
		if e < len(q) && q[e] == '-' {
			e++
		}
		if k := digitsEnd(q, e); k > e {
			j = k
		}
	}
	return j
}

var punctTokens = map[byte]int{
	'"': tokQUOTE, '|': tokOR, '(': tokLP, ')': tokRP, '{': tokLB, '}': tokRB, ':': tokCOLON,
	';': tokSEMICOLON, '-': tokMINUS, '~': tokTILDE, '*': tokSTAR, '%': tokPERCENT, '[': tokLSQB, ']': tokRSQB,
}

// next returns the next token: the longest match among RediSearch's
// token patterns, the higher priority on a tie.
func (l *lexer) next() int {
	for l.pos < len(l.q) {
		i, q := l.pos, l.q
		best, kind, prio := i, -1, -1
		consider := func(end, k, p int) {
			if end > best || end == best && end > i && p > prio {
				best, kind, prio = end, k, p
			}
		}
		consider(numberEnd(q, i), tokNUMBER, 2)
		if q[i] == '@' {
			if e := l.termEnd(i + 1); e > i+1 {
				consider(e, tokMODIFIER, 1)
			}
		}
		if q[i] == '$' {
			if e := l.termEnd(i + 1); e > i+1 {
				consider(e, tokATTRIBUTE, 1)
			}
		}
		if strings.HasPrefix(q[i:], "=>") {
			consider(i+2, tokARROW, 0)
		}
		inf := i
		if inf < len(q) && (q[inf] == '+' || q[inf] == '-') {
			inf++
		}
		if strings.HasPrefix(q[inf:], "inf") {
			consider(inf+3, -2, 3)
		}
		if k, ok := punctTokens[q[i]]; ok {
			consider(i+1, k, 0)
		}
		term := l.termEnd(i)
		if term > i {
			consider(term, tokTERM, 0)
		}
		word := max(term, numberEnd(q, i))
		if word > i && word < len(q) && q[word] == '*' {
			consider(word+1, tokPREFIX, 1)
		}
		if q[i] == '*' && i+1 < len(q) {
			w := max(l.termEnd(i+1), numberEnd(q, i+1))
			if w > i+1 {
				consider(w, tokSUFFIX, 1)
				if w < len(q) && q[w] == '*' {
					consider(w+1, tokCONTAINS, 1)
				}
			}
		}
		if kind < 0 && kind != -2 {
			l.pos++
			continue
		}
		l.pos = best
		text := q[i:best]
		switch kind {
		case tokNUMBER:
			l.tok = qtoken{s: text, pos: i}
			if f, err := strconv.ParseFloat(text, 64); err == nil || errors.Is(err, strconv.ErrRange) {
				l.tok.num = f
			}
		case -2:
			l.tok = qtoken{s: text, pos: i, num: math.Inf(1)}
			if text[0] == '-' {
				l.tok.num = math.Inf(-1)
			}
			kind = tokNUMBER
		case tokMODIFIER, tokATTRIBUTE:
			l.tok.s, l.tok.pos = text[1:], i
		case tokARROW:
			// RediSearch's lexer points past the "=" with the length of
			// "=>", so the text runs one byte beyond the arrow.
			l.tok.s, l.tok.pos = q[i+1:min(i+3, len(q))], i
		case tokTERM:
			l.tok = qtoken{s: text, pos: i}
			if l.stopwords[strings.ToLower(text)] {
				kind = tokSTOPWORD
			}
		case tokPREFIX:
			l.tok = qtoken{s: text[:len(text)-1], pos: i}
		case tokSUFFIX:
			l.tok = qtoken{s: text[1:], pos: i}
		case tokCONTAINS:
			l.tok = qtoken{s: text[1 : len(text)-1], pos: i}
		default:
			l.tok.pos = i
		}
		return kind
	}
	return tokEOF
}

// pval is a parser stack value.
type pval struct {
	tok   qtoken
	node  *qnode
	mods  []string
	num   float64
	incl  bool
	attrs [][2]string
}

type parser1 struct {
	idx  *Index
	opts QueryOptions
	root *qnode
	err  error
}

// parseDialect1 parses a query with the tables of RediSearch's DIALECT 1
// grammar.
func parseDialect1(idx *Index, q string, opts QueryOptions) (Node, error) {
	stop := idx.Stopwords
	if opts.NoStopwords {
		stop = nil
	}
	lx := &lexer{q: q, stopwords: stop}
	p := &parser1{idx: idx, opts: opts}
	states := []int16{0}
	var vals []pval
	kind := lx.next()
	for {
		act := parseActions[states[len(states)-1]][kind]
		if act == 0 {
			act = -int16(parseDefaults[states[len(states)-1]])
		}
		switch {
		case act == acceptAction:
			if p.err != nil {
				return nil, p.err
			}
			if len(opts.InFields) > 0 && p.root != nil {
				var mask uint64
				for _, f := range opts.InFields {
					mask |= idx.textFieldBit(f)
				}
				p.root.setMask(mask)
			}
			return p.root, nil
		case act == 0:
			if p.err != nil {
				return nil, p.err
			}
			return nil, &ErrSyntax{Offset: lx.tok.pos, Near: lx.tok.s}
		case act > 0:
			states = append(states, act-1)
			vals = append(vals, pval{tok: lx.tok})
			kind = lx.next()
		default:
			r := int(-act)
			n := int(ruleLen[r])
			v := p.reduce(r, vals[len(vals)-n:])
			states = states[:len(states)-n]
			vals = vals[:len(vals)-n]
			states = append(states, parseGotos[states[len(states)-1]][ruleLHS[r]]-1)
			vals = append(vals, v)
		}
	}
}

// normalize is a term as RediSearch reads it from a query: unescaped, in
// lower case.
func normalize(s string) string { return strings.ToLower(unescape(s)) }

func (p *parser1) fieldBit(name string) uint64 { return p.idx.textFieldBit(unescape(name)) }

// reduce runs a grammar rule's action on the values of its symbols.
func (p *parser1) reduce(r int, v []pval) pval {
	var out pval
	switch r {
	case ruleQueryExpr:
		p.root = v[0].node
	case ruleQueryStar:
		p.root = newNode(qWildcard)
	case ruleAnd:
		b, c := v[0].node, v[1].node
		switch {
		case b == nil || c == nil:
			out.node = b
			if b == nil {
				out.node = c
			}
		case b.typ == qPhrase && !b.exact && b.mask == allFields:
			out.node = b.add(c)
		default:
			out.node = newNode(qPhrase).add(b, c)
		}
	case ruleExprUnion:
		out.node = v[0].node
	case ruleUnionNew, ruleUnionAdd:
		b, c := v[0].node, v[2].node
		switch {
		case b == nil || c == nil:
			out.node = b
			if b == nil {
				out.node = c
			}
		case b.typ == qUnion && b.mask == allFields:
			out.node = b.add(c)
		default:
			out.node = newNode(qUnion).add(b, c)
		}
	case ruleModifierExpr:
		out.node = v[2].node
		out.node.setMask(p.fieldBit(v[0].tok.s))
	case ruleModifierlistExpr:
		out.node = v[2].node
		var mask uint64
		for _, m := range v[0].mods {
			mask |= p.fieldBit(m)
		}
		out.node.setMask(mask)
	case ruleParens:
		out.node = v[1].node
	case ruleAttribute:
		out.attrs = [][2]string{{v[0].tok.s, v[2].tok.s}}
	case ruleAttrsOne:
		out.attrs = v[0].attrs
	case ruleAttrsAdd:
		out.attrs = v[0].attrs
		out.attrs = append(out.attrs, v[2].attrs...)
	case ruleAttrsSemi:
		out.attrs = v[0].attrs
	case ruleAttributes:
		out.node = v[0].node
		if out.node != nil {
			p.applyAttributes(out.node, v[3].attrs)
		}
	case ruleQuoteTermlist:
		out.node = v[1].node
		out.node.exact, out.node.verbatim = true, true
	case ruleQuoteTerm:
		out.node = tokenNode(normalize(v[1].tok.s))
		out.node.verbatim = true
	case ruleExprTerm:
		out.node = tokenNode(normalize(v[0].tok.s))
	case ruleExprAffix, ruleExprTermlist:
		out.node = v[0].node
	case ruleTermlistNew:
		out.node = newNode(qPhrase).add(tokenNode(normalize(v[0].tok.s)), tokenNode(normalize(v[1].tok.s)))
	case ruleTermlistAdd:
		out.node = v[0].node.add(tokenNode(normalize(v[1].tok.s)))
	case ruleTermlistStopword:
		out.node = v[0].node
	case ruleNot:
		if v[1].node != nil {
			out.node = newNode(qNot).add(v[1].node)
		}
	case ruleOptional:
		if v[1].node != nil {
			out.node = newNode(qOptional).add(v[1].node)
		}
	case ruleAffixPrefix, ruleAffixSuffix, ruleAffixContains:
		out.node = newNode(qPrefix)
		out.node.str = unescape(v[0].tok.s)
		out.node.prefix = r != ruleAffixSuffix
		out.node.suffix = r != ruleAffixPrefix
	case ruleFuzzy1, ruleFuzzy1Stop:
		out.node = fuzzyNode(v[1].tok.s, 1)
	case ruleFuzzy2, ruleFuzzy2Stop:
		out.node = fuzzyNode(v[2].tok.s, 2)
	case ruleFuzzy3, ruleFuzzy3Stop:
		out.node = fuzzyNode(v[3].tok.s, 3)
	case ruleModifier:
		out.tok = v[0].tok
		out.tok.s = unescape(v[0].tok.s)
	case ruleModifierlistNew:
		out.mods = []string{v[0].tok.s, v[2].tok.s}
	case ruleModifierlistAdd:
		out.mods = v[0].mods
		out.mods = append(out.mods, v[2].tok.s)
	case ruleTagExpr:
		if c := v[2].node; c != nil && p.idx.Field(unescape(v[0].tok.s)) != nil {
			out.node = newNode(qTag).add(c.children...)
			out.node.field = unescape(v[0].tok.s)
		}
	case ruleTagsTerm, ruleTagsStopword:
		out.node = newNode(qPhrase).add(tokenNode(v[1].tok.s))
	case ruleTagsAffix, ruleTagsTermlist:
		out.node = newNode(qPhrase).add(v[1].node)
	case ruleTagsAddTerm, ruleTagsAddStopword:
		out.node = v[0].node.add(tokenNode(v[2].tok.s))
	case ruleTagsAddAffix, ruleTagsAddTermlist:
		out.node = v[0].node.add(v[2].node)
	case ruleTagsClose:
		out.node = v[0].node
	case ruleNumericExpr:
		if f := p.idx.Field(v[0].tok.s); f != nil && v[2].node != nil {
			out.node = v[2].node
			out.node.field = f.Name
		}
	case ruleNumericRange:
		out.node = newNode(qNumeric)
		out.node.lo, out.node.loIncl = v[1].num, v[1].incl
		out.node.hi, out.node.hiIncl = v[2].num, v[2].incl
	case ruleGeoExpr:
		if f := p.idx.Field(v[0].tok.s); f != nil && v[2].node != nil {
			out.node = v[2].node
			out.node.field = f.Name
		}
	case ruleGeoFilter:
		n, err := geoNode("", v[1].num, v[2].num, v[3].num, v[4].tok.s)
		if err != nil && p.err == nil {
			p.err = err
		}
		out.node = n
	case ruleNum:
		out.num, out.incl = v[0].tok.num, true
	case ruleNumExclusive:
		out.num, out.incl = v[1].num, false
	case ruleNumNegative:
		out.num, out.incl = -v[1].num, v[1].incl
	case ruleTerm, ruleTermNumber:
		out.tok = v[0].tok
	}
	return out
}

func fuzzyNode(s string, dist int) *qnode {
	n := newNode(qFuzzy)
	n.str, n.dist = normalize(s), dist
	return n
}

var errAttribute = errors.New("Invalid attribute") //nolint:staticcheck // RediSearch's reply text

// applyAttributes applies a node's => {$name: value} attributes.
func (p *parser1) applyAttributes(n *qnode, attrs [][2]string) {
	for _, a := range attrs {
		switch strings.ToLower(a[0]) {
		case "weight":
			if w, err := strconv.ParseFloat(a[1], 64); err == nil && w >= 0 {
				n.weight = w
			} else if p.err == nil {
				p.err = errAttribute
			}
		case "slop", "inorder", "phonetic":
		default:
			if p.err == nil {
				p.err = errAttribute
			}
		}
	}
}
