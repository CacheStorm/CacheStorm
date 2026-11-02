package search

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Document is a key as an index reads it. The indexed form keeps what
// queries match on: each schema field's tokens, numbers and tags, the
// document's index terms with their weighted frequencies, and the raw value
// of SORTABLE fields. A full document (Content) also holds the hash fields
// in their order or the JSON document, for replies.
type Document struct {
	Key    string
	Fields [][2]string
	JSON   *store.JNode
	full   bool
	fields []docField
	// terms are the document's text index terms, as RediSearch's forward
	// index has them: each token, and "+stem" for a token whose stem
	// differs, with its frequency weighted by the field and the fields it
	// occurs in.
	terms  map[string]termFreq
	length uint32
	// id orders documents as RediSearch's document ids do: by when the key
	// was last indexed.
	id int64
	// entry and value are what the document was read from, to tell when
	// the key changed since.
	entry *store.Entry
	value store.Value
}

type termFreq struct {
	freq uint32
	mask uint64
}

// docField is one schema field of a document, by position in the schema.
type docField struct {
	strs []string // TEXT tokens in position order, TAG tags
	nums []float64
	raw  []string // full documents and SORTABLE fields
}

func fieldBit(pos int) uint64 { return 1 << (uint(pos) & 63) }

// termWeight is what one occurrence adds to a term's frequency and the
// document's length: the field weight, at least 1.
func termWeight(w float64) uint32 {
	return max(1, uint32(float32(w)))
}

// stemWeight is the same for a stem, which counts a fifth.
func stemWeight(w float64) uint32 {
	return max(1, uint32(float32(float64(float32(w))*0.2)))
}

// minStemLength is the shortest token whose stem is indexed.
const minStemLength = 4

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
	stemming := idx.Language == "english"
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
			if f.NoIndex {
				continue
			}
			bit, w, sw := fieldBit(i), termWeight(f.Weight), stemWeight(f.Weight)
			for _, raw := range values[i] {
				for _, t := range Tokenize(raw) {
					if idx.Stopwords[t] {
						continue
					}
					df.strs = append(df.strs, t)
					doc.addTerm(t, w, bit)
					doc.length += w
					if stemming && !f.NoStem && len(t) >= minStemLength {
						if st := snowballEnglish(t); st != t {
							doc.addTerm("+"+st, sw, bit)
						}
					}
				}
			}
		}
	}
	return doc, true
}

func (d *Document) addTerm(t string, freq uint32, bit uint64) {
	if d.terms == nil {
		d.terms = make(map[string]termFreq)
	}
	tf := d.terms[t]
	tf.freq += freq
	tf.mask |= bit
	d.terms[t] = tf
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

// tokenSeparators are the bytes that split text into tokens, as
// RediSearch's tokenizer has them; a backslash escapes one.
var tokenSeparators = func() (t [256]bool) {
	for _, c := range []byte(" \t,./(){}[]:;~!@#$%^&*-=+|'`\"<>?") {
		t[c] = true
	}
	return t
}()

// Tokenize splits text into lower-case terms, as RediSearch does: at the
// separators, dropping escaping backslashes, unescaped blanks and control
// characters from each token.
func Tokenize(text string) []string {
	var out []string
	for _, sp := range tokenSpans(text) {
		out = append(out, sp.tok)
	}
	return out
}

// span is a token and the offset and text it was read from.
type span struct {
	start int
	raw   string
	tok   string
}

func tokenSpans(text string) []span {
	var out []span
	start, escaped := 0, false
	for i := 0; i <= len(text); i++ {
		if i < len(text) {
			c := text[i]
			if !tokenSeparators[c] || escaped {
				escaped = !escaped && c == '\\'
				continue
			}
		}
		if t := normalizeToken(text[start:i]); t != "" {
			out = append(out, span{start, text[start:i], t})
		}
		start, escaped = i+1, false
	}
	return out
}

func normalizeToken(s string) string {
	var b []byte
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case (c == ' ' || c == '\t') && !escaped || c < 0x20 || c == 0x7f:
		case c == '\\' && !escaped:
			escaped = true
			continue
		default:
			b = append(b, c)
		}
		escaped = false
	}
	return strings.ToLower(string(b))
}

// QueryOptions change how a query is read.
type QueryOptions struct {
	Dialect     int
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
	// sawStopword is set once a stop word was dropped, which may leave a
	// group empty.
	sawStopword bool
}

