package store

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// JSONPath is a parsed RedisJSON path: JSONPath when it starts with $, the
// legacy syntax (".a.b[0]", "a.b", ".") otherwise. A legacy path answers
// with its first match only.
type JSONPath struct {
	Text   string
	Legacy bool
	steps  []jpStep
}

type jpStepKind uint8

const (
	jpKeys jpStepKind = iota
	jpWildcard
	jpIndexes
	jpSlice
	jpFilter
)

type jpStep struct {
	kind    jpStepKind
	deep    bool
	keys    []string
	indexes []int
	slice   [3]*int
	filter  jpExpr
}

// JSONPathError is a path syntax error.
type JSONPathError struct {
	Pos int
	Msg string
}

func (e *JSONPathError) Error() string {
	return fmt.Sprintf("Error occurred at position %d, Error at position %d: %s", e.Pos, e.Pos, e.Msg)
}

const jpBracketExpect = "expected one of the following: '<string>[,<string>,...]', '<number>', ['start:end:steps'], '*'"

type jpParser struct {
	s   string
	pos int
}

func (p *jpParser) fail(msg string) error { return &JSONPathError{Pos: p.pos, Msg: msg} }

// ParseJSONPath parses a RedisJSON path.
func ParseJSONPath(s string) (*JSONPath, error) {
	path := &JSONPath{Text: s}
	p := &jpParser{s: s}
	if strings.HasPrefix(s, "$") {
		p.pos = 1
	} else {
		path.Legacy = true
		if s == "." {
			return path, nil
		}
		// A legacy path may start with a bare key.
		if s != "" && s[0] != '.' && s[0] != '[' {
			key := p.ident()
			if key == "" {
				return nil, p.fail("expected a key")
			}
			path.steps = append(path.steps, jpStep{kind: jpKeys, keys: []string{key}})
		}
	}
	for p.pos < len(p.s) {
		step, err := p.step()
		if err != nil {
			return nil, err
		}
		path.steps = append(path.steps, step)
	}
	return path, nil
}

// Static reports whether the path names at most one value: only plain keys
// and single indexes, no descent, wildcard, slice or filter.
func (path *JSONPath) Static() bool {
	for _, st := range path.steps {
		if st.deep || st.kind == jpKeys && len(st.keys) != 1 || st.kind == jpIndexes && len(st.indexes) != 1 ||
			st.kind != jpKeys && st.kind != jpIndexes {
			return false
		}
	}
	return true
}

// RootJSONPath is the legacy path ".", the default of commands whose path
// is optional.
func RootJSONPath() *JSONPath { return &JSONPath{Text: ".", Legacy: true} }

// IsRoot reports whether the path names the document itself.
func (path *JSONPath) IsRoot() bool { return len(path.steps) == 0 }

func (p *jpParser) ident() string {
	start := p.pos
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if c == '.' || c == '[' || c == ']' || c == ' ' || c == '(' || c == ')' || c == '=' || c == '<' || c == '>' || c == '!' || c == '&' || c == '|' || c == ',' {
			break
		}
		p.pos++
	}
	return p.s[start:p.pos]
}

func (p *jpParser) step() (jpStep, error) {
	var step jpStep
	switch {
	case strings.HasPrefix(p.s[p.pos:], ".."):
		step.deep = true
		p.pos += 2
		if p.pos < len(p.s) && p.s[p.pos] == '[' {
			return p.bracket(step)
		}
	case p.s[p.pos] == '.':
		p.pos++
	case p.s[p.pos] == '[':
		return p.bracket(step)
	default:
		return step, p.fail("expected '.' or '['")
	}
	if p.pos < len(p.s) && p.s[p.pos] == '*' {
		p.pos++
		step.kind = jpWildcard
		return step, nil
	}
	key := p.ident()
	if key == "" {
		return step, p.fail("expected a key")
	}
	step.kind, step.keys = jpKeys, []string{key}
	return step, nil
}

func (p *jpParser) skipSpace() {
	for p.pos < len(p.s) && p.s[p.pos] == ' ' {
		p.pos++
	}
}

