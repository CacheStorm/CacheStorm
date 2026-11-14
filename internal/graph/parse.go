package graph

import (
	"fmt"
	"strconv"
	"strings"
)

// A query is MATCH patterns [WHERE expr] RETURN [DISTINCT] items
// [ORDER BY keys] [SKIP n] [LIMIT n].
type query struct {
	patterns []pattern
	where    expr
	distinct bool
	items    []returnItem
	order    []sortKey
	skip     int64
	limit    int64
}

type pattern struct {
	nodes []nodePat
	rels  []relPat
}

type nodePat struct {
	name  string
	label string
	props map[string]expr
}

const (
	dirOut = iota
	dirIn
	dirBoth
)

type relPat struct {
	name  string
	types []string
	props map[string]expr
	dir   int
}

type returnItem struct {
	e     expr
	name  string
	agg   string // count, sum, min, max, avg, collect, or ""
	star  bool   // count(*)
	dist  bool   // count(DISTINCT x)
	text  string
	alias bool
}

type sortKey struct {
	e    expr
	text string
	desc bool
}

type expr interface{}

type (
	litExpr  struct{ v any }
	varExpr  struct{ name string }
	propExpr struct {
		of  expr
		key string
	}
	unaryExpr struct {
		op string
		x  expr
	}
	binExpr struct {
		op   string
		l, r expr
	}
	listExpr struct{ items []expr }
	callExpr struct {
		fn   string
		args []expr
	}
	isNullExpr struct {
		x   expr
		not bool
	}
)

var aggregates = map[string]bool{"count": true, "sum": true, "min": true, "max": true, "avg": true, "collect": true}

type parser struct {
	toks []token
	pos  int
	src  []rune
}

func (p *parser) peek() token { return p.toks[p.pos] }

func (p *parser) next() token {
	t := p.toks[p.pos]
	if t.kind != tokEOF {
		p.pos++
	}
	return t
}

func (p *parser) errorf(format string, args ...any) error {
	t := p.peek()
	at := "end of query"
	if t.kind != tokEOF {
		at = fmt.Sprintf("'%s' at offset %d", t.text, t.pos)
	}
	return fmt.Errorf("%s near %s", fmt.Sprintf(format, args...), at)
}

func (p *parser) acceptKw(kw string) bool {
	if p.peek().keyword(kw) {
		p.pos++
		return true
	}
	return false
}

func (p *parser) acceptPunct(s string) bool {
	if p.peek().punct(s) {
		p.pos++
		return true
	}
	return false
}

func (p *parser) expectPunct(s string) error {
	if !p.acceptPunct(s) {
		return p.errorf("expected '%s'", s)
	}
	return nil
}

func (p *parser) ident() (string, error) {
	t := p.peek()
	if t.kind != tokIdent {
		return "", p.errorf("expected a name")
	}
	p.pos++
	return t.text, nil
}

// writeClauses are Cypher clauses that change the graph, which GRAPH.QUERY
// does not run.
var writeClauses = []string{"CREATE", "MERGE", "DELETE", "DETACH", "SET", "REMOVE"}

// clausePosition reports whether toks[i] stands where a clause may start:
// first, or after a closing bracket or a value, not as a label, property,
// variable or map key.
func clausePosition(toks []token, i int) bool {
	if i == 0 {
		return true
	}
	prev := toks[i-1]
	if prev.kind != tokPunct {
		return true
	}
	return prev.text == ")" || prev.text == "]" || prev.text == "}"
}

