package command

import (
	"errors"
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
