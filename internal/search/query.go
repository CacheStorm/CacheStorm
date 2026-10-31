package search

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Document is a key as an index reads it. The indexed form keeps what
// queries match on: each schema field's tokens and stems, numbers and tags,
// and the raw value of SORTABLE fields. A full document (Content) also
// holds the hash fields in their order or the JSON document, for replies.
type Document struct {
	Key    string
	Fields [][2]string
	JSON   *store.JNode
	full   bool
	fields []docField
	length int32
	// entry and value are what the document was read from, to tell when
	// the key changed since.
	entry *store.Entry
	value store.Value
}

// docField is one schema field of a document, by position in the schema.
type docField struct {
	strs  []string // TEXT tokens, TAG tags
	stems []string // TEXT stems, sharing the token when unchanged
	nums  []float64
	raw   []string // full documents and SORTABLE fields
}

// Load reads key's value as idx indexes it; ok is false when the value is
// not of the index's type. full keeps the content for replies.
func (idx *Index) Load(key string, entry *store.Entry, full bool) (*Document, bool) {
	value := entry.Value
	doc := &Document{Key: key, entry: entry, value: value, full: full, fields: make([]docField, len(idx.Fields))}
	values := make([][]string, len(idx.Fields))
	switch v := value.(type) {
	case *store.HashValue:
		if idx.JSON {
			return nil, false
		}
		v.RLock()
		if full {
			v.Range(func(f string, val []byte) bool {
				doc.Fields = append(doc.Fields, [2]string{f, string(val)})
				return true
			})
		}
		for i, f := range idx.Fields {
			if val, ok := v.Fields[f.Identifier]; ok {
				values[i] = []string{string(val)}
			}
		}
		v.RUnlock()
	case *store.JSONValue:
		if !idx.JSON {
			return nil, false
		}
		v.RLock()
		root := v.Root
		for i, f := range idx.Fields {
			for _, m := range f.path.Eval(root) {
				values[i] = append(values[i], jsonScalars(m.Node)...)
			}
		}
		if full {
			doc.JSON = root.Clone()
		}
		v.RUnlock()
	default:
		return nil, false
	}
	for i := range idx.Fields {
		f := &idx.Fields[i]
		df := &doc.fields[i]
		if full || f.Sortable {
			df.raw = values[i]
		}
		switch f.Type {
		case FieldNumeric:
			for _, raw := range values[i] {
				if n, err := strconv.ParseFloat(raw, 64); err == nil {
					df.nums = append(df.nums, n)
				}
			}
		case FieldGeo:
			for _, raw := range values[i] {
				lonText, latText, ok := strings.Cut(raw, ",")
				lon, err1 := strconv.ParseFloat(strings.TrimSpace(lonText), 64)
				lat, err2 := strconv.ParseFloat(strings.TrimSpace(latText), 64)
				if ok && err1 == nil && err2 == nil {
					df.nums = append(df.nums, lon, lat)
				}
			}
		case FieldTag:
			for _, raw := range values[i] {
				for _, tag := range strings.Split(raw, string(f.Separator)) {
					tag = strings.TrimSpace(tag)
					if !f.CaseSensitive {
						tag = strings.ToLower(tag)
					}
					df.strs = append(df.strs, tag)
				}
			}
		case FieldText:
			for _, raw := range values[i] {
				toks := Tokenize(raw)
				df.strs = append(df.strs, toks...)
				for _, t := range toks {
					df.stems = append(df.stems, Stem(t))
					if !idx.Stopwords[t] {
						doc.length++
					}
				}
			}
		}
	}
	return doc, true
}

// stemSet calls fn once for each distinct stem of the document.
func (d *Document) stemSet(fn func(stem string)) {
	seen := make(map[string]struct{})
	for _, f := range d.fields {
		for _, st := range f.stems {
			if _, ok := seen[st]; !ok {
				seen[st] = struct{}{}
				fn(st)
			}
		}
	}
}

// jsonScalars are the indexable values of a JSON match: a scalar, or the
// scalars of an array (for TAG fields over arrays).
func jsonScalars(n *store.JNode) []string {
	switch n.Kind {
	case store.JString:
		return []string{n.Str}
	case store.JNull, store.JObject:
		return nil
	case store.JArray:
		var out []string
		for _, e := range n.Arr {
			if e.Kind != store.JArray && e.Kind != store.JObject {
				out = append(out, jsonScalars(e)...)
			}
		}
		return out
	default:
		return []string{string(store.AppendJSON(nil, n))}
	}
}

