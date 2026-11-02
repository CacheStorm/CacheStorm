package search

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// The kinds a DIALECT 2 token value takes, as RediSearch types them.
const (
	qtTerm = iota
	qtNumeric
	qtSize
	qtParamTerm
	qtTermCase
	qtParamTermCase
	qtParamSize
	qtParamNumeric
	qtWildcard
	qtParamWildcard
)

type lexer2 struct {
	q    string
	pos  int
	tok  qtoken
	kind int // the token type the lexer last set, as RediSearch's does
	sign int
	// pending holds the tokens of a numeric comparison after its first.
	pending []lexed2
}

type lexed2 struct {
	tok  int
	val  qtoken
	sign int
}

func lowerHas(q string, i int, word string) bool {
	return len(q)-i >= len(word) && strings.EqualFold(q[i:i+len(word)], word)
}

// number2End matches [+-]?(d+(.d+)?|.d+|d+.)([eE][+-]?d+)?
func number2End(q string, i int) int {
	j := i
	if j < len(q) && (q[j] == '+' || q[j] == '-') {
		j++
	}
	best := -1
	d := digitsEnd(q, j)
	if d > j {
		best = d
		if d < len(q) && q[d] == '.' {
			best = d + 1
			if e := digitsEnd(q, d+1); e > d+1 {
				best = e
			}
		}
	} else if j < len(q) && q[j] == '.' {
		if e := digitsEnd(q, j+1); e > j+1 {
			best = e
		}
	}
	if best < 0 {
		return i
	}
	if best < len(q) && (q[best] == 'e' || q[best] == 'E') {
		e := best + 1
		if e < len(q) && (q[e] == '+' || q[e] == '-') {
			e++
		}
		if k := digitsEnd(q, e); k > e {
			best = k
		}
	}
	return best
}

func inf2End(q string, i int) int {
	j := i
	if j < len(q) && (q[j] == '+' || q[j] == '-') {
		j++
	}
	if lowerHas(q, j, "inf") {
		return j + 3
	}
	return i
}

// quotedEnd matches a quoted string at i: escaped quotes (exact) or any
// escaped byte (verbatim) continue it.
func quotedEnd(q string, i int, verbatim bool) int {
	if i >= len(q) || q[i] != '"' && q[i] != '\'' {
		return i
	}
	quote := q[i]
	best := i
	for k := i + 1; k < len(q); k++ {
		c := q[k]
		if verbatim && c == '\\' {
			k++
			continue
		}
		if c == quote {
			if k > i+1 {
				best = k + 1
			}
			if verbatim || q[k-1] != '\\' {
				break
			}
		}
	}
	return best
}

// wordEnd is the longest term, number or parameter at i.
func (l *lexer2) wordEnd(i int, attr bool) int {
	t := (&lexer{q: l.q}).termEnd(i)
	end := max(t, number2End(l.q, i))
	if attr && i < len(l.q) && l.q[i] == '$' {
		if e := (&lexer{q: l.q}).termEnd(i + 1); e > i+1 {
			end = max(end, e)
		}
	}
	return end
}

// numericOp matches @field <op> value and returns its parts.
func (l *lexer2) numericOp(i int) (end, opStart, opLen, valStart int, op int) {
	q := l.q
	if q[i] != '@' {
		return i, 0, 0, 0, 0
	}
	f := (&lexer{q: q}).termEnd(i + 1)
	if f == i+1 {
		return i, 0, 0, 0, 0
	}
	j := f
	for j < len(q) && isSpace(q[j]) {
		j++
	}
	bestEnd := i
	for _, o := range []struct {
		s  string
		op int
	}{{"!=", v2TokNOT_EQUAL}, {"==", v2TokEQUALS}, {">=", v2TokGE}, {">", v2TokGT}, {"<=", v2TokLE}, {"<", v2TokLT}} {
		if !strings.HasPrefix(q[j:], o.s) {
			continue
		}
		k := j + len(o.s)
		for k < len(q) && isSpace(q[k]) {
			k++
		}
		v := max(number2End(q, k), inf2End(q, k), digitsEnd(q, k))
		a := k
		if a < len(q) && (q[a] == '+' || q[a] == '-') {
			a++
		}
		if a < len(q) && q[a] == '$' {
			if e := (&lexer{q: q}).termEnd(a + 1); e > a+1 {
				v = max(v, e)
			}
		}
		if v > k && v > bestEnd {
			bestEnd, opStart, opLen, valStart, op = v, j, len(o.s), k, o.op
		}
	}
	return bestEnd, opStart, opLen, valStart, op
}

