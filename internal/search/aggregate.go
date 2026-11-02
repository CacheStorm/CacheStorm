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
	// lists are the values a TOLIST or RANDOM_SAMPLE reducer made: their
	// items joined by NULs, replied as arrays.
	lists map[string]bool
	nulls map[string]bool
	// arrays hold the items of the lists.
	arrays map[string][]cell
	// numv holds, at full precision, the values that are numbers to
	// RediSearch, which hashes and compares them as such.
	numv map[string]float64
	// docID orders a document's row as RediSearch's document id does.
	docID int64
}

// SetDocID gives the row its document's place in the index's order, from 1.
func (r *Row) SetDocID(id int64) { r.docID = id }

// SetNumber sets a row value RediSearch holds as a number, from its text.
func (r *Row) SetNumber(name, value string) {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		r.Set(name, value)
		return
	}
	r.SetFloat(name, f)
}

// SetFloat sets a number, shown as RediSearch prints numbers.
func (r *Row) SetFloat(name string, f float64) {
	r.Set(name, FormatNumber(f))
	if r.numv == nil {
		r.numv = make(map[string]float64)
	}
	r.numv[name] = f
}

// Number returns a value the row holds, or a document field it falls back
// to, when it is a number.
func (r *Row) Number(name string) (float64, bool) {
	if _, ok := r.Value(name); ok {
		f, ok := r.numv[name]
		return f, ok
	}
	if r.doc != nil {
		if f := r.idx.Field(name); f != nil && f.Type == FieldNumeric {
			if nums := r.doc.field(f.pos).nums; len(nums) > 0 {
				return nums[0], true
			}
		}
	}
	return 0, false
}

// IsList reports whether a row value is a list.
func (r *Row) IsList(name string) bool { return r.lists[name] }

// ListItem is an item of a list value; Null items reply as null.
type ListItem struct {
	Value string
	Null  bool
}

// ListItems returns a list value's items.
func (r *Row) ListItems(name string) []ListItem {
	out := make([]ListItem, 0, len(r.arrays[name]))
	for _, c := range r.arrays[name] {
		out = append(out, ListItem{Value: c.v, Null: c.null})
	}
	return out
}

func (r *Row) setList(name string, items []cell) {
	vals := make([]string, len(items))
	for i, c := range items {
		vals[i] = c.v
	}
	r.Set(name, strings.Join(vals, "\x00"))
	if r.lists == nil {
		r.lists = make(map[string]bool)
	}
	if r.arrays == nil {
		r.arrays = make(map[string][]cell)
	}
	r.lists[name] = true
	r.arrays[name] = items
}

// IsNull reports whether a row value is null: a GROUPBY key the rows lack.
func (r *Row) IsNull(name string) bool { return r.nulls[name] }

func (r *Row) markNull(name string) {
	if r.nulls == nil {
		r.nulls = make(map[string]bool)
	}
	r.nulls[name] = true
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
		return FieldValue(r.idx, r.doc, name)
	}
	return "", false
}

// Set adds or replaces a row value.
func (r *Row) Set(name, value string) {
	delete(r.numv, name)
	delete(r.lists, name)
	delete(r.arrays, name)
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
		r.LoadAs(n, n)
	}
}

