package graph

import (
	"fmt"
	"strings"
	"unicode"
)

type tokKind int

const (
	tokEOF tokKind = iota
	tokIdent
	tokString
	tokNumber
	tokPunct
)

type token struct {
	kind tokKind
	text string
	pos  int
}

// keyword reports whether t is the keyword kw, in any case.
func (t token) keyword(kw string) bool {
	return t.kind == tokIdent && strings.EqualFold(t.text, kw)
}

func (t token) punct(p string) bool { return t.kind == tokPunct && t.text == p }

func lex(src string) ([]token, error) {
	var toks []token
	rs := []rune(src)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case r == '/' && i+1 < len(rs) && rs[i+1] == '/':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
		case unicode.IsLetter(r) || r == '_':
			j := i
			for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j]) || rs[j] == '_') {
				j++
			}
			toks = append(toks, token{tokIdent, string(rs[i:j]), i})
			i = j
		case r == '`':
			j := i + 1
			for j < len(rs) && rs[j] != '`' {
				j++
			}
			if j == len(rs) {
				return nil, fmt.Errorf("unterminated identifier at offset %d", i)
			}
			toks = append(toks, token{tokIdent, string(rs[i+1 : j]), i})
			i = j + 1
		case unicode.IsDigit(r) || r == '.' && i+1 < len(rs) && unicode.IsDigit(rs[i+1]):
			j := i
			for j < len(rs) && unicode.IsDigit(rs[j]) {
				j++
			}
			if j+1 < len(rs) && rs[j] == '.' && unicode.IsDigit(rs[j+1]) {
				j++
				for j < len(rs) && unicode.IsDigit(rs[j]) {
					j++
				}
			}
			if j < len(rs) && (rs[j] == 'e' || rs[j] == 'E') {
				k := j + 1
				if k < len(rs) && (rs[k] == '+' || rs[k] == '-') {
					k++
				}
				if k < len(rs) && unicode.IsDigit(rs[k]) {
					for k < len(rs) && unicode.IsDigit(rs[k]) {
						k++
					}
					j = k
				}
			}
			toks = append(toks, token{tokNumber, string(rs[i:j]), i})
			i = j
		case r == '\'' || r == '"':
			var b strings.Builder
			j := i + 1
			for ; j < len(rs) && rs[j] != r; j++ {
				if rs[j] == '\\' && j+1 < len(rs) {
					j++
					switch rs[j] {
					case 'n':
						b.WriteRune('\n')
					case 't':
						b.WriteRune('\t')
					default:
						b.WriteRune(rs[j])
					}
					continue
				}
				b.WriteRune(rs[j])
			}
			if j == len(rs) {
				return nil, fmt.Errorf("unterminated string at offset %d", i)
			}
			toks = append(toks, token{tokString, b.String(), i})
			i = j + 1
		default:
			two := ""
			if i+1 < len(rs) {
				two = string(rs[i : i+2])
			}
			switch two {
			case "<>", "<=", ">=", "!=", "..":
				toks = append(toks, token{tokPunct, two, i})
				i += 2
				continue
			}
			if !strings.ContainsRune("()[]{}:,.-<>=+*/%|$", r) {
				return nil, fmt.Errorf("unexpected character %q at offset %d", r, i)
			}
			toks = append(toks, token{tokPunct, string(r), i})
			i++
		}
	}
	return append(toks, token{tokEOF, "", len(rs)}), nil
}