// next returns the next token of RediSearch's DIALECT 2 lexer: the longest
// match among its patterns, the earlier pattern on a tie.
func (l *lexer2) next() (int, qtoken) {
	if len(l.pending) > 0 {
		p := l.pending[0]
		l.pending = l.pending[1:]
		l.sign = p.sign
		return p.tok, p.val
	}
	l.sign = 1
	q := l.q
	for l.pos < len(q) {
		i := l.pos
		best, kind := i, -1
		consider := func(end, k int) {
			if end > best {
				best, kind = end, k
			}
		}
		consider(digitsEnd(q, i), v2TokSIZE)
		consider(number2End(q, i), v2TokNUMBER)
		if q[i] == '@' {
			if e := (&lexer{q: q}).termEnd(i + 1); e > i+1 {
				consider(e, v2TokMODIFIER)
			}
		}
		if q[i] == '$' {
			if e := (&lexer{q: q}).termEnd(i + 1); e > i+1 {
				consider(e, v2TokATTRIBUTE)
			}
		}
		opEnd, opStart, opLen, valStart, op := l.numericOp(i)
		consider(opEnd, -3)
		if strings.HasPrefix(q[i:], "=>") {
			consider(i+2, v2TokARROW)
		}
		if lowerHas(q, i, "as") {
			consider(i+2, v2TokAS_T)
		}
		consider(inf2End(q, i), -2)
		if strings.HasPrefix(q[i:], `""`) || strings.HasPrefix(q[i:], "''") {
			consider(i+2, -4)
		}
		if k, ok := punctTokens2[q[i]]; ok {
			consider(i+1, k)
		}
		if lowerHas(q, i, "ismissing") {
			consider(i+9, v2TokISMISSING)
		}
		consider((&lexer{q: q}).termEnd(i), v2TokTERM)
		exact := quotedEnd(q, i, false)
		consider(exact, v2TokEXACT)
		if w := l.wordEnd(i, true); w > i && w < len(q) && q[w] == '*' {
			consider(w+1, v2TokPREFIX)
		}
		if exact > i && exact < len(q) && q[exact] == '*' {
			consider(exact+1, -5)
		}
		if q[i] == '*' && i+1 < len(q) {
			if w := l.wordEnd(i+1, true); w > i+1 {
				consider(w, v2TokSUFFIX)
				if w < len(q) && q[w] == '*' {
					consider(w+1, v2TokCONTAINS)
				}
			}
			if e := quotedEnd(q, i+1, false); e > i+1 {
				consider(e, -6)
				if e < len(q) && q[e] == '*' {
					consider(e+1, -7)
				}
			}
		}
		consider(quotedEnd(q, i, true), v2TokVERBATIM)
		if q[i] == 'w' {
			if e := quotedEnd(q, i+1, true); e > i+1 {
				consider(e, v2TokWILDCARD)
			}
		}
		if kind == -1 {
			l.pos++
			continue
		}
		l.pos = best
		text := q[i:best]
		isAttr := func(at int) int {
			if at < len(q) && q[at] == '$' {
				return 1
			}
			return 0
		}
		switch kind {
		case v2TokSIZE, v2TokNUMBER:
			l.tok = qtoken{s: text, pos: i}
			if f, err := strconv.ParseFloat(text, 64); err == nil || errors.Is(err, strconv.ErrRange) {
				l.tok.num = f
			}
		case -2:
			l.tok = qtoken{s: text, pos: i, num: math.Inf(1)}
			if text[0] == '-' {
				l.tok.num = math.Inf(-1)
			}
			kind = v2TokNUMBER
		case v2TokMODIFIER, v2TokATTRIBUTE:
			l.tok.s, l.tok.pos = text[1:], i
		case -3:
			return l.splitNumericOp(i, best, opStart, opLen, valStart, op)
		case v2TokARROW:
			l.tok.s, l.tok.pos = q[i+1:min(i+3, len(q))], i
		case v2TokAS_T, v2TokISMISSING, v2TokTERM:
			l.tok = qtoken{s: text, pos: i}
		case -4:
			l.tok = qtoken{s: "", pos: i}
			kind = v2TokTERM
		case v2TokEXACT:
			l.tok = qtoken{s: text[1 : len(text)-1], pos: i}
		case v2TokPREFIX:
			a := isAttr(i)
			l.kind = qtTerm
			if a == 1 {
				l.kind = qtParamTerm
			}
			l.tok = qtoken{s: q[i+a : best-1], pos: i}
		case -5:
			l.kind = qtTerm
			l.tok = qtoken{s: q[i+1 : best-2], pos: i}
			kind = v2TokPREFIX
		case v2TokSUFFIX, v2TokCONTAINS:
			a := isAttr(i + 1)
			l.kind = qtTerm
			if a == 1 {
				l.kind = qtParamTerm
			}
			end := best
			if kind == v2TokCONTAINS {
				end--
			}
			l.tok = qtoken{s: q[i+1+a : end], pos: i}
		case -6, -7:
			l.kind = qtTerm
			end, k := best-1, v2TokSUFFIX
			if kind == -7 {
				end, k = best-2, v2TokCONTAINS
			}
			l.tok = qtoken{s: q[i+2 : end], pos: i}
			kind = k
		case v2TokVERBATIM:
			a := isAttr(i + 1)
			l.kind = qtTerm
			if a == 1 {
				l.kind = qtParamTerm
			}
			l.tok = qtoken{s: q[i+1+a : best-1], pos: i}
		case v2TokWILDCARD:
			a := isAttr(i + 2)
			l.kind = qtWildcard
			if a == 1 {
				l.kind = qtParamWildcard
			}
			l.tok = qtoken{s: q[i+2+a : best-1], pos: i + 2}
		default:
			l.tok.pos = i
		}
		return kind, l.tok
	}
	return v2TokEOF, l.tok
}

