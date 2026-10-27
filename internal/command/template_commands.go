package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/mail"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterTemplateCommands(router *Router) {
	router.Register(&CommandDef{Name: "EVAL.EXPR", Handler: cmdEVALEXPR})
	router.Register(&CommandDef{Name: "EVAL.FORMAT", Handler: cmdEVALFORMAT})
	router.Register(&CommandDef{Name: "EVAL.JSONPATH", Handler: cmdEVALJSONPATH})
	router.Register(&CommandDef{Name: "EVAL.TEMPLATE", Handler: cmdEVALTEMPLATE})
	router.Register(&CommandDef{Name: "EVAL.REGEX", Handler: cmdEVALREGEX})
	router.Register(&CommandDef{Name: "EVAL.REGEXMATCH", Handler: cmdEVALREGEXMATCH})
	router.Register(&CommandDef{Name: "EVAL.REGEXREPLACE", Handler: cmdEVALREGEXREPLACE})

	router.Register(&CommandDef{Name: "VALIDATE.EMAIL", Handler: cmdVALIDATEEMAIL})
	router.Register(&CommandDef{Name: "VALIDATE.URL", Handler: cmdVALIDATEURL})
	router.Register(&CommandDef{Name: "VALIDATE.IP", Handler: cmdVALIDATEIP})
	router.Register(&CommandDef{Name: "VALIDATE.JSON", Handler: cmdVALIDATEJSON})
	router.Register(&CommandDef{Name: "VALIDATE.INT", Handler: cmdVALIDATEINT})
	router.Register(&CommandDef{Name: "VALIDATE.FLOAT", Handler: cmdVALIDATEFLOAT})
	router.Register(&CommandDef{Name: "VALIDATE.ALPHA", Handler: cmdVALIDATEALPHA})
	router.Register(&CommandDef{Name: "VALIDATE.ALPHANUM", Handler: cmdVALIDATEALPHANUM})
	router.Register(&CommandDef{Name: "VALIDATE.LENGTH", Handler: cmdVALIDATELENGTH})
	router.Register(&CommandDef{Name: "VALIDATE.RANGE", Handler: cmdVALIDATERANGE})

	router.Register(&CommandDef{Name: "STR.FORMAT", Handler: cmdSTRFORMAT})
	router.Register(&CommandDef{Name: "STR.TRUNCATE", Handler: cmdSTRTRUNCATE})
	router.Register(&CommandDef{Name: "STR.PADLEFT", Handler: cmdSTRPADLEFT})
	router.Register(&CommandDef{Name: "STR.PADRIGHT", Handler: cmdSTRPADRIGHT})
	router.Register(&CommandDef{Name: "STR.REVERSE", Handler: cmdSTRREVERSE})
	router.Register(&CommandDef{Name: "STR.REPEAT", Handler: cmdSTRREPEAT})
	router.Register(&CommandDef{Name: "STR.SPLIT", Handler: cmdSTRSPLIT})
	router.Register(&CommandDef{Name: "STR.JOIN", Handler: cmdSTRJOIN})
	router.Register(&CommandDef{Name: "STR.CONTAINS", Handler: cmdSTRCONTAINS})
	router.Register(&CommandDef{Name: "STR.STARTSWITH", Handler: cmdSTRSTARTSWITH})
	router.Register(&CommandDef{Name: "STR.ENDSWITH", Handler: cmdSTRENDSWITH})
	router.Register(&CommandDef{Name: "STR.INDEX", Handler: cmdSTRINDEX})
	router.Register(&CommandDef{Name: "STR.LASTINDEX", Handler: cmdSTRLASTINDEX})
	router.Register(&CommandDef{Name: "STR.REPLACE", Handler: cmdSTRREPLACE})
	router.Register(&CommandDef{Name: "STR.TRIM", Handler: cmdSTRTRIM})
	router.Register(&CommandDef{Name: "STR.TRIMLEFT", Handler: cmdSTRTRIMLEFT})
	router.Register(&CommandDef{Name: "STR.TRIMRIGHT", Handler: cmdSTRTRIMRIGHT})
	router.Register(&CommandDef{Name: "STR.TITLE", Handler: cmdSTRTITLE})
	router.Register(&CommandDef{Name: "STR.WORDS", Handler: cmdSTRWORDS})
	router.Register(&CommandDef{Name: "STR.LINES", Handler: cmdSTRLINES})
}

