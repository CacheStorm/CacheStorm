package command

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// cjsonMaxDepth is lua-cjson's default encode/decode nesting limit.
const cjsonMaxDepth = 1000

// openRedisLibs installs the libraries Redis exposes to scripts besides the
// redis table: cjson, bit and cmsgpack.
func openRedisLibs(L *lua.LState) {
	openCJSON(L)
	openBit(L)
	openCMsgpack(L)
}

func openCJSON(L *lua.LState) {
	mod := L.NewTable()
	null := L.NewUserData()
	mod.RawSetString("null", null)
	mod.RawSetString("_NAME", lua.LString("cjson"))
	mod.RawSetString("_VERSION", lua.LString("2.1.0"))
	mod.RawSetString("encode", L.NewFunction(func(L *lua.LState) int {
		if L.GetTop() != 1 {
			L.RaiseError("bad argument #1 to 'encode' (expected 1 argument)")
		}
		var b strings.Builder
		if err := cjsonEncode(&b, L.Get(1), null, 0); err != nil {
			L.RaiseError("%s", err.Error())
		}
		L.Push(lua.LString(b.String()))
		return 1
	}))
	mod.RawSetString("decode", L.NewFunction(func(L *lua.LState) int {
		if L.GetTop() != 1 {
			L.RaiseError("bad argument #1 to 'decode' (expected 1 argument)")
		}
		var s string
		switch v := L.Get(1).(type) {
		case lua.LString:
			s = string(v)
		case lua.LNumber:
			s = v.String()
		default:
			L.RaiseError("bad argument #1 to 'decode' (string expected, got %s)", v.Type().String())
		}
		d := &cjsonDecoder{L: L, s: s, null: null}
		v, err := d.decode()
		if err != nil {
			L.RaiseError("%s", err.Error())
		}
		L.Push(v)
		return 1
	}))
	// The configuration functions report lua-cjson's defaults; the settings
	// themselves are fixed.
	for name, values := range map[string][]lua.LValue{
		"encode_sparse_array":     {lua.LFalse, lua.LNumber(2), lua.LNumber(10)},
		"encode_max_depth":        {lua.LNumber(cjsonMaxDepth)},
		"decode_max_depth":        {lua.LNumber(cjsonMaxDepth)},
		"encode_number_precision": {lua.LNumber(14)},
		"encode_keep_buffer":      {lua.LTrue},
		"encode_invalid_numbers":  {lua.LFalse},
		"decode_invalid_numbers":  {lua.LTrue},
	} {
		vals := values
		mod.RawSetString(name, L.NewFunction(func(L *lua.LState) int {
			for _, v := range vals {
				L.Push(v)
			}
			return len(vals)
		}))
	}
	mod.RawSetString("new", L.NewFunction(func(L *lua.LState) int {
		L.Push(mod)
		return 1
	}))
	L.SetGlobal("cjson", mod)
}

func cjsonEncode(b *strings.Builder, v lua.LValue, null lua.LValue, depth int) error {
	switch val := v.(type) {
	case *lua.LNilType:
		b.WriteString("null")
	case lua.LBool:
		if bool(val) {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case lua.LNumber:
		f := float64(val)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return replyError("Cannot serialise number: must not be NaN or Inf")
		}
		b.WriteString(cjsonNumber(f))
	case lua.LString:
		cjsonString(b, string(val))
	case *lua.LUserData:
		if v == null {
			b.WriteString("null")
			return nil
		}
		return replyError("Cannot serialise userdata: type not supported")
	case *lua.LTable:
		if depth+1 > cjsonMaxDepth {
			return replyError(fmt.Sprintf("Cannot serialise, excessive nesting (%d)", depth+1))
		}
		n, err := cjsonArrayLength(val)
		if err != nil {
			return err
		}
		if n > 0 {
			b.WriteByte('[')
			for i := 1; i <= n; i++ {
				if i > 1 {
					b.WriteByte(',')
				}
				if err := cjsonEncode(b, val.RawGetInt(i), null, depth+1); err != nil {
					return err
				}
			}
			b.WriteByte(']')
			return nil
		}
		type kv struct {
			key string
			val lua.LValue
		}
		var pairs []kv
		var keyErr error
		val.ForEach(func(k, x lua.LValue) {
			switch key := k.(type) {
			case lua.LString:
				pairs = append(pairs, kv{string(key), x})
			case lua.LNumber:
				pairs = append(pairs, kv{cjsonNumber(float64(key)), x})
			default:
				if keyErr == nil {
					keyErr = replyError("Cannot serialise " + k.Type().String() + ": table key must be a number or string")
				}
			}
		})
		if keyErr != nil {
			return keyErr
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].key < pairs[j].key })
		b.WriteByte('{')
		for i, p := range pairs {
			if i > 0 {
				b.WriteByte(',')
			}
			cjsonString(b, p.key)
			b.WriteByte(':')
			if err := cjsonEncode(b, p.val, null, depth+1); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return replyError("Cannot serialise " + v.Type().String() + ": type not supported")
	}
	return nil
}