// splitNumericOp emits @field <op> value as RediSearch's lexer does: a
// modifier, the operator and the number or parameter.
func (l *lexer2) splitNumericOp(start, end, opStart, opLen, valStart, op int) (int, qtoken) {
	q := l.q
	field := strings.TrimRight(q[start+1:opStart], " \t\n\v\f\r")
	modTok := qtoken{s: field, pos: start}
	opTok := qtoken{s: q[opStart : opStart+opLen], pos: start}
	v := valStart
	sign := 1
	if v+1 < len(q) && (q[v] == '+' || q[v] == '-') && q[v+1] == '$' {
		if q[v] == '-' {
			sign = -1
		}
		v++
	}
	valTok := qtoken{s: q[v:end], pos: start}
	valKind := v2TokNUMBER
	if q[v] == '$' {
		// RediSearch takes the parameter's name one byte too long, and
		// then drops trailing spaces.
		valTok.s = strings.TrimRight(q[v+1:min(end+1, len(q))], " \t\n\v\f\r")
		valKind = v2TokATTRIBUTE
	} else if f, err := strconv.ParseFloat(valTok.s, 64); err == nil || errors.Is(err, strconv.ErrRange) {
		valTok.num = f
	} else if strings.HasSuffix(strings.ToLower(valTok.s), "inf") {
		valTok.num = math.Inf(1)
		if valTok.s[0] == '-' {
			valTok.num = math.Inf(-1)
		}
	}
	// RediSearch splits the comparison on a copy of its token, so the
	// lexer's own token is left as it was.
	l.pending = []lexed2{{tok: op, val: opTok, sign: 1}, {tok: valKind, val: valTok, sign: sign}}
	l.sign = 1
	return v2TokMODIFIER, modTok
}

var punctTokens2 = map[byte]int{
	'"': v2TokQUOTE, '|': v2TokOR, '(': v2TokLP, ')': v2TokRP, '{': v2TokLB, '}': v2TokRB, ':': v2TokCOLON,
	';': v2TokSEMICOLON, '-': v2TokMINUS, '~': v2TokTILDE, '*': v2TokSTAR, '%': v2TokPERCENT, '[': v2TokLSQB, ']': v2TokRSQB,
}

// pval2 is a DIALECT 2 parser stack value.
type pval2 struct {
	tok   qtoken
	kind  int // the token's type
	sign  int
	lexer int // the lexer's token type when the token was read
	node  *qnode
	field *Field
	mods  []*Field
	attrs [][2]string
	num   float64
	vec   [][2]string
}

type parser2 struct {
	idx      *Index
	opts     QueryOptions
	root     *qnode
	err      error
	paramErr error
}

func (p *parser2) fail(err error) {
	if p.err == nil {
		p.err = err
	}
}