// HashField returns a hash field of a full document.
func (d *Document) HashField(name string) (string, bool) {
	for _, kv := range d.Fields {
		if kv[0] == name {
			return kv[1], true
		}
	}
	return "", false
}

func isSeparator(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	return strings.ContainsRune(",.<>{}[]\"':;!@#$%^&*()-+=~/\\|?`", r)
}

// Tokenize splits text into lower-case terms, as RediSearch does.
func Tokenize(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), isSeparator)
}

// Stem reduces an English word to a stem, so plural and inflected forms
// match: a light Porter-style suffix stripper.
func Stem(w string) string {
	if utf8.RuneCountInString(w) < 4 {
		return w
	}
	for _, rule := range [][2]string{{"sses", "ss"}, {"ies", "y"}, {"ness", ""}, {"ing", ""}, {"edly", ""}, {"ed", ""}, {"ly", ""}, {"es", ""}, {"s", ""}} {
		if strings.HasSuffix(w, rule[0]) && !strings.HasSuffix(w, "ss") || rule[0] == "sses" && strings.HasSuffix(w, "sses") {
			stem := strings.TrimSuffix(w, rule[0]) + rule[1]
			if utf8.RuneCountInString(stem) >= 3 {
				return stem
			}
		}
	}
	return w
}

// Node is a parsed query node.
type Node interface{}

type (
	andNode  struct{ children []Node }
	orNode   struct{ children []Node }
	notNode  struct{ child Node }
	allNode  struct{}
	termNode struct {
		fields []string
		text   string
		stem   string
		prefix bool
		phrase []string
	}
	numericNode struct {
		field          string
		lo, hi         float64
		loExcl, hiExcl bool
	}
	tagNode struct {
		field  string
		values []string
	}
	geoNode struct {
		field              string
		lon, lat, radiusKm float64
	}
	emptyNode struct{}
)

// QueryOptions change how a query is read.
type QueryOptions struct {
	Verbatim    bool
	NoStopwords bool
	InFields    []string
	Params      map[string]string
}

// ErrSyntax is a query syntax error, in RediSearch's words.
type ErrSyntax struct {
	Offset int
	Near   string
}

func (e *ErrSyntax) Error() string {
	return fmt.Sprintf("Syntax error at offset %d near %s", e.Offset, e.Near)
}

type queryParser struct {
	s    string
	pos  int
	idx  *Index
	opts QueryOptions
}

// ParseQuery parses a RediSearch query against idx.
func ParseQuery(idx *Index, q string, opts QueryOptions) (Node, error) {
	p := &queryParser{s: q, idx: idx, opts: opts}
	n, err := p.intersection(0)
	if err != nil {
		return nil, err
	}
	p.space()
	if p.pos < len(p.s) {
		return nil, p.fail()
	}
	return n, nil
}

func (p *queryParser) fail() error {
	near := p.s[p.pos:]
	if len(near) > 10 {
		near = near[:10]
	}
	return &ErrSyntax{Offset: p.pos, Near: near}
}

func (p *queryParser) space() {
	for p.pos < len(p.s) && (p.s[p.pos] == ' ' || p.s[p.pos] == '\t') {
		p.pos++
	}
}

// intersection reads space-separated unions until a closing bracket.
func (p *queryParser) intersection(depth int) (Node, error) {
	var children []Node
	for {
		p.space()
		if p.pos >= len(p.s) || p.s[p.pos] == ')' {
			break
		}
		n, err := p.union(depth)
		if err != nil {
			return nil, err
		}
		if n != nil {
			children = append(children, n)
		}
	}
	switch len(children) {
	case 0:
		if depth == 0 && strings.TrimSpace(p.s) != "" {
			return emptyNode{}, nil
		}
		if depth > 0 {
			return nil, p.fail()
		}
		return emptyNode{}, nil
	case 1:
		return children[0], nil
	}
	return &andNode{children}, nil
}