func (p *jpParser) bracket(step jpStep) (jpStep, error) {
	p.pos++
	p.skipSpace()
	if p.pos >= len(p.s) {
		return step, p.fail(jpBracketExpect)
	}
	switch p.s[p.pos] {
	case '*':
		p.pos++
		step.kind = jpWildcard
	case '?':
		p.pos++
		expr, err := p.filterExpr()
		if err != nil {
			return step, err
		}
		step.kind, step.filter = jpFilter, expr
	case '\'', '"':
		step.kind = jpKeys
		for {
			p.skipSpace()
			key, err := p.quoted()
			if err != nil {
				return step, err
			}
			step.keys = append(step.keys, key)
			p.skipSpace()
			if p.pos < len(p.s) && p.s[p.pos] == ',' {
				p.pos++
				continue
			}
			break
		}
	default:
		if err := p.indexesOrSlice(&step); err != nil {
			return step, err
		}
	}
	p.skipSpace()
	if p.pos >= len(p.s) || p.s[p.pos] != ']' {
		return step, p.fail(jpBracketExpect)
	}
	p.pos++
	return step, nil
}

func (p *jpParser) quoted() (string, error) {
	if p.pos >= len(p.s) || (p.s[p.pos] != '\'' && p.s[p.pos] != '"') {
		return "", p.fail(jpBracketExpect)
	}
	quote := p.s[p.pos]
	p.pos++
	var b strings.Builder
	for p.pos < len(p.s) && p.s[p.pos] != quote {
		if p.s[p.pos] == '\\' && p.pos+1 < len(p.s) {
			p.pos++
		}
		b.WriteByte(p.s[p.pos])
		p.pos++
	}
	if p.pos >= len(p.s) {
		return "", p.fail(jpBracketExpect)
	}
	p.pos++
	return b.String(), nil
}

func (p *jpParser) integer() (int, bool) {
	start := p.pos
	if p.pos < len(p.s) && (p.s[p.pos] == '-' || p.s[p.pos] == '+') {
		p.pos++
	}
	for p.pos < len(p.s) && p.s[p.pos] >= '0' && p.s[p.pos] <= '9' {
		p.pos++
	}
	n, err := strconv.Atoi(p.s[start:p.pos])
	if err != nil {
		p.pos = start
		return 0, false
	}
	return n, true
}

func (p *jpParser) indexesOrSlice(step *jpStep) error {
	first, hasFirst := p.integer()
	p.skipSpace()
	if p.pos < len(p.s) && p.s[p.pos] == ':' {
		step.kind = jpSlice
		if hasFirst {
			step.slice[0] = &first
		}
		for part := 1; part < 3 && p.pos < len(p.s) && p.s[p.pos] == ':'; part++ {
			p.pos++
			p.skipSpace()
			if n, ok := p.integer(); ok {
				step.slice[part] = &n
			}
			p.skipSpace()
		}
		return nil
	}
	if !hasFirst {
		return p.fail(jpBracketExpect)
	}
	step.kind, step.indexes = jpIndexes, []int{first}
	for p.pos < len(p.s) && p.s[p.pos] == ',' {
		p.pos++
		p.skipSpace()
		n, ok := p.integer()
		if !ok {
			return p.fail(jpBracketExpect)
		}
		step.indexes = append(step.indexes, n)
		p.skipSpace()
	}
	return nil
}

// jpExpr is a filter expression node.
type jpExpr interface{}

type jpLogic struct {
	and         bool
	left, right jpExpr
}

type jpCompare struct {
	op          string
	left, right jpOperand
}

type jpOperand struct {
	rel     *JSONPath
	root    *JSONPath
	literal *JNode
}

func (p *jpParser) filterExpr() (jpExpr, error) {
	p.skipSpace()
	wrapped := p.pos < len(p.s) && p.s[p.pos] == '('
	if wrapped {
		p.pos++
	}
	expr, err := p.orExpr()
	if err != nil {
		return nil, err
	}
	if wrapped {
		p.skipSpace()
		if p.pos >= len(p.s) || p.s[p.pos] != ')' {
			return nil, p.fail("expected ')'")
		}
		p.pos++
	}
	return expr, nil
}

func (p *jpParser) orExpr() (jpExpr, error) {
	left, err := p.andExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpace()
		if !strings.HasPrefix(p.s[p.pos:], "||") {
			return left, nil
		}
		p.pos += 2
		right, err := p.andExpr()
		if err != nil {
			return nil, err
		}
		left = &jpLogic{left: left, right: right}
	}
}

func (p *jpParser) andExpr() (jpExpr, error) {
	left, err := p.atom()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpace()
		if !strings.HasPrefix(p.s[p.pos:], "&&") {
			return left, nil
		}
		p.pos += 2
		right, err := p.atom()
		if err != nil {
			return nil, err
		}
		left = &jpLogic{and: true, left: left, right: right}
	}
}

var jpOps = []string{"==", "!=", "<=", ">=", "=~", "<", ">"}