// syntaxErr reports an error at a token, as RediSearch's reportSyntaxError
// words it by the token's type.
func syntaxErr(msg string, tok qtoken, kind int) error {
	switch kind {
	case qtTerm, qtTermCase:
		return fmt.Errorf("%s at offset %d near %s", msg, tok.pos, tok.s)
	case qtNumeric:
		return fmt.Errorf("%s at offset %d near %s", msg, tok.pos, cfmtF(tok.num))
	}
	return fmt.Errorf("%s at offset %d", msg, tok.pos)
}

// param returns a parameter's value, failing the query when it is absent.
func (p *parser2) param(name string) (string, bool) {
	v, ok := p.opts.Params[name]
	if !ok && p.paramErr == nil {
		p.paramErr = errors.New("No such parameter `" + name + "`") //nolint:staticcheck // RediSearch's reply text
	}
	return v, ok
}

// paramNumber resolves a numeric parameter.
func (p *parser2) paramNumber(v *pval2) float64 {
	if v.kind != qtParamNumeric {
		return v.num
	}
	s, ok := p.param(v.tok.s)
	if !ok {
		return 0
	}
	f, err := parseParamDouble(s)
	if err != nil {
		if p.paramErr == nil {
			p.paramErr = fmt.Errorf("Invalid numeric value (%s) for parameter `%s`", s, v.tok.s) //nolint:staticcheck // RediSearch's reply text
		}
		return 0
	}
	return f * float64(v.sign)
}

func parseParamDouble(s string) (float64, error) {
	switch strings.ToLower(s) {
	case "inf", "+inf":
		return math.Inf(1), nil
	case "-inf":
		return math.Inf(-1), nil
	}
	return strconv.ParseFloat(s, 64)
}

// tokenFor builds the token node of a term value: words in lower case,
// numbers as written and not expanded, parameters by their value.
func (p *parser2) tokenFor(v *pval2) *qnode {
	switch v.kind {
	case qtTerm:
		return tokenNode(normalize(v.tok.s))
	case qtTermCase:
		return tokenNode(v.tok.s)
	case qtNumeric, qtSize:
		n := tokenNode(v.tok.s)
		n.verbatim = true
		return n
	case qtParamTermCase:
		s, _ := p.param(v.tok.s)
		return tokenNode(s)
	}
	s, _ := p.param(v.tok.s)
	n := tokenNode(normalize(s))
	if _, err := parseParamDouble(s); err == nil {
		n.verbatim = true
	}
	return n
}

func (p *parser2) affix(v *pval2, prefix, suffix bool) *qnode {
	n := newNode(qPrefix)
	n.prefix, n.suffix = prefix, suffix
	if v.lexer == qtParamTerm {
		s, _ := p.param(v.tok.s)
		n.str = normalize(s)
	} else {
		n.str = unescape(v.tok.s)
	}
	return n
}

// parseDialect2 parses a query with the tables of RediSearch's DIALECT 2
// grammar.
func parseDialect2(idx *Index, q string, opts QueryOptions) (Node, error) {
	lx := &lexer2{q: q, sign: 1}
	p := &parser2{idx: idx, opts: opts}
	states := []int16{0}
	var vals []pval2
	kind, tok := lx.next()
	lexKind := lx.kind
	for {
		state := states[len(states)-1]
		act := v2Actions[state][kind]
		for k := kind; act == 0 && v2Fallback[k] != 0; {
			k = int(v2Fallback[k])
			act = v2Actions[state][k]
		}
		if act == 0 {
			act = -int16(v2Defaults[state])
		}
		switch {
		case act == acceptAction:
			if p.err != nil {
				return nil, p.err
			}
			if p.paramErr != nil {
				return nil, p.paramErr
			}
			return p.root, nil
		case act == 0:
			if p.err != nil {
				return nil, p.err
			}
			return nil, &ErrSyntax{Offset: tok.pos, Near: tok.s}
		case act > 0:
			if p.err != nil {
				return nil, p.err
			}
			states = append(states, act-1)
			vals = append(vals, pval2{tok: tok, sign: lx.sign, lexer: lexKind})
			kind, tok = lx.next()
			lexKind = lx.kind
		default:
			r := int(-act)
			n := int(v2RuleLen[r])
			v := p.reduce(r, vals[len(vals)-n:])
			states = states[:len(states)-n]
			vals = vals[:len(vals)-n]
			states = append(states, v2Gotos[states[len(states)-1]][v2RuleLHS[r]]-1)
			vals = append(vals, v)
		}
	}
}

