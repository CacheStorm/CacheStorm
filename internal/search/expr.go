package search

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/cmath"
	"github.com/cachestorm/cachestorm/internal/store"
)

// APPLY and FILTER expressions follow RediSearch's: its Ragel lexer (a
// signed number is one token, unknown punctuation is skipped), its lemon
// grammar, and its evaluator, whose error status persists from row to row
// of a query.

// Value types, numbered as RediSearch's RSValueType for its messages.
const (
	xUndef = 0
	xNum   = 1
	xStr   = 3
	xNull  = 4
	xArr   = 6
	xOwn   = 7
)

// xval is an RSValue. A missing property evaluates to a null that exists()
// tells apart from other nulls.
type xval struct {
	t       uint8
	n       float64
	s       string
	arr     []xval
	missing bool
}

func xnum(f float64) xval   { return xval{t: xNum, n: f} }
func xstr(s string) xval    { return xval{t: xStr, s: s} }
func (v xval) isNull() bool { return v.t == xNull }
func (v xval) isString() bool {
	return v.t == xStr || v.t == xOwn
}

// toNumber is RSValue_ToNumber.
func (v xval) toNumber() (float64, bool) {
	switch v.t {
	case xNum:
		return v.n, true
	case xStr, xOwn:
		return parseCNumber(v.s)
	}
	return 0, false
}

// parseCNumber reads a whole string as strtod does, an empty one as 0.
func parseCNumber(s string) (float64, bool) {
	s = strings.TrimLeft(s, " \t\n\v\f\r")
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		var ne *strconv.NumError
		if errors.As(err, &ne) && ne.Err == strconv.ErrRange && !math.IsInf(f, 0) && f != 0 {
			return f, true
		}
		return 0, false
	}
	if strings.ContainsAny(s, "_xXpP") {
		return 0, false
	}
	return f, true
}

func (v xval) boolTest() bool {
	switch v.t {
	case xArr:
		return len(v.arr) != 0
	case xNum:
		return v.n != 0
	case xStr, xOwn:
		return v.s != ""
	}
	return false
}

// numString is RSValue_NumToString: an integer as %lld, else %.12g.
func numString(f float64) string { return FormatNumber(f) }

// toString is RSValue_ToString.
func (v xval) toString() xval {
	switch v.t {
	case xStr, xOwn:
		return v
	case xNum:
		return xstr(numString(v.n))
	}
	return xstr("")
}

// convertString is RSValue_ConvertStringPtrLen: numbers as %f.
func (v xval) convertString() string {
	switch v.t {
	case xStr, xOwn:
		return v.s
	case xNum:
		return fmt.Sprintf("%f", v.n)
	}
	return ""
}

// ptrString is RSValue_StringPtrLen, which has no string for anything
// but a string; C prints its absence as (null).
func (v xval) ptrString() (string, bool) {
	if v.isString() {
		return v.s, true
	}
	return "", false
}

// exprStatus is a query's error status: the first error stays until
// exists() clears it.
type exprStatus struct {
	set bool
	msg string
}

func (st *exprStatus) setError(msg string) {
	if !st.set {
		st.set, st.msg = true, msg
	}
}

// EvalState carries an aggregation's error status across its APPLY and
// FILTER steps.
type EvalState struct{ st exprStatus }

// Err is the status' error, or nil.
func (s *EvalState) Err() error {
	if s == nil || !s.st.set {
		return nil
	}
	return errors.New(s.st.msg)
}

// Set reports whether an error is set.
func (s *EvalState) Set() bool { return s != nil && s.st.set }

const (
	evalOK = iota
	evalErr
	evalNull
)

// Expression nodes.
const (
	nProp = iota
	nLit
	nFunc
	nOp
	nPred
	nNot
)

// Predicate conditions.
const (
	cEq = iota
	cNe
	cLt
	cLe
	cGt
	cGe
	cAnd
	cOr
)

var condNames = map[int]string{cEq: "==", cNe: "!=", cLt: "<", cLe: "<=", cGt: ">", cGe: ">=", cAnd: "&&", cOr: "||"}

type xnode struct {
	kind  int
	name  string
	lit   xval
	null  bool
	op    byte
	cond  int
	fn    *exprFunc
	args  []*xnode
	l, r  *xnode
	decay bool
}

// Expr is a parsed APPLY or FILTER expression.
type Expr struct {
	root *xnode
}

// Properties returns the properties the expression reads, in the order
// RediSearch resolves them.
func (e *Expr) Properties() []string {
	var out []string
	var walk func(n *xnode)
	walk = func(n *xnode) {
		if n == nil {
			return
		}
		switch n.kind {
		case nProp:
			out = append(out, n.name)
		case nFunc:
			for _, a := range n.args {
				walk(a)
			}
		case nOp, nPred:
			walk(n.l)
			walk(n.r)
		case nNot:
			walk(n.l)
		}
	}
	walk(e.root)
	return out
}

// Lexing.

const (
	tEOF = iota
	tNumber
	tProperty
	tSymbol
	tString
	tLP
	tRP
	tMinus
	tPlus
	tDiv
	tTimes
	tMod
	tPow
	tComma
	tEq
	tNe
	tLt
	tLe
	tGt
	tGe
	tAnd
	tOr
	tNot
)

type xtoken struct {
	kind int
	pos  int
	text string
	num  float64
}

func isCPunct(c byte) bool {
	return c >= '!' && c <= '/' || c >= ':' && c <= '@' || c >= '[' && c <= '`' || c >= '{' && c <= '~'
}