// cjsonArrayLength mirrors lua-cjson: a table whose keys are all positive
// integers is an array of length max key (holes encode as null); 0 means
// "encode as an object".
func cjsonArrayLength(t *lua.LTable) (int, error) {
	maxKey, items := 0, 0
	isArray := true
	t.ForEach(func(k, _ lua.LValue) {
		if !isArray {
			return
		}
		n, ok := k.(lua.LNumber)
		if !ok || float64(n) < 1 || float64(n) != math.Floor(float64(n)) {
			isArray = false
			return
		}
		if int(n) > maxKey {
			maxKey = int(n)
		}
		items++
	})
	if !isArray {
		return 0, nil
	}
	if maxKey > 10 && maxKey > items*2 {
		return 0, replyError("Cannot serialise table: excessively sparse array")
	}
	return maxKey, nil
}

func cjsonNumber(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e14 {
		return strconv.FormatInt(int64(f), 10)
	}
	return fmt.Sprintf("%.14g", f)
}

func cjsonString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '/':
			b.WriteString(`\/`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if c < 0x20 || c == 0x7f {
				fmt.Fprintf(b, `\u%04x`, c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
}

// cjsonDecoder is a JSON decoder with lua-cjson's tokenizer and error
// messages ("Expected value but found T_END at character 1").
type cjsonDecoder struct {
	L     *lua.LState
	s     string
	pos   int
	null  lua.LValue
	depth int
}

const (
	cjObjBegin = iota
	cjObjEnd
	cjArrBegin
	cjArrEnd
	cjString
	cjNumber
	cjBoolean
	cjNull
	cjColon
	cjComma
	cjEnd
	cjWhitespace
	cjError
)

var cjTokenNames = []string{"T_OBJ_BEGIN", "T_OBJ_END", "T_ARR_BEGIN", "T_ARR_END", "T_STRING", "T_NUMBER", "T_BOOLEAN", "T_NULL", "T_COLON", "T_COMMA", "T_END", "T_WHITESPACE", "T_ERROR"}

type cjToken struct {
	typ   int
	index int
	str   string
	num   float64
	b     bool
}

func (d *cjsonDecoder) setError(tok *cjToken, msg string) {
	tok.typ = cjError
	tok.index = d.pos
	tok.str = msg
}

func (d *cjsonDecoder) next() cjToken {
	for d.pos < len(d.s) {
		c := d.s[d.pos]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		d.pos++
	}
	tok := cjToken{index: d.pos}
	if d.pos >= len(d.s) {
		tok.typ = cjEnd
		return tok
	}
	c := d.s[d.pos]
	single := map[byte]int{'{': cjObjBegin, '}': cjObjEnd, '[': cjArrBegin, ']': cjArrEnd, ':': cjColon, ',': cjComma}
	if t, ok := single[c]; ok {
		tok.typ = t
		d.pos++
		return tok
	}
	switch {
	case c == '"':
		d.stringToken(&tok)
	case c == '-' || (c >= '0' && c <= '9'):
		d.numberToken(&tok)
	case strings.HasPrefix(d.s[d.pos:], "true"):
		tok.typ, tok.b = cjBoolean, true
		d.pos += 4
	case strings.HasPrefix(d.s[d.pos:], "false"):
		tok.typ, tok.b = cjBoolean, false
		d.pos += 5
	case strings.HasPrefix(d.s[d.pos:], "null"):
		tok.typ = cjNull
		d.pos += 4
	default:
		d.setError(&tok, "invalid token")
	}
	return tok
}

func (d *cjsonDecoder) numberToken(tok *cjToken) {
	i := d.pos
	if i < len(d.s) && d.s[i] == '-' {
		i++
	}
	digits := 0
	for i < len(d.s) && d.s[i] >= '0' && d.s[i] <= '9' {
		i++
		digits++
	}
	if i < len(d.s) && d.s[i] == '.' {
		i++
		for i < len(d.s) && d.s[i] >= '0' && d.s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits > 0 && i < len(d.s) && (d.s[i] == 'e' || d.s[i] == 'E') {
		j := i + 1
		if j < len(d.s) && (d.s[j] == '+' || d.s[j] == '-') {
			j++
		}
		if j < len(d.s) && d.s[j] >= '0' && d.s[j] <= '9' {
			for j < len(d.s) && d.s[j] >= '0' && d.s[j] <= '9' {
				j++
			}
			i = j
		}
	}
	if digits == 0 {
		d.setError(tok, "invalid number")
		return
	}
	f, err := strconv.ParseFloat(d.s[d.pos:i], 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		d.setError(tok, "invalid number")
		return
	}
	tok.typ, tok.num = cjNumber, f
	d.pos = i
}

func (d *cjsonDecoder) stringToken(tok *cjToken) {
	d.pos++
	var b strings.Builder
	for {
		if d.pos >= len(d.s) {
			d.setError(tok, "unexpected end of string")
			return
		}
		c := d.s[d.pos]
		if c == '"' {
			d.pos++
			tok.typ, tok.str = cjString, b.String()
			return
		}
		if c != '\\' {
			b.WriteByte(c)
			d.pos++
			continue
		}
		if d.pos+1 >= len(d.s) {
			d.setError(tok, "invalid escape code")
			return
		}
		switch e := d.s[d.pos+1]; e {
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
			r, n, ok := d.unicodeEscape(d.pos)
			if !ok {
				d.setError(tok, "invalid unicode escape code")
				return
			}
			b.WriteRune(r)
			d.pos += n
			continue
		default:
			d.setError(tok, "invalid escape code")
			return
		}
		d.pos += 2
	}
}

// unicodeEscape decodes \uXXXX (and a following low surrogate) at i and
// returns the rune and the number of bytes consumed.
func (d *cjsonDecoder) unicodeEscape(i int) (rune, int, bool) {
	hex4 := func(at int) (rune, bool) {
		if at+6 > len(d.s) || d.s[at] != '\\' || d.s[at+1] != 'u' {
			return 0, false
		}
		v, err := strconv.ParseUint(d.s[at+2:at+6], 16, 32)
		return rune(v), err == nil
	}
	r, ok := hex4(i)
	if !ok {
		return 0, 0, false
	}
	if r >= 0xdc00 && r <= 0xdfff {
		return 0, 0, false
	}
	if r >= 0xd800 && r <= 0xdbff {
		lo, ok := hex4(i + 6)
		if !ok || lo < 0xdc00 || lo > 0xdfff {
			return 0, 0, false
		}
		return 0x10000 + (r-0xd800)<<10 + (lo - 0xdc00), 12, true
	}
	return r, 6, true
}

func (d *cjsonDecoder) fail(expected string, tok cjToken) error {
	found := ""
	if tok.typ == cjError {
		found = tok.str
	} else {
		found = cjTokenNames[tok.typ]
	}
	return replyError(fmt.Sprintf("Expected %s but found %s at character %d", expected, found, tok.index+1))
}

func (d *cjsonDecoder) decode() (lua.LValue, error) {
	v, err := d.value(d.next())
	if err != nil {
		return nil, err
	}
	if tok := d.next(); tok.typ != cjEnd {
		return nil, d.fail("the end", tok)
	}
	return v, nil
}

func (d *cjsonDecoder) value(tok cjToken) (lua.LValue, error) {
	switch tok.typ {
	case cjString:
		return lua.LString(tok.str), nil
	case cjNumber:
		return lua.LNumber(tok.num), nil
	case cjBoolean:
		return lua.LBool(tok.b), nil
	case cjNull:
		return d.null, nil
	case cjObjBegin, cjArrBegin:
		d.depth++
		defer func() { d.depth-- }()
		if d.depth > cjsonMaxDepth {
			return nil, replyError(fmt.Sprintf("Found too many nested data structures (%d) at character %d", d.depth, tok.index+1))
		}
		if tok.typ == cjObjBegin {
			return d.object()
		}
		return d.array()
	}
	return nil, d.fail("value", tok)
}

func (d *cjsonDecoder) object() (lua.LValue, error) {
	tbl := d.L.NewTable()
	tok := d.next()
	if tok.typ == cjObjEnd {
		return tbl, nil
	}
	for {
		if tok.typ != cjString {
			return nil, d.fail("object key string", tok)
		}
		key := tok.str
		if tok = d.next(); tok.typ != cjColon {
			return nil, d.fail("colon", tok)
		}
		v, err := d.value(d.next())
		if err != nil {
			return nil, err
		}
		tbl.RawSetString(key, v)
		tok = d.next()
		if tok.typ == cjObjEnd {
			return tbl, nil
		}
		if tok.typ != cjComma {
			return nil, d.fail("comma or object end", tok)
		}
		tok = d.next()
	}
}

func (d *cjsonDecoder) array() (lua.LValue, error) {
	tbl := d.L.NewTable()
	tok := d.next()
	if tok.typ == cjArrEnd {
		return tbl, nil
	}
	for i := 1; ; i++ {
		v, err := d.value(tok)
		if err != nil {
			return nil, err
		}
		tbl.RawSetInt(i, v)
		tok = d.next()
		if tok.typ == cjArrEnd {
			return tbl, nil
		}
		if tok.typ != cjComma {
			return nil, d.fail("comma or array end", tok)
		}
		tok = d.next()
	}
}

// openBit installs the LuaBitOp library Redis bundles (32-bit operations).
func openBit(L *lua.LState) {
	mod := L.NewTable()
	arg := func(L *lua.LState, i int) int32 {
		return luaToBit(float64(L.CheckNumber(i)))
	}
	push := func(L *lua.LState, v int32) int {
		L.Push(lua.LNumber(v))
		return 1
	}
	fold := func(op func(a, b int32) int32) lua.LGFunction {
		return func(L *lua.LState) int {
			v := arg(L, 1)
			for i := 2; i <= L.GetTop(); i++ {
				v = op(v, arg(L, i))
			}
			return push(L, v)
		}
	}
	fns := map[string]lua.LGFunction{
		"tobit": func(L *lua.LState) int { return push(L, arg(L, 1)) },
		"bnot":  func(L *lua.LState) int { return push(L, ^arg(L, 1)) },
		"band":  fold(func(a, b int32) int32 { return a & b }),
		"bor":   fold(func(a, b int32) int32 { return a | b }),
		"bxor":  fold(func(a, b int32) int32 { return a ^ b }),
		"lshift": func(L *lua.LState) int {
			return push(L, int32(uint32(arg(L, 1))<<(uint32(arg(L, 2))&31)))
		},
		"rshift": func(L *lua.LState) int {
			return push(L, int32(uint32(arg(L, 1))>>(uint32(arg(L, 2))&31)))
		},
		"arshift": func(L *lua.LState) int {
			return push(L, arg(L, 1)>>(uint32(arg(L, 2))&31))
		},
		"rol": func(L *lua.LState) int {
			x, n := uint32(arg(L, 1)), uint32(arg(L, 2))&31
			return push(L, int32(x<<n|x>>((32-n)&31)))
		},
		"ror": func(L *lua.LState) int {
			x, n := uint32(arg(L, 1)), uint32(arg(L, 2))&31
			return push(L, int32(x>>n|x<<((32-n)&31)))
		},
		"bswap": func(L *lua.LState) int {
			x := uint32(arg(L, 1))
			return push(L, int32(x>>24|(x>>8)&0xff00|(x<<8)&0xff0000|x<<24))
		},
		"tohex": func(L *lua.LState) int {
			x := uint32(arg(L, 1))
			n := int32(8)
			if L.GetTop() >= 2 {
				n = arg(L, 2)
			}
			digits := "0123456789abcdef"
			if n < 0 {
				digits = "0123456789ABCDEF"
				n = -n
			}
			if n > 8 {
				n = 8
			}
			buf := make([]byte, n)
			for i := n - 1; i >= 0; i-- {
				buf[i] = digits[x&15]
				x >>= 4
			}
			L.Push(lua.LString(buf))
			return 1
		},
	}
	for name, fn := range fns {
		mod.RawSetString(name, L.NewFunction(fn))
	}
	L.SetGlobal("bit", mod)
}

// luaToBit normalises a number to a 32-bit integer the way LuaBitOp does:
// round to the nearest integer, then wrap modulo 2^32.
func luaToBit(f float64) int32 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	f = math.RoundToEven(f)
	return int32(uint32(int64(math.Mod(f, 4294967296))))
}

// installRedisHelpers adds the redis.* helpers that do not dispatch commands.
func installRedisHelpers(L *lua.LState, redisTable *lua.LTable) {
	L.SetField(redisTable, "error_reply", L.NewFunction(func(L *lua.LState) int {
		msg, ok := L.Get(1).(lua.LString)
		if L.GetTop() != 1 || !ok {
			L.Push(luaErrTable(L, "ERR wrong number or type of arguments"))
			return 1
		}
		text := string(msg)
		if !strings.HasPrefix(text, "-") {
			text = "-" + text
		}
		L.Push(luaErrTable(L, luaErrorText(text)))
		return 1
	}))
	L.SetField(redisTable, "status_reply", L.NewFunction(func(L *lua.LState) int {
		msg, ok := L.Get(1).(lua.LString)
		if L.GetTop() != 1 || !ok {
			L.Push(luaErrTable(L, "ERR wrong number or type of arguments"))
			return 1
		}
		tbl := L.NewTable()
		tbl.RawSetString("ok", msg)
		L.Push(tbl)
		return 1
	}))
	L.SetField(redisTable, "sha1hex", L.NewFunction(func(L *lua.LState) int {
		if L.GetTop() != 1 {
			raiseLuaErr(L, "ERR wrong number of arguments")
		}
		h := sha1.Sum([]byte(L.ToString(1)))
		L.Push(lua.LString(hex.EncodeToString(h[:])))
		return 1
	}))
	L.SetField(redisTable, "log", L.NewFunction(func(L *lua.LState) int {
		if L.GetTop() < 2 {
			raiseLuaErr(L, "ERR redis.log() requires two arguments or more.")
		}
		level, ok := L.Get(1).(lua.LNumber)
		if !ok {
			raiseLuaErr(L, "ERR First argument must be a number (log level).")
		}
		if level < 0 || level > 3 {
			raiseLuaErr(L, "ERR Invalid log level.")
		}
		return 0
	}))
	L.SetField(redisTable, "setresp", L.NewFunction(func(L *lua.LState) int {
		if L.GetTop() != 1 {
			raiseLuaErr(L, "ERR redis.setresp() requires one argument.")
		}
		if v := L.ToInt(1); v != 2 && v != 3 {
			raiseLuaErr(L, "ERR RESP version must be 2 or 3.")
		}
		return 0
	}))
	L.SetField(redisTable, "set_repl", L.NewFunction(func(L *lua.LState) int {
		if L.GetTop() != 1 {
			raiseLuaErr(L, "ERR redis.set_repl() requires one argument.")
		}
		if v := L.ToInt(1); v < 0 || v > 3 {
			raiseLuaErr(L, "ERR Invalid replication flags. Use REPL_AOF, REPL_REPLICA, REPL_ALL or REPL_NONE.")
		}
		return 0
	}))
	L.SetField(redisTable, "replicate_commands", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LTrue)
		return 1
	}))
	L.SetField(redisTable, "breakpoint", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LFalse)
		return 1
	}))
	L.SetField(redisTable, "debug", L.NewFunction(func(L *lua.LState) int { return 0 }))
	for name, v := range map[string]lua.LValue{
		"LOG_DEBUG": lua.LNumber(0), "LOG_VERBOSE": lua.LNumber(1),
		"LOG_NOTICE": lua.LNumber(2), "LOG_WARNING": lua.LNumber(3),
		"REPL_NONE": lua.LNumber(0), "REPL_AOF": lua.LNumber(1),
		"REPL_SLAVE": lua.LNumber(2), "REPL_REPLICA": lua.LNumber(2),
		"REPL_ALL":      lua.LNumber(3),
		"REDIS_VERSION": lua.LString("7.4.0"), "REDIS_VERSION_NUM": lua.LNumber(0x070400),
	} {
		L.SetField(redisTable, name, v)
	}
}