func intersectionStep(b, c *qnode) *qnode {
	switch {
	case b == nil:
		return c
	case c == nil:
		return b
	case b.typ == qPhrase && !b.exact && b.mask == allFields:
		return b.add(c)
	case c.typ == qPhrase && !c.exact && c.mask == allFields:
		return c.add(b)
	}
	return newNode(qPhrase).add(b, c)
}

func unionStep(b, c *qnode) *qnode {
	switch {
	case b == nil:
		return c
	case c == nil:
		return b
	case b.typ == qUnion && b.mask == allFields:
		return b.add(c)
	case c.typ == qUnion && c.mask == allFields:
		return c.add(b)
	}
	return newNode(qUnion).add(b, c)
}

// wantField checks a modifier's field type, reporting it otherwise.
func (p *parser2) wantField(v *pval2, t FieldType) bool {
	if v.field == nil {
		return false
	}
	if v.field.Type != t {
		p.fail(syntaxErr("Expected a "+t.String()+" field", v.tok, v.lexer))
		return false
	}
	return true
}

func (p *parser2) numericNode(f *Field, lo, hi float64, loIncl, hiIncl bool) *qnode {
	n := newNode(qNumeric)
	n.field, n.lo, n.hi, n.loIncl, n.hiIncl = f.Name, lo, hi, loIncl, hiIncl
	return n
}