// ParseQuery parses a RediSearch query against idx: in the default
// dialect with RediSearch's own grammar, then adds the stems of its terms.
func ParseQuery(idx *Index, q string, opts QueryOptions) (Node, error) {
	var n Node
	var err error
	if opts.Dialect <= 1 {
		n, err = parseDialect1(idx, q, opts)
	} else {
		n, err = parseLegacy(idx, q, opts)
	}
	if err != nil {
		return nil, err
	}
	if err := validate(n); err != nil {
		return nil, err
	}
	if !opts.Verbatim {
		idx.expand(&n)
	}
	return n, nil
}

// validate checks what RediSearch checks of a parsed query.
func validate(n Node) error {
	if n == nil {
		return nil
	}
	if n.typ == qNumeric && n.lo > n.hi {
		return fmt.Errorf("Invalid numeric range (min > max): @%s:[%s %s]", n.field, cfmtF(n.lo), cfmtF(n.hi)) //nolint:staticcheck // RediSearch's reply text
	}
	for _, c := range n.children {
		if err := validate(c); err != nil {
			return err
		}
	}
	return nil
}

// parseLegacy parses the later dialects: a hand-written reading of the
// query syntax.
func parseLegacy(idx *Index, q string, opts QueryOptions) (Node, error) {
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
		if depth > 0 && !p.sawStopword {
			return nil, p.fail()
		}
		return nil, nil
	case 1:
		return children[0], nil
	}
	return newNode(qPhrase).add(children...), nil
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
	return newNode(qUnion).add(kept...), nil
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
		return newNode(qNot).add(n), nil
	case '~':
		p.pos++
		n, err := p.unary(depth)
		if err != nil || n == nil {
			return nil, err
		}
		return newNode(qOptional).add(n), nil
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
		spans := tokenSpans(p.s[p.pos : p.pos+end])
		// In the default dialect a phrase is a term, or two terms and
		// then any words: a stop word before the second term is an error.
		if !p.opts.NoStopwords {
			for i := 0; i < len(spans) && i < 2; i++ {
				if p.idx.Stopwords[spans[i].tok] {
					return nil, &ErrSyntax{Offset: p.pos + spans[i].start, Near: spans[i].raw}
				}
			}
		}
		var words []string
		for _, sp := range spans {
			words = append(words, sp.tok)
		}
		words = p.terms(words)
		p.pos += end + 1
		if len(words) == 0 {
			return nil, nil
		}
		ph := newNode(qPhrase)
		ph.exact, ph.verbatim = true, true
		for _, w := range words {
			ph.add(tokenNode(w))
		}
		p.restrict(ph, fields)
		return ph, nil
	case '*':
		p.pos++
		return newNode(qWildcard), nil
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
		p.sawStopword = true
		return nil, nil
	}
	var n *qnode
	switch {
	case len(toks) > 1:
		n = newNode(qPhrase)
		n.exact, n.verbatim = true, true
		for _, t := range toks {
			n.add(tokenNode(t))
		}
	case prefix:
		n = newNode(qPrefix)
		n.str, n.prefix = toks[0], true
	default:
		n = tokenNode(toks[0])
	}
	p.restrict(n, fields)
	return n, nil
}

func (p *queryParser) scope(fields []string) []string {
	if fields != nil {
		return fields
	}
	return p.opts.InFields
}

// restrict limits n to the fields that scope it.
func (p *queryParser) restrict(n *qnode, fields []string) {
	scope := p.scope(fields)
	if scope == nil {
		return
	}
	var mask uint64
	for _, f := range scope {
		mask |= p.idx.textFieldBit(f)
	}
	n.setMask(mask)
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
		if p.idx.Field(names[0]) == nil || len(values) == 0 {
			return nil, nil
		}
		tn := newNode(qTag)
		tn.field = names[0]
		for _, v := range values {
			if strings.HasSuffix(v, "*") {
				pn := newNode(qPrefix)
				pn.str, pn.prefix = strings.TrimSuffix(v, "*"), true
				tn.add(pn)
			} else {
				tn.add(tokenNode(v))
			}
		}
		return tn, nil
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
	n := newNode(qNumeric)
	n.field, n.lo, n.hi, n.loIncl, n.hiIncl = field, lo, hi, !loExcl, !hiExcl
	return n, nil
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
	if err1 != nil || err2 != nil || err3 != nil {
		return nil, errGeoUnit
	}
	return geoNode(field, lon, lat, r, parts[3])
}

