package graph

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

var errWriteQuery = errors.New("GRAPH.QUERY runs read queries (MATCH ... RETURN); write with GRAPH.ADDNODE, GRAPH.ADDEDGE, GRAPH.DELNODE and GRAPH.DELEDGE")

// MaxMatches bounds the rows a query's MATCH may produce, so a cartesian
// product of large graphs fails instead of exhausting memory: a row is a
// slice of the variables' values, some tens of bytes.
const MaxMatches = 250_000

var errTooManyMatches = fmt.Errorf("the query matches more than %d rows", MaxMatches)

// Result is a query's reply: column names and rows of values. A value is
// nil, bool, int64, float64, string, []any, *Node or *Edge.
type Result struct {
	Columns []string
	Rows    [][]any
}

// Query parses and runs a read-only Cypher query over s.
func Query(s *Snapshot, src string) (*Result, error) {
	q, err := parseQuery(src)
	if err != nil {
		return nil, err
	}
	if err := q.check(); err != nil {
		return nil, err
	}
	rows, err := s.match(q)
	if err != nil {
		return nil, err
	}
	return q.project(rows)
}

// check refuses variables that no pattern binds and a name bound both as a
// node and as a relationship.
func (q *query) check() error {
	kinds := map[string]string{}
	bind := func(name, kind string) error {
		if name == "" {
			return nil
		}
		if k, ok := kinds[name]; ok && k != kind {
			return fmt.Errorf("%s is bound both as a node and as a relationship", name)
		}
		kinds[name] = kind
		return nil
	}
	for _, p := range q.patterns {
		for _, n := range p.nodes {
			if err := bind(n.name, "node"); err != nil {
				return err
			}
		}
		for _, r := range p.rels {
			if err := bind(r.name, "rel"); err != nil {
				return err
			}
		}
	}
	aliases := map[string]bool{}
	for _, it := range q.items {
		if it.alias {
			aliases[it.name] = true
		}
	}
	var walk func(e expr, allowAlias bool) error
	walk = func(e expr, allowAlias bool) error {
		switch x := e.(type) {
		case varExpr:
			if _, ok := kinds[x.name]; !ok && (!allowAlias || !aliases[x.name]) {
				return fmt.Errorf("%s not defined", x.name)
			}
		case propExpr:
			return walk(x.of, allowAlias)
		case unaryExpr:
			return walk(x.x, allowAlias)
		case binExpr:
			if err := walk(x.l, allowAlias); err != nil {
				return err
			}
			return walk(x.r, allowAlias)
		case listExpr:
			for _, it := range x.items {
				if err := walk(it, allowAlias); err != nil {
					return err
				}
			}
		case callExpr:
			for _, a := range x.args {
				if err := walk(a, allowAlias); err != nil {
					return err
				}
			}
		case isNullExpr:
			return walk(x.x, allowAlias)
		}
		return nil
	}
	for _, p := range q.patterns {
		for _, n := range p.nodes {
			for _, v := range n.props {
				if err := walk(v, false); err != nil {
					return err
				}
			}
		}
		for _, r := range p.rels {
			for _, v := range r.props {
				if err := walk(v, false); err != nil {
					return err
				}
			}
		}
	}
	if q.where != nil {
		if err := walk(q.where, false); err != nil {
			return err
		}
	}
	for _, it := range q.items {
		if it.e != nil {
			if err := walk(it.e, false); err != nil {
				return err
			}
		}
	}
	for _, k := range q.order {
		if err := walk(k.e, true); err != nil {
			return err
		}
	}
	return nil
}

type binding map[string]any

// matched are the rows MATCH found: the values of vars, one slice a row.
type matched struct {
	vars []string
	rows [][]any
}

// bind fills b with row i.
func (m *matched) bind(b binding, i int) {
	for j, v := range m.vars {
		b[v] = m.rows[i][j]
	}
}

// variables are the names the patterns bind, in order of appearance.
func (q *query) variables() []string {
	var vars []string
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}
	for _, p := range q.patterns {
		for i, n := range p.nodes {
			add(n.name)
			if i < len(p.rels) {
				add(p.rels[i].name)
			}
		}
	}
	return vars
}