func parseQuery(src string) (*query, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, src: []rune(src)}
	for i, t := range toks {
		if !clausePosition(toks, i) {
			continue
		}
		for _, w := range writeClauses {
			if t.keyword(w) {
				return nil, errWriteQuery
			}
		}
	}
	q := &query{limit: -1}
	if !p.acceptKw("MATCH") {
		if p.peek().keyword("OPTIONAL") || p.peek().keyword("WITH") || p.peek().keyword("UNWIND") || p.peek().keyword("CALL") {
			return nil, p.errorf("unsupported clause")
		}
		return nil, p.errorf("a query starts with MATCH")
	}
	for {
		pat, err := p.parsePattern()
		if err != nil {
			return nil, err
		}
		q.patterns = append(q.patterns, pat)
		if p.acceptPunct(",") {
			continue
		}
		if p.acceptKw("MATCH") {
			continue
		}
		break
	}
	if p.acceptKw("WHERE") {
		if q.where, err = p.parseExpr(); err != nil {
			return nil, err
		}
	}
	if !p.acceptKw("RETURN") {
		return nil, p.errorf("expected RETURN")
	}
	q.distinct = p.acceptKw("DISTINCT")
	for {
		item, err := p.parseReturnItem()
		if err != nil {
			return nil, err
		}
		q.items = append(q.items, item)
		if !p.acceptPunct(",") {
			break
		}
	}
	if p.acceptKw("ORDER") {
		if !p.acceptKw("BY") {
			return nil, p.errorf("expected BY")
		}
		for {
			start := p.peek().pos
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			k := sortKey{e: e, text: p.textFrom(start)}
			if p.acceptKw("DESC") || p.acceptKw("DESCENDING") {
				k.desc = true
			} else if !p.acceptKw("ASC") {
				p.acceptKw("ASCENDING")
			}
			q.order = append(q.order, k)
			if !p.acceptPunct(",") {
				break
			}
		}
	}
	if p.acceptKw("SKIP") {
		if q.skip, err = p.count("SKIP"); err != nil {
			return nil, err
		}
	}
	if p.acceptKw("LIMIT") {
		if q.limit, err = p.count("LIMIT"); err != nil {
			return nil, err
		}
	}
	if p.peek().kind != tokEOF {
		return nil, p.errorf("unexpected input")
	}
	return q, nil
}

// textFrom is the query text from offset start to the current token,
// trimmed, to match ORDER BY expressions with RETURN items.
func (p *parser) textFrom(start int) string {
	end := p.peek().pos
	if end > len(p.src) {
		end = len(p.src)
	}
	return strings.Join(strings.Fields(string(p.src[start:end])), " ")
}

func (p *parser) count(what string) (int64, error) {
	t := p.next()
	n, err := strconv.ParseInt(t.text, 10, 64)
	if t.kind != tokNumber || err != nil || n < 0 {
		p.pos--
		return 0, p.errorf("%s takes a non-negative integer", what)
	}
	return n, nil
}

func (p *parser) parsePattern() (pattern, error) {
	var pat pattern
	n, err := p.parseNode()
	if err != nil {
		return pat, err
	}
	pat.nodes = append(pat.nodes, n)
	for p.peek().punct("-") || p.peek().punct("<") {
		r, err := p.parseRel()
		if err != nil {
			return pat, err
		}
		n, err := p.parseNode()
		if err != nil {
			return pat, err
		}
		pat.rels = append(pat.rels, r)
		pat.nodes = append(pat.nodes, n)
	}
	return pat, nil
}

func (p *parser) parseNode() (nodePat, error) {
	var n nodePat
	if err := p.expectPunct("("); err != nil {
		return n, err
	}
	if p.peek().kind == tokIdent {
		n.name = p.next().text
	}
	if p.acceptPunct(":") {
		label, err := p.ident()
		if err != nil {
			return n, err
		}
		n.label = label
		if p.peek().punct(":") {
			return n, p.errorf("a node has one label")
		}
	}
	if p.peek().punct("{") {
		props, err := p.parseProps()
		if err != nil {
			return n, err
		}
		n.props = props
	}
	return n, p.expectPunct(")")
}

