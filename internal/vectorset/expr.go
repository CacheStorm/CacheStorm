package vectorset

import (
	"bytes"
	"math"
	"strings"
)

// Expr is a compiled VSIM FILTER expression, a port of Redis 8.2's expr.c.
// It is immutable once compiled, so Match may run concurrently.
type Expr struct {
	prog     []exprInstr
	maxStack int
}

type exprKind uint8

const (
	kindNum exprKind = iota
	kindStr
	kindTuple
	kindNull
)

type exprValue struct {
	kind  exprKind
	num   float64
	str   []byte
	tuple []exprValue
}

type exprOp uint8

const (
	opOParen exprOp = iota
	opCParen
	opNot
	opPow
	opMult
	opDiv
	opMod
	opSum
	opDiff
	opGT
	opGTE
	opLT
	opLTE
	opEQ
	opNEQ
	opIn
	opAnd
	opOr
)

type exprOpDef struct {
	name  string
	code  exprOp
	prec  int
	arity int
}

var exprOpTable = []exprOpDef{
	{"(", opOParen, 7, 0},
	{")", opCParen, 7, 0},
	{"!", opNot, 6, 1},
	{"not", opNot, 6, 1},
	{"**", opPow, 5, 2},
	{"*", opMult, 4, 2},
	{"/", opDiv, 4, 2},
	{"%", opMod, 4, 2},
	{"+", opSum, 3, 2},
	{"-", opDiff, 3, 2},
	{">", opGT, 2, 2},
	{">=", opGTE, 2, 2},
	{"<", opLT, 2, 2},
	{"<=", opLTE, 2, 2},
	{"==", opEQ, 2, 2},
	{"!=", opNEQ, 2, 2},
	{"in", opIn, 2, 2},
	{"and", opAnd, 1, 2},
	{"&&", opAnd, 1, 2},
	{"or", opOr, 0, 2},
	{"||", opOr, 0, 2},
}

var exprOpPrec, exprOpArity [opOr + 1]int

func init() {
	for i := len(exprOpTable) - 1; i >= 0; i-- {
		exprOpPrec[exprOpTable[i].code] = exprOpTable[i].prec
		exprOpArity[exprOpTable[i].code] = exprOpTable[i].arity
	}
}

const opSpecialChars = "+-*%/!()<>=|&"

type exprTokKind uint8

const (
	tokValue exprTokKind = iota
	tokSelector
	tokOp
	tokEOF
)

type exprInstr struct {
	kind exprTokKind
	op   exprOp
	val  exprValue
	sel  string
}

type exprLexer struct {
	s string
	p int
}

func (l *exprLexer) at(i int) byte {
	if i < len(l.s) {
		return l.s[i]
	}
	return 0
}

func isOperatorChar(c byte) bool {
	return isAlphaC(c) || strings.IndexByte(opSpecialChars, c) >= 0
}

func isSelectorChar(c byte) bool {
	return isAlphaC(c) || isDigitC(c) || c == '_' || c == '-'
}

func (l *exprLexer) skipSpaces() {
	for isSpaceC(l.at(l.p)) {
		l.p++
	}
}

// parseOperatorOrLiteral consumes the whole run of letters and operator
// characters, then keeps only its longest operator prefix. On failure the
// position stays at the end of the run, which is where Redis reports the
// error. Only an exact four-byte run "null" is the null literal, so "null)"
// is a syntax error in Redis too.
func (l *exprLexer) parseOperatorOrLiteral() (exprInstr, bool) {
	start := l.p
	for {
		c := l.at(l.p)
		if c == 0 || !isOperatorChar(c) {
			break
		}
		l.p++
	}
	run := l.s[start:l.p]
	if run == "null" {
		return exprInstr{kind: tokValue, val: exprValue{kind: kindNull}}, true
	}
	best := 0
	var code exprOp
	for _, o := range exprOpTable {
		if len(o.name) > best && strings.HasPrefix(run, o.name) {
			best = len(o.name)
			code = o.code
		}
	}
	if best == 0 {
		return exprInstr{}, false
	}
	l.p = start + best
	return exprInstr{kind: tokOp, op: code}, true
}

// parseNumber accepts digits, '.', 'e' and 'E' plus a leading '-', so an
// exponent sign ("1e-5") is not part of the number and makes it invalid.
func (l *exprLexer) parseNumber() (exprValue, bool) {
	start := l.p
	for {
		c := l.at(l.p)
		if !isDigitC(c) && c != '.' && c != 'e' && c != 'E' && (l.p != start || c != '-') {
			break
		}
		if l.p-start >= 255 {
			return exprValue{}, false
		}
		l.p++
	}
	lit := []byte(l.s[start:l.p])
	v, n := strtod(lit)
	if n != len(lit) {
		return exprValue{}, false
	}
	return exprValue{kind: kindNum, num: v}, true
}