func isCSpace(c byte) bool { return c == ' ' || c >= '\t' && c <= '\r' }

func isCCntrl(c byte) bool { return c < 32 || c == 127 }

func isAlpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// matchNumber is [+-]? digit+ ('.' digit+)? ([eE] [+-]? digit+)?.
func matchNumber(s string, i int) int {
	j := i
	if j < len(s) && (s[j] == '+' || s[j] == '-') {
		j++
	}
	d := j
	for j < len(s) && isDigit(s[j]) {
		j++
	}
	if j == d {
		return 0
	}
	if j+1 < len(s) && s[j] == '.' && isDigit(s[j+1]) {
		j += 2
		for j < len(s) && isDigit(s[j]) {
			j++
		}
	}
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		k := j + 1
		if k < len(s) && (s[k] == '+' || s[k] == '-') {
			k++
		}
		if k < len(s) && isDigit(s[k]) {
			for k < len(s) && isDigit(s[k]) {
				k++
			}
			j = k
		}
	}
	return j - i
}

func matchInf(s string, i int) int {
	j := i
	if j < len(s) && (s[j] == '+' || s[j] == '-') {
		j++
	}
	if strings.HasPrefix(s[j:], "inf") {
		return j + 3 - i
	}
	return 0
}

func matchProperty(s string, i int) int {
	if s[i] != '@' {
		return 0
	}
	j := i + 1
	for j < len(s) {
		c := s[j]
		switch {
		case c == '\\' && j+1 < len(s) && (isCPunct(s[j+1]) || isCSpace(s[j+1])):
			j += 2
		case c == '_' || !isCPunct(c) && !isCCntrl(c) && !isCSpace(c) && c != '\\':
			j++
		default:
			if j == i+1 {
				return 0
			}
			return j - i
		}
	}
	if j == i+1 {
		return 0
	}
	return j - i
}

func matchSymbol(s string, i int) int {
	if !isAlpha(s[i]) {
		return 0
	}
	j := i + 1
	for j < len(s) && (isAlpha(s[j]) || isDigit(s[j]) || s[j] == '_') {
		j++
	}
	return j - i
}

// matchString is a quoted string whose escapes take punctuation, spaces
// or a backslash; the longest match wins.
func matchString(s string, i int) int {
	q := s[i]
	if q != '"' && q != '\'' {
		return 0
	}
	best := 0
	var walk func(j int)
	seen := map[int]bool{}
	walk = func(j int) {
		for j < len(s) {
			if seen[j] {
				return
			}
			seen[j] = true
			c := s[j]
			if c == '\\' && j+1 < len(s) && (isCPunct(s[j+1]) || isCSpace(s[j+1])) {
				walk(j + 2)
			}
			if c == q {
				if j+1-i > best {
					best = j + 1 - i
				}
				return
			}
			if c == '\n' {
				return
			}
			j++
		}
	}
	walk(i + 1)
	return best
}

var exprOperators = []struct {
	s    string
	kind int
}{
	{"(", tLP}, {")", tRP}, {"-", tMinus}, {"<", tLt}, {"<=", tLe}, {">", tGt}, {">=", tGe}, {"==", tEq},
	{"!", tNot}, {"!=", tNe}, {"&&", tAnd}, {"||", tOr}, {"+", tPlus}, {"%", tMod}, {"^", tPow}, {"/", tDiv},
	{"*", tTimes}, {",", tComma},
}

// lexExpr splits an expression as RediSearch's scanner does: the longest
// match wins, the earlier rule on a tie, and a byte nothing matches ends
// the input.
func lexExpr(s string) []xtoken {
	var out []xtoken
	for i := 0; i < len(s); {
		best, kind := 0, -1
		try := func(n, k int) {
			if n > best {
				best, kind = n, k
			}
		}
		try(matchNumber(s, i), tNumber)
		try(matchProperty(s, i), tProperty)
		try(matchSymbol(s, i), tSymbol)
		if n := matchInf(s, i); n > best {
			best, kind = n, -2
		}
		for _, op := range exprOperators {
			if strings.HasPrefix(s[i:], op.s) {
				try(len(op.s), op.kind)
			}
		}
		try(matchString(s, i), tString)
		c := s[i]
		if best == 0 {
			if isCSpace(c) || isCPunct(c) || isCCntrl(c) {
				i++
				continue
			}
			break
		}
		tok := xtoken{kind: kind, pos: i}
		switch kind {
		case tNumber:
			tok.text = s[i : i+best]
			if f, err := strconv.ParseFloat(tok.text, 64); err == nil || errors.Is(err, strconv.ErrRange) {
				tok.num = f
			}
		case -2:
			tok.kind, tok.text = tNumber, s[i:i+best]
			tok.num = math.Inf(1)
			if c == '-' {
				tok.num = math.Inf(-1)
			}
		case tProperty:
			tok.text = s[i+1 : i+best]
		case tSymbol, tString:
			tok.text = s[i : i+best]
		}
		out = append(out, tok)
		i += best
	}
	return out
}

// Parsing.

// Parser contexts, which decide the tokens that may follow an expression.
const (
	ctxParen = iota + 1
	ctxCall
)

