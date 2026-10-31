package search

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Row is one FT.AGGREGATE row: named values in order, and the document it
// came from, whose fields steps may read without LOAD.
type Row struct {
	Names  []string
	Values []string
	doc    *Document
	idx    *Index
}

// Get returns a row value by name (without the @), falling back to the
// document's fields.
func (r *Row) Get(name string) (string, bool) {
	for i, n := range r.Names {
		if n == name {
			return r.Values[i], true
		}
	}
	if r.doc != nil {
		if v, ok := r.doc.Value(name); ok {
			return v, true
		}
		if v, ok := r.doc.HashField(name); ok {
			return v, true
		}
	}
	return "", false
}

// Set adds or replaces a row value.
func (r *Row) Set(name, value string) {
	for i, n := range r.Names {
		if n == name {
			r.Values[i] = value
			return
		}
	}
	r.Names = append(r.Names, name)
	r.Values = append(r.Values, value)
}

// NewRow starts a row for doc.
func (idx *Index) NewRow(doc *Document) *Row { return &Row{doc: doc, idx: idx} }

// Load copies the named fields (attributes, hash fields or JSON paths)
// into the row; "*" loads every hash field or the JSON document.
func (r *Row) Load(names []string) {
	for _, n := range names {
		n = strings.TrimPrefix(n, "@")
		if n == "*" {
			if r.doc.JSON != nil {
				r.Set("$", jsonText(r.doc))
				continue
			}
			for _, kv := range r.doc.Fields {
				r.Set(kv[0], kv[1])
			}
			continue
		}
		if v, ok := FieldValue(r.idx, r.doc, n); ok {
			r.Set(n, v)
		}
	}
}

// FieldValue returns what RETURN or LOAD shows for name: an attribute's
// value, a hash field, or a JSON path's first match.
func FieldValue(idx *Index, doc *Document, name string) (string, bool) {
	if doc.JSON != nil {
		if f := idx.Field(name); f != nil {
			if ms := f.path.Eval(doc.JSON); len(ms) > 0 {
				return jsonOut(ms[0].Node), true
			}
			return "", false
		}
		if strings.HasPrefix(name, "$") {
			p, err := parseJSONPathCached(name)
			if err == nil {
				if ms := p.Eval(doc.JSON); len(ms) > 0 {
					return jsonOut(ms[0].Node), true
				}
			}
		}
		return "", false
	}
	if f := idx.Field(name); f != nil {
		return doc.HashField(f.Identifier)
	}
	return doc.HashField(name)
}

// Reducer is one GROUPBY reducer.
type Reducer struct {
	Fn   string
	Args []string
	As   string
}

// Group groups rows by the props and runs the reducers on each group.
func Group(rows []*Row, props []string, reducers []Reducer) ([]*Row, error) {
	type group struct {
		key  []string
		rows []*Row
	}
	var order []string
	groups := make(map[string]*group)
	for _, r := range rows {
		key := make([]string, len(props))
		for i, p := range props {
			key[i], _ = r.Get(strings.TrimPrefix(p, "@"))
		}
		k := strings.Join(key, "\x00")
		g, ok := groups[k]
		if !ok {
			g = &group{key: key}
			groups[k] = g
			order = append(order, k)
		}
		g.rows = append(g.rows, r)
	}
	if len(rows) == 0 && len(props) == 0 {
		groups[""] = &group{}
		order = append(order, "")
	}
	out := make([]*Row, 0, len(order))
	for _, k := range order {
		g := groups[k]
		row := &Row{}
		for i, p := range props {
			row.Set(strings.TrimPrefix(p, "@"), g.key[i])
		}
		for _, red := range reducers {
			v, err := reduce(red, g.rows)
			if err != nil {
				return nil, err
			}
			name := red.As
			if name == "" {
				name = "__generated_alias" + strings.ToLower(red.Fn) + strings.ToLower(strings.Join(trimAt(red.Args), ""))
			}
			row.Set(name, v)
		}
		out = append(out, row)
	}
	return out, nil
}

func trimAt(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = strings.TrimPrefix(a, "@")
	}
	return out
}

func numbers(rows []*Row, field string) []float64 {
	var out []float64
	for _, r := range rows {
		if v, ok := r.Get(strings.TrimPrefix(field, "@")); ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				out = append(out, f)
			}
		}
	}
	return out
}

