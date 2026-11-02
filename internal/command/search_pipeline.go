package command

import (
	"errors"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/search"
)

// pipelineKeys follows RediSearch's lookup tables, which decide what each
// FT.AGGREGATE step may read and in which order a row's fields are sent.
// Until the first GROUPBY a step may also read a SORTABLE field of the
// schema (any field after LOAD *), and a SORTBY, GROUPBY or reducer loads
// any schema field itself; the fields it reads join the row.
type pipelineKeys struct {
	idx   *search.Index
	root  bool
	all   bool
	known map[string]bool
	order []string
}

func newPipelineKeys(idx *search.Index) *pipelineKeys {
	return &pipelineKeys{idx: idx, root: true, known: make(map[string]bool)}
}

func (p *pipelineKeys) add(name string) {
	if !p.known[name] {
		p.known[name] = true
		p.order = append(p.order, name)
	}
}

// schemaField finds a field by its attribute name, as the lookup's schema
// cache does.
func (p *pipelineKeys) schemaField(name string) *search.Field {
	if !p.root {
		return nil
	}
	for i := range p.idx.Fields {
		if p.idx.Fields[i].Name == name {
			return &p.idx.Fields[i]
		}
	}
	return nil
}

// read resolves a property a step reads; load lets it load any schema
// field. A field read from the schema is copied into the rows: a SORTABLE
// one as its sorting vector holds it, lowercased unless UNF.
func (p *pipelineKeys) read(rows []*search.Row, name string, load bool) bool {
	if p.known[name] {
		return true
	}
	f := p.schemaField(name)
	if f == nil {
		return false
	}
	sortable := f.Sortable || p.all
	if !sortable && !load {
		return false
	}
	lower := f.Sortable && !f.UNF && (f.Type == search.FieldText || f.Type == search.FieldTag)
	for _, r := range rows {
		if v, ok := r.Get(name); ok {
			if lower {
				v = strings.ToLower(v)
			}
			r.Set(name, v)
		}
	}
	p.add(name)
	return true
}

func (p *pipelineKeys) readExpr(rows []*search.Row, expr *search.Expr) error {
	for _, name := range expr.Properties() {
		if !p.read(rows, name, false) {
			return errors.New("Property `" + name + "` not loaded nor in pipeline") //nolint:staticcheck // RediSearch's reply text
		}
	}
	return nil
}

// group checks a GROUPBY's properties and reducers and starts the group's
// lookup.
func (p *pipelineKeys) group(rows []*search.Row, props []string, reducers []search.Reducer) error {
	seen := make(map[string]bool)
	for _, prop := range props {
		name := strings.TrimPrefix(prop, "@")
		if !p.read(rows, name, true) {
			return errors.New("No such property `" + name + "`") //nolint:staticcheck // RediSearch's reply text
		}
		if seen[name] {
			return errors.New("Property `" + name + "` specified more than once") //nolint:staticcheck // RediSearch's reply text
		}
		seen[name] = true
	}
	for _, red := range reducers {
		for _, name := range reducerProperties(red) {
			if !p.read(rows, name, true) {
				return errors.New("Property is not present in document or pipeline: `" + name + "`") //nolint:staticcheck // RediSearch's reply text
			}
		}
	}
	p.root, p.all = false, false
	p.known, p.order = make(map[string]bool), nil
	for _, prop := range props {
		p.add(strings.TrimPrefix(prop, "@"))
	}
	for _, red := range reducers {
		p.add(red.Name())
	}
	return nil
}

// reducerProperties are the properties a reducer reads: its first
// argument, and FIRST_VALUE's BY property.
func reducerProperties(red search.Reducer) []string {
	if strings.EqualFold(red.Fn, "COUNT") || len(red.Args) == 0 {
		return nil
	}
	out := []string{strings.TrimPrefix(red.Args[0], "@")}
	if strings.EqualFold(red.Fn, "FIRST_VALUE") && len(red.Args) >= 3 && strings.EqualFold(red.Args[1], "BY") {
		out = append(out, strings.TrimPrefix(red.Args[2], "@"))
	}
	return out
}