type exprParser struct {
	toks []xtoken
	i    int
	// near is the text of the last token that had one, which lemon's
	// syntax error prints with the failing token's offset.
	near string
	ctx  []int
	// frames are the operator levels being parsed, innermost last: the
	// least precedence an operator needs to continue each, and whether it
	// is a comparison's right side, which another comparison cannot follow.
	frames []parseFrame
	// reason is the error a failed reduction left: lemon reduces when the
	// next token arrives, and reports it alone when that token fits, with
	// a syntax error when it does not, and not at all when it is the end
	// of the input, where the tree is built with a hole instead.
	reason string
}

type parseFrame struct {
	min      int
	cmpRight bool
}

type exprStop struct{ msg string }

func (e *exprStop) Error() string { return e.msg }

func (p *exprParser) peek() xtoken {
	if p.i < len(p.toks) {
		return p.toks[p.i]
	}
	t := xtoken{kind: tEOF}
	if len(p.toks) > 0 {
		t.pos = p.toks[len(p.toks)-1].pos
	}
	return t
}

func (p *exprParser) advance() xtoken {
	t := p.peek()
	if p.i < len(p.toks) {
		p.i++
		switch t.kind {
		case tNumber, tProperty, tSymbol, tString:
			p.near = t.text
		}
	}
	return t
}

// syntaxError reports the token the grammar cannot take, as lemon does.
func (p *exprParser) syntaxError() error {
	t := p.peek()
	if t.kind != tEOF {
		switch t.kind {
		case tNumber, tProperty, tSymbol, tString:
			p.near = t.text
		}
	}
	msg := fmt.Sprintf("Syntax error at offset %d near '%s'", t.pos, p.near)
	if p.reason != "" {
		msg += ": " + p.reason
	}
	return &exprStop{msg}
}

// fits reports whether a token may follow a complete expression here.
func (p *exprParser) fits(kind int) bool {
	ctx := 0
	if len(p.ctx) > 0 {
		ctx = p.ctx[len(p.ctx)-1]
	}
	switch kind {
	case tEOF:
		return ctx == 0
	case tRP:
		return ctx != 0
	case tComma:
		return ctx == ctxCall
	}
	prec := binaryPrec(kind)
	if prec == 0 {
		return false
	}
	for i := len(p.frames) - 1; i >= 0; i-- {
		f := p.frames[i]
		if prec > f.min {
			return true
		}
		if f.cmpRight && prec == precCmp {
			return false
		}
	}
	return true
}

// fail is a reduction that finds an error: it yields a hole, which only
// the end of the input lets the parse go on with.
func (p *exprParser) fail(msg string) (*xnode, error) {
	p.reason = msg
	t := p.peek()
	if !p.fits(t.kind) {
		return nil, p.syntaxError()
	}
	if t.kind != tEOF {
		return nil, &exprStop{msg}
	}
	return nil, nil
}

// ParseExpr parses an APPLY or FILTER expression.
func ParseExpr(s string) (*Expr, error) {
	p := &exprParser{toks: lexExpr(s)}
	n, err := p.parse(0)
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tEOF {
		return nil, p.syntaxError()
	}
	if n == nil {
		return nil, errors.New(p.reason)
	}
	return &Expr{root: n}, nil
}

const (
	precOr = iota + 1
	precAnd
	precNot
	precCmp
	precAdd
	precMul
	precPow
)

func binaryPrec(kind int) int {
	switch kind {
	case tOr:
		return precOr
	case tAnd:
		return precAnd
	case tEq, tNe, tLt, tLe, tGt, tGe:
		return precCmp
	case tPlus, tMinus:
		return precAdd
	case tTimes, tDiv, tMod:
		return precMul
	case tPow:
		return precPow
	}
	return 0
}

// parse reads an expression whose operators bind tighter than min.
func (p *exprParser) parse(min int) (*xnode, error) {
	return p.parseFrame(min, false)
}

func (p *exprParser) parseFrame(min int, cmpRight bool) (*xnode, error) {
	depth := len(p.frames)
	p.frames = append(p.frames, parseFrame{min: min, cmpRight: cmpRight})
	defer func() { p.frames = p.frames[:depth] }()
	left, err := p.prefix()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		prec := binaryPrec(t.kind)
		if prec == 0 || prec <= min {
			return left, nil
		}
		p.advance()
		next := prec
		if t.kind == tPow {
			next = prec - 1
		}
		right, err := p.parseFrame(next, prec == precCmp)
		if err != nil {
			return nil, err
		}
		left = combine(t.kind, left, right)
		if prec == precCmp && binaryPrec(p.peek().kind) == precCmp {
			return nil, p.syntaxError()
		}
	}
}

func combine(kind int, l, r *xnode) *xnode {
	ops := map[int]byte{tPlus: '+', tMinus: '-', tTimes: '*', tDiv: '/', tMod: '%', tPow: '^'}
	if op, ok := ops[kind]; ok {
		return &xnode{kind: nOp, op: op, l: l, r: r}
	}
	conds := map[int]int{tEq: cEq, tNe: cNe, tLt: cLt, tLe: cLe, tGt: cGt, tGe: cGe, tAnd: cAnd, tOr: cOr}
	return &xnode{kind: nPred, cond: conds[kind], l: l, r: r}
}