// union reads unary nodes joined by |, which binds tighter than the
// intersection, as in RediSearch's default dialect.
func (p *queryParser) union(depth int) (Node, error) {
	first, err := p.unary(depth)
	if err != nil {
		return nil, err
	}
	nodes := []Node{first}
	for {
		p.space()
		if p.pos >= len(p.s) || p.s[p.pos] != '|' {
			break
		}
		p.pos++
		p.space()
		n, err := p.unary(depth)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	var kept []Node
	for _, n := range nodes {
		if n != nil {
			kept = append(kept, n)
		}
	}
	switch len(kept) {
	case 0:
		return nil, nil
	case 1:
		return kept[0], nil
	}
	return &orNode{kept}, nil
}

func (p *queryParser) unary(depth int) (Node, error) {
	if p.pos >= len(p.s) {
		return nil, p.fail()
	}
	switch p.s[p.pos] {
	case '-':
		p.pos++
		n, err := p.unary(depth)
		if err != nil || n == nil {
			return nil, err
		}
		return &notNode{n}, nil
	case '~':
		p.pos++
		return p.unary(depth)
	}
	return p.atom(nil, depth)
}

// atom reads one query element; fields scope its terms.
func (p *queryParser) atom(fields []string, depth int) (Node, error) {
	if p.pos >= len(p.s) {
		return nil, p.fail()
	}
	switch p.s[p.pos] {
	case '(':
		start := p.pos
		p.pos++
		saved := p.opts.InFields
		if fields != nil {
			p.opts.InFields = fields
		}
		n, err := p.intersection(depth + 1)
		p.opts.InFields = saved
		if err != nil {
			p.pos = start
			return nil, p.fail()
		}
		if p.pos >= len(p.s) || p.s[p.pos] != ')' {
			p.pos = start
			return nil, p.fail()
		}
		p.pos++
		return n, nil
	case '@':
		return p.fieldExpr(depth)
	case '"':
		p.pos++
		end := strings.IndexByte(p.s[p.pos:], '"')
		if end < 0 {
			return nil, p.fail()
		}
		words := p.terms(Tokenize(p.s[p.pos : p.pos+end]))
		p.pos += end + 1
		if len(words) == 0 {
			return nil, nil
		}
		return &termNode{fields: p.scope(fields), phrase: words}, nil
	case '*':
		p.pos++
		return allNode{}, nil
	case '$':
		name := p.word()
		v, ok := p.opts.Params[strings.TrimPrefix(name, "$")]
		if !ok {
			return nil, errors.New("No such parameter `" + strings.TrimPrefix(name, "$") + "`")
		}
		return p.termOf(v, fields)
	case ')', '|':
		return nil, p.fail()
	}
	w := p.word()
	if w == "" {
		return nil, p.fail()
	}
	return p.termOf(w, fields)
}

func (p *queryParser) termOf(w string, fields []string) (Node, error) {
	prefix := strings.HasSuffix(w, "*")
	w = strings.TrimSuffix(strings.Trim(w, "%"), "*")
	toks := Tokenize(w)
	if len(toks) == 0 {
		return nil, nil
	}
	if !prefix && len(toks) == 1 && !p.opts.NoStopwords && p.idx.Stopwords[toks[0]] {
		return nil, nil
	}
	if len(toks) > 1 {
		return &termNode{fields: p.scope(fields), phrase: toks}, nil
	}
	return &termNode{fields: p.scope(fields), text: toks[0], stem: Stem(toks[0]), prefix: prefix}, nil
}

func (p *queryParser) scope(fields []string) []string {
	if fields != nil {
		return fields
	}
	return p.opts.InFields
}

func (p *queryParser) terms(words []string) []string {
	if p.opts.NoStopwords {
		return words
	}
	var out []string
	for _, w := range words {
		if !p.idx.Stopwords[w] {
			out = append(out, w)
		}
	}
	return out
}

// word reads a bare term: everything up to a space or an operator; a
// backslash escapes the next character.
func (p *queryParser) word() string {
	var b strings.Builder
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if c == '\\' && p.pos+1 < len(p.s) {
			b.WriteByte(p.s[p.pos+1])
			p.pos += 2
			continue
		}
		if c == ' ' || c == '\t' || c == '(' || c == ')' || c == '|' || c == '"' || c == '{' || c == '}' || c == '[' || c == ']' || c == '@' {
			break
		}
		b.WriteByte(c)
		p.pos++
	}
	return b.String()
}

func (p *queryParser) fieldExpr(depth int) (Node, error) {
	start := p.pos
	p.pos++
	colon := strings.IndexByte(p.s[p.pos:], ':')
	if colon <= 0 {
		p.pos = start
		return nil, p.fail()
	}
	names := strings.Split(p.s[p.pos:p.pos+colon], "|")
	p.pos += colon + 1
	p.space()
	if p.pos >= len(p.s) {
		return nil, p.fail()
	}
	var field *Field
	if len(names) == 1 {
		field = p.idx.Field(names[0])
	}
	switch p.s[p.pos] {
	case '[':
		end := strings.IndexByte(p.s[p.pos:], ']')
		if end < 0 {
			return nil, p.fail()
		}
		parts := strings.Fields(p.s[p.pos+1 : p.pos+end])
		p.pos += end + 1
		for i, part := range parts {
			if strings.HasPrefix(part, "$") {
				v, ok := p.opts.Params[part[1:]]
				if !ok {
					return nil, errors.New("No such parameter `" + part[1:] + "`")
				}
				parts[i] = v
			}
		}
		if field != nil && field.Type == FieldGeo {
			return parseGeo(names[0], parts)
		}
		return parseNumericRange(names[0], parts)
	case '{':
		end := strings.IndexByte(p.s[p.pos:], '}')
		if end < 0 {
			return nil, p.fail()
		}
		var values []string
		for _, v := range strings.Split(p.s[p.pos+1:p.pos+end], "|") {
			v = strings.ReplaceAll(strings.TrimSpace(v), "\\", "")
			if strings.HasPrefix(v, "$") {
				if pv, ok := p.opts.Params[v[1:]]; ok {
					v = pv
				}
			}
			if v != "" {
				values = append(values, v)
			}
		}
		p.pos += end + 1
		return &tagNode{field: names[0], values: values}, nil
	}
	return p.atom(names, depth)
}