func cmdEVALEXPR(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	result, err := evaluateExpression(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteBulkString(result)
}

var (
	errExprSyntax    = errors.New("ERR invalid expression")
	errExprDivZero   = errors.New("ERR division by zero")
	errExprNonFinite = errors.New("ERR expression result is not a finite number")
	errExprTooDeep   = errors.New("ERR expression nested too deeply")
)

const maxExprDepth = 256

// exprParser evaluates arithmetic over float64 with the grammar
//
//	expr  = term { ("+" | "-") term }
//	term  = unary { ("*" | "/" | "%") unary }
//	unary = ("+" | "-") unary | power
//	power = atom [ "^" unary ]
//	atom  = number | "(" expr ")"
//
// so "^" binds tighter than unary minus and is right-associative
// (-2^2 = -4, 2^3^2 = 512) and "%" is the floating-point remainder.
type exprParser struct {
	s     string
	pos   int
	depth int
}

func evaluateExpression(expr string) (string, error) {
	p := &exprParser{s: expr}
	v, err := p.expr()
	if err != nil {
		return "", err
	}
	p.skipSpace()
	if p.pos != len(p.s) {
		return "", errExprSyntax
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "", errExprNonFinite
	}
	if v == 0 {
		return "0", nil
	}
	if v == math.Trunc(v) && math.Abs(v) < 1e21 {
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	}
	return strconv.FormatFloat(v, 'g', -1, 64), nil
}

func (p *exprParser) skipSpace() {
	for p.pos < len(p.s) && (p.s[p.pos] == ' ' || p.s[p.pos] == '\t') {
		p.pos++
	}
}

func (p *exprParser) peek() byte {
	p.skipSpace()
	if p.pos < len(p.s) {
		return p.s[p.pos]
	}
	return 0
}

func (p *exprParser) expr() (float64, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxExprDepth {
		return 0, errExprTooDeep
	}
	v, err := p.term()
	if err != nil {
		return 0, err
	}
	for {
		switch p.peek() {
		case '+':
			p.pos++
			r, err := p.term()
			if err != nil {
				return 0, err
			}
			v += r
		case '-':
			p.pos++
			r, err := p.term()
			if err != nil {
				return 0, err
			}
			v -= r
		default:
			return v, nil
		}
	}
}

func (p *exprParser) term() (float64, error) {
	v, err := p.unary()
	if err != nil {
		return 0, err
	}
	for {
		op := p.peek()
		if op != '*' && op != '/' && op != '%' {
			return v, nil
		}
		p.pos++
		r, err := p.unary()
		if err != nil {
			return 0, err
		}
		switch op {
		case '*':
			v *= r
		case '/':
			if r == 0 {
				return 0, errExprDivZero
			}
			v /= r
		default:
			if r == 0 {
				return 0, errExprDivZero
			}
			v = math.Mod(v, r)
		}
	}
}

func (p *exprParser) unary() (float64, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxExprDepth {
		return 0, errExprTooDeep
	}
	switch p.peek() {
	case '-':
		p.pos++
		v, err := p.unary()
		return -v, err
	case '+':
		p.pos++
		return p.unary()
	}
	return p.power()
}

func (p *exprParser) power() (float64, error) {
	base, err := p.atom()
	if err != nil {
		return 0, err
	}
	if p.peek() != '^' {
		return base, nil
	}
	p.pos++
	exp, err := p.unary()
	if err != nil {
		return 0, err
	}
	return math.Pow(base, exp), nil
}

func (p *exprParser) atom() (float64, error) {
	c := p.peek()
	if c == '(' {
		p.pos++
		v, err := p.expr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, errExprSyntax
		}
		p.pos++
		return v, nil
	}
	start := p.pos
	for p.pos < len(p.s) && (p.s[p.pos] >= '0' && p.s[p.pos] <= '9' || p.s[p.pos] == '.') {
		p.pos++
	}
	if p.pos < len(p.s) && (p.s[p.pos] == 'e' || p.s[p.pos] == 'E') && p.pos > start {
		save := p.pos
		p.pos++
		if p.pos < len(p.s) && (p.s[p.pos] == '+' || p.s[p.pos] == '-') {
			p.pos++
		}
		digits := p.pos
		for p.pos < len(p.s) && p.s[p.pos] >= '0' && p.s[p.pos] <= '9' {
			p.pos++
		}
		if p.pos == digits {
			p.pos = save
		}
	}
	if p.pos == start {
		return 0, errExprSyntax
	}
	v, err := strconv.ParseFloat(p.s[start:p.pos], 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return 0, errExprNonFinite
		}
		return 0, errExprSyntax
	}
	return v, nil
}

