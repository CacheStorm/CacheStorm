package vectorset

// A port of Redis's fastjson.c: it seeks one top-level field of a JSON object
// without validating the rest of the document, then turns that field's value
// into an expression value. Nested objects, and arrays holding them, are not
// convertible and make the lookup fail.

const jsonMaxDepth = 1000

func jsonIsNumberChar(c byte) bool {
	return isDigitC(c) || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E'
}

func jsonSkipSpaces(b []byte, p int) int {
	for p < len(b) && isSpaceC(b[p]) {
		p++
	}
	return p
}

func jsonSkipString(b []byte, p int) (int, bool) {
	if p >= len(b) || b[p] != '"' {
		return p, false
	}
	p++
	for p < len(b) {
		if b[p] == '\\' {
			p += 2
			continue
		}
		if b[p] == '"' {
			return p + 1, true
		}
		p++
	}
	return p, false
}

// jsonSkipBracketed only counts the bracket kind it started with, as Redis
// does, so mismatched brackets of the other kind are skipped over.
func jsonSkipBracketed(b []byte, p int, opener, closer byte) (int, bool) {
	depth := 1
	p++
	for p < len(b) && depth > 0 {
		c := b[p]
		if c == '"' {
			var ok bool
			if p, ok = jsonSkipString(b, p); !ok {
				return p, false
			}
			continue
		}
		switch c {
		case opener:
			depth++
		case closer:
			depth--
		}
		p++
	}
	return p, depth == 0
}

func jsonSkipLiteral(b []byte, p int, lit string) (int, bool) {
	if p+len(lit) > len(b) || string(b[p:p+len(lit)]) != lit {
		return p, false
	}
	return p + len(lit), true
}

func jsonSkipValue(b []byte, p int) (int, bool) {
	p = jsonSkipSpaces(b, p)
	if p >= len(b) {
		return p, false
	}
	switch b[p] {
	case '"':
		return jsonSkipString(b, p)
	case '{':
		return jsonSkipBracketed(b, p, '{', '}')
	case '[':
		return jsonSkipBracketed(b, p, '[', ']')
	case 't':
		return jsonSkipLiteral(b, p, "true")
	case 'f':
		return jsonSkipLiteral(b, p, "false")
	case 'n':
		return jsonSkipLiteral(b, p, "null")
	}
	start := p
	for p < len(b) && jsonIsNumberChar(b[p]) {
		p++
	}
	return p, p > start
}

// jsonParseString decodes only \n \r \t \\ \"; any other escaped byte is kept
// without its backslash (so A becomes "u0041").
func jsonParseString(b []byte, p int) (exprValue, int, bool) {
	if p >= len(b) || b[p] != '"' {
		return exprValue{}, p, false
	}
	start := p + 1
	q := start
	esc, hasEsc := false, false
	for q < len(b) {
		if esc {
			esc = false
			hasEsc = true
			q++
			continue
		}
		if b[q] == '\\' {
			esc = true
			q++
			continue
		}
		if b[q] == '"' {
			break
		}
		q++
	}
	if q >= len(b) {
		return exprValue{}, p, false
	}
	if !hasEsc {
		return exprValue{kind: kindStr, str: b[start:q:q]}, q + 1, true
	}
	out := make([]byte, 0, q-start)
	esc = false
	for r := start; r < q; r++ {
		c := b[r]
		if esc {
			switch c {
			case 'n':
				c = '\n'
			case 'r':
				c = '\r'
			case 't':
				c = '\t'
			}
			out = append(out, c)
			esc = false
			continue
		}
		if c == '\\' {
			esc = true
			continue
		}
		out = append(out, c)
	}
	return exprValue{kind: kindStr, str: out}, q + 1, true
}