func (p *parser) parseRel() (relPat, error) {
	r := relPat{dir: dirBoth}
	in := p.acceptPunct("<")
	if err := p.expectPunct("-"); err != nil {
		return r, err
	}
	if p.acceptPunct("[") {
		if p.peek().kind == tokIdent {
			r.name = p.next().text
		}
		if p.acceptPunct(":") {
			for {
				t, err := p.ident()
				if err != nil {
					return r, err
				}
				r.types = append(r.types, t)
				if !p.acceptPunct("|") {
					break
				}
				p.acceptPunct(":")
			}
		}
		if p.peek().punct("*") {
			return r, p.errorf("variable-length relationships are not supported")
		}
		if p.peek().punct("{") {
			props, err := p.parseProps()
			if err != nil {
				return r, err
			}
			r.props = props
		}
		if err := p.expectPunct("]"); err != nil {
			return r, err
		}
	}
	if err := p.expectPunct("-"); err != nil {
		return r, err
	}
	out := p.acceptPunct(">")
	switch {
	case in && out:
		return r, p.errorf("a relationship has one direction")
	case in:
		r.dir = dirIn
	case out:
		r.dir = dirOut
	}
	return r, nil
}

func (p *parser) parseProps() (map[string]expr, error) {
	if err := p.expectPunct("{"); err != nil {
		return nil, err
	}
	props := map[string]expr{}
	if p.acceptPunct("}") {
		return props, nil
	}
	for {
		key, err := p.ident()
		if err != nil {
			return nil, err
		}
		if err := p.expectPunct(":"); err != nil {
			return nil, err
		}
		v, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		props[key] = v
		if !p.acceptPunct(",") {
			break
		}
	}
	return props, p.expectPunct("}")
}

func (p *parser) parseReturnItem() (returnItem, error) {
	start := p.peek().pos
	var item returnItem
	if t, nt := p.peek(), p.toks[min(p.pos+1, len(p.toks)-1)]; t.kind == tokIdent && aggregates[strings.ToLower(t.text)] && nt.punct("(") {
		item.agg = strings.ToLower(t.text)
		p.pos += 2
		if item.agg == "count" && p.acceptPunct("*") {
			item.star = true
		} else {
			item.dist = p.acceptKw("DISTINCT")
			e, err := p.parseExpr()
			if err != nil {
				return item, err
			}
			item.e = e
		}
		if err := p.expectPunct(")"); err != nil {
			return item, err
		}
		if t := p.peek(); !t.keyword("AS") && !t.punct(",") && !t.keyword("ORDER") && !t.keyword("SKIP") && !t.keyword("LIMIT") && t.kind != tokEOF {
			return item, p.errorf("an aggregate must be a whole RETURN item")
		}
	} else {
		e, err := p.parseExpr()
		if err != nil {
			return item, err
		}
		item.e = e
	}
	item.text = p.textFrom(start)
	item.name = item.text
	if p.acceptKw("AS") {
		name, err := p.ident()
		if err != nil {
			return item, err
		}
		item.name, item.alias = name, true
	}
	return item, nil
}

// Expressions, loosest first: OR, XOR, AND, NOT, comparison, + -, * / %,
// unary minus, property access, primary.

func (p *parser) parseExpr() (expr, error) { return p.parseOr() }

func (p *parser) parseOr() (expr, error) {
	l, err := p.parseXor()
	for err == nil && p.acceptKw("OR") {
		var r expr
		if r, err = p.parseXor(); err == nil {
			l = binExpr{"OR", l, r}
		}
	}
	return l, err
}

func (p *parser) parseXor() (expr, error) {
	l, err := p.parseAnd()
	for err == nil && p.acceptKw("XOR") {
		var r expr
		if r, err = p.parseAnd(); err == nil {
			l = binExpr{"XOR", l, r}
		}
	}
	return l, err
}

func (p *parser) parseAnd() (expr, error) {
	l, err := p.parseNot()
	for err == nil && p.acceptKw("AND") {
		var r expr
		if r, err = p.parseNot(); err == nil {
			l = binExpr{"AND", l, r}
		}
	}
	return l, err
}