func cmdEVALFORMAT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	format := ctx.ArgString(0)
	args := make([]interface{}, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		args[i-1] = ctx.ArgString(i)
	}

	result := formatString(format, args...)

	return ctx.WriteBulkString(result)
}

func formatString(format string, args ...interface{}) string {
	result := ""
	argIdx := 0
	i := 0

	for i < len(format) {
		if format[i] == '{' && i+1 < len(format) && format[i+1] == '}' {
			if argIdx < len(args) {
				result += fmt.Sprintf("%v", args[argIdx])
				argIdx++
			}
			i += 2
		} else if format[i] == '{' && i+1 < len(format) && format[i+1] >= '0' && format[i+1] <= '9' {
			j := i + 1
			for j < len(format) && format[j] >= '0' && format[j] <= '9' {
				j++
			}
			if j < len(format) && format[j] == '}' {
				idx := 0
				for k := i + 1; k < j; k++ {
					idx = idx*10 + int(format[k]-'0')
				}
				if idx < len(args) {
					result += fmt.Sprintf("%v", args[idx])
				}
				i = j + 1
				continue
			}
			result += string(format[i])
			i++
		} else {
			result += string(format[i])
			i++
		}
	}

	return result
}

func cmdEVALJSONPATH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	doc, err := decodeOrderedJSON(ctx.Arg(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	v, found, err := extractJSONPath(doc, ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !found {
		return ctx.WriteNull()
	}
	out, err := encodeOrderedJSON(v)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkBytes(out)
}

var (
	errJSONPathSyntax      = errors.New("ERR invalid JSONPath")
	errJSONPathUnsupported = errors.New("ERR JSONPath wildcards, recursive descent, slices and filters are not supported")
)

// extractJSONPath resolves a path made of "$", ".name", "['name']",
// "[\"name\"]" and "[index]" steps; a negative index counts from the end.
// found is false when a step names a missing member or element.
func extractJSONPath(doc any, path string) (any, bool, error) {
	if path == "" || path[0] != '$' {
		return nil, false, errJSONPathSyntax
	}
	cur, found := doc, true
	member := func(name string) {
		if !found {
			return
		}
		obj, ok := cur.(*jsonObject)
		if !ok {
			found = false
			return
		}
		cur, found = obj.get(name)
	}
	for i := 1; i < len(path); {
		switch path[i] {
		case '.':
			i++
			start := i
			for i < len(path) && path[i] != '.' && path[i] != '[' {
				i++
			}
			name := path[start:i]
			if name == "*" || (name == "" && i < len(path) && path[i] == '.') {
				return nil, false, errJSONPathUnsupported
			}
			if name == "" {
				return nil, false, errJSONPathSyntax
			}
			member(name)
		case '[':
			i++
			if i < len(path) && (path[i] == '\'' || path[i] == '"') {
				quote := path[i]
				i++
				var name strings.Builder
				for i < len(path) && path[i] != quote {
					if path[i] == '\\' && i+1 < len(path) {
						i++
					}
					name.WriteByte(path[i])
					i++
				}
				if i+1 >= len(path) || path[i+1] != ']' {
					return nil, false, errJSONPathSyntax
				}
				i += 2
				member(name.String())
				continue
			}
			start := i
			for i < len(path) && path[i] != ']' {
				i++
			}
			if i >= len(path) {
				return nil, false, errJSONPathSyntax
			}
			sel := strings.TrimSpace(path[start:i])
			i++
			if sel == "*" || strings.ContainsAny(sel, ":?,") {
				return nil, false, errJSONPathUnsupported
			}
			idx, err := strconv.Atoi(sel)
			if err != nil {
				return nil, false, errJSONPathSyntax
			}
			if !found {
				continue
			}
			arr, ok := cur.([]any)
			if idx < 0 {
				idx += len(arr)
			}
			if !ok || idx < 0 || idx >= len(arr) {
				found = false
				continue
			}
			cur = arr[idx]
		default:
			return nil, false, errJSONPathSyntax
		}
	}
	if !found {
		return nil, false, nil
	}
	return cur, true, nil
}

func parseJSONObject(s string) map[string]string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil
	}

	s = s[1 : len(s)-1]
	obj := make(map[string]string)
	depth := 0
	start := 0
	inString := false
	key := ""

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' && (i == 0 || s[i-1] != '\\') {
			inString = !inString
		} else if !inString {
			if c == '{' || c == '[' {
				depth++
			} else if c == '}' || c == ']' {
				depth--
			} else if c == ':' && depth == 0 && key == "" {
				key = strings.TrimSpace(s[start:i])
				key = strings.Trim(key, "\"")
				start = i + 1
			} else if c == ',' && depth == 0 {
				if key != "" {
					obj[key] = strings.TrimSpace(s[start:i])
					key = ""
				}
				start = i + 1
			}
		}
	}

	if key != "" && start < len(s) {
		obj[key] = strings.TrimSpace(s[start:])
	}

	return obj
}

func cmdEVALTEMPLATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	template := ctx.ArgString(0)

	vars := make(map[string]string)
	for i := 1; i+1 < ctx.ArgCount(); i += 2 {
		vars[ctx.ArgString(i)] = ctx.ArgString(i + 1)
	}

	result := applyTemplate(template, vars)

	return ctx.WriteBulkString(result)
}

func applyTemplate(template string, vars map[string]string) string {
	result := ""
	i := 0

	for i < len(template) {
		if i+1 < len(template) && template[i] == '{' && template[i+1] == '{' {
			j := i + 2
			for j < len(template) && (template[j] != '}' || j+1 >= len(template) || template[j+1] != '}') {
				j++
			}
			if j < len(template) {
				key := strings.TrimSpace(template[i+2 : j])
				if val, ok := vars[key]; ok {
					result += val
				}
				i = j + 2
				continue
			}
		}
		result += string(template[i])
		i++
	}

	return result
}

func cmdEVALREGEX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	re, err := regexp.Compile(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(fmt.Errorf("ERR invalid regex: %v", err))
	}

	if re.MatchString(ctx.ArgString(1)) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdEVALREGEXMATCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	re, err := regexp.Compile(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(fmt.Errorf("ERR invalid regex: %v", err))
	}

	matches := re.FindAllString(ctx.ArgString(1), -1)

	results := make([]*resp.Value, len(matches))
	for i, m := range matches {
		results[i] = resp.BulkString(m)
	}

	return ctx.WriteArray(results)
}

func cmdEVALREGEXREPLACE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	re, err := regexp.Compile(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(fmt.Errorf("ERR invalid regex: %v", err))
	}

	result := re.ReplaceAllString(ctx.ArgString(2), ctx.ArgString(1))

	return ctx.WriteBulkString(result)
}

func cmdVALIDATEEMAIL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if isValidEmail(ctx.ArgString(0)) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

// isValidEmail accepts a bare RFC 5322 addr-spec (no display name, comments
// or angle brackets) whose domain is a dotted hostname of at most 254 bytes.
func isValidEmail(email string) bool {
	if len(email) > 254 {
		return false
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Name != "" || addr.Address != email {
		return false
	}
	at := strings.LastIndexByte(email, '@')
	domain := email[at+1:]
	return strings.Contains(domain, ".") && isValidHostname(domain)
}

func isValidHostname(host string) bool {
	host = strings.TrimSuffix(host, ".")
	if host == "" || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if r < 0x80 && !(r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
				return false
			}
		}
	}
	return true
}

func cmdVALIDATEURL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if isValidURL(ctx.ArgString(0)) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

