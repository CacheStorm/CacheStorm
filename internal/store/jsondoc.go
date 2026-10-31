package store

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// JKind is the kind of a JSON value. Numbers keep RedisJSON's distinction:
// an integer (signed, or unsigned above the int64 range) prints without a
// fraction, a float always with one.
type JKind uint8

const (
	JNull JKind = iota
	JBool
	JInt
	JUint
	JFloat
	JString
	JArray
	JObject
)

// JNode is one value of a JSON document. Objects keep their keys in
// insertion order; removing a key moves the last key into its place, as
// RedisJSON's maps do.
type JNode struct {
	Kind  JKind
	Bool  bool
	Int   int64
	Uint  uint64
	Float float64
	Str   string
	Arr   []*JNode
	Keys  []string
	Vals  []*JNode
}

// TypeName is the name JSON.TYPE reports.
func (n *JNode) TypeName() string {
	switch n.Kind {
	case JNull:
		return "null"
	case JBool:
		return "boolean"
	case JInt, JUint:
		return "integer"
	case JFloat:
		return "number"
	case JString:
		return "string"
	case JArray:
		return "array"
	default:
		return "object"
	}
}

// IsNumber reports whether n is an integer or a float.
func (n *JNode) IsNumber() bool { return n.Kind == JInt || n.Kind == JUint || n.Kind == JFloat }

// Number returns n's numeric value as a float.
func (n *JNode) Number() float64 {
	switch n.Kind {
	case JInt:
		return float64(n.Int)
	case JUint:
		return float64(n.Uint)
	}
	return n.Float
}

// Index returns the position of key in an object, or -1.
func (n *JNode) Index(key string) int {
	for i, k := range n.Keys {
		if k == key {
			return i
		}
	}
	return -1
}

// Get returns the value of key in an object.
func (n *JNode) Get(key string) *JNode {
	if i := n.Index(key); i >= 0 {
		return n.Vals[i]
	}
	return nil
}

// Put sets key in an object, appending it when new.
func (n *JNode) Put(key string, v *JNode) {
	if i := n.Index(key); i >= 0 {
		n.Vals[i] = v
		return
	}
	n.Keys = append(n.Keys, key)
	n.Vals = append(n.Vals, v)
}

// Remove deletes key from an object, moving the last key into its place.
func (n *JNode) Remove(key string) bool {
	i := n.Index(key)
	if i < 0 {
		return false
	}
	last := len(n.Keys) - 1
	n.Keys[i], n.Vals[i] = n.Keys[last], n.Vals[last]
	n.Keys, n.Vals = n.Keys[:last], n.Vals[:last]
	return true
}

func (n *JNode) Clone() *JNode {
	c := *n
	if n.Arr != nil {
		c.Arr = make([]*JNode, len(n.Arr))
		for i, v := range n.Arr {
			c.Arr[i] = v.Clone()
		}
	}
	if n.Keys != nil {
		c.Keys = append([]string(nil), n.Keys...)
		c.Vals = make([]*JNode, len(n.Vals))
		for i, v := range n.Vals {
			c.Vals[i] = v.Clone()
		}
	}
	return &c
}

// Equal reports whether a and b are the same JSON value; numbers compare
// by value.
func (a *JNode) Equal(b *JNode) bool {
	if a.IsNumber() && b.IsNumber() {
		if a.Kind == JFloat || b.Kind == JFloat {
			return a.Number() == b.Number()
		}
		if a.Kind == b.Kind {
			return a.Int == b.Int && a.Uint == b.Uint
		}
		return false
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case JBool:
		return a.Bool == b.Bool
	case JString:
		return a.Str == b.Str
	case JArray:
		if len(a.Arr) != len(b.Arr) {
			return false
		}
		for i := range a.Arr {
			if !a.Arr[i].Equal(b.Arr[i]) {
				return false
			}
		}
	case JObject:
		if len(a.Keys) != len(b.Keys) {
			return false
		}
		for i, k := range a.Keys {
			other := b.Get(k)
			if other == nil || !a.Vals[i].Equal(other) {
				return false
			}
		}
	}
	return true
}

// MemorySize estimates n's memory as JSON.DEBUG MEMORY reports it.
func (n *JNode) MemorySize() int64 {
	switch n.Kind {
	case JString:
		return 24 + int64(len(n.Str))
	case JArray:
		size := int64(24)
		for _, v := range n.Arr {
			size += v.MemorySize()
		}
		return size
	case JObject:
		size := int64(48)
		for i, k := range n.Keys {
			size += 32 + int64(len(k)) + n.Vals[i].MemorySize()
		}
		return size
	}
	return 8
}

// JSONMaxDepth is how deeply values may nest, serde_json's recursion limit.
const JSONMaxDepth = 128