// match finds every binding of the patterns' variables, in the order of
// node and edge ids, that satisfies WHERE. A relationship is bound at most
// once per row, as Cypher's MATCH requires.
func (s *Snapshot) match(q *query) (*matched, error) {
	m := &matched{vars: q.variables()}
	used := map[uint64]bool{}
	b := binding{}
	var err error
	var matchPattern func(pi int) bool
	var matchStep func(pi, step int, at *Node) bool

	emit := func() bool {
		if q.where != nil {
			v, e := eval(q.where, b)
			if e != nil {
				err = e
				return false
			}
			if t, _ := v.(bool); !t {
				return true
			}
		}
		if len(m.rows) >= MaxMatches {
			err = errTooManyMatches
			return false
		}
		row := make([]any, len(m.vars))
		for i, v := range m.vars {
			row[i] = b[v]
		}
		m.rows = append(m.rows, row)
		return true
	}
	// tryNode binds n to the pattern node and continues with cont; it
	// reports false to stop the search.
	tryNode := func(np nodePat, n *Node, cont func() bool) bool {
		if np.label != "" && n.Label != np.label {
			return true
		}
		if np.name != "" {
			if bound, ok := b[np.name]; ok {
				if bn, _ := bound.(*Node); bn != n {
					return true
				}
				ok, e := propsMatch(np.props, n.Properties, b)
				if e != nil {
					err = e
					return false
				}
				if !ok {
					return true
				}
				return cont()
			}
		}
		ok, e := propsMatch(np.props, n.Properties, b)
		if e != nil {
			err = e
			return false
		}
		if !ok {
			return true
		}
		if np.name != "" {
			b[np.name] = n
			defer delete(b, np.name)
		}
		return cont()
	}
	matchStep = func(pi, step int, at *Node) bool {
		p := q.patterns[pi]
		if step == len(p.rels) {
			return matchPattern(pi + 1)
		}
		rp, np := p.rels[step], p.nodes[step+1]
		try := func(e *Edge, other uint64) bool {
			if used[e.ID] || !typeMatch(rp.types, e.Relation) {
				return true
			}
			if rp.name != "" {
				if bound, ok := b[rp.name]; ok {
					if be, _ := bound.(*Edge); be != e {
						return true
					}
				}
			}
			ok, ex := propsMatch(rp.props, e.Properties, b)
			if ex != nil {
				err = ex
				return false
			}
			if !ok {
				return true
			}
			used[e.ID] = true
			defer delete(used, e.ID)
			_, wasBound := b[rp.name]
			if rp.name != "" && !wasBound {
				b[rp.name] = e
				defer delete(b, rp.name)
			}
			n := s.nodes[other]
			return tryNode(np, n, func() bool { return matchStep(pi, step+1, n) })
		}
		if rp.dir == dirOut || rp.dir == dirBoth {
			for _, e := range s.out[at.ID] {
				if !try(e, e.To) {
					return false
				}
			}
		}
		if rp.dir == dirIn || rp.dir == dirBoth {
			for _, e := range s.in[at.ID] {
				if rp.dir == dirBoth && e.From == e.To {
					continue // a self-loop was met leaving the node
				}
				if !try(e, e.From) {
					return false
				}
			}
		}
		return true
	}
	matchPattern = func(pi int) bool {
		if pi == len(q.patterns) {
			return emit()
		}
		np := q.patterns[pi].nodes[0]
		if np.name != "" {
			if bound, ok := b[np.name].(*Node); ok {
				return tryNode(np, bound, func() bool { return matchStep(pi, 0, bound) })
			}
		}
		for _, id := range s.nodeOrder {
			n := s.nodes[id]
			if !tryNode(np, n, func() bool { return matchStep(pi, 0, n) }) {
				return false
			}
		}
		return true
	}
	matchPattern(0)
	return m, err
}