// parseString keeps escapes verbatim: Redis only uses the backslash to skip
// the following byte while looking for the closing quote.
func (l *exprLexer) parseString() (exprValue, bool) {
	quote := l.s[l.p]
	l.p++
	start := l.p
	for l.p < len(l.s) {
		c := l.s[l.p]
		if c == '\\' && l.at(l.p+1) != 0 {
			l.p += 2
			continue
		}
		if c == quote {
			v := exprValue{kind: kindStr, str: []byte(l.s[start:l.p])}
			l.p++
			return v, true
		}
		l.p++
	}
	return exprValue{}, false
}

// parseTuple accepts only number and string elements and tolerates a
// trailing comma, as Redis does.
func (l *exprLexer) parseTuple() (exprValue, bool) {
	l.p++
	t := exprValue{kind: kindTuple, tuple: []exprValue{}}
	for {
		l.skipSpaces()
		if l.at(l.p) == ']' {
			l.p++
			return t, true
		}
		var ele exprValue
		var ok bool
		switch c := l.at(l.p); {
		case isDigitC(c) || c == '-':
			ele, ok = l.parseNumber()
		case c == '"' || c == '\'':
			ele, ok = l.parseString()
		}
		if !ok {
			return exprValue{}, false
		}
		t.tuple = append(t.tuple, ele)
		l.skipSpaces()
		if l.at(l.p) == ']' {
			l.p++
			return t, true
		}
		if l.at(l.p) != ',' {
			return exprValue{}, false
		}
		l.p++
	}
}

func (l *exprLexer) tokenize() ([]exprInstr, int) {
	var toks []exprInstr
	for {
		l.skipSpaces()
		minusIsNumber := len(toks) == 0 ||
			(toks[len(toks)-1].kind == tokOp && toks[len(toks)-1].op != opCParen)
		c := l.at(l.p)
		var cur exprInstr
		ok := true
		switch {
		case c == 0:
			cur = exprInstr{kind: tokEOF}
		case isDigitC(c) || (minusIsNumber && c == '-' && isDigitC(l.at(l.p+1))):
			cur.val, ok = l.parseNumber()
		case c == '"' || c == '\'':
			cur.val, ok = l.parseString()
		case c == '.' && (l.p+1 == len(l.s) || isSelectorChar(l.at(l.p+1))):
			// Redis tests the byte after the dot with strchr(), which also
			// matches the terminating NUL, so a trailing "." is a selector
			// with an empty name that never matches.
			l.p++
			start := l.p
			for isSelectorChar(l.at(l.p)) {
				l.p++
			}
			cur = exprInstr{kind: tokSelector, sel: l.s[start:l.p]}
		case c == '[':
			cur.val, ok = l.parseTuple()
		case isOperatorChar(c):
			cur, ok = l.parseOperatorOrLiteral()
		default:
			ok = false
		}
		if !ok {
			return nil, l.p
		}
		toks = append(toks, cur)
		if cur.kind == tokEOF {
			return toks, 0
		}
	}
}

// Compile compiles a FILTER expression. On a syntax error it returns nil and
// the byte offset Redis's exprCompile reports in errpos.
//
// Tokenizer errors report where scanning stopped. Every later error
// (unbalanced parentheses, missing operands, leftover values) reports 0,
// because Redis never fills in the token offsets it reports there. Like
// Redis, which copies the argument as a C string, the expression ends at its
// first NUL byte.
func Compile(src string) (e *Expr, errpos int) {
	if i := strings.IndexByte(src, 0); i >= 0 {
		src = src[:i]
	}
	l := &exprLexer{s: src}
	toks, pos := l.tokenize()
	if toks == nil {
		return nil, pos
	}

	var prog []exprInstr
	var ops []exprOp
	items := 0
	depth := 0
	emit := func(op exprOp) bool {
		a := exprOpArity[op]
		if items < a {
			return false
		}
		prog = append(prog, exprInstr{kind: tokOp, op: op})
		items = items - a + 1
		return true
	}
	for _, t := range toks {
		if t.kind == tokEOF {
			break
		}
		if t.kind != tokOp {
			prog = append(prog, t)
			items++
			if items > depth {
				depth = items
			}
			continue
		}
		switch t.op {
		case opOParen:
			ops = append(ops, t.op)
			continue
		case opCParen:
			for {
				if len(ops) == 0 {
					return nil, 0
				}
				top := ops[len(ops)-1]
				ops = ops[:len(ops)-1]
				if top == opOParen {
					break
				}
				if !emit(top) {
					return nil, 0
				}
			}
			continue
		}
		prec := exprOpPrec[t.op]
		for len(ops) > 0 {
			top := ops[len(ops)-1]
			if top == opOParen {
				break
			}
			tp := exprOpPrec[top]
			if tp < prec || (t.op == opPow && tp <= prec) {
				break
			}
			ops = ops[:len(ops)-1]
			if !emit(top) {
				return nil, 0
			}
		}
		ops = append(ops, t.op)
	}
	for len(ops) > 0 {
		top := ops[len(ops)-1]
		ops = ops[:len(ops)-1]
		if top == opOParen || !emit(top) {
			return nil, 0
		}
	}
	if items != 1 {
		return nil, 0
	}
	return &Expr{prog: prog, maxStack: depth}, 0
}