// jsonParseNumber reads at most 255 number characters, like the fixed buffer
// in Redis; the remainder of a longer number is left unread.
func jsonParseNumber(b []byte, p int) (exprValue, int, bool) {
	start := p
	for p < len(b) && p-start < 255 && jsonIsNumberChar(b[p]) {
		p++
	}
	if p == start {
		return exprValue{}, start, false
	}
	v, n := strtod(b[start:p])
	if n != p-start {
		return exprValue{}, start, false
	}
	return exprValue{kind: kindNum, num: v}, p, true
}

func jsonParseLiteral(b []byte, p int, lit string, v exprValue) (exprValue, int, bool) {
	l := len(lit)
	if p+l > len(b) || string(b[p:p+l]) != lit {
		return exprValue{}, p, false
	}
	if p+l < len(b) {
		c := b[p+l]
		if !isSpaceC(c) && c != ',' && c != ']' && c != '}' {
			return exprValue{}, p, false
		}
	}
	return v, p + l, true
}

func jsonParseArray(b []byte, p int, depth int) (exprValue, int, bool) {
	if p >= len(b) || b[p] != '[' {
		return exprValue{}, p, false
	}
	p = jsonSkipSpaces(b, p+1)
	t := exprValue{kind: kindTuple}
	if p < len(b) && b[p] == ']' {
		return t, p + 1, true
	}
	for {
		ele, np, ok := jsonParseValue(b, p, depth+1)
		if !ok {
			return exprValue{}, np, false
		}
		t.tuple = append(t.tuple, ele)
		p = jsonSkipSpaces(b, np)
		if p >= len(b) {
			return exprValue{}, p, false
		}
		switch b[p] {
		case ',':
			p = jsonSkipSpaces(b, p+1)
		case ']':
			return t, p + 1, true
		default:
			return exprValue{}, p, false
		}
	}
}

func jsonParseValue(b []byte, p int, depth int) (exprValue, int, bool) {
	if depth >= jsonMaxDepth {
		return exprValue{}, p, false
	}
	p = jsonSkipSpaces(b, p)
	if p >= len(b) {
		return exprValue{}, p, false
	}
	switch c := b[p]; c {
	case '"':
		return jsonParseString(b, p)
	case '[':
		return jsonParseArray(b, p, depth)
	case 't':
		return jsonParseLiteral(b, p, "true", exprValue{kind: kindNum, num: 1})
	case 'f':
		return jsonParseLiteral(b, p, "false", exprValue{kind: kindNum, num: 0})
	case 'n':
		return jsonParseLiteral(b, p, "null", exprValue{kind: kindNull})
	default:
		if isDigitC(c) || c == '-' || c == '+' {
			return jsonParseNumber(b, p)
		}
		return exprValue{}, p, false
	}
}

// jsonSeekField returns the offset of the value of the first top-level key
// equal to field, comparing the raw (still escaped) key bytes. Anything after
// that value is never looked at.
func jsonSeekField(b []byte, field string) int {
	p := jsonSkipSpaces(b, 0)
	if p >= len(b) || b[p] != '{' {
		return -1
	}
	p++
	for {
		p = jsonSkipSpaces(b, p)
		if p >= len(b) || b[p] != '"' {
			return -1
		}
		keyEnd, ok := jsonSkipString(b, p)
		if !ok {
			return -1
		}
		match := string(b[p+1:keyEnd-1]) == field
		p = jsonSkipSpaces(b, keyEnd)
		if p >= len(b) || b[p] != ':' {
			return -1
		}
		p = jsonSkipSpaces(b, p+1)
		if p >= len(b) {
			return -1
		}
		if match {
			return p
		}
		if p, ok = jsonSkipValue(b, p); !ok {
			return -1
		}
		p = jsonSkipSpaces(b, p)
		if p >= len(b) || b[p] != ',' {
			return -1
		}
		p++
	}
}

func jsonExtractField(b []byte, field string) (exprValue, bool) {
	p := jsonSeekField(b, field)
	if p < 0 {
		return exprValue{}, false
	}
	v, _, ok := jsonParseValue(b, p, 0)
	return v, ok
}