func typeMatch(types []string, rel string) bool {
	if len(types) == 0 {
		return true
	}
	for _, t := range types {
		if t == rel {
			return true
		}
	}
	return false
}

func propsMatch(want map[string]expr, have map[string]string, b binding) (bool, error) {
	for k, e := range want {
		v, err := eval(e, b)
		if err != nil {
			return false, err
		}
		got, ok := have[k]
		if !ok {
			return false, nil
		}
		eq := equal(got, v)
		if eq != true {
			return false, nil
		}
	}
	return true, nil
}

// project turns matched rows into the RETURN columns: grouped when an item
// aggregates, then DISTINCT, ORDER BY, SKIP and LIMIT.
func (q *query) project(m *matched) (*Result, error) {
	res := &Result{}
	for _, it := range q.items {
		res.Columns = append(res.Columns, it.name)
	}
	type outRow struct {
		vals []any
		src  int // the matched row, -1 for a group
	}
	b := binding{}
	var out []outRow
	aggregating := false
	for _, it := range q.items {
		if it.agg != "" {
			aggregating = true
		}
	}
	if !aggregating {
		for ri := range m.rows {
			m.bind(b, ri)
			vals := make([]any, len(q.items))
			for i, it := range q.items {
				v, err := eval(it.e, b)
				if err != nil {
					return nil, err
				}
				vals[i] = v
			}
			out = append(out, outRow{vals, ri})
		}
	} else {
		type group struct {
			keys []any
			accs []*accumulator
		}
		var groups []*group
		index := map[string]*group{}
		for ri := range m.rows {
			m.bind(b, ri)
			keys := make([]any, 0, len(q.items))
			for _, it := range q.items {
				if it.agg == "" {
					v, err := eval(it.e, b)
					if err != nil {
						return nil, err
					}
					keys = append(keys, v)
				}
			}
			k := valueKey(keys)
			g := index[k]
			if g == nil {
				g = &group{keys: keys}
				for _, it := range q.items {
					if it.agg != "" {
						g.accs = append(g.accs, newAccumulator(it))
					}
				}
				index[k] = g
				groups = append(groups, g)
			}
			ai := 0
			for _, it := range q.items {
				if it.agg == "" {
					continue
				}
				if err := g.accs[ai].add(b); err != nil {
					return nil, err
				}
				ai++
			}
		}
		if len(groups) == 0 && len(m.rows) == 0 {
			// Aggregating nothing without grouping keys still gives a row.
			plain := true
			for _, it := range q.items {
				if it.agg == "" {
					plain = false
				}
			}
			if plain {
				g := &group{}
				for _, it := range q.items {
					g.accs = append(g.accs, newAccumulator(it))
				}
				groups = append(groups, g)
			}
		}
		for _, g := range groups {
			vals := make([]any, len(q.items))
			ki, ai := 0, 0
			for i, it := range q.items {
				if it.agg == "" {
					vals[i] = g.keys[ki]
					ki++
				} else {
					vals[i] = g.accs[ai].result()
					ai++
				}
			}
			out = append(out, outRow{vals: vals, src: -1})
		}
	}
	if q.distinct {
		seen := map[string]bool{}
		kept := out[:0]
		for _, r := range out {
			k := valueKey(r.vals)
			if !seen[k] {
				seen[k] = true
				kept = append(kept, r)
			}
		}
		out = kept
	}
	if len(q.order) > 0 {
		keys := make([][]any, len(out))
		for i, r := range out {
			env := binding{}
			if r.src >= 0 && !q.distinct {
				m.bind(env, r.src)
			}
			for j, it := range q.items {
				env[it.name] = r.vals[j]
			}
			for _, k := range q.order {
				col := -1
				for j, it := range q.items {
					if it.text == k.text || it.name == k.text {
						col = j
					}
				}
				if col >= 0 {
					keys[i] = append(keys[i], r.vals[col])
					continue
				}
				if aggregating || q.distinct {
					return nil, fmt.Errorf("ORDER BY %s must name a RETURN item when the query aggregates or is DISTINCT", k.text)
				}
				v, err := eval(k.e, env)
				if err != nil {
					return nil, err
				}
				keys[i] = append(keys[i], v)
			}
		}
		idx := make([]int, len(out))
		for i := range idx {
			idx[i] = i
		}
		sort.SliceStable(idx, func(a, b int) bool {
			for j, k := range q.order {
				c := orderCompare(keys[idx[a]][j], keys[idx[b]][j])
				if k.desc {
					c = -c
				}
				if c != 0 {
					return c < 0
				}
			}
			return false
		})
		sorted := make([]outRow, len(out))
		for i, j := range idx {
			sorted[i] = out[j]
		}
		out = sorted
	}
	if q.skip > 0 {
		if q.skip >= int64(len(out)) {
			out = nil
		} else {
			out = out[q.skip:]
		}
	}
	if q.limit >= 0 && q.limit < int64(len(out)) {
		out = out[:q.limit]
	}
	for _, r := range out {
		res.Rows = append(res.Rows, r.vals)
	}
	return res, nil
}