// reduce runs a DIALECT 2 grammar rule's action.
func (p *parser2) reduce(r int, v []pval2) pval2 {
	var out pval2
	switch r {
	case v2RuleQueryExpr:
		p.root = v[0].node
	case v2RuleQueryStar:
		p.root = newNode(qWildcard)
	case v2RuleExprText, v2RuleExprUnion, v2RuleTextExprUnion, v2RuleTextAffix, v2RuleTextVerbatim:
		out.node = v[0].node
	case v2RuleAnd, v2RuleAndTextExpr, v2RuleAndExprText, v2RuleTextAnd:
		out.node = intersectionStep(v[0].node, v[1].node)
	case v2RuleUnionNew, v2RuleUnionAdd, v2RuleUnionTextExpr, v2RuleUnionExprText, v2RuleTextUnionNew, v2RuleTextUnionAdd:
		out.node = unionStep(v[0].node, v[2].node)
	case v2RuleModifierExpr:
		if p.wantField(&v[0], FieldText) && v[2].node != nil {
			out.node = v[2].node
			out.node.setMask(fieldBit(v[0].field.pos))
		}
	case v2RuleModifierlistExpr:
		if v[2].node != nil && v[0].mods != nil {
			var mask uint64
			for _, f := range v[0].mods {
				mask |= fieldBit(f.pos)
			}
			out.node = v[2].node
			out.node.setMask(mask)
		}
	case v2RuleParens, v2RuleTextParens:
		out.node = v[1].node
	case v2RuleAttribute:
		val := v[2].tok.s
		if v[2].kind == qtParamTerm {
			if s, ok := p.param(val); ok {
				val = s
			}
		}
		out.attrs = [][2]string{{v[0].tok.s, val}}
	case v2RuleAttrsOne, v2RuleAttrsSemi:
		out.attrs = v[0].attrs
	case v2RuleAttrsAdd:
		out.attrs = v[0].attrs
		out.attrs = append(out.attrs, v[2].attrs...)
	case v2RuleAttributes, v2RuleTextAttributes:
		out.node = v[0].node
		if out.node != nil {
			p.applyAttributes(out.node, v[3].attrs)
		}
	case v2RuleExact:
		ph := newNode(qPhrase)
		ph.exact, ph.verbatim = true, true
		for _, w := range exactWords(v[0].tok.s) {
			ph.add(tokenNode(normalize(w)))
		}
		out.node = ph
	case v2RuleQuoteAttr, v2RuleSquoteAttr:
		out.node = tokenNode(normalize("$" + v[1].tok.s))
		out.node.verbatim = true
	case v2RuleTextTerm:
		if v[0].kind != qtTerm || p.opts.NoStopwords || !p.idx.Stopwords[strings.ToLower(v[0].tok.s)] {
			out.node = p.tokenFor(&v[0])
		}
	case v2RuleTermlistNew:
		out.node = newNode(qPhrase).add(p.tokenFor(&v[0]), p.tokenFor(&v[1]))
	case v2RuleTermlistAdd:
		out.node = v[0].node.add(p.tokenFor(&v[1]))
	case v2RuleNot, v2RuleTextNot:
		if v[1].node != nil {
			out.node = newNode(qNot).add(v[1].node)
		}
	case v2RuleOptional, v2RuleTextOptional:
		if v[1].node != nil {
			out.node = newNode(qOptional).add(v[1].node)
		}
	case v2RuleAffixPrefix:
		out.node = p.affix(&v[0], true, false)
	case v2RuleAffixSuffix:
		out.node = p.affix(&v[0], false, true)
	case v2RuleAffixContains:
		out.node = p.affix(&v[0], true, true)
	case v2RuleVerbatim:
		n := newNode(qWildcardQuery)
		n.str = v[0].tok.s
		if v[0].lexer == qtParamWildcard {
			n.str, _ = p.param(v[0].tok.s)
		}
		out.node = n
	case v2RuleFuzzy1, v2RuleFuzzy2, v2RuleFuzzy3:
		d := map[int]int{v2RuleFuzzy1: 1, v2RuleFuzzy2: 2, v2RuleFuzzy3: 3}[r]
		t := v[d]
		s := t.tok.s
		if t.kind == qtParamTerm {
			s, _ = p.param(s)
		}
		out.node = fuzzyNode(s, d)
	case v2RuleModifier:
		out.tok, out.lexer = v[0].tok, v[0].lexer
		out.tok.s = unescape(v[0].tok.s)
		if out.field = p.idx.Field(out.tok.s); out.field == nil {
			p.fail(syntaxErr("Unknown field", out.tok, out.lexer))
		}
	case v2RuleModifierlistNew, v2RuleModifierlistAdd:
		var first *Field
		if r == v2RuleModifierlistNew {
			if !p.wantField(&v[0], FieldText) {
				break
			}
			first = v[0].field
		} else if v[0].mods == nil {
			break
		}
		second := p.idx.Field(v[2].tok.s)
		switch {
		case second == nil:
			p.fail(syntaxErr("Unknown field", v[2].tok, v[2].kind))
		case second.Type != FieldText:
			p.fail(syntaxErr("Expected a TEXT field", v[2].tok, v[2].kind))
		case first != nil:
			out.mods = []*Field{first, second}
		default:
			out.mods = v[0].mods
			out.mods = append(out.mods, second)
		}
	case v2RuleIsmissing:
		if f := v[2].field; f != nil {
			if !f.IndexMissing {
				p.fail(syntaxErr("'ismissing' requires defining the field with 'INDEXMISSING'", v[2].tok, v[2].lexer))
			} else {
				out.node = newNode(qMissing)
				out.node.field = f.Name
			}
		}
	case v2RuleTagExpr:
		if p.wantField(&v[0], FieldTag) && v[3].node != nil {
			out.node = newNode(qTag).add(v[3].node.children...)
			out.node.field = v[0].field.Name
		}
	case v2RuleTagsTerm:
		out.node = newNode(qPhrase).add(p.tokenFor(&v[0]))
	case v2RuleTagsAffix, v2RuleTagsVerbatim, v2RuleTagsTermlist:
		out.node = newNode(qPhrase).add(v[0].node)
	case v2RuleTagsAddTerm:
		out.node = v[0].node.add(p.tokenFor(&v[2]))
	case v2RuleTagsAddAffix, v2RuleTagsAddVerbatim, v2RuleTagsAddTermlist:
		out.node = v[0].node.add(v[2].node)
	case v2RuleNumericExpr:
		if p.wantField(&v[0], FieldNumeric) && v[2].node != nil {
			out.node = v[2].node
			out.node.field = v[0].field.Name
		}
	case v2RuleRangeInclIncl, v2RuleRangeExclIncl, v2RuleRangeInclExcl, v2RuleRangeExclExcl:
		n := newNode(qNumeric)
		n.lo, n.loIncl = p.rangeBound(&v[1], r == v2RuleRangeInclIncl || r == v2RuleRangeInclExcl)
		n.hi, n.hiIncl = p.rangeBound(&v[2], r == v2RuleRangeInclIncl || r == v2RuleRangeExclIncl)
		out.node = n
	case v2RuleRangeSingle:
		x := p.paramNumber(&v[1])
		out.node = newNode(qNumeric)
		out.node.lo, out.node.hi, out.node.loIncl, out.node.hiIncl = x, x, true, true
	case v2RuleNumNe, v2RuleNumEq, v2RuleNumGt, v2RuleNumGe, v2RuleNumLt, v2RuleNumLe:
		if !p.wantField(&v[0], FieldNumeric) {
			break
		}
		x := p.paramNumber(&v[2])
		f := v[0].field
		switch r {
		case v2RuleNumNe:
			out.node = newNode(qNot).add(p.numericNode(f, x, x, true, true))
		case v2RuleNumEq:
			out.node = p.numericNode(f, x, x, true, true)
		case v2RuleNumGt:
			out.node = p.numericNode(f, x, math.Inf(1), false, true)
		case v2RuleNumGe:
			out.node = p.numericNode(f, x, math.Inf(1), true, true)
		case v2RuleNumLt:
			out.node = p.numericNode(f, math.Inf(-1), x, true, false)
		default:
			out.node = p.numericNode(f, math.Inf(-1), x, true, true)
		}
	case v2RuleGeoExpr:
		if p.wantField(&v[0], FieldGeo) && v[2].node != nil {
			out.node = v[2].node
			out.node.field = v[0].field.Name
		}
	case v2RuleGeoFilter:
		unit := v[4].tok.s
		if v[4].kind == qtParamTerm {
			unit, _ = p.param(unit)
		}
		n, err := geoNode("", p.paramNumber(&v[1]), p.paramNumber(&v[2]), p.paramNumber(&v[3]), unit)
		if err != nil && p.paramErr == nil {
			p.paramErr = err
		}
		out.node = n
	case v2RuleGeometryExpr:
		p.wantField(&v[0], FieldGeoShape)
	case v2RuleQueryKnnExpr, v2RuleQueryKnnText, v2RuleQueryKnnStar, v2RuleQueryKnnExprAttrs, v2RuleQueryKnnTextAttrs, v2RuleQueryKnnStarAttrs:
		vn := v[3].node
		if vn == nil {
			break
		}
		p.root = vn
		if len(v) == 9 {
			p.applyAttributes(vn, v[7].attrs)
		}
		if r == v2RuleQueryKnnStar || r == v2RuleQueryKnnStarAttrs {
			vn.byScore = true
		} else if v[0].node != nil {
			vn.add(v[0].node)
		}
	case v2RuleVqAttrsScore, v2RuleVqScore, v2RuleVqAttrs, v2RuleVq:
		out.node = v[0].node
		if out.node == nil {
			break
		}
		if r == v2RuleVqAttrsScore || r == v2RuleVqAttrs {
			out.node.vecAttrs = v[1].vec
		}
		if r == v2RuleVqAttrsScore || r == v2RuleVqScore {
			sf := v[len(v)-1]
			name := sf.tok.s
			if sf.kind == qtParamTermCase {
				name, _ = p.param(name)
			}
			out.node.yield = name
		}
	case v2RuleScoreField:
		out = v[1]
	case v2RuleVectorCommand:
		if !p.wantField(&v[2], FieldVector) {
			break
		}
		if !strings.EqualFold(v[0].tok.s, "KNN") {
			p.fail(syntaxErr("Syntax error: Expecting Vector Similarity command", v[0].tok, qtTerm))
			break
		}
		n := newNode(qVector)
		n.knn, n.field, n.str = true, v[2].field.Name, v[3].tok.s
		n.blob, _ = p.param(v[3].tok.s)
		n.yield = "__" + v[2].tok.s + "_score"
		k := v[1]
		if k.kind == qtParamSize {
			s, ok := p.param(k.tok.s)
			if ok {
				if x, err := strconv.ParseInt(s, 10, 64); err == nil && x >= 0 {
					n.k = int(x)
				} else if p.paramErr == nil {
					p.paramErr = fmt.Errorf("Invalid numeric value (%s) for parameter `%s`", s, k.tok.s) //nolint:staticcheck // RediSearch's reply text
				}
			}
		} else {
			n.k = int(k.num)
		}
		out.node = n
	case v2RuleVectorAttribute:
		val := v[1].tok.s
		if v[1].kind == qtParamTerm {
			val, _ = p.param(val)
		}
		out.vec = [][2]string{{v[0].tok.s, val}}
	case v2RuleVattrsAdd:
		out.vec = v[0].vec
		out.vec = append(out.vec, v[1].vec...)
	case v2RuleVattrsOne:
		out.vec = v[0].vec
	case v2RuleVectorRangeExpr:
		if p.wantField(&v[0], FieldVector) && v[3].node != nil {
			out.node = v[3].node
			out.node.field = v[0].field.Name
		}
	case v2RuleVectorRangeCommand:
		if !strings.EqualFold(v[0].tok.s, "VECTOR_RANGE") {
			p.fail(syntaxErr("Syntax error: expecting vector similarity range command", v[0].tok, qtTerm))
			break
		}
		n := newNode(qVector)
		n.radius, n.str = p.paramNumber(&v[1]), v[2].tok.s
		n.blob, _ = p.param(v[2].tok.s)
		out.node = n
	case v2RuleNumSize, v2RuleNum:
		out.num, out.kind = v[0].tok.num, qtNumeric
	case v2RuleNumNegative:
		out.num, out.kind = -v[1].num, qtNumeric
	case v2RuleTerm:
		out.tok, out.kind = v[0].tok, qtTerm
	case v2RuleTermNumber:
		out.tok, out.kind = v[0].tok, qtNumeric
	case v2RuleTermSize:
		out.tok, out.kind = v[0].tok, qtSize
	case v2RuleParamTerm:
		out = v[0]
	case v2RuleParamTermAttr:
		out.tok, out.kind = v[0].tok, qtParamTerm
	case v2RuleParamTermCase:
		out.tok, out.kind = v[0].tok, qtTermCase
	case v2RuleParamTermCaseAttr:
		out.tok, out.kind = v[0].tok, qtParamTermCase
	case v2RuleParamSize:
		out.tok, out.kind, out.num = v[0].tok, qtSize, v[0].tok.num
	case v2RuleParamSizeAttr:
		out.tok, out.kind = v[0].tok, qtParamSize
	case v2RuleParamNumAttr:
		out.tok, out.kind, out.sign = v[0].tok, qtParamNumeric, v[0].sign
	case v2RuleParamNumNegAttr:
		out.tok, out.kind, out.sign = v[1].tok, qtParamNumeric, -1
	case v2RuleParamNum:
		out.num, out.kind = v[0].num, qtNumeric
	case v2RuleExclNum:
		out.num, out.kind = v[1].num, qtNumeric
	case v2RuleExclAttr:
		out.tok, out.kind, out.sign = v[1].tok, qtParamNumeric, 1
	case v2RuleExclNegAttr:
		out.tok, out.kind, out.sign = v[2].tok, qtParamNumeric, -1
	}
	if out.sign == 0 {
		out.sign = 1
	}
	return out
}