func (p *jpParser) atom() (jpExpr, error) {
	p.skipSpace()
	if p.pos < len(p.s) && p.s[p.pos] == '(' {
		p.pos++
		expr, err := p.orExpr()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.pos >= len(p.s) || p.s[p.pos] != ')' {
			return nil, p.fail("expected ')'")
		}
		p.pos++
		return expr, nil
	}
	left, err := p.operand()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	for _, op := range jpOps {
		if strings.HasPrefix(p.s[p.pos:], op) {
			p.pos += len(op)
			right, err := p.operand()
			if err != nil {
				return nil, err
			}
			return &jpCompare{op: op, left: left, right: right}, nil
		}
	}
	return &jpCompare{left: left}, nil
}

// subPath reads a relative (@) or absolute ($) path inside a filter.
func (p *jpParser) subPath() (*JSONPath, error) {
	p.pos++
	path := &JSONPath{}
	for p.pos < len(p.s) && (p.s[p.pos] == '.' || p.s[p.pos] == '[') {
		step, err := p.step()
		if err != nil {
			return nil, err
		}
		path.steps = append(path.steps, step)
	}
	return path, nil
}

func (p *jpParser) operand() (jpOperand, error) {
	p.skipSpace()
	if p.pos >= len(p.s) {
		return jpOperand{}, p.fail("expected one of the following: single_filter")
	}
	switch c := p.s[p.pos]; {
	case c == '@':
		rel, err := p.subPath()
		return jpOperand{rel: rel}, err
	case c == '$':
		root, err := p.subPath()
		return jpOperand{root: root}, err
	case c == '\'' || c == '"':
		s, err := p.quoted()
		return jpOperand{literal: &JNode{Kind: JString, Str: s}}, err
	case c == '-' || c >= '0' && c <= '9':
		start := p.pos
		p.pos++
		for p.pos < len(p.s) && strings.IndexByte("0123456789.eE+-", p.s[p.pos]) >= 0 {
			p.pos++
		}
		n, err := ParseJSON(p.s[start:p.pos])
		if err != nil {
			return jpOperand{}, p.fail("expected a number")
		}
		return jpOperand{literal: n}, nil
	default:
		for word, n := range map[string]*JNode{"true": {Kind: JBool, Bool: true}, "false": {Kind: JBool}, "null": {Kind: JNull}} {
			if strings.HasPrefix(p.s[p.pos:], word) {
				p.pos += len(word)
				return jpOperand{literal: n}, nil
			}
		}
	}
	return jpOperand{}, p.fail("expected one of the following: single_filter")
}

// JSONMatch is one value a path selects, with where it sits so it can be
// replaced or removed. Parent is nil for the document itself.
type JSONMatch struct {
	Node   *JNode
	Parent *JNode
	Key    string
	Index  int
}

// Eval returns the values path selects in root, in RedisJSON's order.
func (path *JSONPath) Eval(root *JNode) []JSONMatch {
	return evalSteps(root, root, path.steps)
}

func evalSteps(root, doc *JNode, steps []jpStep) []JSONMatch {
	cur := []JSONMatch{{Node: doc}}
	for _, step := range steps {
		var next []JSONMatch
		for _, m := range cur {
			if step.deep {
				next = deepApply(root, m.Node, step, next)
			} else {
				next = applyStep(root, m.Node, step, next)
			}
		}
		cur = next
	}
	return cur
}

// deepApply applies step to n and every value under it: a node's own
// matches first, then each child's, as RedisJSON orders a descendant search.
func deepApply(root, n *JNode, step jpStep, out []JSONMatch) []JSONMatch {
	out = applyStep(root, n, step, out)
	switch n.Kind {
	case JArray:
		for _, c := range n.Arr {
			out = deepApply(root, c, step, out)
		}
	case JObject:
		for _, c := range n.Vals {
			out = deepApply(root, c, step, out)
		}
	}
	return out
}

func applyStep(root, n *JNode, step jpStep, out []JSONMatch) []JSONMatch {
	switch step.kind {
	case jpKeys:
		if n.Kind == JObject {
			for _, k := range step.keys {
				if v := n.Get(k); v != nil {
					out = append(out, JSONMatch{Node: v, Parent: n, Key: k})
				}
			}
		}
	case jpWildcard:
		out = appendChildren(n, out, nil)
	case jpIndexes:
		if n.Kind == JArray {
			for _, i := range step.indexes {
				if i < 0 {
					i += len(n.Arr)
				}
				if i >= 0 && i < len(n.Arr) {
					out = append(out, JSONMatch{Node: n.Arr[i], Parent: n, Index: i})
				}
			}
		}
	case jpSlice:
		if n.Kind == JArray {
			for _, i := range sliceIndexes(len(n.Arr), step.slice) {
				out = append(out, JSONMatch{Node: n.Arr[i], Parent: n, Index: i})
			}
		}
	case jpFilter:
		out = appendChildren(n, out, func(c *JNode) bool { return evalFilter(root, c, step.filter) })
	}
	return out
}