type accumulator struct {
	item  returnItem
	count int64
	sum   float64
	ints  bool
	isum  int64
	best  any
	list  []any
	seen  map[string]bool
}

func newAccumulator(it returnItem) *accumulator {
	return &accumulator{item: it, ints: true, seen: map[string]bool{}}
}

func (a *accumulator) add(b binding) error {
	if a.item.star {
		a.count++
		return nil
	}
	v, err := eval(a.item.e, b)
	if err != nil || v == nil {
		return err
	}
	if a.item.dist {
		k := valueKey([]any{v})
		if a.seen[k] {
			return nil
		}
		a.seen[k] = true
	}
	switch a.item.agg {
	case "count":
		a.count++
	case "sum", "avg":
		n, ok := toNumber(v)
		if !ok {
			return fmt.Errorf("%s(%s) needs numbers, got %s", a.item.agg, a.item.text, describe(v))
		}
		a.count++
		if i, isInt := n.(int64); isInt && a.ints {
			a.isum += i
		} else {
			a.ints = false
		}
		a.sum += asFloat(n)
	case "min", "max":
		if a.best == nil {
			a.best = v
			return nil
		}
		c := orderCompare(v, a.best)
		if a.item.agg == "min" && c < 0 || a.item.agg == "max" && c > 0 {
			a.best = v
		}
	case "collect":
		a.list = append(a.list, v)
	}
	return nil
}

func (a *accumulator) result() any {
	switch a.item.agg {
	case "count":
		return a.count
	case "sum":
		if a.ints {
			return a.isum
		}
		return a.sum
	case "avg":
		if a.count == 0 {
			return nil
		}
		return a.sum / float64(a.count)
	case "collect":
		if a.list == nil {
			return []any{}
		}
		return a.list
	}
	return a.best
}

// --- evaluation ---