func parseBound(s string) (float64, bool, error) {
	excl := strings.HasPrefix(s, "(")
	s = strings.TrimPrefix(s, "(")
	switch strings.ToLower(s) {
	case "-inf":
		return math.Inf(-1), excl, nil
	case "inf", "+inf":
		return math.Inf(1), excl, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, excl, err
}

func parseNumericRange(field string, parts []string) (Node, error) {
	if len(parts) == 1 {
		parts = append(parts, parts[0])
	}
	if len(parts) != 2 {
		return nil, errNumericRange
	}
	lo, loExcl, err1 := parseBound(parts[0])
	hi, hiExcl, err2 := parseBound(parts[1])
	if err1 != nil || err2 != nil {
		return nil, errNumericRange
	}
	return &numericNode{field: field, lo: lo, hi: hi, loExcl: loExcl, hiExcl: hiExcl}, nil
}

//nolint:staticcheck // RediSearch's reply texts
var (
	errNumericRange = errors.New("Expecting numeric or parameter")
	errGeoUnit      = errors.New("Invalid GeoFilter unit")
)

var geoUnits = map[string]float64{"m": 0.001, "km": 1, "mi": 1.609344, "ft": 0.0003048}

func parseGeo(field string, parts []string) (Node, error) {
	if len(parts) != 4 {
		return nil, errNumericRange
	}
	lon, err1 := strconv.ParseFloat(parts[0], 64)
	lat, err2 := strconv.ParseFloat(parts[1], 64)
	r, err3 := strconv.ParseFloat(parts[2], 64)
	unit, ok := geoUnits[strings.ToLower(parts[3])]
	if err1 != nil || err2 != nil || err3 != nil || !ok {
		return nil, errGeoUnit
	}
	return &geoNode{field: field, lon: lon, lat: lat, radiusKm: r * unit}, nil
}

// NumericFilter and GeoFilter build the nodes of FT.SEARCH's FILTER and
// GEOFILTER options.
func NumericFilter(field string, lo, hi string) (Node, error) {
	return parseNumericRange(field, []string{lo, hi})
}

func GeoFilter(field string, lon, lat, radius, unit string) (Node, error) {
	return parseGeo(field, []string{lon, lat, radius, unit})
}

// And joins nodes into one that requires all of them.
func And(nodes ...Node) Node { return &andNode{nodes} }

// Match reports whether doc satisfies n and the score it earns there.
func (idx *Index) Match(n Node, doc *Document, opts QueryOptions, stats *Stats) (bool, float64) {
	switch n := n.(type) {
	case allNode:
		return true, 1
	case emptyNode:
		return false, 0
	case *andNode:
		total := 0.0
		for _, c := range n.children {
			ok, s := idx.Match(c, doc, opts, stats)
			if !ok {
				return false, 0
			}
			total += s
		}
		return true, total
	case *orNode:
		matched, total := false, 0.0
		for _, c := range n.children {
			if ok, s := idx.Match(c, doc, opts, stats); ok {
				matched, total = true, total+s
			}
		}
		return matched, total
	case *notNode:
		ok, _ := idx.Match(n.child, doc, opts, stats)
		return !ok, 0
	case *termNode:
		return idx.matchTerm(n, doc, opts, stats)
	case *numericNode:
		f := idx.Field(n.field)
		if f == nil || f.Type != FieldNumeric {
			return false, 0
		}
		for _, v := range doc.fields[f.pos].nums {
			if (v > n.lo || v == n.lo && !n.loExcl) && (v < n.hi || v == n.hi && !n.hiExcl) {
				return true, 0
			}
		}
		return false, 0
	case *tagNode:
		f := idx.Field(n.field)
		if f == nil || f.Type != FieldTag {
			return false, 0
		}
		for _, tag := range doc.fields[f.pos].strs {
			for _, want := range n.values {
				prefix := strings.HasSuffix(want, "*")
				want = strings.TrimSuffix(want, "*")
				if !f.CaseSensitive {
					want = strings.ToLower(want)
				}
				if tag == want || prefix && strings.HasPrefix(tag, want) {
					return true, 0
				}
			}
		}
		return false, 0
	case *geoNode:
		f := idx.Field(n.field)
		if f == nil || f.Type != FieldGeo {
			return false, 0
		}
		coords := doc.fields[f.pos].nums
		for i := 0; i+1 < len(coords); i += 2 {
			if haversineKm(n.lon, n.lat, coords[i], coords[i+1]) <= n.radiusKm {
				return true, 0
			}
		}
		return false, 0
	}
	return false, 0
}

func haversineKm(lon1, lat1, lon2, lat2 float64) float64 {
	const r = 6372.797560856
	rad := math.Pi / 180
	dLat, dLon := (lat2-lat1)*rad, (lon2-lon1)*rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

// textFields are the TEXT fields a term searches.
func (idx *Index) textFields(scope []string) []*Field {
	var out []*Field
	for i := range idx.Fields {
		f := &idx.Fields[i]
		if f.Type != FieldText || f.NoIndex {
			continue
		}
		if scope != nil {
			in := false
			for _, s := range scope {
				in = in || s == f.Name || s == f.Identifier
			}
			if !in {
				continue
			}
		}
		out = append(out, f)
	}
	return out
}

func (idx *Index) termMatches(f *Field, token, stem, term, termStem string, prefix bool, opts QueryOptions) bool {
	if prefix {
		return strings.HasPrefix(token, term)
	}
	if token == term {
		return true
	}
	if opts.Verbatim || f.NoStem || idx.Language != "english" {
		return false
	}
	return stem == termStem
}

// matchTerm matches a term or phrase; its score is BM25 over the
// occurrences, each weighted by its field's weight, as RediSearch's
// BM25STD scorer counts them.
func (idx *Index) matchTerm(n *termNode, doc *Document, opts QueryOptions, stats *Stats) (bool, float64) {
	weighted := 0.0
	for _, f := range idx.textFields(n.fields) {
		df := &doc.fields[f.pos]
		toks, stems := df.strs, df.stems
		if n.phrase != nil {
			for i := 0; i+len(n.phrase) <= len(toks); i++ {
				all := true
				for j, w := range n.phrase {
					all = all && idx.termMatches(f, toks[i+j], stems[i+j], w, Stem(w), false, opts)
				}
				if all {
					weighted += f.Weight
				}
			}
			continue
		}
		for i, t := range toks {
			if idx.termMatches(f, t, stems[i], n.text, n.stem, n.prefix, opts) {
				weighted += f.Weight
			}
		}
	}
	if weighted == 0 {
		return false, 0
	}
	term := n.text
	if n.phrase != nil {
		term = n.phrase[0]
	}
	return true, stats.bm25(doc, term, weighted)
}

// Stats are the corpus figures BM25 scoring needs; df is a stem's
// document frequency.
type Stats struct {
	Docs   int
	AvgLen float64
	total  int
	df     func(stem string) int
}

func (s *Stats) add(d *Document, sign int) {
	s.Docs += sign
	s.total += sign * int(d.length)
	s.AvgLen = 0
	if s.Docs > 0 {
		s.AvgLen = float64(s.total) / float64(s.Docs)
	}
}

func (s *Stats) bm25(doc *Document, term string, tf float64) float64 {
	const k1, b = 1.2, 0.75
	if s == nil || s.Docs == 0 || s.df == nil {
		return tf
	}
	n := float64(s.df(Stem(term)))
	idf := math.Log(1 + (float64(s.Docs)-n+0.5)/(n+0.5))
	norm := 1.0
	if s.AvgLen > 0 {
		norm = 1 - b + b*float64(doc.length)/s.AvgLen
	}
	return idf * tf * (k1 + 1) / (tf + k1*norm)
}

// jsonText is a JSON document as RETURN or LOAD * shows it.
func jsonText(doc *Document) string { return string(store.AppendJSON(nil, doc.JSON)) }

// jsonOut shows a JSON match: a string as its text, anything else as JSON.
func jsonOut(n *store.JNode) string {
	if n.Kind == store.JString {
		return n.Str
	}
	return string(store.AppendJSON(nil, n))
}

func parseJSONPathCached(s string) (*store.JSONPath, error) { return store.ParseJSONPath(s) }