func (p *exprParser) prefix() (*xnode, error) {
	t := p.peek()
	switch t.kind {
	case tNot:
		p.advance()
		x, err := p.parse(precNot)
		if err != nil || x == nil {
			return nil, err
		}
		if x.kind == nNot {
			return x.l, nil
		}
		return &xnode{kind: nNot, l: x}, nil
	case tLP:
		return p.paren()
	case tNumber:
		p.advance()
		return &xnode{kind: nLit, lit: xnum(t.num)}, nil
	case tString:
		p.advance()
		return &xnode{kind: nLit, lit: xstr(unescapeLiteral(t.text))}, nil
	case tProperty:
		p.advance()
		return &xnode{kind: nProp, name: t.text}, nil
	case tSymbol:
		p.advance()
		if p.peek().kind == tLP {
			return p.call(t)
		}
		if t.text == "NULL" {
			return &xnode{kind: nLit, null: true, lit: xval{t: xNull}}, nil
		}
		return p.fail("Unknown symbol '" + t.text + "'")
	}
	return nil, p.syntaxError()
}

func (p *exprParser) paren() (*xnode, error) {
	p.advance()
	p.ctx = append(p.ctx, ctxParen)
	saved := p.frames
	p.frames = nil
	defer func() { p.frames = saved }()
	x, err := p.parse(0)
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tRP {
		return nil, p.syntaxError()
	}
	p.advance()
	p.ctx = p.ctx[:len(p.ctx)-1]
	return x, nil
}

func (p *exprParser) call(name xtoken) (*xnode, error) {
	p.advance()
	p.ctx = append(p.ctx, ctxCall)
	saved := p.frames
	p.frames = nil
	defer func() { p.frames = saved }()
	var args []*xnode
	if p.peek().kind != tRP {
		if p.peek().kind != tComma {
			a, err := p.parse(0)
			if err != nil {
				return nil, err
			}
			args = append(args, a)
		}
		for p.peek().kind == tComma {
			p.advance()
			a, err := p.parse(0)
			if err != nil {
				return nil, err
			}
			args = append(args, a)
		}
		if p.peek().kind != tRP {
			return nil, p.syntaxError()
		}
	}
	p.advance()
	p.ctx = p.ctx[:len(p.ctx)-1]
	p.frames = saved
	fn := exprFunctions[strings.ToLower(name.text)]
	switch {
	case fn == nil:
		return p.fail("Unknown function name '" + name.text + "'")
	case len(args) < fn.min || fn.max >= 0 && len(args) > fn.max:
		if fn.min == fn.max {
			return p.fail(fmt.Sprintf("Function '%s' expects %d arguments, but got %d", name.text, fn.min, len(args)))
		}
		return p.fail(fmt.Sprintf("Function '%s' expects between %d and %d arguments, but got %d", name.text, fn.min, fn.max, len(args)))
	}
	return &xnode{kind: nFunc, name: strings.ToLower(name.text), fn: fn, args: args}, nil
}

// Lookup checks the expression's properties as RediSearch's lookup does,
// in order, after a hole a failed reduction left.
func (e *Expr) Lookup(check func(name string) error) error {
	var walk func(n *xnode) error
	hole := func(n *xnode, what string) error {
		if n == nil {
			return errors.New("Missing (or badly formatted) value for " + what) //nolint:staticcheck // RediSearch's reply text
		}
		return walk(n)
	}
	walk = func(n *xnode) error {
		switch n.kind {
		case nProp:
			return check(n.name)
		case nFunc:
			for _, a := range n.args {
				if err := hole(a, "expr->func.args->args[ii]"); err != nil {
					return err
				}
			}
		case nOp:
			if err := hole(n.l, "expr->op.left"); err != nil {
				return err
			}
			return hole(n.r, "expr->op.right")
		case nPred:
			if err := hole(n.l, "expr->pred.left"); err != nil {
				return err
			}
			return hole(n.r, "expr->pred.right")
		case nNot:
			return hole(n.l, "expr->inverted.child")
		}
		return nil
	}
	return walk(e.root)
}