var functions = map[string]func(any) (any, error){
	"id": func(v any) (any, error) {
		switch x := v.(type) {
		case *Node:
			return int64(x.ID), nil
		case *Edge:
			return int64(x.ID), nil
		case nil:
			return nil, nil
		}
		return nil, fmt.Errorf("id() takes a node or a relationship, got %s", describe(v))
	},
	"labels": func(v any) (any, error) {
		if n, ok := v.(*Node); ok {
			return []any{n.Label}, nil
		}
		if v == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("labels() takes a node, got %s", describe(v))
	},
	"type": func(v any) (any, error) {
		if e, ok := v.(*Edge); ok {
			return e.Relation, nil
		}
		if v == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("type() takes a relationship, got %s", describe(v))
	},
	"tolower": func(v any) (any, error) {
		if s, ok := v.(string); ok {
			return strings.ToLower(s), nil
		}
		return nil, nil
	},
	"toupper": func(v any) (any, error) {
		if s, ok := v.(string); ok {
			return strings.ToUpper(s), nil
		}
		return nil, nil
	},
	"tostring": func(v any) (any, error) {
		switch x := v.(type) {
		case nil:
			return nil, nil
		case string:
			return x, nil
		case int64, float64, bool:
			return formatScalar(x), nil
		}
		return nil, fmt.Errorf("toString() takes a scalar, got %s", describe(v))
	},
	"tointeger": func(v any) (any, error) {
		n, ok := toNumber(v)
		if !ok {
			return nil, nil
		}
		if f, isFloat := n.(float64); isFloat {
			return int64(math.Trunc(f)), nil
		}
		return n, nil
	},
	"tofloat": func(v any) (any, error) {
		n, ok := toNumber(v)
		if !ok {
			return nil, nil
		}
		return asFloat(n), nil
	},
	"size": func(v any) (any, error) {
		switch x := v.(type) {
		case string:
			return int64(len([]rune(x))), nil
		case []any:
			return int64(len(x)), nil
		case nil:
			return nil, nil
		}
		return nil, fmt.Errorf("size() takes a string or a list, got %s", describe(v))
	},
	"abs": func(v any) (any, error) {
		n, ok := toNumber(v)
		if !ok {
			return nil, nil
		}
		if i, isInt := n.(int64); isInt {
			if i < 0 {
				return -i, nil
			}
			return i, nil
		}
		return math.Abs(n.(float64)), nil
	},
}