func reduce(red Reducer, rows []*Row) (string, error) {
	arg := func(i int) string {
		if i < len(red.Args) {
			return red.Args[i]
		}
		return ""
	}
	switch strings.ToUpper(red.Fn) {
	case "COUNT":
		return FormatNumber(float64(len(rows))), nil
	case "COUNT_DISTINCT", "COUNT_DISTINCTISH":
		seen := make(map[string]bool)
		for _, r := range rows {
			if v, ok := r.Get(strings.TrimPrefix(arg(0), "@")); ok {
				seen[v] = true
			}
		}
		return FormatNumber(float64(len(seen))), nil
	case "SUM":
		sum := 0.0
		for _, v := range numbers(rows, arg(0)) {
			sum += v
		}
		return FormatNumber(sum), nil
	case "AVG":
		vs := numbers(rows, arg(0))
		if len(vs) == 0 {
			return "nan", nil
		}
		sum := 0.0
		for _, v := range vs {
			sum += v
		}
		return FormatNumber(sum / float64(len(vs))), nil
	case "MIN", "MAX":
		vs := numbers(rows, arg(0))
		if len(vs) == 0 {
			return "inf", nil
		}
		best := vs[0]
		for _, v := range vs {
			if strings.EqualFold(red.Fn, "MIN") {
				best = math.Min(best, v)
			} else {
				best = math.Max(best, v)
			}
		}
		return FormatNumber(best), nil
	case "STDDEV":
		vs := numbers(rows, arg(0))
		if len(vs) < 2 {
			return "0", nil
		}
		mean := 0.0
		for _, v := range vs {
			mean += v
		}
		mean /= float64(len(vs))
		sq := 0.0
		for _, v := range vs {
			sq += (v - mean) * (v - mean)
		}
		return FormatNumber(math.Sqrt(sq / float64(len(vs)-1))), nil
	case "QUANTILE":
		vs := numbers(rows, arg(0))
		q, err := strconv.ParseFloat(arg(1), 64)
		if err != nil || len(vs) == 0 {
			return "0", nil
		}
		sort.Float64s(vs)
		return FormatNumber(vs[int(math.Min(float64(len(vs)-1), math.Floor(q*float64(len(vs)))))]), nil
	case "FIRST_VALUE":
		if len(rows) == 0 {
			return "", nil
		}
		v, _ := rows[0].Get(strings.TrimPrefix(arg(0), "@"))
		return v, nil
	case "TOLIST", "RANDOM_SAMPLE":
		seen := make(map[string]bool)
		var vals []string
		for _, r := range rows {
			if v, ok := r.Get(strings.TrimPrefix(arg(0), "@")); ok && !seen[v] {
				seen[v] = true
				vals = append(vals, v)
			}
		}
		return strings.Join(vals, "\x00"), nil
	}
	return "", errors.New("Bad arguments for REDUCE: unknown reducer " + red.Fn) //nolint:staticcheck // RediSearch's reply text
}

// SortKey is one SORTBY property.
type SortKey struct {
	Field string
	Desc  bool
}

// SortRows orders rows by the keys: numbers numerically, otherwise as
// strings, missing values last.
func SortRows(rows []*Row, keys []SortKey) {
	sort.SliceStable(rows, func(i, j int) bool {
		for _, k := range keys {
			a, okA := rows[i].Get(k.Field)
			b, okB := rows[j].Get(k.Field)
			if okA != okB {
				return okA
			}
			c := CompareValues(a, b)
			if c != 0 {
				if k.Desc {
					return c > 0
				}
				return c < 0
			}
		}
		return false
	})
}

// CompareValues compares numerically when both parse as numbers.
func CompareValues(a, b string) int {
	x, errA := strconv.ParseFloat(a, 64)
	y, errB := strconv.ParseFloat(b, 64)
	if errA == nil && errB == nil {
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	}
	return strings.Compare(a, b)
}