func (p *parser) parseNot() (expr, error) {
	if p.acceptKw("NOT") {
		x, err := p.parseNot()
		return unaryExpr{"NOT", x}, err
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() (expr, error) {
	l, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		switch {
		case t.kind == tokPunct && (t.text == "=" || t.text == "<>" || t.text == "!=" || t.text == "<" || t.text == ">" || t.text == "<=" || t.text == ">="):
			p.pos++
			r, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			op := t.text
			if op == "!=" {
				op = "<>"
			}
			l = binExpr{op, l, r}
		case t.keyword("IS"):
			p.pos++
			not := p.acceptKw("NOT")
			if !p.acceptKw("NULL") {
				return nil, p.errorf("expected NULL")
			}
			l = isNullExpr{l, not}
		case t.keyword("IN"), t.keyword("CONTAINS"):
			p.pos++
			r, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			l = binExpr{strings.ToUpper(t.text), l, r}
		case t.keyword("STARTS"), t.keyword("ENDS"):
			p.pos++
			if !p.acceptKw("WITH") {
				return nil, p.errorf("expected WITH")
			}
			r, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			l = binExpr{strings.ToUpper(t.text), l, r}
		default:
			return l, nil
		}
	}
}

func (p *parser) parseAdd() (expr, error) {
	l, err := p.parseMul()
	for err == nil && (p.peek().punct("+") || p.peek().punct("-")) {
		op := p.next().text
		var r expr
		if r, err = p.parseMul(); err == nil {
			l = binExpr{op, l, r}
		}
	}
	return l, err
}

func (p *parser) parseMul() (expr, error) {
	l, err := p.parseUnary()
	for err == nil && (p.peek().punct("*") || p.peek().punct("/") || p.peek().punct("%")) {
		op := p.next().text
		var r expr
		if r, err = p.parseUnary(); err == nil {
			l = binExpr{op, l, r}
		}
	}
	return l, err
}

func (p *parser) parseUnary() (expr, error) {
	if p.acceptPunct("-") {
		x, err := p.parseUnary()
		return unaryExpr{"-", x}, err
	}
	if p.acceptPunct("+") {
		return p.parseUnary()
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() (expr, error) {
	x, err := p.parsePrimary()
	for err == nil && p.acceptPunct(".") {
		var key string
		if key, err = p.ident(); err == nil {
			x = propExpr{x, key}
		}
	}
	return x, err
}

func (p *parser) parsePrimary() (expr, error) {
	t := p.peek()
	switch {
	case t.kind == tokNumber:
		p.pos++
		if i, err := strconv.ParseInt(t.text, 10, 64); err == nil {
			return litExpr{i}, nil
		}
		f, err := strconv.ParseFloat(t.text, 64)
		if err != nil {
			return nil, p.errorf("bad number %s", t.text)
		}
		return litExpr{f}, nil
	case t.kind == tokString:
		p.pos++
		return litExpr{t.text}, nil
	case t.keyword("TRUE"):
		p.pos++
		return litExpr{true}, nil
	case t.keyword("FALSE"):
		p.pos++
		return litExpr{false}, nil
	case t.keyword("NULL"):
		p.pos++
		return litExpr{nil}, nil
	case t.punct("("):
		p.pos++
		x, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return x, p.expectPunct(")")
	case t.punct("["):
		p.pos++
		var items []expr
		if !p.acceptPunct("]") {
			for {
				x, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				items = append(items, x)
				if !p.acceptPunct(",") {
					break
				}
			}
			if err := p.expectPunct("]"); err != nil {
				return nil, err
			}
		}
		return listExpr{items}, nil
	case t.punct("$"):
		return nil, p.errorf("parameters are not supported")
	case t.kind == tokIdent:
		p.pos++
		if !p.acceptPunct("(") {
			return varExpr{t.text}, nil
		}
		fn := strings.ToLower(t.text)
		if aggregates[fn] {
			p.pos -= 2
			return nil, p.errorf("an aggregate must be a whole RETURN item")
		}
		if _, ok := functions[fn]; !ok {
			p.pos -= 2
			return nil, p.errorf("unknown function %s", t.text)
		}
		var args []expr
		if !p.acceptPunct(")") {
			for {
				x, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				args = append(args, x)
				if !p.acceptPunct(",") {
					break
				}
			}
			if err := p.expectPunct(")"); err != nil {
				return nil, err
			}
		}
		if len(args) != 1 {
			return nil, p.errorf("%s takes one argument", t.text)
		}
		return callExpr{fn, args}, nil
	}
	return nil, p.errorf("expected an expression")
}