// unescapeLiteral drops a string literal's quotes and the backslash
// before punctuation or a space.
func unescapeLiteral(s string) string {
	s = s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && (isCPunct(s[i+1]) || isCSpace(s[i+1])) {
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// Evaluation.

type exprEval struct {
	row *Row
	st  *exprStatus
}

// rowValue is a row's value of a property as RediSearch's lookup holds it.
func (r *Row) rowValue(name string) (xval, bool) {
	c := r.cell(name)
	if !c.present {
		return xval{}, false
	}
	return cellValue(c), true
}

func cellValue(c cell) xval {
	switch {
	case c.null:
		return xval{t: xNull}
	case c.isArr:
		out := make([]xval, len(c.arr))
		for i, item := range c.arr {
			out[i] = cellValue(item)
		}
		return xval{t: xArr, arr: out}
	case c.num:
		return xnum(c.f)
	case c.lit:
		return xstr(c.v)
	}
	return xval{t: xOwn, s: c.v}
}

func (ev *exprEval) eval(n *xnode, temp bool) (xval, int) {
	switch n.kind {
	case nProp:
		v, ok := ev.row.rowValue(n.name)
		if !ok {
			ev.st.setError("Could not find the value for a parameter name, consider using EXISTS if applicable for " + n.name)
			return xval{t: xNull, missing: true}, evalNull
		}
		return v, evalOK
	case nLit:
		if n.null {
			if n.decay {
				return xval{t: xUndef}, evalOK
			}
			if temp {
				n.decay = true
			}
		}
		return n.lit, evalOK
	case nFunc:
		return ev.call(n, temp)
	case nOp:
		return ev.op(n)
	case nPred:
		return ev.pred(n)
	case nNot:
		v, rc := ev.eval(n.l, true)
		if rc != evalOK {
			return xval{}, evalErr
		}
		return xnum(b2f(!v.boolTest())), evalOK
	}
	return xval{}, evalErr
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// cNaN64 is the NaN x86 makes of an invalid operation, which C prints as
// -nan.
var cNaN64 = math.Float64frombits(0xfff8000000000000)

func (ev *exprEval) op(n *xnode) (xval, int) {
	l, rc := ev.eval(n.l, true)
	if rc != evalOK {
		return xval{}, evalErr
	}
	r, rc := ev.eval(n.r, true)
	if rc != evalOK {
		return xval{}, evalErr
	}
	a, okA := l.toNumber()
	b, okB := r.toNumber()
	if !okA || !okB {
		ev.st.setError("Could not convert value to a number")
		return xval{}, evalErr
	}
	return xnum(arith(n.op, a, b)), evalOK
}

func arith(op byte, a, b float64) float64 {
	switch op {
	case '+':
		return a + b
	case '-':
		return a - b
	case '*':
		return a * b
	case '/':
		return a / b
	case '%':
		return cFmod(a, b)
	case '^':
		return cPow(a, b)
	}
	return 0
}

// cFmod and cPow give glibc's NaNs: an operand's, else x86's -nan.
func cFmod(a, b float64) float64 {
	switch {
	case a != a:
		return a
	case b != b:
		return b
	case b == 0 || math.IsInf(a, 0):
		return cNaN64
	}
	return math.Mod(a, b)
}

func cPow(a, b float64) float64 {
	r := math.Pow(a, b)
	if r != r {
		switch {
		case a != a && b != 0:
			return a
		case b != b && a != 1:
			return b
		}
		return cNaN64
	}
	return r
}

func (ev *exprEval) pred(n *xnode) (xval, int) {
	logical := n.cond == cAnd || n.cond == cOr
	operand := func(x *xnode) (xval, int, bool) {
		v, rc := ev.eval(x, true)
		if rc == evalErr {
			return v, evalErr, true
		}
		if rc == evalNull && !logical {
			return v, evalErr, true
		}
		if logical {
			b := v.boolTest()
			if n.cond == cAnd && !b || n.cond == cOr && b {
				return xnum(b2f(b)), evalOK, true
			}
		}
		return v, evalOK, false
	}
	l, rc, stop := operand(n.l)
	if stop {
		return l, rc
	}
	r, rc, stop := operand(n.r)
	if stop {
		return r, rc
	}
	var res bool
	switch n.cond {
	case cEq:
		res = valueEqual(l, r, ev.st)
	case cNe:
		res = !valueEqual(l, r, ev.st)
	case cLt:
		res = valueCmp(l, r, ev.st) < 0
	case cLe:
		res = valueCmp(l, r, ev.st) <= 0
	case cGt:
		res = valueCmp(l, r, ev.st) > 0
	case cGe:
		res = valueCmp(l, r, ev.st) >= 0
	case cAnd:
		res = l.boolTest() && r.boolTest()
	case cOr:
		res = l.boolTest() || r.boolTest()
	}
	if ev.st.set {
		return xval{}, evalErr
	}
	return xnum(b2f(res)), evalOK
}

func cmpNum(a, b float64) int {
	switch {
	case a > b:
		return 1
	case a < b:
		return -1
	}
	return 0
}

// cmpSame is RSValue_CmpNC for values of one type.
func cmpSame(a, b xval, st *exprStatus) int {
	switch a.t {
	case xNum:
		return cmpNum(a.n, b.n)
	case xStr, xOwn:
		return strings.Compare(a.s, b.s)
	case xArr:
		if len(a.arr) > 0 && len(b.arr) > 0 {
			return valueCmp(a.arr[0], b.arr[0], st)
		}
		return len(a.arr) - len(b.arr)
	}
	return 0
}

// valueCmp is RSValue_Cmp with an error status.
func valueCmp(a, b xval, st *exprStatus) int {
	if a.t == b.t {
		return cmpSame(a, b, st)
	}
	switch {
	case a.isNull():
		return -1
	case b.isNull():
		return 1
	case a.t == xNum:
		y, ok := b.toNumber()
		if !ok {
			s, has := b.ptrString()
			if !has {
				s = "(null)"
			}
			st.setError("Error converting string '" + s + "' to number")
			return 0
		}
		return cmpNum(a.n, y)
	case b.t == xNum:
		x, ok := a.toNumber()
		if !ok {
			s, has := a.ptrString()
			if !has {
				s = "(null)"
			}
			st.setError("Error converting string '" + s + "' to number")
			return 0
		}
		return cmpNum(x, b.n)
	}
	return strings.Compare(a.convertString(), b.convertString())
}

// valueEqual is RSValue_Equal: a number against a string that is not one
// is unequal without an error.
func valueEqual(a, b xval, st *exprStatus) bool {
	if a.t == b.t {
		return cmpSame(a, b, st) == 0
	}
	switch {
	case a.isNull() || b.isNull():
		return false
	case a.t == xNum:
		y, ok := b.toNumber()
		return ok && a.n == y
	case b.t == xNum:
		x, ok := a.toNumber()
		return ok && x == b.n
	}
	return a.convertString() == b.convertString()
}

func (ev *exprEval) call(n *xnode, temp bool) (xval, int) {
	if n.name == "case" {
		cond, rc := ev.eval(n.args[0], true)
		if rc != evalOK {
			return xval{}, rc
		}
		if cond.boolTest() {
			return ev.eval(n.args[1], temp)
		}
		return ev.eval(n.args[2], temp)
	}
	args := make([]xval, len(n.args))
	for i, a := range n.args {
		v, rc := ev.eval(a, true)
		if rc == evalErr || rc == evalNull && n.name != "exists" {
			return xval{}, evalErr
		}
		args[i] = v
	}
	return n.fn.f(ev, args)
}

// apply evaluates the expression on a row.
func (e *Expr) eval(r *Row, st *exprStatus, root bool) (xval, int) {
	ev := &exprEval{row: r, st: st}
	return ev.eval(e.root, !root)
}

// Apply sets name on the row to the expression's value, and reports
// false when RediSearch's evaluator fails on the row.
func (e *Expr) Apply(r *Row, name string, s *EvalState) bool {
	v, rc := e.eval(r, &s.st, true)
	if rc != evalOK {
		return false
	}
	r.setValue(name, v)
	return true
}

// Keep reports whether a FILTER keeps the row, and false for ok when
// RediSearch's evaluator fails on it.
func (e *Expr) Keep(r *Row, s *EvalState) (keep, ok bool) {
	v, rc := e.eval(r, &s.st, false)
	if rc != evalOK {
		return false, false
	}
	return v.boolTest(), true
}

// setValue stores an evaluated value in the row with its type.
func (r *Row) setValue(name string, v xval) {
	switch v.t {
	case xNum:
		r.SetFloat(name, v.n)
	case xNull, xUndef:
		r.Set(name, "")
		r.markNull(name)
	case xArr:
		items := make([]cell, len(v.arr))
		for i, item := range v.arr {
			items[i] = valueCell(item)
		}
		r.setList(name, items)
	case xStr:
		r.Set(name, v.s)
		r.markLiteral(name)
	default:
		r.Set(name, v.s)
	}
}

func valueCell(v xval) cell {
	switch v.t {
	case xNum:
		return cell{v: numString(v.n), f: v.n, present: true, num: true}
	case xNull, xUndef:
		return cell{present: true, null: true}
	case xArr:
		items := make([]cell, len(v.arr))
		for i, item := range v.arr {
			items[i] = valueCell(item)
		}
		return cell{present: true, isArr: true, arr: items}
	}
	return cell{v: v.s, present: true, lit: v.t == xStr}
}

// ProfileExprType is how FT.PROFILE names an APPLY or FILTER step, by its
// expression's root.
func ProfileExprType(e *Expr) string {
	n := e.root
	switch n.kind {
	case nProp:
		return "Property " + n.name
	case nFunc:
		return "Function " + n.name
	case nNot:
		return "Inverted"
	case nOp:
		return "Operator " + string(n.op)
	case nPred:
		return "Predicate " + condNames[n.cond]
	}
	if n.null {
		return "Literal NULL"
	}
	if n.lit.t == xNum {
		return "Literal " + numString(n.lit.n)
	}
	return "Literal " + n.lit.s
}

// Functions.

type exprFunc struct {
	min, max int
	f        func(ev *exprEval, args []xval) (xval, int)
}

var exprFunctions map[string]*exprFunc

func init() {
	exprFunctions = map[string]*exprFunc{
		"lower":         {1, 1, func(_ *exprEval, a []xval) (xval, int) { return mapBytes(a[0], lowerByte) }},
		"upper":         {1, 1, func(_ *exprEval, a []xval) (xval, int) { return mapBytes(a[0], upperByte) }},
		"substr":        {3, 3, fnSubstr},
		"format":        {1, -1, fnFormat},
		"split":         {1, 3, fnSplit},
		"matched_terms": {0, 1, func(*exprEval, []xval) (xval, int) { return xval{t: xNull}, evalOK }},
		"to_number":     {1, 1, fnToNumber},
		"to_str":        {1, 1, func(_ *exprEval, a []xval) (xval, int) { return a[0].toString(), evalOK }},
		"exists":        {1, 1, fnExists},
		"case":          {3, 3, nil},
		"startswith":    {2, 2, fnStartsWith},
		"contains":      {2, 2, fnContains},
		"strlen":        {1, 1, fnStrlen},
		"log":           {1, 1, mathFunc(cLog)},
		"floor":         {1, 1, mathFunc(math.Floor)},
		"abs":           {1, 1, mathFunc(cFabs)},
		"ceil":          {1, 1, mathFunc(math.Ceil)},
		"sqrt":          {1, 1, mathFunc(cSqrt)},
		"log2":          {1, 1, mathFunc(cLog2)},
		"exp":           {1, 1, mathFunc(math.Exp)},
		"timefmt":       {1, 2, fnTimefmt},
		"parsetime":     {2, 2, fnParsetime},
		"hour":          {1, 1, dateFunc(func(t tmTime) float64 { t.sec, t.min = 0, 0; return t.fastTimegm() })},
		"minute":        {1, 1, fnMinute},
		"day":           {1, 1, dateFunc(func(t tmTime) float64 { t.sec, t.min, t.hour = 0, 0, 0; return t.fastTimegm() })},
		"month": {1, 1, dateFunc(func(t tmTime) float64 {
			t.sec, t.min, t.hour, t.yday = 0, 0, 0, t.yday-(t.mday-1)
			return t.fastTimegm()
		})},
		"monthofyear": {1, 1, dateFunc(func(t tmTime) float64 { return float64(t.mon) })},
		"year":        {1, 1, dateFunc(func(t tmTime) float64 { return float64(t.year + 1900) })},
		"dayofmonth":  {1, 1, dateFunc(func(t tmTime) float64 { return float64(t.mday) })},
		"dayofweek":   {1, 1, fnDayOfWeek},
		"dayofyear":   {1, 1, dateFunc(func(t tmTime) float64 { return float64(t.yday) })},
		"geodistance": {2, 4, fnGeodistance},
	}
}

func lowerByte(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

func upperByte(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}

func mapBytes(v xval, f func(byte) byte) (xval, int) {
	s, ok := v.ptrString()
	if !ok {
		return xval{t: xNull}, evalOK
	}
	b := []byte(s)
	for i := range b {
		b[i] = f(b[i])
	}
	return xstr(string(b)), evalOK
}

// validate is VALIDATE_ARG_TYPE and VALIDATE_ARG_ISSTRING.
func validate(ev *exprEval, fname string, a []xval, idx int, wantNumber bool) bool {
	v := a[idx]
	if wantNumber && v.t == xNum || !wantNumber && v.isString() {
		return true
	}
	check := "VALIDATE_ARG__STRING(v, 0)"
	if wantNumber {
		check = "VALIDATE_ARG__TYPE(v, RSValue_Number)"
	}
	ev.st.setError(fmt.Sprintf("Invalid type (%d) for argument %d in function '%s'. %s was false.", v.t, idx, fname, check))
	return false
}

func fnSubstr(ev *exprEval, a []xval) (xval, int) {
	if !validate(ev, "substr", a, 1, true) || !validate(ev, "substr", a, 2, true) {
		return xval{}, evalErr
	}
	str, ok := a[0].ptrString()
	if !ok {
		ev.st.setError("Invalid type for substr. Expected string")
		return xval{}, evalErr
	}
	sz := len(str)
	offset, length := cInt(a[1].n), cInt(a[2].n)
	if offset < 0 {
		offset += sz
	}
	if offset < 0 || offset > sz {
		offset = sz
	}
	if length < 0 {
		length = max(0, sz-offset+length)
	}
	if offset+length > sz {
		length = sz - offset
	}
	return xstr(str[offset : offset+length]), evalOK
}

// cInt is C's (int) cast of a double, saturating where C is undefined.
func cInt(f float64) int {
	switch {
	case f != f:
		return math.MinInt32
	case f >= math.MaxInt32:
		return math.MinInt32
	case f <= math.MinInt32:
		return math.MinInt32
	}
	return int(f)
}

func fnToNumber(ev *exprEval, a []xval) (xval, int) {
	f, ok := a[0].toNumber()
	if !ok {
		s, has := a[0].ptrString()
		if !has {
			s = "(null)"
		}
		ev.st.setError("to_number: cannot convert string '" + s + "'")
		return xval{}, evalErr
	}
	return xnum(f), evalOK
}

func fnFormat(ev *exprEval, a []xval) (xval, int) {
	if !validate(ev, "format", a, 0, false) {
		return xval{}, evalErr
	}
	fmtStr := a[0].s
	var out strings.Builder
	last := 0
	argix := 1
	for i := 0; i < len(fmtStr); i++ {
		if fmtStr[i] != '%' {
			continue
		}
		if i == len(fmtStr)-1 {
			ev.st.setError("Bad format string!")
			return xval{t: xNull}, evalErr
		}
		out.WriteString(fmtStr[last:i])
		last = i + 2
		i++
		if fmtStr[i] == '%' {
			out.WriteByte('%')
			continue
		}
		if argix == len(a) {
			ev.st.setError("Not enough arguments for format")
			return xval{t: xNull}, evalErr
		}
		arg := a[argix]
		argix++
		if fmtStr[i] != 's' {
			ev.st.setError("Unknown format specifier passed")
			return xval{t: xNull}, evalErr
		}
		if arg.isNull() {
			out.WriteString("(null)")
		} else {
			out.WriteString(arg.toString().s)
		}
	}
	if last < len(fmtStr) {
		out.WriteString(fmtStr[last:])
	}
	return xstr(out.String()), evalOK
}

func fnSplit(ev *exprEval, a []xval) (xval, int) {
	if !validate(ev, "split", a, 0, false) {
		return xval{}, evalErr
	}
	sep, strip := ",", " "
	if len(a) >= 2 {
		if !validate(ev, "split", a, 1, false) {
			return xval{}, evalErr
		}
		sep = cString(a[1].s)
	}
	if len(a) == 3 {
		if !validate(ev, "split", a, 2, false) {
			return xval{}, evalErr
		}
		strip = cString(a[2].s)
	}
	str := a[0].s
	var items []xval
	tok := 0
	for len(items) < 1024 && tok < len(str) {
		next := strpbrk(str, tok, sep)
		end := len(str)
		if next >= 0 {
			end = next
		}
		if end > tok {
			sp, ep := tok, end-1
			for sp <= ep && inCSet(strip, str[sp]) {
				sp++
			}
			for ep > sp && inCSet(strip, str[ep]) {
				ep--
			}
			if sp <= ep {
				items = append(items, xstr(str[sp:ep+1]))
			}
		}
		if next < 0 {
			break
		}
		tok = next + 1
	}
	return xval{t: xArr, arr: items}, evalOK
}

// cString is a value as C reads it, up to its first NUL.
func cString(s string) string {
	if i := strings.IndexByte(s, 0); i >= 0 {
		return s[:i]
	}
	return s
}

// strpbrk finds the first byte of set at or after i, stopping at a NUL.
func strpbrk(s string, i int, set string) int {
	for j := i; j < len(s); j++ {
		if s[j] == 0 {
			return -1
		}
		if strings.IndexByte(set, s[j]) >= 0 {
			return j
		}
	}
	return -1
}

// inCSet is strchr(set, c) != NULL, which a NUL always is.
func inCSet(set string, c byte) bool { return c == 0 || strings.IndexByte(set, c) >= 0 }

func fnExists(ev *exprEval, a []xval) (xval, int) {
	if a[0].missing {
		ev.st.set, ev.st.msg = false, ""
		return xnum(0), evalOK
	}
	return xnum(1), evalOK
}

func fnStartsWith(ev *exprEval, a []xval) (xval, int) {
	if !validate(ev, "startswith", a, 0, false) || !validate(ev, "startswith", a, 1, false) {
		return xval{}, evalErr
	}
	return xnum(b2f(strings.HasPrefix(cString(a[0].s), cString(a[1].s)))), evalOK
}

func fnContains(ev *exprEval, a []xval) (xval, int) {
	if !validate(ev, "contains", a, 0, false) || !validate(ev, "contains", a, 1, false) {
		return xval{}, evalErr
	}
	str, sub := cString(a[0].s), cString(a[1].s)
	if len(a[1].s) == 0 {
		return xnum(float64(len(a[0].s) + 1)), evalOK
	}
	if sub == "" {
		return xnum(float64(len(str) + 1)), evalOK
	}
	n := 0
	for i := 0; i+len(sub) <= len(str); i++ {
		if str[i:i+len(sub)] == sub {
			n++
		}
	}
	return xnum(float64(n)), evalOK
}

func fnStrlen(ev *exprEval, a []xval) (xval, int) {
	if !validate(ev, "strlen", a, 0, false) {
		return xval{}, evalErr
	}
	return xnum(float64(len(a[0].s))), evalOK
}

func mathFunc(f func(float64) float64) func(*exprEval, []xval) (xval, int) {
	return func(_ *exprEval, a []xval) (xval, int) {
		d, ok := a[0].toNumber()
		if !ok {
			return xnum(math.NaN()), evalOK
		}
		return xnum(f(d)), evalOK
	}
}

func cLog(x float64) float64 {
	switch {
	case x != x:
		return x
	case x < 0:
		return cNaN64
	}
	return cmath.Log(x)
}

func cLog2(x float64) float64 {
	switch {
	case x != x:
		return x
	case x < 0:
		return cNaN64
	}
	return math.Log2(x)
}

func cSqrt(x float64) float64 {
	if x < 0 {
		return cNaN64
	}
	return math.Sqrt(x)
}

func cFabs(x float64) float64 { return math.Float64frombits(math.Float64bits(x) &^ (1 << 63)) }

func fnGeodistance(ev *exprEval, a []xval) (xval, int) {
	var p [2]store.GeoPoint
	parseField := func(v xval, out *store.GeoPoint) bool {
		switch {
		case v.isString():
			lon, lat, ok := parseGeoString(v.s, ev.st)
			*out = store.GeoPoint{Lon: lon, Lat: lat}
			return ok
		case v.t == xNum:
			pt, ok := decodeGeoScore(v.n)
			*out = pt
			return ok
		}
		return false
	}
	lonLat := func(x, y xval, out *store.GeoPoint) bool {
		lon, okA := x.toNumber()
		lat, okB := y.toNumber()
		*out = store.GeoPoint{Lon: lon, Lat: lat}
		return okA && okB
	}
	ok := true
	switch len(a) {
	case 2:
		ok = parseField(a[0], &p[0]) && parseField(a[1], &p[1])
	case 4:
		ok = lonLat(a[0], a[1], &p[0]) && lonLat(a[2], a[3], &p[1])
	case 3:
		if _, isNum := a[0].toNumber(); isNum {
			ok = lonLat(a[0], a[1], &p[0]) && parseField(a[2], &p[1])
		} else {
			ok = parseField(a[0], &p[0]) && lonLat(a[1], a[2], &p[1])
		}
	}
	if !ok {
		return xnum(math.NaN()), evalOK
	}
	d := store.GeoDistanceMetersExact(p[0], p[1])
	return xnum(math.Round(d*100) / 100), evalOK
}

// parseGeoString is RediSearch's parseGeo: "lon,lat" or "lon lat".
func parseGeoString(s string, st *exprStatus) (float64, float64, bool) {
	if len(s) > 128 {
		st.setError("Geo string cannot be longer than 128 bytes")
		return 0, 0, false
	}
	s = cString(s)
	i := strings.IndexAny(s, " ,")
	if i < 0 {
		st.setError("Invalid geo string")
		return 0, 0, false
	}
	lon, okA := parseGeoNumber(s[:i])
	lat, okB := parseGeoNumber(s[i+1:])
	if !okA || !okB {
		st.setError("Invalid geo string")
		return 0, 0, false
	}
	return lon, lat, true
}

func parseGeoNumber(s string) (float64, bool) {
	if s == "" {
		return 0, true
	}
	return parseCNumber(s)
}

// decodeGeoScore is RediSearch's decodeGeo of a 52-bit geohash.
func decodeGeoScore(f float64) (store.GeoPoint, bool) {
	if f != f {
		f = math.Inf(-1)
	}
	return store.GeoPointFromScore(f), true
}