func eval(e expr, b binding) (any, error) {
	switch x := e.(type) {
	case litExpr:
		return x.v, nil
	case varExpr:
		return b[x.name], nil
	case propExpr:
		of, err := eval(x.of, b)
		if err != nil {
			return nil, err
		}
		switch o := of.(type) {
		case *Node:
			if v, ok := o.Properties[x.key]; ok {
				return v, nil
			}
			return nil, nil
		case *Edge:
			if v, ok := o.Properties[x.key]; ok {
				return v, nil
			}
			return nil, nil
		case nil:
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read property %s of %s", x.key, describe(of))
	case listExpr:
		out := make([]any, len(x.items))
		for i, it := range x.items {
			v, err := eval(it, b)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	case callExpr:
		v, err := eval(x.args[0], b)
		if err != nil {
			return nil, err
		}
		return functions[x.fn](v)
	case isNullExpr:
		v, err := eval(x.x, b)
		if err != nil {
			return nil, err
		}
		return (v == nil) != x.not, nil
	case unaryExpr:
		v, err := eval(x.x, b)
		if err != nil || v == nil {
			return nil, err
		}
		if x.op == "NOT" {
			t, ok := v.(bool)
			if !ok {
				return nil, fmt.Errorf("NOT takes a boolean, got %s", describe(v))
			}
			return !t, nil
		}
		n, ok := toNumber(v)
		if !ok {
			return nil, fmt.Errorf("cannot negate %s", describe(v))
		}
		if i, isInt := n.(int64); isInt {
			return -i, nil
		}
		return -n.(float64), nil
	case binExpr:
		return evalBinary(x, b)
	}
	return nil, fmt.Errorf("cannot evaluate %T", e)
}

func evalBinary(x binExpr, b binding) (any, error) {
	l, err := eval(x.l, b)
	if err != nil {
		return nil, err
	}
	switch x.op {
	case "AND", "OR", "XOR":
		r, err := eval(x.r, b)
		if err != nil {
			return nil, err
		}
		return logic(x.op, l, r)
	}
	r, err := eval(x.r, b)
	if err != nil {
		return nil, err
	}
	switch x.op {
	case "=":
		return equal(l, r), nil
	case "<>":
		eq := equal(l, r)
		if eq == nil {
			return nil, nil
		}
		return !eq.(bool), nil
	case "<", ">", "<=", ">=":
		if l == nil || r == nil {
			return nil, nil
		}
		c, ok := compare(l, r)
		if !ok {
			return nil, nil
		}
		switch x.op {
		case "<":
			return c < 0, nil
		case ">":
			return c > 0, nil
		case "<=":
			return c <= 0, nil
		}
		return c >= 0, nil
	case "IN":
		list, ok := r.([]any)
		if r == nil {
			return nil, nil
		}
		if !ok {
			return nil, fmt.Errorf("IN takes a list, got %s", describe(r))
		}
		sawNull := l == nil
		for _, it := range list {
			eq := equal(l, it)
			if eq == true {
				return true, nil
			}
			if eq == nil {
				sawNull = true
			}
		}
		if sawNull {
			return nil, nil
		}
		return false, nil
	case "CONTAINS", "STARTS", "ENDS":
		ls, lok := l.(string)
		rs, rok := r.(string)
		if !lok || !rok {
			return nil, nil
		}
		switch x.op {
		case "CONTAINS":
			return strings.Contains(ls, rs), nil
		case "STARTS":
			return strings.HasPrefix(ls, rs), nil
		}
		return strings.HasSuffix(ls, rs), nil
	}
	return arith(x.op, l, r)
}

func logic(op string, l, r any) (any, error) {
	lb, lok := l.(bool)
	rb, rok := r.(bool)
	if l != nil && !lok || r != nil && !rok {
		return nil, fmt.Errorf("%s takes booleans", op)
	}
	switch op {
	case "AND":
		if l != nil && !lb || r != nil && !rb {
			return false, nil
		}
		if l == nil || r == nil {
			return nil, nil
		}
		return true, nil
	case "OR":
		if l != nil && lb || r != nil && rb {
			return true, nil
		}
		if l == nil || r == nil {
			return nil, nil
		}
		return false, nil
	}
	if l == nil || r == nil {
		return nil, nil
	}
	return lb != rb, nil
}

func arith(op string, l, r any) (any, error) {
	if l == nil || r == nil {
		return nil, nil
	}
	if op == "+" {
		ls, lstr := l.(string)
		rs, rstr := r.(string)
		if lstr && rstr {
			return ls + rs, nil
		}
		if ll, ok := l.([]any); ok {
			if rl, ok := r.([]any); ok {
				return append(append([]any{}, ll...), rl...), nil
			}
			return append(append([]any{}, ll...), r), nil
		}
	}
	ln, lok := toNumber(l)
	rn, rok := toNumber(r)
	if !lok || !rok {
		if ls, ok := l.(string); ok && op == "+" {
			return ls + formatScalar(r), nil
		}
		if rs, ok := r.(string); ok && op == "+" {
			return formatScalar(l) + rs, nil
		}
		return nil, fmt.Errorf("cannot apply %s to %s and %s", op, describe(l), describe(r))
	}
	li, lInt := ln.(int64)
	ri, rInt := rn.(int64)
	if lInt && rInt {
		switch op {
		case "+":
			return li + ri, nil
		case "-":
			return li - ri, nil
		case "*":
			return li * ri, nil
		case "/", "%":
			if ri == 0 {
				return nil, errors.New("division by zero")
			}
			if op == "/" {
				return li / ri, nil
			}
			return li % ri, nil
		}
	}
	lf, rf := asFloat(ln), asFloat(rn)
	switch op {
	case "+":
		return lf + rf, nil
	case "-":
		return lf - rf, nil
	case "*":
		return lf * rf, nil
	case "/":
		return lf / rf, nil
	}
	return math.Mod(lf, rf), nil
}

// toNumber reads v as a number: numbers as they are, and strings that
// parse as one, since GRAPH.ADDNODE stores properties as strings.
func toNumber(v any) (any, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case float64:
		return x, true
	case string:
		s := strings.TrimSpace(x)
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i, true
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
			return f, true
		}
	}
	return nil, false
}

func asFloat(n any) float64 {
	if i, ok := n.(int64); ok {
		return float64(i)
	}
	return n.(float64)
}

func isNumber(v any) bool {
	switch v.(type) {
	case int64, float64:
		return true
	}
	return false
}