func geoNode(field string, lon, lat, radius float64, unit string) (Node, error) {
	if _, ok := geoUnits[strings.ToLower(unit)]; !ok {
		return nil, errGeoUnit
	}
	n := newNode(qGeo)
	n.field, n.lon, n.lat, n.radius, n.unit = field, lon, lat, radius, strings.ToLower(unit)
	return n, nil
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
func And(nodes ...Node) Node {
	for _, n := range nodes {
		if n == nil {
			return nil
		}
	}
	return newNode(qPhrase).add(nodes...)
}

// Match reports whether doc satisfies n and the score it earns there:
// RediSearch's BM25STD, a sum over the index terms the document matched.
func (idx *Index) Match(n Node, doc *Document, opts QueryOptions, stats *Stats) (bool, float64) {
	if n == nil {
		return false, 0
	}
	switch n.typ {
	case qWildcard:
		return true, stats.bm25(1, 1, doc) * n.weight
	case qPhrase:
		total := 0.0
		for _, c := range n.children {
			ok, s := idx.Match(c, doc, opts, stats)
			if !ok {
				return false, 0
			}
			total += s
		}
		if n.exact && !idx.exactPhrase(n, doc) {
			return false, 0
		}
		return true, total * n.weight
	case qUnion:
		matched, total := false, 0.0
		for _, c := range n.children {
			if ok, s := idx.Match(c, doc, opts, stats); ok {
				matched, total = true, total+s
			}
		}
		return matched, total * n.weight
	case qToken:
		tf, ok := doc.terms[n.str]
		if !ok || tf.mask&n.mask == 0 {
			return false, 0
		}
		return true, stats.termScore(n.str, tf.freq, doc) * n.weight
	case qPrefix, qFuzzy:
		// A union that stops at its first term the document holds.
		for _, t := range n.terms {
			if tf, ok := doc.terms[t]; ok && tf.mask&n.mask != 0 {
				return true, stats.termScore(t, tf.freq, doc) * n.weight
			}
		}
		return false, 0
	case qNot:
		ok, _ := idx.Match(n.children[0], doc, opts, stats)
		return !ok, 0
	case qOptional:
		if ok, s := idx.Match(n.children[0], doc, opts, stats); ok {
			return true, s * n.weight
		}
		return true, 0
	case qNumeric:
		f := idx.Field(n.field)
		if f == nil || f.Type != FieldNumeric {
			return false, 0
		}
		for _, v := range doc.fields[f.pos].nums {
			if (v > n.lo || v == n.lo && n.loIncl) && (v < n.hi || v == n.hi && n.hiIncl) {
				return true, 0
			}
		}
		return false, 0
	case qTag:
		return idx.matchTag(n, doc, stats)
	case qGeo:
		f := idx.Field(n.field)
		if f == nil || f.Type != FieldGeo {
			return false, 0
		}
		km := n.radius * geoUnits[n.unit]
		coords := doc.fields[f.pos].nums
		for i := 0; i+1 < len(coords); i += 2 {
			if haversineKm(n.lon, n.lat, coords[i], coords[i+1]) <= km {
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

// Stats are the corpus figures BM25 scoring needs: the number of documents
// and their total length, and an index term's or a tag's document count.
type Stats struct {
	Docs     int
	total    uint64
	termDocs func(term string) int
	tagDocs  func(pos int, tag string) int
	// score is the documents' score, the index's default score.
	score float64
}

func (s *Stats) add(d *Document, sign int) {
	s.Docs += sign
	if sign > 0 {
		s.total += uint64(d.length)
	} else {
		s.total -= uint64(d.length)
	}
}

func (s *Stats) avgLen() float64 {
	if s.Docs == 0 {
		return 0
	}
	return float64(s.total) / float64(s.Docs)
}

// idf is BM25's inverse document frequency of a term n documents hold,
// computed in single precision as RediSearch does.
func (s *Stats) idf(n int) float64 {
	total := max(s.Docs, n)
	return math.Log(float64(1 + (float32(total-n)+0.5)/(float32(n)+0.5)))
}

func (s *Stats) termScore(term string, freq uint32, doc *Document) float64 {
	return s.bm25(s.idf(s.termDocs(term)), float64(freq), doc)
}

// bm25 is one term's BM25STD score, with RediSearch's single-precision
// constants (k1 1.2, b 0.75) and the document score applied.
func (s *Stats) bm25(idf, f float64, doc *Document) float64 {
	const k1, b = float32(1.2), float32(0.75)
	norm := float64(1-b) + float64(b*float32(doc.length))/s.avgLen()
	return s.score * (idf * f * float64(k1+1) / (f + float64(k1)*norm))
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