// isValidURL accepts an absolute URL with an authority ("scheme://host...")
// whose host is a hostname or IP literal and whose port, if any, is 1-65535.
func isValidURL(raw string) bool {
	for _, r := range raw {
		if r <= ' ' || r == 0x7f {
			return false
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Opaque != "" || u.Host == "" {
		return false
	}
	if port := u.Port(); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil || p < 1 || p > 65535 {
			return false
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return false
	}
	host := u.Hostname()
	if strings.HasPrefix(u.Host, "[") {
		addr, err := netip.ParseAddr(host)
		return err == nil && addr.Is6()
	}
	return isValidHostname(host)
}

func cmdVALIDATEIP(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if isValidIP(ctx.ArgString(0)) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

// isValidIP accepts dotted-decimal IPv4 (no leading zeros) and IPv6,
// including IPv4-mapped forms, but not IPv6 zones.
func isValidIP(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	return err == nil && addr.Zone() == ""
}

func cmdVALIDATEJSON(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	jsonStr := ctx.ArgString(0)

	if json.Valid([]byte(jsonStr)) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdVALIDATEINT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)

	if isInteger(s) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func isInteger(s string) bool {
	if len(s) == 0 {
		return false
	}

	start := 0
	if s[0] == '-' || s[0] == '+' {
		start = 1
	}

	if start >= len(s) {
		return false
	}

	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

func cmdVALIDATEFLOAT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)

	if isFloat(s) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func isFloat(s string) bool {
	if len(s) == 0 {
		return false
	}

	start := 0
	if s[0] == '-' || s[0] == '+' {
		start = 1
	}

	dotCount := 0
	digitCount := 0

	for i := start; i < len(s); i++ {
		if s[i] == '.' {
			dotCount++
			if dotCount > 1 {
				return false
			}
		} else if s[i] >= '0' && s[i] <= '9' {
			digitCount++
		} else {
			return false
		}
	}

	return digitCount > 0
}

func cmdVALIDATEALPHA(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)

	if isAlpha(s) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func isAlpha(s string) bool {
	if len(s) == 0 {
		return false
	}

	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}

	return true
}

func cmdVALIDATEALPHANUM(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)

	if isAlphaNum(s) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func isAlphaNum(s string) bool {
	if len(s) == 0 {
		return false
	}

	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}

	return true
}

func cmdVALIDATELENGTH(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	min := parseInt64(ctx.ArgString(1))
	max := parseInt64(ctx.ArgString(2))

	length := int64(len(s))

	if length >= min && length <= max {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

// cmdVALIDATERANGE reports whether min <= value <= max (both bounds
// inclusive). Numbers are compared at 512-bit precision so that integers
// beyond 2^53 and decimals such as 0.1 compare exactly as written.
func cmdVALIDATERANGE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	var nums [3]*big.Float
	for i := range nums {
		f, err := parseTemplateFloat(ctx.ArgString(i))
		if err != nil {
			return ctx.WriteError(err)
		}
		nums[i] = f
	}
	val, min, max := nums[0], nums[1], nums[2]
	if min.Cmp(max) > 0 {
		return ctx.WriteError(errors.New("ERR min is greater than max"))
	}

	if val.Cmp(min) >= 0 && val.Cmp(max) <= 0 {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func parseTemplateFloat(s string) (*big.Float, error) {
	if f, err := strconv.ParseFloat(s, 64); err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, ErrNotFloat
	}
	f, _, err := big.ParseFloat(s, 10, 512, big.ToNearestEven)
	if err != nil {
		return nil, ErrNotFloat
	}
	return f, nil
}

func cmdSTRFORMAT(ctx *Context) error {
	return cmdEVALFORMAT(ctx)
}

func cmdSTRTRUNCATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	maxLen := int(parseInt64(ctx.ArgString(1)))
	suffix := "..."
	if ctx.ArgCount() >= 3 {
		suffix = ctx.ArgString(2)
	}

	if len(s) <= maxLen {
		return ctx.WriteBulkString(s)
	}

	if maxLen <= len(suffix) {
		return ctx.WriteBulkString(s[:maxLen])
	}

	return ctx.WriteBulkString(s[:maxLen-len(suffix)] + suffix)
}

func cmdSTRPADLEFT(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	totalLen := int(parseInt64(ctx.ArgString(1)))
	padChar := ctx.ArgString(2)
	if len(padChar) == 0 {
		padChar = " "
	}

	for len(s) < totalLen {
		s = string(padChar[0]) + s
	}

	return ctx.WriteBulkString(s)
}

func cmdSTRPADRIGHT(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	totalLen := int(parseInt64(ctx.ArgString(1)))
	padChar := ctx.ArgString(2)
	if len(padChar) == 0 {
		padChar = " "
	}

	for len(s) < totalLen {
		s += string(padChar[0])
	}

	return ctx.WriteBulkString(s)
}

func cmdSTRREVERSE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	runes := []rune(s)

	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return ctx.WriteBulkString(string(runes))
}

func cmdSTRREPEAT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	count := int(parseInt64(ctx.ArgString(1)))

	result := ""
	for i := 0; i < count; i++ {
		result += s
	}

	return ctx.WriteBulkString(result)
}