func appendChildren(n *JNode, out []JSONMatch, keep func(*JNode) bool) []JSONMatch {
	switch n.Kind {
	case JArray:
		for i, c := range n.Arr {
			if keep == nil || keep(c) {
				out = append(out, JSONMatch{Node: c, Parent: n, Index: i})
			}
		}
	case JObject:
		for i, c := range n.Vals {
			if keep == nil || keep(c) {
				out = append(out, JSONMatch{Node: c, Parent: n, Key: n.Keys[i]})
			}
		}
	}
	return out
}

func sliceIndexes(n int, s [3]*int) []int {
	step := 1
	if s[2] != nil {
		step = *s[2]
	}
	if step == 0 {
		return nil
	}
	norm := func(v *int, def int) int {
		if v == nil {
			return def
		}
		i := *v
		if i < 0 {
			i += n
		}
		return i
	}
	var out []int
	if step > 0 {
		start, end := max(norm(s[0], 0), 0), min(norm(s[1], n), n)
		for i := start; i < end; i += step {
			out = append(out, i)
		}
		return out
	}
	start, end := min(norm(s[0], n-1), n-1), max(norm(s[1], -1), -1)
	for i := start; i > end; i += step {
		out = append(out, i)
	}
	return out
}

func evalFilter(root, n *JNode, e jpExpr) bool {
	switch e := e.(type) {
	case *jpLogic:
		if e.and {
			return evalFilter(root, n, e.left) && evalFilter(root, n, e.right)
		}
		return evalFilter(root, n, e.left) || evalFilter(root, n, e.right)
	case *jpCompare:
		left := operandValue(root, n, e.left)
		if e.op == "" {
			return left != nil
		}
		right := operandValue(root, n, e.right)
		if left == nil || right == nil {
			return false
		}
		return compareJSON(left, right, e.op)
	}
	return false
}

func operandValue(root, n *JNode, o jpOperand) *JNode {
	var matches []JSONMatch
	switch {
	case o.literal != nil:
		return o.literal
	case o.rel != nil:
		matches = evalSteps(root, n, o.rel.steps)
	case o.root != nil:
		matches = evalSteps(root, root, o.root.steps)
	}
	if len(matches) == 0 {
		return nil
	}
	return matches[0].Node
}

func compareJSON(a, b *JNode, op string) bool {
	if op == "=~" {
		if a.Kind != JString || b.Kind != JString {
			return false
		}
		re, err := regexp.Compile(b.Str)
		return err == nil && re.MatchString(a.Str)
	}
	var cmp int
	switch {
	case a.IsNumber() && b.IsNumber():
		x, y := a.Number(), b.Number()
		switch {
		case x < y:
			cmp = -1
		case x > y:
			cmp = 1
		}
	case a.Kind == JString && b.Kind == JString:
		cmp = strings.Compare(a.Str, b.Str)
	default:
		equal := a.Equal(b)
		switch op {
		case "==":
			return equal
		case "!=":
			return !equal
		}
		return false
	}
	switch op {
	case "==":
		return cmp == 0
	case "!=":
		return cmp != 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	}
	return false
}

// SetTargets returns where JSON.SET writes: the existing values the path
// selects, or else, when its last step names one key, that key in each
// object its other steps select (a nil node marks a new key). An index past
// an array's end is ErrJSONIndexRange.
func (path *JSONPath) SetTargets(root *JNode) ([]JSONMatch, error) {
	if matches := path.Eval(root); len(matches) > 0 || len(path.steps) == 0 {
		return matches, nil
	}
	last := path.steps[len(path.steps)-1]
	parents := evalSteps(root, root, path.steps[:len(path.steps)-1])
	if last.kind == jpIndexes && !last.deep {
		for _, p := range parents {
			if p.Node.Kind == JArray {
				return nil, ErrJSONIndexRange
			}
		}
		return nil, nil
	}
	if last.kind != jpKeys || last.deep || len(last.keys) != 1 {
		return nil, nil
	}
	var out []JSONMatch
	for _, p := range parents {
		if p.Node.Kind == JObject {
			out = append(out, JSONMatch{Parent: p.Node, Key: last.keys[0]})
		}
	}
	return out, nil
}