// rangeBound reads a numeric range bound; a parameter may carry "(" to
// make itself exclusive.
func (p *parser2) rangeBound(v *pval2, incl bool) (float64, bool) {
	if v.kind != qtParamNumeric {
		return v.num, incl
	}
	s, ok := p.param(v.tok.s)
	if !ok {
		return 0, incl
	}
	if strings.HasPrefix(s, "(") {
		s, incl = s[1:], false
	}
	f, err := parseParamDouble(s)
	if err != nil && p.paramErr == nil {
		p.paramErr = fmt.Errorf("Invalid numeric value (%s) for parameter `%s`", s, v.tok.s) //nolint:staticcheck // RediSearch's reply text
	}
	return f * float64(v.sign), incl
}

// exactWords splits a quoted phrase as RediSearch's parser does.
func exactWords(s string) []string {
	var out []string
	start, escaped := 0, false
	for i := 0; i <= len(s); i++ {
		if i < len(s) {
			c := s[i]
			if !exactSeparators[c] || escaped {
				escaped = !escaped && c == '\\'
				continue
			}
		}
		if i > start {
			out = append(out, s[start:i])
		}
		start, escaped = i+1, false
	}
	return out
}

var exactSeparators = func() (t [256]bool) {
	for _, c := range []byte(" \t,./(){}[]:;~!@#%^&*-=+|'`\"<>?") {
		t[c] = true
	}
	return t
}()

// applyAttributes applies a node's => {$name: value} attributes.
func (p *parser2) applyAttributes(n *qnode, attrs [][2]string) {
	for _, a := range attrs {
		name := strings.ToLower(a[0])
		switch {
		case name == "weight":
			if w, err := strconv.ParseFloat(a[1], 64); err == nil && w >= 0 {
				n.weight = w
				continue
			}
			p.fail(errAttribute)
		case n.typ == qVector && name == "yield_distance_as":
			n.yield = a[1]
		case n.typ == qVector:
			n.vecAttrs = append(n.vecAttrs, [2]string{a[0], a[1]})
		case name == "slop" || name == "inorder" || name == "phonetic":
		default:
			p.fail(errAttribute)
		}
	}
}
