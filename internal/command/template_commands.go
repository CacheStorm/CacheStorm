package command

import (
	"fmt"
	"strconv"
	"strings"

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
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	expr := ctx.ArgString(0)

	result, err := evaluateExpression(expr)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteBulkString(result)
}

func evaluateExpression(expr string) (string, error) {
	expr = strings.TrimSpace(expr)

	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		expr = expr[1 : len(expr)-1]
	}

	ops := []string{"+", "-", "*", "/", "%", "^"}
	for _, op := range ops {
		parts := splitExpr(expr, op)
		if len(parts) == 2 {
			left, err := evaluateExpression(parts[0])
			if err != nil {
				return "", err
			}
			right, err := evaluateExpression(parts[1])
			if err != nil {
				return "", err
			}

			l, _ := strconv.ParseFloat(left, 64)
			r, _ := strconv.ParseFloat(right, 64)

			var result float64
			switch op {
			case "+":
				result = l + r
			case "-":
				result = l - r
			case "*":
				result = l * r
			case "/":
				if r == 0 {
					return "", fmt.Errorf("division by zero")
				}
				result = l / r
			case "%":
				result = float64(int64(l) % int64(r))
			case "^":
				result = pow(l, r)
			}

			if result == float64(int64(result)) {
				return strconv.FormatInt(int64(result), 10), nil
			}
			return strconv.FormatFloat(result, 'f', -1, 64), nil
		}
	}

	return expr, nil
}

func splitExpr(expr, op string) []string {
	depth := 0
	for i := len(expr) - 1; i >= 0; i-- {
		c := expr[i]
		if c == ')' {
			depth++
		} else if c == '(' {
			depth--
		} else if depth == 0 && string(c) == op {
			return []string{expr[:i], expr[i+1:]}
		}
	}
	return nil
}

func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
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
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	jsonStr := ctx.ArgString(0)
	path := ctx.ArgString(1)

	result := extractJSONPath(jsonStr, path)

	return ctx.WriteBulkString(result)
}

func extractJSONPath(jsonStr, path string) string {
	if path == "$" {
		return jsonStr
	}

	if !strings.HasPrefix(path, "$.") {
		return ""
	}

	path = path[2:]
	parts := strings.Split(path, ".")

	current := jsonStr

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") {
			idxStr := part[1 : len(part)-1]
			idx := 0
			for _, c := range idxStr {
				if c >= '0' && c <= '9' {
					idx = idx*10 + int(c-'0')
				}
			}

			items := parseJSONArray(current)
			if idx < len(items) {
				current = items[idx]
			} else {
				return ""
			}
		} else {
			obj := parseJSONObject(current)
			if val, ok := obj[part]; ok {
				current = val
			} else {
				return ""
			}
		}
	}

	return current
}

func parseJSONArray(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil
	}

	s = s[1 : len(s)-1]
	items := make([]string, 0)
	depth := 0
	start := 0
	inString := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' && (i == 0 || s[i-1] != '\\') {
			inString = !inString
		} else if !inString {
			if c == '{' || c == '[' {
				depth++
			} else if c == '}' || c == ']' {
				depth--
			} else if c == ',' && depth == 0 {
				items = append(items, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}

	if start < len(s) {
		items = append(items, strings.TrimSpace(s[start:]))
	}

	return items
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
			for j < len(template) && !(template[j] == '}' && j+1 < len(template) && template[j+1] == '}') {
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

	pattern := ctx.ArgString(0)
	input := ctx.ArgString(1)

	matched := matchRegex(pattern, input)

	if matched {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func matchRegex(pattern, input string) bool {
	if pattern == ".*" {
		return true
	}

	if strings.HasPrefix(pattern, "^") {
		pattern = pattern[1:]
		if !strings.HasPrefix(input, pattern) {
			return false
		}
	}

	if strings.HasSuffix(pattern, "$") {
		pattern = pattern[:len(pattern)-1]
		if !strings.HasSuffix(input, pattern) {
			return false
		}
	}

	return strings.Contains(input, pattern)
}

func cmdEVALREGEXMATCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	pattern := ctx.ArgString(0)
	input := ctx.ArgString(1)

	matches := findMatches(pattern, input)

	results := make([]*resp.Value, len(matches))
	for i, m := range matches {
		results[i] = resp.BulkString(m)
	}

	return ctx.WriteArray(results)
}

func findMatches(pattern, input string) []string {
	matches := make([]string, 0)

	start := 0
	for {
		idx := strings.Index(input[start:], pattern)
		if idx == -1 {
			break
		}
		matches = append(matches, input[start+idx:start+idx+len(pattern)])
		start += idx + len(pattern)
	}

	return matches
}

func cmdEVALREGEXREPLACE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	pattern := ctx.ArgString(0)
	replacement := ctx.ArgString(1)
	input := ctx.ArgString(2)

	result := strings.ReplaceAll(input, pattern, replacement)

	return ctx.WriteBulkString(result)
}

func cmdVALIDATEEMAIL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	email := ctx.ArgString(0)

	if isValidEmail(email) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func isValidEmail(email string) bool {
	atIdx := strings.Index(email, "@")
	if atIdx <= 0 || atIdx >= len(email)-1 {
		return false
	}

	dotIdx := strings.LastIndex(email[atIdx:], ".")
	if dotIdx <= 1 || dotIdx >= len(email[atIdx:])-1 {
		return false
	}

	return true
}

func cmdVALIDATEURL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	url := ctx.ArgString(0)

	if isValidURL(url) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func isValidURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "ftp://")
}

func cmdVALIDATEIP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ip := ctx.ArgString(0)

	if isValidIP(ip) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func isValidIP(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		num := 0
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
			num = num*10 + int(c-'0')
		}
		if num < 0 || num > 255 {
			return false
		}
	}

	return true
}

func cmdVALIDATEJSON(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	jsonStr := ctx.ArgString(0)

	if isValidJSON(jsonStr) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func isValidJSON(s string) bool {
	s = strings.TrimSpace(s)
	if (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
		return true
	}
	return false
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
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
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
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
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

func cmdVALIDATERANGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	val, _ := parseTemplateFloat(ctx.Arg(0))
	min, _ := parseTemplateFloat(ctx.Arg(1))
	max, _ := parseTemplateFloat(ctx.Arg(2))

	if val >= min && val <= max {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func parseTemplateFloat(data []byte) (float64, error) {
	s := string(data)
	s = strings.TrimSpace(s)

	var result float64
	var sign float64 = 1
	var decimal float64 = 1
	inDecimal := false

	i := 0
	if i < len(s) && s[i] == '-' {
		sign = -1
		i++
	} else if i < len(s) && s[i] == '+' {
		i++
	}

	for ; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			digit := float64(s[i] - '0')
			if inDecimal {
				decimal *= 10
				result += digit / decimal
			} else {
				result = result*10 + digit
			}
		} else if s[i] == '.' && !inDecimal {
			inDecimal = true
		} else {
			break
		}
	}

	return sign * result, nil
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
		s = s + string(padChar[0])
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

	result := strings.ToUpper(string(s[0]))
	if len(s) > 1 {
		result += strings.ToLower(s[1:])
	}

	return ctx.WriteBulkString(result)
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