// equal is Cypher's =: null when either side is null; a string property
// equals a number when it reads as that number.
func equal(l, r any) any {
	if l == nil || r == nil {
		return nil
	}
	if isNumber(l) || isNumber(r) {
		ln, lok := toNumber(l)
		rn, rok := toNumber(r)
		if !lok || !rok {
			return false
		}
		return asFloat(ln) == asFloat(rn)
	}
	switch a := l.(type) {
	case string:
		b, ok := r.(string)
		return ok && a == b
	case bool:
		b, ok := r.(bool)
		return ok && a == b
	case *Node:
		b, ok := r.(*Node)
		return ok && a == b
	case *Edge:
		b, ok := r.(*Edge)
		return ok && a == b
	case []any:
		b, ok := r.([]any)
		if !ok || len(a) != len(b) {
			return false
		}
		for i := range a {
			eq := equal(a[i], b[i])
			if eq != true {
				return eq
			}
		}
		return true
	}
	return false
}

// compare orders two values of comparable kinds: numbers (strings that
// read as numbers included) or strings.
func compare(l, r any) (int, bool) {
	if isNumber(l) || isNumber(r) {
		ln, lok := toNumber(l)
		rn, rok := toNumber(r)
		if !lok || !rok {
			return 0, false
		}
		a, b := asFloat(ln), asFloat(rn)
		switch {
		case a < b:
			return -1, true
		case a > b:
			return 1, true
		}
		return 0, true
	}
	ls, lok := l.(string)
	rs, rok := r.(string)
	if lok && rok {
		return strings.Compare(ls, rs), true
	}
	lb, lok := l.(bool)
	rb, rok := r.(bool)
	if lok && rok {
		switch {
		case lb == rb:
			return 0, true
		case !lb:
			return -1, true
		}
		return 1, true
	}
	return 0, false
}

// orderCompare orders any two values for ORDER BY, min and max: values of
// one kind by compare, kinds by a fixed rank, null last.
func orderCompare(l, r any) int {
	if c, ok := compare(l, r); ok {
		return c
	}
	lr, rr := rank(l), rank(r)
	switch {
	case lr < rr:
		return -1
	case lr > rr:
		return 1
	}
	switch a := l.(type) {
	case *Node:
		return cmpID(a.ID, r.(*Node).ID)
	case *Edge:
		return cmpID(a.ID, r.(*Edge).ID)
	}
	return strings.Compare(valueKey([]any{l}), valueKey([]any{r}))
}

func cmpID(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func rank(v any) int {
	switch v.(type) {
	case *Node:
		return 0
	case *Edge:
		return 1
	case []any:
		return 2
	case string:
		return 3
	case bool:
		return 4
	case int64, float64:
		return 5
	case nil:
		return 7
	}
	return 6
}

func describe(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "a boolean"
	case int64:
		return "an integer"
	case float64:
		return "a float"
	case string:
		return "a string"
	case []any:
		return "a list"
	case *Node:
		return "a node"
	case *Edge:
		return "a relationship"
	}
	return fmt.Sprintf("%T", v)
}

func formatScalar(v any) string {
	switch x := v.(type) {
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return FormatFloat(x)
	case bool:
		return strconv.FormatBool(x)
	case string:
		return x
	}
	return ""
}

// FormatFloat writes a float as RedisGraph does: with up to 15 significant
// digits.
func FormatFloat(f float64) string {
	return strconv.FormatFloat(f, 'g', 15, 64)
}

// valueKey is a string identifying a list of values, for grouping and
// DISTINCT: numbers that are equal give one key.
func valueKey(vals []any) string {
	var b strings.Builder
	for _, v := range vals {
		switch x := v.(type) {
		case nil:
			b.WriteString("n;")
		case bool:
			fmt.Fprintf(&b, "b%t;", x)
		case int64:
			fmt.Fprintf(&b, "f%v;", float64(x))
		case float64:
			fmt.Fprintf(&b, "f%v;", x)
		case string:
			fmt.Fprintf(&b, "s%d:%s;", len(x), x)
		case *Node:
			fmt.Fprintf(&b, "N%d;", x.ID)
		case *Edge:
			fmt.Fprintf(&b, "E%d;", x.ID)
		case []any:
			fmt.Fprintf(&b, "l%d[%s];", len(x), valueKey(x))
		}
	}
	return b.String()
}