// JSONSyntaxError is a parse error in serde_json's words, which RedisJSON
// replies with.
type JSONSyntaxError struct {
	Msg          string
	Line, Column int
}

func (e *JSONSyntaxError) Error() string {
	return fmt.Sprintf("%s at line %d column %d", e.Msg, e.Line, e.Column)
}

type jsonParser struct {
	s     string
	pos   int
	depth int
}

// ParseJSON parses one JSON value.
func ParseJSON(s string) (*JNode, error) {
	p := &jsonParser{s: s}
	p.skipSpace()
	n, err := p.value()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos < len(p.s) {
		return nil, p.errAt("trailing characters", p.pos+1)
	}
	return n, nil
}

// errAt reports msg at the 1-based byte offset at, as a line and column.
func (p *jsonParser) errAt(msg string, at int) error {
	line, col := 1, 0
	for i := 0; i < at && i < len(p.s); i++ {
		if p.s[i] == '\n' {
			line, col = line+1, 0
		} else {
			col++
		}
	}
	return &JSONSyntaxError{Msg: msg, Line: line, Column: col}
}

// eof reports an input that ended inside what, at its last character.
func (p *jsonParser) eof(what string) error {
	return p.errAt("EOF while parsing "+what, len(p.s))
}

func (p *jsonParser) skipSpace() {
	for p.pos < len(p.s) {
		switch p.s[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *jsonParser) value() (*JNode, error) {
	if p.pos >= len(p.s) {
		return nil, p.eof("a value")
	}
	switch c := p.s[p.pos]; {
	case c == '{':
		return p.object()
	case c == '[':
		return p.array()
	case c == '"':
		s, err := p.str()
		if err != nil {
			return nil, err
		}
		return &JNode{Kind: JString, Str: s}, nil
	case c == 't':
		return p.ident("true", &JNode{Kind: JBool, Bool: true})
	case c == 'f':
		return p.ident("false", &JNode{Kind: JBool})
	case c == 'n':
		return p.ident("null", &JNode{Kind: JNull})
	case c == '-' || c >= '0' && c <= '9':
		return p.number()
	default:
		return nil, p.errAt("expected value", p.pos+1)
	}
}

func (p *jsonParser) ident(word string, n *JNode) (*JNode, error) {
	for i := 0; i < len(word); i++ {
		if p.pos >= len(p.s) {
			return nil, p.eof("a value")
		}
		if p.s[p.pos] != word[i] {
			return nil, p.errAt("expected ident", p.pos+1)
		}
		p.pos++
	}
	return n, nil
}

func (p *jsonParser) nest() error {
	p.depth++
	if p.depth > JSONMaxDepth {
		return p.errAt("recursion limit exceeded", p.pos+1)
	}
	return nil
}

func (p *jsonParser) array() (*JNode, error) {
	if err := p.nest(); err != nil {
		return nil, err
	}
	defer func() { p.depth-- }()
	p.pos++
	n := &JNode{Kind: JArray, Arr: []*JNode{}}
	p.skipSpace()
	if p.pos < len(p.s) && p.s[p.pos] == ']' {
		p.pos++
		return n, nil
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.s) {
			return nil, p.eof("a list")
		}
		if p.s[p.pos] == ']' {
			return nil, p.errAt("trailing comma", p.pos+1)
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		n.Arr = append(n.Arr, v)
		p.skipSpace()
		if p.pos >= len(p.s) {
			return nil, p.eof("a list")
		}
		switch p.s[p.pos] {
		case ',':
			p.pos++
		case ']':
			p.pos++
			return n, nil
		default:
			return nil, p.errAt("expected `,` or `]`", p.pos+1)
		}
	}
}

func (p *jsonParser) object() (*JNode, error) {
	if err := p.nest(); err != nil {
		return nil, err
	}
	defer func() { p.depth-- }()
	p.pos++
	n := &JNode{Kind: JObject, Keys: []string{}, Vals: []*JNode{}}
	p.skipSpace()
	if p.pos < len(p.s) && p.s[p.pos] == '}' {
		p.pos++
		return n, nil
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.s) {
			return nil, p.eof("an object")
		}
		switch p.s[p.pos] {
		case '"':
		case '}':
			return nil, p.errAt("trailing comma", p.pos+1)
		default:
			return nil, p.errAt("key must be a string", p.pos+1)
		}
		key, err := p.str()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.pos >= len(p.s) {
			return nil, p.eof("an object")
		}
		if p.s[p.pos] != ':' {
			return nil, p.errAt("expected `:`", p.pos+1)
		}
		p.pos++
		p.skipSpace()
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		n.Put(key, v)
		p.skipSpace()
		if p.pos >= len(p.s) {
			return nil, p.eof("an object")
		}
		switch p.s[p.pos] {
		case ',':
			p.pos++
		case '}':
			p.pos++
			return n, nil
		default:
			return nil, p.errAt("expected `,` or `}`", p.pos+1)
		}
	}
}

func (p *jsonParser) str() (string, error) {
	p.pos++
	var b strings.Builder
	start := p.pos
	for {
		if p.pos >= len(p.s) {
			return "", p.eof("a string")
		}
		c := p.s[p.pos]
		switch {
		case c == '"':
			b.WriteString(p.s[start:p.pos])
			p.pos++
			return b.String(), nil
		case c < 0x20:
			return "", p.errAt("control character (\\u0000-\\u001F) found while parsing a string", p.pos+1)
		case c == '\\':
			b.WriteString(p.s[start:p.pos])
			p.pos++
			if p.pos >= len(p.s) {
				return "", p.eof("a string")
			}
			switch e := p.s[p.pos]; e {
			case '"', '\\', '/':
				b.WriteByte(e)
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				r, err := p.unicodeEscape()
				if err != nil {
					return "", err
				}
				b.WriteRune(r)
				start = p.pos
				continue
			default:
				return "", p.errAt("invalid escape", p.pos+1)
			}
			p.pos++
			start = p.pos
		default:
			p.pos++
		}
	}
}

// unicodeEscape reads the hex digits after \u, and a low surrogate after a
// high one; p.pos is at the u.
func (p *jsonParser) unicodeEscape() (rune, error) {
	hex := func() (rune, error) {
		if p.pos+4 >= len(p.s) {
			p.pos = len(p.s)
			return 0, p.eof("a string")
		}
		v, err := strconv.ParseUint(p.s[p.pos+1:p.pos+5], 16, 16)
		if err != nil {
			return 0, p.errAt("invalid escape", p.pos+2)
		}
		p.pos += 5
		return rune(v), nil
	}
	r, err := hex()
	if err != nil {
		return 0, err
	}
	switch {
	case r >= 0xDC00 && r <= 0xDFFF:
		return 0, p.errAt("lone leading surrogate in hex escape", p.pos)
	case r >= 0xD800 && r <= 0xDBFF:
		if p.pos+1 >= len(p.s) || p.s[p.pos] != '\\' || p.s[p.pos+1] != 'u' {
			return 0, p.errAt("unexpected end of hex escape", p.pos)
		}
		p.pos++
		low, err := hex()
		if err != nil {
			return 0, err
		}
		if low < 0xDC00 || low > 0xDFFF {
			return 0, p.errAt("lone leading surrogate in hex escape", p.pos)
		}
		return 0x10000 + (r-0xD800)<<10 + (low - 0xDC00), nil
	}
	return r, nil
}

func (p *jsonParser) number() (*JNode, error) {
	start := p.pos
	if p.s[p.pos] == '-' {
		p.pos++
	}
	digits := func() int {
		n := 0
		for p.pos < len(p.s) && p.s[p.pos] >= '0' && p.s[p.pos] <= '9' {
			p.pos++
			n++
		}
		return n
	}
	if p.pos >= len(p.s) {
		return nil, p.eof("a value")
	}
	if p.s[p.pos] == '0' {
		p.pos++
		if p.pos < len(p.s) && p.s[p.pos] >= '0' && p.s[p.pos] <= '9' {
			return nil, p.errAt("invalid number", p.pos+1)
		}
	} else if digits() == 0 {
		return nil, p.errAt("invalid number", p.pos+1)
	}
	isFloat := false
	if p.pos < len(p.s) && p.s[p.pos] == '.' {
		isFloat = true
		p.pos++
		if p.pos >= len(p.s) {
			return nil, p.eof("a value")
		}
		if digits() == 0 {
			return nil, p.errAt("invalid number", p.pos+1)
		}
	}
	if p.pos < len(p.s) && (p.s[p.pos] == 'e' || p.s[p.pos] == 'E') {
		isFloat = true
		p.pos++
		if p.pos < len(p.s) && (p.s[p.pos] == '+' || p.s[p.pos] == '-') {
			p.pos++
		}
		if p.pos >= len(p.s) {
			return nil, p.eof("a value")
		}
		if digits() == 0 {
			return nil, p.errAt("invalid number", p.pos+1)
		}
	}
	text := p.s[start:p.pos]
	if !isFloat {
		if i, err := strconv.ParseInt(text, 10, 64); err == nil {
			return &JNode{Kind: JInt, Int: i}, nil
		}
		if u, err := strconv.ParseUint(text, 10, 64); err == nil {
			return &JNode{Kind: JUint, Uint: u}, nil
		}
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil && !isFloat {
		// An integer beyond 64 bits becomes a float, as in serde_json.
		err = nil
	}
	if err != nil || math.IsInf(f, 0) {
		return nil, p.errAt("number out of range", p.pos)
	}
	return &JNode{Kind: JFloat, Float: f}, nil
}

// FormatJSONFloat prints f as serde_json does (ryu): plain decimal notation
// with at least one fractional digit for magnitudes in [1e-5, 1e16), the
// shortest exponent notation otherwise.
func FormatJSONFloat(f float64) string {
	if f == 0 {
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}
	if abs := math.Abs(f); abs >= 1e-5 && abs < 1e16 {
		s := strconv.FormatFloat(f, 'f', -1, 64)
		if !strings.ContainsRune(s, '.') {
			s += ".0"
		}
		return s
	}
	s := strconv.FormatFloat(f, 'e', -1, 64)
	mant, exp, _ := strings.Cut(s, "e")
	e, err := strconv.Atoi(exp)
	if err != nil {
		return s
	}
	return mant + "e" + strconv.Itoa(e)
}

// JSONFormat holds JSON.GET's INDENT, NEWLINE and SPACE strings.
type JSONFormat struct {
	Indent, Newline, Space string
}

// AppendJSON serializes n compactly, as RedisJSON does.
func AppendJSON(dst []byte, n *JNode) []byte {
	return appendJSONNode(dst, n, nil, 0)
}

// AppendJSONFormatted serializes n with f's layout.
func AppendJSONFormatted(dst []byte, n *JNode, f *JSONFormat) []byte {
	return appendJSONNode(dst, n, f, 0)
}

func appendJSONNode(dst []byte, n *JNode, f *JSONFormat, depth int) []byte {
	newline := func(d int) {
		if f == nil {
			return
		}
		dst = append(dst, f.Newline...)
		for i := 0; i < d; i++ {
			dst = append(dst, f.Indent...)
		}
	}
	switch n.Kind {
	case JNull:
		return append(dst, "null"...)
	case JBool:
		return strconv.AppendBool(dst, n.Bool)
	case JInt:
		return strconv.AppendInt(dst, n.Int, 10)
	case JUint:
		return strconv.AppendUint(dst, n.Uint, 10)
	case JFloat:
		return append(dst, FormatJSONFloat(n.Float)...)
	case JString:
		return AppendJSONString(dst, n.Str)
	case JArray:
		dst = append(dst, '[')
		for i, v := range n.Arr {
			if i > 0 {
				dst = append(dst, ',')
			}
			newline(depth + 1)
			dst = appendJSONNode(dst, v, f, depth+1)
		}
		if len(n.Arr) > 0 {
			newline(depth)
		}
		return append(dst, ']')
	default:
		dst = append(dst, '{')
		for i, k := range n.Keys {
			if i > 0 {
				dst = append(dst, ',')
			}
			newline(depth + 1)
			dst = AppendJSONString(dst, k)
			dst = append(dst, ':')
			if f != nil {
				dst = append(dst, f.Space...)
			}
			dst = appendJSONNode(dst, n.Vals[i], f, depth+1)
		}
		if len(n.Keys) > 0 {
			newline(depth)
		}
		return append(dst, '}')
	}
}

// AppendJSONString quotes s as serde_json does: only the quote, the
// backslash and control characters are escaped.
func AppendJSONString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '"':
			dst = append(dst, '\\', '"')
		case c == '\\':
			dst = append(dst, '\\', '\\')
		case c == '\n':
			dst = append(dst, '\\', 'n')
		case c == '\r':
			dst = append(dst, '\\', 'r')
		case c == '\t':
			dst = append(dst, '\\', 't')
		case c == '\b':
			dst = append(dst, '\\', 'b')
		case c == '\f':
			dst = append(dst, '\\', 'f')
		case c < 0x20:
			dst = append(dst, fmt.Sprintf("\\u%04x", c)...)
		case c < utf8.RuneSelf:
			dst = append(dst, c)
		default:
			_, size := utf8.DecodeRuneInString(s[i:])
			dst = append(dst, s[i:i+size]...)
			i += size
			continue
		}
		i++
	}
	return append(dst, '"')
}

// errJSONNumberOverflow is returned when arithmetic leaves no finite result.
var errJSONNumberOverflow = errors.New("ERR result is not a finite number")

// JSONArith applies JSON.NUMINCRBY (mult false) or JSON.NUMMULTBY to n:
// integers stay integers (wrapping as RedisJSON's i64 arithmetic does),
// anything with a float becomes a float.
func JSONArith(n, by *JNode, mult bool) (*JNode, error) {
	if n.Kind == JInt && by.Kind == JInt {
		if mult {
			return &JNode{Kind: JInt, Int: n.Int * by.Int}, nil
		}
		return &JNode{Kind: JInt, Int: n.Int + by.Int}, nil
	}
	var f float64
	if mult {
		f = n.Number() * by.Number()
	} else {
		f = n.Number() + by.Number()
	}
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return nil, errJSONNumberOverflow
	}
	return &JNode{Kind: JFloat, Float: f}, nil
}