func cmdSTRSPLIT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	sep := ctx.ArgString(1)

	parts := strings.Split(s, sep)

	results := make([]*resp.Value, len(parts))
	for i, p := range parts {
		results[i] = resp.BulkString(p)
	}

	return ctx.WriteArray(results)
}

func cmdSTRJOIN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	sep := ctx.ArgString(0)
	parts := make([]string, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		parts[i-1] = ctx.ArgString(i)
	}

	return ctx.WriteBulkString(strings.Join(parts, sep))
}

func cmdSTRCONTAINS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	substr := ctx.ArgString(1)

	if strings.Contains(s, substr) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSTRSTARTSWITH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	prefix := ctx.ArgString(1)

	if strings.HasPrefix(s, prefix) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSTRENDSWITH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	suffix := ctx.ArgString(1)

	if strings.HasSuffix(s, suffix) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSTRINDEX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	substr := ctx.ArgString(1)

	idx := strings.Index(s, substr)

	return ctx.WriteInteger(int64(idx))
}

func cmdSTRLASTINDEX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	substr := ctx.ArgString(1)

	idx := strings.LastIndex(s, substr)

	return ctx.WriteInteger(int64(idx))
}

func cmdSTRREPLACE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	old := ctx.ArgString(1)
	new := ctx.ArgString(2)

	count := -1
	if ctx.ArgCount() >= 4 {
		count = int(parseInt64(ctx.ArgString(3)))
	}

	if count == -1 {
		return ctx.WriteBulkString(strings.ReplaceAll(s, old, new))
	}

	return ctx.WriteBulkString(strings.Replace(s, old, new, count))
}

func cmdSTRTRIM(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	cutset := " \t\n\r"
	if ctx.ArgCount() >= 2 {
		cutset = ctx.ArgString(1)
	}

	return ctx.WriteBulkString(strings.Trim(s, cutset))
}

func cmdSTRTRIMLEFT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	cutset := " \t\n\r"
	if ctx.ArgCount() >= 2 {
		cutset = ctx.ArgString(1)
	}

	return ctx.WriteBulkString(strings.TrimLeft(s, cutset))
}

func cmdSTRTRIMRIGHT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)
	cutset := " \t\n\r"
	if ctx.ArgCount() >= 2 {
		cutset = ctx.ArgString(1)
	}

	return ctx.WriteBulkString(strings.TrimRight(s, cutset))
}

func cmdSTRTITLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)

	if len(s) == 0 {
		return ctx.WriteBulkString(s)
	}

	var b strings.Builder
	b.Grow(len(s))
	startOfWord := true
	for _, r := range s {
		if unicode.IsSpace(r) {
			startOfWord = true
			b.WriteRune(r)
			continue
		}
		if startOfWord {
			b.WriteRune(unicode.ToUpper(r))
			startOfWord = false
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
	}

	return ctx.WriteBulkString(b.String())
}

func cmdSTRWORDS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)

	words := strings.Fields(s)

	results := make([]*resp.Value, len(words))
	for i, w := range words {
		results[i] = resp.BulkString(w)
	}

	return ctx.WriteArray(results)
}

func cmdSTRLINES(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	s := ctx.ArgString(0)

	lines := strings.Split(s, "\n")

	results := make([]*resp.Value, len(lines))
	for i, l := range lines {
		results[i] = resp.BulkString(strings.TrimRight(l, "\r"))
	}

	return ctx.WriteArray(results)
}