func exprToNum(v *exprValue) float64 {
	switch v.kind {
	case kindNum:
		return v.num
	case kindStr:
		if len(v.str) >= 256 {
			return 0
		}
		s := v.str
		// Redis copies the string into a C buffer, so strtod stops at a NUL.
		if i := bytes.IndexByte(s, 0); i >= 0 {
			s = s[:i]
		}
		n, used := strtod(s)
		if used != len(s) {
			return 0
		}
		return n
	}
	return 0
}

func exprToBool(v *exprValue) bool {
	switch v.kind {
	case kindNum:
		return v.num != 0
	case kindStr:
		return len(v.str) != 0
	case kindNull:
		return false
	}
	return true
}

func exprEqual(a, b *exprValue) bool {
	if a.kind == kindStr && b.kind == kindStr {
		return bytes.Equal(a.str, b.str)
	}
	if a.kind == kindNum && b.kind == kindNum {
		return a.num == b.num
	}
	if a.kind == kindNull || b.kind == kindNull {
		return a.kind == b.kind
	}
	return exprToNum(a) == exprToNum(b)
}

func exprBool(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// Match runs the expression against an element's JSON attributes (Redis's
// exprRun). Any selector whose field is missing or not convertible, or JSON
// that cannot be read up to that field, makes the whole expression false.
func (e *Expr) Match(attrs []byte) bool {
	var buf [16]exprValue
	stack := buf[:0]
	if e.maxStack > len(buf) {
		stack = make([]exprValue, 0, e.maxStack)
	}
	for i := range e.prog {
		in := &e.prog[i]
		switch in.kind {
		case tokSelector:
			if in.sel == "" {
				return false
			}
			v, ok := jsonExtractField(attrs, in.sel)
			if !ok {
				return false
			}
			stack = append(stack, v)
			continue
		case tokValue:
			stack = append(stack, in.val)
			continue
		}
		n := len(stack)
		b := &stack[n-1]
		if in.op == opNot {
			stack[n-1] = exprValue{kind: kindNum, num: exprBool(!exprToBool(b))}
			continue
		}
		a := &stack[n-2]
		var r float64
		switch in.op {
		case opPow:
			r = cpow(exprToNum(a), exprToNum(b))
		case opMult:
			r = exprToNum(a) * exprToNum(b)
		case opDiv:
			r = exprToNum(a) / exprToNum(b)
		case opMod:
			r = math.Mod(exprToNum(a), exprToNum(b))
		case opSum:
			r = exprToNum(a) + exprToNum(b)
		case opDiff:
			r = exprToNum(a) - exprToNum(b)
		case opGT:
			r = exprBool(exprToNum(a) > exprToNum(b))
		case opGTE:
			r = exprBool(exprToNum(a) >= exprToNum(b))
		case opLT:
			r = exprBool(exprToNum(a) < exprToNum(b))
		case opLTE:
			r = exprBool(exprToNum(a) <= exprToNum(b))
		case opEQ:
			r = exprBool(exprEqual(a, b))
		case opNEQ:
			r = exprBool(!exprEqual(a, b))
		case opIn:
			if b.kind == kindTuple {
				for j := range b.tuple {
					if exprEqual(a, &b.tuple[j]) {
						r = 1
						break
					}
				}
			} else if a.kind == kindStr && b.kind == kindStr {
				r = exprBool(bytes.Contains(b.str, a.str))
			}
		case opAnd:
			r = exprBool(exprToBool(a) && exprToBool(b))
		case opOr:
			r = exprBool(exprToBool(a) || exprToBool(b))
		}
		stack = stack[:n-1]
		stack[n-2] = exprValue{kind: kindNum, num: r}
	}
	if len(stack) == 0 {
		return false
	}
	return exprToBool(&stack[len(stack)-1])
}
