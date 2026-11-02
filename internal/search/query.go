package search

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cachestorm/cachestorm/internal/cmath"
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
	vec  []float64 // a VECTOR field's vector
	raw  []string  // full documents and SORTABLE fields
}

func fieldBit(pos int) uint64 { return 1 << (uint(pos) & 63) }

var noField docField

// field returns a schema field of the document; a field FT.ALTER added
// after the document was indexed is empty.
func (d *Document) field(pos int) *docField {
	if pos < len(d.fields) {
		return &d.fields[pos]
	}
	return &noField
}

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
	var jsonVecs [][]*store.JNode
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
		jsonVecs = make([][]*store.JNode, len(idx.Fields))
		for i, f := range idx.Fields {
			for _, m := range f.path.Eval(root) {
				if f.Type == FieldVector {
					jsonVecs[i] = append(jsonVecs[i], m.Node)
					continue
				}
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
			// A value that is not a number fails the whole document, which
			// RediSearch then leaves out of the index.
			for _, raw := range values[i] {
				n, err := parseIndexNumber(raw)
				if err != nil {
					return nil, false
				}
				df.nums = append(df.nums, n)
			}
		case FieldVector:
			if jsonVecs != nil {
				for _, n := range jsonVecs[i] {
					vec, ok := f.Vector.jsonVector(n)
					if !ok {
						return nil, false
					}
					df.vec = vec
				}
				continue
			}
			for _, raw := range values[i] {
				vec, ok := f.Vector.decode([]byte(raw))
				if !ok {
					return nil, false
				}
				df.vec = vec
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
					for _, g := range idx.synonymGroups(t) {
						doc.addTerm(g, w, bit)
					}
				}
			}
		}
	}
	return doc, true
}

// parseIndexNumber reads a NUMERIC field's value as RediSearch does.
func parseIndexNumber(s string) (float64, error) {
	if strings.HasPrefix(strings.TrimLeft(s, "+-"), "0x") || strings.HasPrefix(strings.TrimLeft(s, "+-"), "0X") || strings.Contains(s, "_") {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseFloat(s, 64)
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

// ParseQuery parses a RediSearch query against idx: in the default
// dialect with RediSearch's own grammar, then adds the stems of its terms.
func ParseQuery(idx *Index, q string, opts QueryOptions) (Node, error) {
	var n Node
	var err error
	if opts.Dialect <= 1 {
		n, err = parseDialect1(idx, q, opts)
	} else {
		n, err = parseDialect2(idx, q, opts)
		if err == nil && n != nil && len(opts.InFields) > 0 {
			var mask uint64
			for _, f := range opts.InFields {
				mask |= idx.textFieldBit(f)
			}
			n.setMask(mask)
		}
	}
	if err != nil {
		return nil, err
	}
	if err := idx.validate(n); err != nil {
		return nil, err
	}
	if !opts.Verbatim {
		idx.expand(&n)
	}
	return n, nil
}

// validate checks what RediSearch checks of a parsed query.
func (idx *Index) validate(n Node) error {
	if n == nil {
		return nil
	}
	if n.typ == qNumeric && n.lo > n.hi {
		return fmt.Errorf("Invalid numeric range (min > max): @%s:[%s %s]", n.field, cfmtF(n.lo), cfmtF(n.hi)) //nolint:staticcheck // RediSearch's reply text
	}
	if n.typ == qVector {
		if f := idx.Field(n.field); f != nil && f.Vector != nil {
			if err := checkVectorAttrs(f.Vector, n); err != nil {
				return err
			}
		}
	}
	for _, c := range n.children {
		if err := idx.validate(c); err != nil {
			return err
		}
	}
	return nil
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
	case qVector:
		f := idx.Field(n.field)
		if f == nil || f.Vector == nil || stats.vectors == nil {
			return false, 0
		}
		vec := doc.field(f.pos).vec
		return vec != nil && f.Vector.distance(vec, stats.vectors[n]) <= n.radius, 0
	case qMissing:
		f := idx.Field(n.field)
		if f == nil {
			return false, 0
		}
		df := doc.field(f.pos)
		return len(df.strs) == 0 && len(df.nums) == 0 && df.vec == nil && len(df.raw) == 0, 0
	case qPrefix, qFuzzy, qWildcardQuery:
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
		for _, v := range doc.field(f.pos).nums {
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
		coords := doc.field(f.pos).nums
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
	// vectors are the query vectors of a query's vector ranges.
	vectors map[*qnode][]float64
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
	return logF32(1 + (float32(total-n)+0.5)/(float32(n)+0.5))
}

// logF32 is C's log of a single-precision argument, cached: the IDF
// arguments repeat from query to query.
var (
	logCache     sync.Map
	logCacheSize atomic.Int64
)

func logF32(x float32) float64 {
	if v, ok := logCache.Load(x); ok {
		return v.(float64)
	}
	v := cmath.Log(float64(x))
	if logCacheSize.Add(1) > 1<<16 {
		logCache.Clear()
		logCacheSize.Store(1)
	}
	logCache.Store(x, v)
	return v
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