// FormatNumber prints a number as RediSearch replies with it.
func FormatNumber(f float64) string {
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	if math.IsNaN(f) {
		return "nan"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e17 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// Expr is a parsed APPLY or FILTER expression.
type Expr struct {
	root exprNode
}

type exprNode interface{ eval(r *Row) value }

type value struct {
	num   float64
	str   string
	isNum bool
	null  bool
}

func numValue(f float64) value { return value{num: f, isNum: true} }

func (v value) number() (float64, bool) {
	if v.isNum {
		return v.num, true
	}
	f, err := strconv.ParseFloat(v.str, 64)
	return f, err == nil
}

func (v value) String() string {
	if v.null {
		return ""
	}
	if v.isNum {
		return FormatNumber(v.num)
	}
	return v.str
}

func (v value) truthy() bool {
	if v.null {
		return false
	}
	if f, ok := v.number(); ok {
		return f != 0
	}
	return v.str != ""
}

type (
	litNode   struct{ v value }
	fieldNode struct{ name string }
	unaryOp   struct {
		op string
		x  exprNode
	}
	binaryOp struct {
		op   string
		l, r exprNode
	}
	callNode struct {
		fn   string
		args []exprNode
	}
)

func (n litNode) eval(*Row) value { return n.v }
func (n fieldNode) eval(r *Row) value {
	if v, ok := r.Get(n.name); ok {
		return value{str: v}
	}
	return value{null: true}
}
func (n unaryOp) eval(r *Row) value {
	x := n.x.eval(r)
	if n.op == "!" {
		return numValue(b2f(!x.truthy()))
	}
	f, _ := x.number()
	return numValue(-f)
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (n binaryOp) eval(r *Row) value {
	l := n.l.eval(r)
	switch n.op {
	case "&&":
		return numValue(b2f(l.truthy() && n.r.eval(r).truthy()))
	case "||":
		return numValue(b2f(l.truthy() || n.r.eval(r).truthy()))
	}
	rv := n.r.eval(r)
	switch n.op {
	case "==", "!=", "<", "<=", ">", ">=":
		var c int
		x, okX := l.number()
		y, okY := rv.number()
		if okX && okY {
			c = CompareValues(FormatNumber(x), FormatNumber(y))
			if x < y {
				c = -1
			} else if x > y {
				c = 1
			}
		} else {
			c = strings.Compare(l.String(), rv.String())
		}
		res := map[string]bool{"==": c == 0, "!=": c != 0, "<": c < 0, "<=": c <= 0, ">": c > 0, ">=": c >= 0}[n.op]
		return numValue(b2f(res))
	}
	x, okX := l.number()
	y, okY := rv.number()
	if !okX || !okY {
		return value{null: true}
	}
	switch n.op {
	case "+":
		return numValue(x + y)
	case "-":
		return numValue(x - y)
	case "*":
		return numValue(x * y)
	case "/":
		return numValue(x / y)
	case "%":
		return numValue(float64(int64(x) % int64(math.Max(1, y))))
	case "^":
		return numValue(math.Pow(x, y))
	}
	return value{null: true}
}

func (n callNode) eval(r *Row) value {
	args := make([]value, len(n.args))
	for i, a := range n.args {
		args[i] = a.eval(r)
	}
	num := func(i int) float64 {
		if i < len(args) {
			f, _ := args[i].number()
			return f
		}
		return 0
	}
	str := func(i int) string {
		if i < len(args) {
			return args[i].String()
		}
		return ""
	}
	switch n.fn {
	case "upper":
		return value{str: strings.ToUpper(str(0))}
	case "lower":
		return value{str: strings.ToLower(str(0))}
	case "abs":
		return numValue(math.Abs(num(0)))
	case "floor":
		return numValue(math.Floor(num(0)))
	case "ceil":
		return numValue(math.Ceil(num(0)))
	case "sqrt":
		return numValue(math.Sqrt(num(0)))
	case "log":
		return numValue(math.Log(num(0)))
	case "log2":
		return numValue(math.Log2(num(0)))
	case "exp":
		return numValue(math.Exp(num(0)))
	case "strlen":
		return numValue(float64(len(str(0))))
	case "exists":
		return numValue(b2f(len(args) > 0 && !args[0].null))
	case "contains":
		return numValue(float64(strings.Count(str(0), str(1))))
	case "startswith":
		return numValue(b2f(strings.HasPrefix(str(0), str(1))))
	case "substr":
		s := str(0)
		off, n := int(num(1)), int(num(2))
		if off < 0 {
			off += len(s)
		}
		off = min(max(off, 0), len(s))
		end := len(s)
		if len(args) > 2 && n >= 0 {
			end = min(off+n, len(s))
		}
		return value{str: s[off:end]}
	}
	return value{null: true}
}

// Eval returns the expression's value for r as a string.
func (e *Expr) Eval(r *Row) string { return e.root.eval(r).String() }

// Truthy reports whether a FILTER expression holds for r.
func (e *Expr) Truthy(r *Row) bool { return e.root.eval(r).truthy() }

type exprParser struct {
	s   string
	pos int
}

// ParseExpr parses an APPLY or FILTER expression.
func ParseExpr(s string) (*Expr, error) {
	p := &exprParser{s: s}
	n, err := p.binary(0)
	if err != nil {
		return nil, err
	}
	p.space()
	if p.pos < len(p.s) {
		return nil, errors.New("Syntax error at offset " + strconv.Itoa(p.pos) + " near " + p.s[p.pos:]) //nolint:staticcheck // RediSearch's reply text
	}
	return &Expr{root: n}, nil
}

func (p *exprParser) space() {
	for p.pos < len(p.s) && p.s[p.pos] == ' ' {
		p.pos++
	}
}

var exprLevels = [][]string{{"||"}, {"&&"}, {"==", "!=", "<=", ">=", "<", ">"}, {"+", "-"}, {"*", "/", "%"}, {"^"}}

func (p *exprParser) binary(level int) (exprNode, error) {
	if level == len(exprLevels) {
		return p.unary()
	}
	left, err := p.binary(level + 1)
	if err != nil {
		return nil, err
	}
	for {
		p.space()
		matched := ""
		for _, op := range exprLevels[level] {
			if strings.HasPrefix(p.s[p.pos:], op) {
				matched = op
				break
			}
		}
		if matched == "" {
			return left, nil
		}
		p.pos += len(matched)
		right, err := p.binary(level + 1)
		if err != nil {
			return nil, err
		}
		left = binaryOp{op: matched, l: left, r: right}
	}
}

func (p *exprParser) unary() (exprNode, error) {
	p.space()
	if p.pos < len(p.s) && (p.s[p.pos] == '!' || p.s[p.pos] == '-') {
		op := string(p.s[p.pos])
		p.pos++
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		return unaryOp{op: op, x: x}, nil
	}
	return p.primary()
}

func (p *exprParser) primary() (exprNode, error) {
	p.space()
	if p.pos >= len(p.s) {
		return nil, errors.New("Syntax error: unexpected end of expression") //nolint:staticcheck // RediSearch's reply text
	}
	c := p.s[p.pos]
	switch {
	case c == '(':
		p.pos++
		n, err := p.binary(0)
		if err != nil {
			return nil, err
		}
		p.space()
		if p.pos >= len(p.s) || p.s[p.pos] != ')' {
			return nil, errors.New("Syntax error: missing )") //nolint:staticcheck // RediSearch's reply text
		}
		p.pos++
		return n, nil
	case c == '@':
		p.pos++
		start := p.pos
		for p.pos < len(p.s) && (isIdentByte(p.s[p.pos]) || p.s[p.pos] == '.' || p.s[p.pos] == '$') {
			p.pos++
		}
		return fieldNode{name: p.s[start:p.pos]}, nil
	case c == '\'' || c == '"':
		end := strings.IndexByte(p.s[p.pos+1:], c)
		if end < 0 {
			return nil, errors.New("Syntax error: unterminated string") //nolint:staticcheck // RediSearch's reply text
		}
		v := p.s[p.pos+1 : p.pos+1+end]
		p.pos += end + 2
		return litNode{value{str: v}}, nil
	case c >= '0' && c <= '9' || c == '.':
		start := p.pos
		for p.pos < len(p.s) && strings.IndexByte("0123456789.eE", p.s[p.pos]) >= 0 {
			p.pos++
		}
		f, err := strconv.ParseFloat(p.s[start:p.pos], 64)
		if err != nil {
			return nil, errors.New("Syntax error: bad number") //nolint:staticcheck // RediSearch's reply text
		}
		return litNode{numValue(f)}, nil
	case isIdentByte(c):
		start := p.pos
		for p.pos < len(p.s) && isIdentByte(p.s[p.pos]) {
			p.pos++
		}
		name := strings.ToLower(p.s[start:p.pos])
		p.space()
		if p.pos >= len(p.s) || p.s[p.pos] != '(' {
			return nil, errors.New("Syntax error: unknown symbol " + name) //nolint:staticcheck // RediSearch's reply text
		}
		p.pos++
		var args []exprNode
		for {
			p.space()
			if p.pos < len(p.s) && p.s[p.pos] == ')' {
				p.pos++
				break
			}
			a, err := p.binary(0)
			if err != nil {
				return nil, err
			}
			args = append(args, a)
			p.space()
			if p.pos < len(p.s) && p.s[p.pos] == ',' {
				p.pos++
			}
		}
		return callNode{fn: name, args: args}, nil
	}
	return nil, errors.New("Syntax error at offset " + strconv.Itoa(p.pos)) //nolint:staticcheck // RediSearch's reply text
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