// LoadAs copies a field into the row under another name; __key is the
// document's key.
func (r *Row) LoadAs(name, as string) {
	if name == "__key" {
		r.Set(as, r.doc.Key)
		return
	}
	if v, ok := FieldValue(r.idx, r.doc, name); ok {
		if f := r.idx.Field(name); f != nil && f.Type == FieldNumeric {
			if nums := r.doc.field(f.pos).nums; len(nums) > 0 {
				r.SetFloat(as, nums[0])
				return
			}
		}
		r.Set(as, v)
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
		// A NUMERIC field reads as the number it holds.
		if nums := doc.field(f.pos).nums; f.Type == FieldNumeric && len(nums) > 0 {
			return FormatNumber(nums[0]), true
		}
		if raw := doc.field(f.pos).raw; len(raw) > 0 {
			return raw[0], true
		}
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

// Name is the field the reducer's value goes to.
func (red Reducer) Name() string {
	if red.As != "" {
		return red.As
	}
	return "__generated_alias" + strings.ToLower(red.Fn) + strings.ToLower(strings.Join(trimAt(red.Args), ""))
}

// Group groups rows by the props and runs the reducers on each group.
func Group(rows []*Row, props []string, reducers []Reducer) ([]*Row, error) {
	type group struct {
		key  []string
		null []bool
		num  []bool
		f    []float64
		rows []*Row
	}
	// Groups come in the order RediSearch's hash table holds their hashes;
	// each item of a list value makes its own group, an empty list a null.
	var table khashOrder
	groups := make(map[uint64]*group)
	cells := make([]cell, len(props))
	var extract func(r *Row, i int, h uint64)
	extract = func(r *Row, i int, h uint64) {
		if i == len(props) {
			g, ok := groups[h]
			if !ok {
				g = &group{key: make([]string, len(props)), null: make([]bool, len(props)), num: make([]bool, len(props)), f: make([]float64, len(props))}
				for j, c := range cells {
					g.key[j], g.null[j], g.num[j], g.f[j] = c.v, c.null || !c.present, c.num, c.f
				}
				groups[h] = g
				table.put(h)
			}
			g.rows = append(g.rows, r)
			return
		}
		c := r.cell(props[i])
		if !c.present {
			c = cell{present: true, null: true}
		}
		switch {
		case !c.isArr:
			cells[i] = c
			extract(r, i+1, c.hash(h))
		case len(c.arr) == 0:
			cells[i] = cell{present: true, null: true}
			extract(r, i+1, h+1)
		default:
			for _, item := range c.arr {
				cells[i] = item
				extract(r, i+1, item.hash(h))
			}
		}
	}
	for _, r := range rows {
		extract(r, 0, 0)
	}
	if len(rows) == 0 && len(props) == 0 {
		groups[0] = &group{}
		table.put(0)
	}
	order := table.order()
	out := make([]*Row, 0, len(order))
	for _, k := range order {
		g := groups[k]
		row := &Row{}
		for i, p := range props {
			name := strings.TrimPrefix(p, "@")
			if g.num[i] {
				row.SetFloat(name, g.f[i])
			} else {
				row.Set(name, g.key[i])
			}
			if g.null[i] {
				row.markNull(name)
			}
		}
		for _, red := range reducers {
			v, err := reduce(red, g.rows)
			if err != nil {
				return nil, err
			}
			name := red.Name()
			switch {
			case v.isArr:
				row.setList(name, v.arr)
			case v.num:
				row.SetFloat(name, v.f)
			case v.null:
				row.Set(name, "")
				row.markNull(name)
			default:
				row.Set(name, v.v)
			}
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
	if f == math.Trunc(f) && math.Abs(f) < 1<<63 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', 12, 64)
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

// EvalTyped evaluates the expression and returns the number RediSearch
// holds when the result is one.
func (e *Expr) EvalTyped(r *Row) (string, float64, bool) {
	if f, ok := e.root.(fieldNode); ok {
		v, _ := r.Get(f.name)
		n, isNum := r.Number(f.name)
		return v, n, isNum
	}
	v := e.root.eval(r)
	return v.String(), v.num, v.isNum && !v.null
}

// Missing returns a property the expression reads that r lacks, which
// RediSearch refuses to evaluate; exists() may read anything.
func (e *Expr) Missing(r *Row) (string, bool) { return missingField(e.root, r) }

func missingField(n exprNode, r *Row) (string, bool) {
	switch n := n.(type) {
	case fieldNode:
		if _, ok := r.Get(n.name); !ok {
			return n.name, true
		}
	case unaryOp:
		return missingField(n.x, r)
	case binaryOp:
		if name, ok := missingField(n.l, r); ok {
			return name, true
		}
		return missingField(n.r, r)
	case callNode:
		if n.fn == "exists" {
			return "", false
		}
		for _, a := range n.args {
			if name, ok := missingField(a, r); ok {
				return name, true
			}
		}
	}
	return "", false
}

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

// Properties returns the properties the expression reads, in the order
// RediSearch resolves them.
func (e *Expr) Properties() []string {
	var out []string
	var walk func(n exprNode)
	walk = func(n exprNode) {
		switch n := n.(type) {
		case fieldNode:
			out = append(out, n.name)
		case unaryOp:
			walk(n.x)
		case binaryOp:
			walk(n.l)
			walk(n.r)
		case callNode:
			for _, a := range n.args {
				walk(a)
			}
		}
	}
	walk(e.root)
	return out
}

// Yields returns the fields a query's vector clauses return distances as.
func Yields(n Node) []string {
	if n == nil {
		return nil
	}
	var out []string
	if n.yield != "" {
		out = append(out, n.yield)
	}
	for _, c := range n.children {
		out = append(out, Yields(c)...)
	}
	return out
}

// Value returns a value the row holds, without reading the document.
func (r *Row) Value(name string) (string, bool) {
	for i, n := range r.Names {
		if n == name {
			return r.Values[i], true
		}
	}
	return "", false
}