// sortBy checks a SORTBY's properties.
func (p *pipelineKeys) sortBy(rows []*search.Row, keys []search.SortKey) error {
	for _, k := range keys {
		if !p.read(rows, k.Field, true) {
			return errors.New("Property `" + k.Field + "` not loaded nor in schema") //nolint:staticcheck // RediSearch's reply text
		}
	}
	return nil
}

// aggregateArrange is a GROUPBY segment's arrange step: RediSearch merges
// the SORTBY and LIMIT of a segment into one step that runs where the
// first of them stands. A sorter keeps offset+limit rows, or 10 without a
// LIMIT or MAX; a LIMIT pages the rows there, so a FILTER after it filters
// the page.
type aggregateArrange struct {
	keys          []search.SortKey
	offset, limit int
	sorted        bool
	limited       bool
	filterAfter   bool
	last          bool
}

// scanArrange reads the arrange step of the segment that starts at args[i]
// (a SORTBY or LIMIT) and runs to the next GROUPBY.
func scanArrange(ctx *Context, i int) (*aggregateArrange, error) {
	a := &aggregateArrange{last: true}
	seenSort := false
	for n := ctx.ArgCount(); i < n; i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "GROUPBY":
			a.last = false
			return a, nil
		case "FILTER":
			a.filterAfter = true
			i++
		case "SORTBY":
			if seenSort {
				return nil, errors.New("Multiple SORTBY steps are not allowed. Sort multiple fields in a single step") //nolint:staticcheck // RediSearch's reply text
			}
			seenSort = true
			props, last, err := countedArgs(ctx, i)
			if err != nil {
				return nil, err
			}
			i = last
			a.keys = nil
			for _, p := range props {
				switch {
				case strings.HasPrefix(p, "@"):
					a.keys = append(a.keys, search.SortKey{Field: p[1:]})
				case strings.EqualFold(p, "ASC"), strings.EqualFold(p, "DESC"):
					if len(a.keys) > 0 {
						a.keys[len(a.keys)-1].Desc = strings.EqualFold(p, "DESC")
					}
				default:
					return nil, errors.New("MISSING ASC or DESC after sort field (" + p + ")")
				}
			}
			a.sorted = len(a.keys) > 0
			if i+1 < n && strings.EqualFold(ctx.ArgString(i+1), "MAX") {
				if i+2 >= n {
					return nil, errors.New("Bad arguments for MAX: Expected an argument, but none provided") //nolint:staticcheck // RediSearch's reply text
				}
				m, err := strconv.ParseUint(ctx.ArgString(i+2), 10, 32)
				if err != nil {
					return nil, errors.New("Bad arguments for MAX: Could not convert argument to expected type") //nolint:staticcheck // RediSearch's reply text
				}
				a.limit = int(m)
				i += 2
			}
		case "LIMIT":
			off, num, err := parseLimit(ctx, i)
			if err != nil {
				return nil, err
			}
			a.offset, a.limit, a.limited = off, num, true
			i += 2
		default:
			i = aggregateStepEnd(ctx, i)
		}
	}
	return a, nil
}

// aggregateStepEnd returns the index of the last argument of the step at
// args[i].
func aggregateStepEnd(ctx *Context, i int) int {
	n := ctx.ArgCount()
	switch strings.ToUpper(ctx.ArgString(i)) {
	case "LOAD":
		if i+1 < n && ctx.ArgString(i+1) == "*" {
			return i + 1
		}
		if _, last, err := countedArgs(ctx, i); err == nil {
			return last
		}
	case "PARAMS":
		if _, last, err := countedArgs(ctx, i); err == nil {
			return last
		}
	case "APPLY":
		if i+3 < n && strings.EqualFold(ctx.ArgString(i+2), "AS") {
			return i + 3
		}
		return i + 1
	case "TIMEOUT", "DIALECT":
		return i + 1
	case "WITHCURSOR":
		for i+2 < n && (strings.EqualFold(ctx.ArgString(i+1), "COUNT") || strings.EqualFold(ctx.ArgString(i+1), "MAXIDLE")) {
			i += 2
		}
	}
	return i
}
