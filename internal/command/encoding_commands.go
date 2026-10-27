package command

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/cachestorm/cachestorm/internal/resp"
	"gopkg.in/yaml.v3"
)

// errInvalidPercentEncoding is returned for a '%' that is not followed by
// exactly two hex digits, matching Redis' "invalid percent-encoding" reply.
var errInvalidPercentEncoding = errors.New("ERR invalid percent-encoding")

var encodingRandMu sync.Mutex
var encodingRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func RegisterEncodingCommands(router *Router) {
	router.Register(&CommandDef{Name: "MSGPACK.ENCODE", Handler: cmdMSGPACKENCODE})
	router.Register(&CommandDef{Name: "MSGPACK.DECODE", Handler: cmdMSGPACKDECODE})

	router.Register(&CommandDef{Name: "URL.ENCODE", Handler: cmdURLENCODE})
	router.Register(&CommandDef{Name: "URL.DECODE", Handler: cmdURLDECODE})

	router.Register(&CommandDef{Name: "XML.ENCODE", Handler: cmdXMLENCODE})
	router.Register(&CommandDef{Name: "XML.DECODE", Handler: cmdXMLDECODE})

	router.Register(&CommandDef{Name: "YAML.ENCODE", Handler: cmdYAMLENCODE})
	router.Register(&CommandDef{Name: "YAML.DECODE", Handler: cmdYAMLDECODE})

	router.Register(&CommandDef{Name: "CBOR.ENCODE", Handler: cmdCBORENCODE})
	router.Register(&CommandDef{Name: "CBOR.DECODE", Handler: cmdCBORDECODE})

	router.Register(&CommandDef{Name: "CSV.ENCODE", Handler: cmdCSVENCODE})
	router.Register(&CommandDef{Name: "CSV.DECODE", Handler: cmdCSVDECODE})

	router.Register(&CommandDef{Name: "UUID.GEN", Handler: cmdUUIDGEN})
	router.Register(&CommandDef{Name: "UUID.VALIDATE", Handler: cmdUUIDVALIDATE})
	router.Register(&CommandDef{Name: "UUID.VERSION", Handler: cmdUUIDVERSION})

	router.Register(&CommandDef{Name: "ULID.GEN", Handler: cmdULIDGEN})
	router.Register(&CommandDef{Name: "ULID.EXTRACT", Handler: cmdULIDEXTRACT})

	router.Register(&CommandDef{Name: "TIMESTAMP.NOW", Handler: cmdTIMESTAMPNOW})
	router.Register(&CommandDef{Name: "TIMESTAMP.PARSE", Handler: cmdTIMESTAMPPARSE})
	router.Register(&CommandDef{Name: "TIMESTAMP.FORMAT", Handler: cmdTIMESTAMPFORMAT})
	router.Register(&CommandDef{Name: "TIMESTAMP.ADD", Handler: cmdTIMESTAMPADD})
	router.Register(&CommandDef{Name: "TIMESTAMP.DIFF", Handler: cmdTIMESTAMPDIFF})
	router.Register(&CommandDef{Name: "TIMESTAMP.STARTOF", Handler: cmdTIMESTAMPSTARTOF})
	router.Register(&CommandDef{Name: "TIMESTAMP.ENDOF", Handler: cmdTIMESTAMPENDOF})

	router.Register(&CommandDef{Name: "DIFF.TEXT", Handler: cmdDIFFTEXT})
	router.Register(&CommandDef{Name: "DIFF.JSON", Handler: cmdDIFFJSON})

	router.Register(&CommandDef{Name: "POOL.CREATE", Handler: cmdPOOLCREATE})
	router.Register(&CommandDef{Name: "POOL.GET", Handler: cmdPOOLGET})
	router.Register(&CommandDef{Name: "POOL.PUT", Handler: cmdPOOLPUT})
	router.Register(&CommandDef{Name: "POOL.CLEAR", Handler: cmdPOOLCLEAR})
	router.Register(&CommandDef{Name: "POOL.STATS", Handler: cmdPOOLSTATS})
}

func cmdMSGPACKENCODE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	v, err := decodeOrderedJSON(ctx.Arg(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	encoded, err := msgpackEncode(nil, v)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkBytes(encoded)
}

func cmdMSGPACKDECODE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	d := &binDecoder{data: ctx.Arg(0)}
	v, err := d.msgpackValue(0)
	if err == nil && d.pos != len(d.data) {
		err = errMsgpackInvalid
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	out, err := encodeOrderedJSON(v)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkBytes(out)
}

const maxCodecDepth = 512

var (
	errCodecJSONInput   = errors.New("ERR invalid JSON input")
	errCodecTooDeep     = errors.New("ERR nesting too deep")
	errCodecIntRange    = errors.New("ERR integer out of range for this format")
	errCodecFloatRange  = errors.New("ERR number out of range")
	errCodecNonFinite   = errors.New("ERR NaN and Infinity cannot be represented in JSON")
	errCodecUTF8        = errors.New("ERR string is not valid UTF-8")
	errCodecMapKey      = errors.New("ERR map keys must be strings")
	errMsgpackInvalid   = errors.New("ERR invalid msgpack data")
	errMsgpackUnsupport = errors.New("ERR msgpack bin and ext types are not supported")
	errCBORInvalid      = errors.New("ERR invalid CBOR data")
	errCBORUnsupported  = errors.New("ERR CBOR byte strings, tags and simple values other than false/true/null are not supported")
)

// jsonObject keeps object keys in document order; a duplicate key keeps its
// first position and its last value.
type jsonObject struct {
	keys []string
	vals map[string]any
}

func newJSONObject() *jsonObject {
	return &jsonObject{vals: make(map[string]any)}
}

func (o *jsonObject) set(k string, v any) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func (o *jsonObject) get(k string) (any, bool) {
	v, ok := o.vals[k]
	return v, ok
}

// decodeOrderedJSON parses one JSON document into nil, bool, string,
// json.Number, []any and *jsonObject values.
func decodeOrderedJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := readOrderedJSON(dec, 0)
	if err != nil {
		if err == errCodecTooDeep {
			return nil, err
		}
		return nil, errCodecJSONInput
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errCodecJSONInput
	}
	return v, nil
}

func readOrderedJSON(dec *json.Decoder, depth int) (any, error) {
	if depth > maxCodecDepth {
		return nil, errCodecTooDeep
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	switch delim {
	case '[':
		arr := []any{}
		for dec.More() {
			v, err := readOrderedJSON(dec, depth+1)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return arr, nil
	case '{':
		obj := newJSONObject()
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, ok := kt.(string)
			if !ok {
				return nil, errCodecJSONInput
			}
			v, err := readOrderedJSON(dec, depth+1)
			if err != nil {
				return nil, err
			}
			obj.set(key, v)
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return obj, nil
	}
	return nil, errCodecJSONInput
}

func encodeOrderedJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeOrderedJSON(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeOrderedJSON(buf *bytes.Buffer, v any) error {
	switch t := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		buf.WriteString(strconv.FormatBool(t))
	case string:
		writeJSONString(buf, t)
	case json.Number:
		buf.WriteString(string(t))
	case int64:
		buf.WriteString(strconv.FormatInt(t, 10))
	case uint64:
		buf.WriteString(strconv.FormatUint(t, 10))
	case *big.Int:
		buf.WriteString(t.String())
	case float64:
		s, err := formatJSONFloat(t)
		if err != nil {
			return err
		}
		buf.WriteString(s)
	case []any:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeOrderedJSON(buf, e); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case *jsonObject:
		buf.WriteByte('{')
		for i, k := range t.keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeJSONString(buf, k)
			buf.WriteByte(':')
			if err := writeOrderedJSON(buf, t.vals[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return fmt.Errorf("ERR unsupported value type %T", v)
	}
	return nil
}

func writeJSONString(buf *bytes.Buffer, s string) {
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	buf.Truncate(buf.Len() - 1)
}

// formatJSONFloat always yields a fraction or exponent so that a decoded
// float re-encodes as a float rather than an integer.
func formatJSONFloat(f float64) (string, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "", errCodecNonFinite
	}
	format := byte('f')
	if abs := math.Abs(f); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	s := strconv.FormatFloat(f, format, -1, 64)
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s, nil
}

// classifyJSONNumber returns a float64 for numbers written with a fraction or
// exponent and a *big.Int for integers.
func classifyJSONNumber(n json.Number) (any, error) {
	s := string(n)
	if strings.ContainsAny(s, ".eE") {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, errCodecFloatRange
		}
		return f, nil
	}
	i, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, errCodecJSONInput
	}
	return i, nil
}

func msgpackEncode(out []byte, v any) ([]byte, error) {
	switch t := v.(type) {
	case nil:
		return append(out, 0xc0), nil
	case bool:
		if t {
			return append(out, 0xc3), nil
		}
		return append(out, 0xc2), nil
	case string:
		n := len(t)
		switch {
		case n < 32:
			out = append(out, 0xa0|byte(n))
		case n <= 0xff:
			out = append(out, 0xd9, byte(n))
		case n <= 0xffff:
			out = binary.BigEndian.AppendUint16(append(out, 0xda), uint16(n))
		default:
			out = binary.BigEndian.AppendUint32(append(out, 0xdb), uint32(n))
		}
		return append(out, t...), nil
	case json.Number:
		num, err := classifyJSONNumber(t)
		if err != nil {
			return nil, err
		}
		if f, ok := num.(float64); ok {
			return binary.BigEndian.AppendUint64(append(out, 0xcb), math.Float64bits(f)), nil
		}
		return msgpackEncodeInt(out, num.(*big.Int))
	case []any:
		n := len(t)
		switch {
		case n < 16:
			out = append(out, 0x90|byte(n))
		case n <= 0xffff:
			out = binary.BigEndian.AppendUint16(append(out, 0xdc), uint16(n))
		default:
			out = binary.BigEndian.AppendUint32(append(out, 0xdd), uint32(n))
		}
		var err error
		for _, e := range t {
			if out, err = msgpackEncode(out, e); err != nil {
				return nil, err
			}
		}
		return out, nil
	case *jsonObject:
		n := len(t.keys)
		switch {
		case n < 16:
			out = append(out, 0x80|byte(n))
		case n <= 0xffff:
			out = binary.BigEndian.AppendUint16(append(out, 0xde), uint16(n))
		default:
			out = binary.BigEndian.AppendUint32(append(out, 0xdf), uint32(n))
		}
		var err error
		for _, k := range t.keys {
			if out, err = msgpackEncode(out, k); err != nil {
				return nil, err
			}
			if out, err = msgpackEncode(out, t.vals[k]); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("ERR unsupported value type %T", v)
}

func msgpackEncodeInt(out []byte, i *big.Int) ([]byte, error) {
	if i.Sign() >= 0 {
		if !i.IsUint64() {
			return nil, errCodecIntRange
		}
		u := i.Uint64()
		switch {
		case u <= 0x7f:
			return append(out, byte(u)), nil
		case u <= 0xff:
			return append(out, 0xcc, byte(u)), nil
		case u <= 0xffff:
			return binary.BigEndian.AppendUint16(append(out, 0xcd), uint16(u)), nil
		case u <= 0xffffffff:
			return binary.BigEndian.AppendUint32(append(out, 0xce), uint32(u)), nil
		}
		return binary.BigEndian.AppendUint64(append(out, 0xcf), u), nil
	}
	if !i.IsInt64() {
		return nil, errCodecIntRange
	}
	n := i.Int64()
	switch {
	case n >= -32:
		return append(out, byte(int8(n))), nil
	case n >= math.MinInt8:
		return append(out, 0xd0, byte(int8(n))), nil
	case n >= math.MinInt16:
		return binary.BigEndian.AppendUint16(append(out, 0xd1), uint16(int16(n))), nil
	case n >= math.MinInt32:
		return binary.BigEndian.AppendUint32(append(out, 0xd2), uint32(int32(n))), nil
	}
	return binary.BigEndian.AppendUint64(append(out, 0xd3), uint64(n)), nil
}

type binDecoder struct {
	data []byte
	pos  int
}

func (d *binDecoder) take(n uint64, bad error) ([]byte, error) {
	if n > uint64(len(d.data)-d.pos) {
		return nil, bad
	}
	b := d.data[d.pos : d.pos+int(n)]
	d.pos += int(n)
	return b, nil
}

func (d *binDecoder) uintN(size int, bad error) (uint64, error) {
	b, err := d.take(uint64(size), bad)
	if err != nil {
		return 0, err
	}
	switch size {
	case 1:
		return uint64(b[0]), nil
	case 2:
		return uint64(binary.BigEndian.Uint16(b)), nil
	case 4:
		return uint64(binary.BigEndian.Uint32(b)), nil
	}
	return binary.BigEndian.Uint64(b), nil
}

func decodedUint(u uint64) any {
	if u <= math.MaxInt64 {
		return int64(u)
	}
	return u
}

func (d *binDecoder) msgpackValue(depth int) (any, error) {
	if depth > maxCodecDepth {
		return nil, errCodecTooDeep
	}
	b, err := d.take(1, errMsgpackInvalid)
	if err != nil {
		return nil, err
	}
	c := b[0]
	switch {
	case c <= 0x7f:
		return int64(c), nil
	case c >= 0xe0:
		return int64(int8(c)), nil
	case c&0xf0 == 0x80:
		return d.msgpackMap(uint64(c&0x0f), depth)
	case c&0xf0 == 0x90:
		return d.msgpackArray(uint64(c&0x0f), depth)
	case c&0xe0 == 0xa0:
		return d.msgpackString(uint64(c & 0x1f))
	}
	switch c {
	case 0xc0:
		return nil, nil
	case 0xc2:
		return false, nil
	case 0xc3:
		return true, nil
	case 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8:
		return nil, errMsgpackUnsupport
	case 0xca:
		u, err := d.uintN(4, errMsgpackInvalid)
		if err != nil {
			return nil, err
		}
		return float64(math.Float32frombits(uint32(u))), nil
	case 0xcb:
		u, err := d.uintN(8, errMsgpackInvalid)
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(u), nil
	case 0xcc, 0xcd, 0xce, 0xcf:
		u, err := d.uintN(1<<(c-0xcc), errMsgpackInvalid)
		if err != nil {
			return nil, err
		}
		return decodedUint(u), nil
	case 0xd0, 0xd1, 0xd2, 0xd3:
		size := 1 << (c - 0xd0)
		u, err := d.uintN(size, errMsgpackInvalid)
		if err != nil {
			return nil, err
		}
		switch size {
		case 1:
			return int64(int8(u)), nil
		case 2:
			return int64(int16(u)), nil
		case 4:
			return int64(int32(u)), nil
		}
		return int64(u), nil
	case 0xd9, 0xda, 0xdb:
		n, err := d.uintN(1<<(c-0xd9), errMsgpackInvalid)
		if err != nil {
			return nil, err
		}
		return d.msgpackString(n)
	case 0xdc, 0xdd:
		n, err := d.uintN(2<<(c-0xdc), errMsgpackInvalid)
		if err != nil {
			return nil, err
		}
		return d.msgpackArray(n, depth)
	case 0xde, 0xdf:
		n, err := d.uintN(2<<(c-0xde), errMsgpackInvalid)
		if err != nil {
			return nil, err
		}
		return d.msgpackMap(n, depth)
	}
	return nil, errMsgpackInvalid
}

func (d *binDecoder) msgpackString(n uint64) (any, error) {
	b, err := d.take(n, errMsgpackInvalid)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(b) {
		return nil, errCodecUTF8
	}
	return string(b), nil
}

func (d *binDecoder) msgpackArray(n uint64, depth int) (any, error) {
	if n > uint64(len(d.data)-d.pos) {
		return nil, errMsgpackInvalid
	}
	arr := make([]any, 0, n)
	for i := uint64(0); i < n; i++ {
		v, err := d.msgpackValue(depth + 1)
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
	}
	return arr, nil
}

func (d *binDecoder) msgpackMap(n uint64, depth int) (any, error) {
	if n > uint64(len(d.data)-d.pos)/2 {
		return nil, errMsgpackInvalid
	}
	obj := newJSONObject()
	for i := uint64(0); i < n; i++ {
		k, err := d.msgpackValue(depth + 1)
		if err != nil {
			return nil, err
		}
		key, ok := k.(string)
		if !ok {
			return nil, errCodecMapKey
		}
		v, err := d.msgpackValue(depth + 1)
		if err != nil {
			return nil, err
		}
		obj.set(key, v)
	}
	return obj, nil
}

func cmdURLENCODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.ArgString(0)
	encoded := urlEncode(data)
	return ctx.WriteBulkString(encoded)
}

func urlEncode(s string) string {
	hexChars := "0123456789ABCDEF"
	result := ""

	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			result += string(c)
		} else if c == ' ' {
			result += "+"
		} else {
			result += "%" + string(hexChars[c>>4]) + string(hexChars[c&0x0F])
		}
	}

	return result
}

func cmdURLDECODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.ArgString(0)
	decoded, err := urlDecode(data)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(decoded)
}

func urlDecode(s string) (string, error) {
	var result strings.Builder
	i := 0

	for i < len(s) {
		switch s[i] {
		case '%':
			// A percent-escape must be followed by exactly two hex digits.
			// Decoding anything else silently produced a WRONG BYTE: hexToByte
			// had no else branch, so an invalid digit contributed nothing
			// while still shifting the accumulator — "%zz" yielded a NUL
			// byte and "%4z" yielded '@'. The error return below had always
			// been dead code, so this path was never actually reported.
			if i+3 > len(s) {
				return "", errInvalidPercentEncoding
			}
			val, ok := hexToByte(s[i+1 : i+3])
			if !ok {
				return "", errInvalidPercentEncoding
			}
			result.WriteByte(val)
			i += 3
		case '+':
			result.WriteByte(' ')
			i++
		default:
			result.WriteByte(s[i])
			i++
		}
	}

	return result.String(), nil
}

// hexToByte converts exactly two hex digits to a byte and reports whether both
// digits were valid.
func hexToByte(hex string) (byte, bool) {
	if len(hex) != 2 {
		return 0, false
	}
	var result byte
	for i := 0; i < 2; i++ {
		var digit byte
		switch c := hex[i]; {
		case c >= '0' && c <= '9':
			digit = byte(c - '0')
		case c >= 'A' && c <= 'F':
			digit = byte(c - 'A' + 10)
		case c >= 'a' && c <= 'f':
			digit = byte(c - 'a' + 10)
		default:
			return 0, false
		}
		result = result<<4 | digit
	}
	return result, true
}

var (
	errXMLTagName     = errors.New("ERR invalid XML tag name")
	errXMLChar        = errors.New("ERR value contains characters that XML 1.0 cannot represent")
	errXMLInvalid     = errors.New("ERR invalid XML format")
	errXMLUnsupported = errors.New("ERR only a single element with text content and no attributes is supported")
)

func cmdXMLENCODE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	tag := ctx.ArgString(0)
	value := ctx.ArgString(1)
	if !isXMLTagName(tag) {
		return ctx.WriteError(errXMLTagName)
	}
	if !isXMLText(value) {
		return ctx.WriteError(errXMLChar)
	}

	return ctx.WriteBulkString("<" + tag + ">" + escapeXML(value) + "</" + tag + ">")
}

func isXMLTagName(s string) bool {
	if s == "" || !utf8.ValidString(s) {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || unicode.IsLetter(r):
		case i > 0 && (r == '-' || r == '.' || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r)):
		default:
			return false
		}
	}
	return true
}

func isXMLText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if r < 0x20 || (r >= 0xD800 && r <= 0xDFFF) || r == 0xFFFE || r == 0xFFFF {
			return false
		}
	}
	return true
}

var xmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"\"", "&quot;",
	"'", "&apos;",
	"\r", "&#xD;",
)

func escapeXML(s string) string {
	return xmlEscaper.Replace(s)
}

func cmdXMLDECODE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	tag, value, err := parseSimpleXML(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(tag),
		resp.BulkString(value),
	})
}

// parseSimpleXML accepts one element whose content is text (character data,
// CDATA, comments), optionally surrounded by a prolog, comments and
// whitespace. Attributes, namespaces and child elements are rejected.
func parseSimpleXML(s string) (string, string, error) {
	dec := xml.NewDecoder(strings.NewReader(s))
	var tag string
	var text strings.Builder
	state := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", errXMLInvalid
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if state != 0 {
				return "", "", errXMLUnsupported
			}
			if len(t.Attr) > 0 || t.Name.Space != "" {
				return "", "", errXMLUnsupported
			}
			tag = t.Name.Local
			state = 1
		case xml.EndElement:
			state = 2
		case xml.CharData:
			switch state {
			case 1:
				text.Write(t)
			default:
				if len(bytes.TrimSpace(t)) > 0 {
					return "", "", errXMLInvalid
				}
			}
		case xml.Directive:
			return "", "", errXMLUnsupported
		}
	}
	if state != 2 {
		return "", "", errXMLInvalid
	}
	return tag, text.String(), nil
}

func cmdYAMLENCODE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	v, err := decodeOrderedJSON(ctx.Arg(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	node, err := yamlNodeFromJSON(v)
	if err != nil {
		return ctx.WriteError(err)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return ctx.WriteError(fmt.Errorf("ERR %v", err))
	}
	if err := enc.Close(); err != nil {
		return ctx.WriteError(fmt.Errorf("ERR %v", err))
	}
	return ctx.WriteBulkBytes(buf.Bytes())
}

func yamlNodeFromJSON(v any) (*yaml.Node, error) {
	scalar := func(tag, value string) *yaml.Node {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
	}
	switch t := v.(type) {
	case nil:
		return scalar("!!null", "null"), nil
	case bool:
		return scalar("!!bool", strconv.FormatBool(t)), nil
	case string:
		n := scalar("!!str", t)
		if strings.ContainsAny(t, "\n\r") || !utf8.ValidString(t) {
			n.Style = yaml.DoubleQuotedStyle
		}
		return n, nil
	case json.Number:
		num, err := classifyJSONNumber(t)
		if err != nil {
			return nil, err
		}
		if f, ok := num.(float64); ok {
			s, err := formatJSONFloat(f)
			if err != nil {
				return nil, err
			}
			return scalar("!!float", s), nil
		}
		return scalar("!!int", num.(*big.Int).String()), nil
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, e := range t {
			c, err := yamlNodeFromJSON(e)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, c)
		}
		return n, nil
	case *jsonObject:
		n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for _, k := range t.keys {
			kn, _ := yamlNodeFromJSON(k)
			vn, err := yamlNodeFromJSON(t.vals[k])
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, kn, vn)
		}
		return n, nil
	}
	return nil, fmt.Errorf("ERR unsupported value type %T", v)
}

var (
	errYAMLInvalid     = errors.New("ERR invalid YAML")
	errYAMLMultiDoc    = errors.New("ERR multiple YAML documents are not supported")
	errYAMLUnsupported = errors.New("ERR YAML value has no JSON equivalent (binary, custom tag, merge key or non-scalar key)")
	errYAMLTooLarge    = errors.New("ERR YAML document expands to too many nodes")
)

const maxYAMLNodes = 100000

func cmdYAMLDECODE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	dec := yaml.NewDecoder(bytes.NewReader(ctx.Arg(0)))
	var doc yaml.Node
	var v any
	err := dec.Decode(&doc)
	switch {
	case err == io.EOF:
		v = nil
	case err != nil:
		return ctx.WriteError(errYAMLInvalid)
	default:
		var extra yaml.Node
		if err := dec.Decode(&extra); err != io.EOF {
			if err == nil {
				return ctx.WriteError(errYAMLMultiDoc)
			}
			return ctx.WriteError(errYAMLInvalid)
		}
		budget := maxYAMLNodes
		v, err = yamlNodeToJSON(&doc, 0, &budget)
		if err != nil {
			return ctx.WriteError(err)
		}
	}
	out, err := encodeOrderedJSON(v)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkBytes(out)
}

func yamlNodeToJSON(n *yaml.Node, depth int, budget *int) (any, error) {
	if depth > maxCodecDepth {
		return nil, errCodecTooDeep
	}
	*budget--
	if *budget < 0 {
		return nil, errYAMLTooLarge
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return yamlNodeToJSON(n.Content[0], depth+1, budget)
	case yaml.AliasNode:
		return yamlNodeToJSON(n.Alias, depth+1, budget)
	case yaml.SequenceNode:
		arr := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			v, err := yamlNodeToJSON(c, depth+1, budget)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	case yaml.MappingNode:
		obj := newJSONObject()
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			for k.Kind == yaml.AliasNode {
				k = k.Alias
			}
			if k.Kind != yaml.ScalarNode || k.ShortTag() == "!!merge" || k.ShortTag() == "!!binary" {
				return nil, errYAMLUnsupported
			}
			v, err := yamlNodeToJSON(n.Content[i+1], depth+1, budget)
			if err != nil {
				return nil, err
			}
			obj.set(k.Value, v)
		}
		return obj, nil
	case yaml.ScalarNode:
		switch n.ShortTag() {
		case "!!null":
			return nil, nil
		case "!!bool":
			var b bool
			if err := n.Decode(&b); err != nil {
				return nil, errYAMLInvalid
			}
			return b, nil
		case "!!int":
			var i int64
			if err := n.Decode(&i); err == nil {
				return i, nil
			}
			var u uint64
			if err := n.Decode(&u); err == nil {
				return u, nil
			}
			return nil, errCodecIntRange
		case "!!float":
			var f float64
			if err := n.Decode(&f); err != nil {
				return nil, errYAMLInvalid
			}
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return nil, errCodecNonFinite
			}
			return f, nil
		case "!!str", "!!timestamp":
			return n.Value, nil
		}
	}
	return nil, errYAMLUnsupported
}

func cmdCBORENCODE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	v, err := decodeOrderedJSON(ctx.Arg(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	encoded, err := cborEncode(nil, v)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkBytes(encoded)
}

func cborHead(out []byte, major byte, arg uint64) []byte {
	m := major << 5
	switch {
	case arg < 24:
		return append(out, m|byte(arg))
	case arg <= 0xff:
		return append(out, m|24, byte(arg))
	case arg <= 0xffff:
		return binary.BigEndian.AppendUint16(append(out, m|25), uint16(arg))
	case arg <= 0xffffffff:
		return binary.BigEndian.AppendUint32(append(out, m|26), uint32(arg))
	}
	return binary.BigEndian.AppendUint64(append(out, m|27), arg)
}

var bigOne = big.NewInt(1)

func cborEncode(out []byte, v any) ([]byte, error) {
	switch t := v.(type) {
	case nil:
		return append(out, 0xf6), nil
	case bool:
		if t {
			return append(out, 0xf5), nil
		}
		return append(out, 0xf4), nil
	case string:
		return append(cborHead(out, 3, uint64(len(t))), t...), nil
	case json.Number:
		num, err := classifyJSONNumber(t)
		if err != nil {
			return nil, err
		}
		if f, ok := num.(float64); ok {
			return binary.BigEndian.AppendUint64(append(out, 0xfb), math.Float64bits(f)), nil
		}
		i := num.(*big.Int)
		if i.Sign() >= 0 {
			if !i.IsUint64() {
				return nil, errCodecIntRange
			}
			return cborHead(out, 0, i.Uint64()), nil
		}
		n := new(big.Int).Neg(i)
		n.Sub(n, bigOne)
		if !n.IsUint64() {
			return nil, errCodecIntRange
		}
		return cborHead(out, 1, n.Uint64()), nil
	case []any:
		out = cborHead(out, 4, uint64(len(t)))
		var err error
		for _, e := range t {
			if out, err = cborEncode(out, e); err != nil {
				return nil, err
			}
		}
		return out, nil
	case *jsonObject:
		out = cborHead(out, 5, uint64(len(t.keys)))
		var err error
		for _, k := range t.keys {
			out = append(cborHead(out, 3, uint64(len(k))), k...)
			if out, err = cborEncode(out, t.vals[k]); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("ERR unsupported value type %T", v)
}

func cmdCBORDECODE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	d := &binDecoder{data: ctx.Arg(0)}
	v, err := d.cborValue(0)
	if err == nil && d.pos != len(d.data) {
		err = errCBORInvalid
	}
	if err == errCBORBreak {
		err = errCBORInvalid
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	out, err := encodeOrderedJSON(v)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkBytes(out)
}

var errCBORBreak = errors.New("ERR unexpected CBOR break")

// cborArg reads the argument of an initial byte; indefinite reports ai == 31.
func (d *binDecoder) cborArg(ai byte) (arg uint64, indefinite bool, err error) {
	switch {
	case ai < 24:
		return uint64(ai), false, nil
	case ai <= 27:
		arg, err = d.uintN(1<<(ai-24), errCBORInvalid)
		return arg, false, err
	case ai == 31:
		return 0, true, nil
	}
	return 0, false, errCBORInvalid
}

func (d *binDecoder) cborValue(depth int) (any, error) {
	if depth > maxCodecDepth {
		return nil, errCodecTooDeep
	}
	b, err := d.take(1, errCBORInvalid)
	if err != nil {
		return nil, err
	}
	major, ai := b[0]>>5, b[0]&0x1f
	if major == 7 {
		return d.cborSimple(ai)
	}
	arg, indefinite, err := d.cborArg(ai)
	if err != nil {
		return nil, err
	}
	switch major {
	case 0:
		if indefinite {
			return nil, errCBORInvalid
		}
		return decodedUint(arg), nil
	case 1:
		if indefinite {
			return nil, errCBORInvalid
		}
		if arg <= math.MaxInt64 {
			return -1 - int64(arg), nil
		}
		n := new(big.Int).SetUint64(arg)
		n.Add(n, bigOne)
		return n.Neg(n), nil
	case 2, 6:
		return nil, errCBORUnsupported
	case 3:
		if !indefinite {
			return d.cborText(arg)
		}
		var sb strings.Builder
		for {
			if d.pos < len(d.data) && d.data[d.pos] == 0xff {
				d.pos++
				return sb.String(), nil
			}
			c, err := d.take(1, errCBORInvalid)
			if err != nil {
				return nil, err
			}
			if c[0]>>5 != 3 || c[0]&0x1f == 31 {
				return nil, errCBORInvalid
			}
			n, _, err := d.cborArg(c[0] & 0x1f)
			if err != nil {
				return nil, err
			}
			s, err := d.cborText(n)
			if err != nil {
				return nil, err
			}
			sb.WriteString(s.(string))
		}
	case 4:
		if !indefinite && arg > uint64(len(d.data)-d.pos) {
			return nil, errCBORInvalid
		}
		arr := []any{}
		for i := uint64(0); indefinite || i < arg; i++ {
			v, err := d.cborValue(depth + 1)
			if err == errCBORBreak {
				if indefinite {
					return arr, nil
				}
				return nil, errCBORInvalid
			}
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	}
	if !indefinite && arg > uint64(len(d.data)-d.pos)/2 {
		return nil, errCBORInvalid
	}
	obj := newJSONObject()
	for i := uint64(0); indefinite || i < arg; i++ {
		k, err := d.cborValue(depth + 1)
		if err == errCBORBreak {
			if indefinite {
				return obj, nil
			}
			return nil, errCBORInvalid
		}
		if err != nil {
			return nil, err
		}
		key, ok := k.(string)
		if !ok {
			return nil, errCodecMapKey
		}
		v, err := d.cborValue(depth + 1)
		if err == errCBORBreak {
			return nil, errCBORInvalid
		}
		if err != nil {
			return nil, err
		}
		obj.set(key, v)
	}
	return obj, nil
}

func (d *binDecoder) cborText(n uint64) (any, error) {
	b, err := d.take(n, errCBORInvalid)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(b) {
		return nil, errCodecUTF8
	}
	return string(b), nil
}

func (d *binDecoder) cborSimple(ai byte) (any, error) {
	switch ai {
	case 20:
		return false, nil
	case 21:
		return true, nil
	case 22:
		return nil, nil
	case 25:
		u, err := d.uintN(2, errCBORInvalid)
		if err != nil {
			return nil, err
		}
		return halfToFloat64(uint16(u)), nil
	case 26:
		u, err := d.uintN(4, errCBORInvalid)
		if err != nil {
			return nil, err
		}
		return float64(math.Float32frombits(uint32(u))), nil
	case 27:
		u, err := d.uintN(8, errCBORInvalid)
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(u), nil
	case 31:
		return nil, errCBORBreak
	case 28, 29, 30:
		return nil, errCBORInvalid
	}
	return nil, errCBORUnsupported
}

func halfToFloat64(h uint16) float64 {
	exp := int(h>>10) & 0x1f
	mant := float64(h & 0x3ff)
	var f float64
	switch exp {
	case 0:
		f = math.Ldexp(mant, -24)
	case 31:
		if mant == 0 {
			f = math.Inf(1)
		} else {
			f = math.NaN()
		}
	default:
		f = math.Ldexp(mant+1024, exp-25)
	}
	if h&0x8000 != 0 {
		return -f
	}
	return f
}

func cmdCSVENCODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	values := make([]string, ctx.ArgCount())
	for i := 0; i < ctx.ArgCount(); i++ {
		values[i] = ctx.ArgString(i)
	}

	encoded := csvEncode(values)
	return ctx.WriteBulkString(encoded)
}

func csvEncode(values []string) string {
	result := ""
	for i, v := range values {
		if i > 0 {
			result += ","
		}
		if strings.Contains(v, ",") || strings.Contains(v, "\"") || strings.Contains(v, "\n") {
			v = "\"" + strings.ReplaceAll(v, "\"", "\"\"") + "\""
		}
		result += v
	}
	return result
}

func cmdCSVDECODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.ArgString(0)
	values := csvDecode(data)

	results := make([]*resp.Value, len(values))
	for i, v := range values {
		results[i] = resp.BulkString(v)
	}

	return ctx.WriteArray(results)
}

func csvDecode(s string) []string {
	values := make([]string, 0)
	inQuote := false
	current := ""

	for i := 0; i < len(s); i++ {
		c := s[i]

		if c == '"' {
			if inQuote && i+1 < len(s) && s[i+1] == '"' {
				current += "\""
				i++
			} else {
				inQuote = !inQuote
			}
		} else if c == ',' && !inQuote {
			values = append(values, current)
			current = ""
		} else {
			current += string(c)
		}
	}

	values = append(values, current)
	return values
}

func cmdUUIDGEN(ctx *Context) error {
	uuid := generateUUID()
	return ctx.WriteBulkString(uuid)
}

func generateUUID() string {
	const hexChars = "0123456789abcdef"
	uuid := make([]byte, 36)

	for i := 0; i < 36; i++ {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			uuid[i] = '-'
		} else {
			uuid[i] = hexChars[randomInt(16)]
		}
	}

	uuid[14] = '4'
	uuid[19] = hexChars[8+randomInt(4)]

	return string(uuid)
}

func randomInt(n int) int {
	encodingRandMu.Lock()
	defer encodingRandMu.Unlock()
	return encodingRand.Intn(n)
}

func cmdUUIDVALIDATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	uuid := ctx.ArgString(0)

	if isValidUUID(uuid) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func isValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}

	for i := 0; i < 36; i++ {
		c := s[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else {
			// Every non-separator position must be a hex digit. The four
			// separator positions are already enforced above, so a '-' can
			// never legitimately appear here: excusing it let malformed
			// strings such as "-2345678-e89b-12d3-a456-426614174000" validate,
			// and UUID.VERSION then read that '-' as the version nibble and
			// returned a nonsense version instead of an error.
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
				return false
			}
		}
	}

	return true
}

func cmdUUIDVERSION(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	uuid := ctx.ArgString(0)

	if !isValidUUID(uuid) {
		return ctx.WriteError(fmt.Errorf("ERR invalid UUID"))
	}

	// uuid[14] is the version NIBBLE, so it is one hex digit. Subtracting '0'
	// only works for '0'-'9'; a letter nibble ('a'-'f', 'A'-'F') must decode to
	// 10..15, not to its ASCII offset (49..54 / 17..22). For 'a'-'f' and
	// 'A'-'F' alike, c&0x0f yields 1..6, so +9 gives 10..15.
	c := uuid[14]
	version := int64(c - '0')
	if c >= 'a' || c >= 'A' {
		version = int64(c&0x0f) + 9
	}
	return ctx.WriteInteger(version)
}

func cmdULIDGEN(ctx *Context) error {
	ulid := generateULID()
	return ctx.WriteBulkString(ulid)
}

func generateULID() string {
	const encoding = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	now := time.Now().UnixMilli()

	result := make([]byte, 26)

	for i := 9; i >= 0; i-- {
		result[i] = encoding[now&0x1F]
		now >>= 5
	}

	for i := 10; i < 26; i++ {
		result[i] = encoding[randomInt(32)]
	}

	return string(result)
}

func cmdULIDEXTRACT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ulid := ctx.ArgString(0)

	if len(ulid) != 26 {
		return ctx.WriteError(fmt.Errorf("ERR invalid ULID"))
	}

	timestamp := extractULIDTimestamp(ulid)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("timestamp"),
		resp.IntegerValue(timestamp),
		resp.BulkString("datetime"),
		resp.BulkString(time.UnixMilli(timestamp).UTC().Format(time.RFC3339)),
	})
}

func extractULIDTimestamp(ulid string) int64 {
	const decoding = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

	decodeMap := make(map[byte]int64)
	for i := 0; i < 32; i++ {
		decodeMap[decoding[i]] = int64(i)
	}

	var timestamp int64
	for i := 0; i < 10; i++ {
		timestamp <<= 5
		if val, ok := decodeMap[ulid[i]]; ok {
			timestamp |= val
		}
	}

	return timestamp
}

func cmdTIMESTAMPNOW(ctx *Context) error {
	now := time.Now()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("unix"),
		resp.IntegerValue(now.Unix()),
		resp.BulkString("unix_milli"),
		resp.IntegerValue(now.UnixMilli()),
		resp.BulkString("iso"),
		resp.BulkString(now.UTC().Format(time.RFC3339)),
	})
}

func parseTimestampInput(input string) (time.Time, bool) {
	if n, err := strconv.ParseInt(input, 10, 64); err == nil {
		if len(strings.TrimLeft(input, "+-")) >= 13 {
			return time.UnixMilli(n), true
		}
		return time.Unix(n, 0), true
	}
	for _, layout := range []string{time.RFC3339, time.DateOnly, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, input); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func cmdTIMESTAMPPARSE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	input := ctx.ArgString(0)

	t, ok := parseTimestampInput(input)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR invalid timestamp format"))
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("unix"),
		resp.IntegerValue(t.Unix()),
		resp.BulkString("iso"),
		resp.BulkString(t.UTC().Format(time.RFC3339)),
	})
}

func cmdTIMESTAMPFORMAT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	unix := parseInt64(ctx.ArgString(0))
	layout := ctx.ArgString(1)

	t := time.Unix(unix, 0)
	formatted := t.Format(layout)

	return ctx.WriteBulkString(formatted)
}

func cmdTIMESTAMPADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	unix := parseInt64(ctx.ArgString(0))
	unit := strings.ToLower(ctx.ArgString(1))
	amount := parseInt64(ctx.ArgString(2))

	t := time.Unix(unix, 0)

	switch unit {
	case "second", "seconds":
		t = t.Add(time.Duration(amount) * time.Second)
	case "minute", "minutes":
		t = t.Add(time.Duration(amount) * time.Minute)
	case "hour", "hours":
		t = t.Add(time.Duration(amount) * time.Hour)
	case "day", "days":
		t = t.AddDate(0, 0, int(amount))
	case "week", "weeks":
		t = t.AddDate(0, 0, int(amount)*7)
	case "month", "months":
		t = t.AddDate(0, int(amount), 0)
	case "year", "years":
		t = t.AddDate(int(amount), 0, 0)
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown unit: %s", unit))
	}

	return ctx.WriteInteger(t.Unix())
}

func cmdTIMESTAMPDIFF(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	unix1 := parseInt64(ctx.ArgString(0))
	unix2 := parseInt64(ctx.ArgString(1))
	unit := strings.ToLower(ctx.ArgString(2))

	t1 := time.Unix(unix1, 0)
	t2 := time.Unix(unix2, 0)

	diff := t2.Sub(t1)

	var result int64
	switch unit {
	case "second", "seconds":
		result = int64(diff.Seconds())
	case "minute", "minutes":
		result = int64(diff.Minutes())
	case "hour", "hours":
		result = int64(diff.Hours())
	case "day", "days":
		result = int64(diff.Hours() / 24)
	case "millisecond", "milliseconds":
		result = diff.Milliseconds()
	case "microsecond", "microseconds":
		result = diff.Microseconds()
	case "nanosecond", "nanoseconds":
		result = diff.Nanoseconds()
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown unit: %s", unit))
	}

	return ctx.WriteInteger(result)
}

// timestampPeriodStart truncates t (in UTC) to the start of unit. Weeks start
// on Sunday.
func timestampPeriodStart(t time.Time, unit string) (time.Time, bool) {
	t = t.UTC()
	switch unit {
	case "second", "seconds":
		return t.Truncate(time.Second), true
	case "minute", "minutes":
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC), true
	case "hour", "hours":
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC), true
	case "day", "days":
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
	case "week", "weeks":
		return time.Date(t.Year(), t.Month(), t.Day()-int(t.Weekday()), 0, 0, 0, 0, time.UTC), true
	case "month", "months":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), true
	case "year", "years":
		return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC), true
	}
	return t, false
}

func timestampPeriodArgs(ctx *Context) (time.Time, string, error) {
	unix, err := strconv.ParseInt(ctx.ArgString(0), 10, 64)
	if err != nil {
		return time.Time{}, "", ErrNotInteger
	}
	return time.Unix(unix, 0), strings.ToLower(ctx.ArgString(1)), nil
}

func cmdTIMESTAMPSTARTOF(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	t, unit, err := timestampPeriodArgs(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	start, ok := timestampPeriodStart(t, unit)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR unknown unit: %s", unit))
	}
	return ctx.WriteInteger(start.Unix())
}

// cmdTIMESTAMPENDOF returns the last whole second of the period containing
// the timestamp, i.e. the start of the next period minus one second.
func cmdTIMESTAMPENDOF(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	t, unit, err := timestampPeriodArgs(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	start, ok := timestampPeriodStart(t, unit)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR unknown unit: %s", unit))
	}
	var next time.Time
	switch unit {
	case "second", "seconds":
		next = start.Add(time.Second)
	case "minute", "minutes":
		next = start.Add(time.Minute)
	case "hour", "hours":
		next = start.Add(time.Hour)
	case "day", "days":
		next = start.AddDate(0, 0, 1)
	case "week", "weeks":
		next = start.AddDate(0, 0, 7)
	case "month", "months":
		next = start.AddDate(0, 1, 0)
	default:
		next = start.AddDate(1, 0, 0)
	}
	return ctx.WriteInteger(next.Unix() - 1)
}

func cmdDIFFTEXT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	diff, err := computeTextDiff(ctx.ArgString(0), ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}

	lines := make([]*resp.Value, len(diff.lines))
	for i, l := range diff.lines {
		lines[i] = resp.BulkString(l)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("added"),
		resp.IntegerValue(diff.added),
		resp.BulkString("removed"),
		resp.IntegerValue(diff.removed),
		resp.BulkString("unchanged"),
		resp.IntegerValue(diff.unchanged),
		resp.BulkString("diff"),
		resp.ArrayValue(lines),
	})
}

type textDiff struct {
	added     int64
	removed   int64
	unchanged int64
	lines     []string
}

const (
	maxTextDiffEdits = 2000
	maxTextDiffLines = 200000
)

var errTextDiffTooLarge = fmt.Errorf("ERR texts are too large to diff (more than %d lines or %d changed lines)", maxTextDiffLines, maxTextDiffEdits)

func splitDiffLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// computeTextDiff produces a minimal line diff (Myers). Each output line is
// prefixed with ' ' (unchanged), '-' (only in the first text) or '+' (only in
// the second); within a change, removals precede additions. A trailing
// newline does not start an extra empty line.
func computeTextDiff(text1, text2 string) (textDiff, error) {
	a, b := splitDiffLines(text1), splitDiffLines(text2)
	n, m := len(a), len(b)
	if n+m > maxTextDiffLines {
		return textDiff{}, errTextDiffTooLarge
	}
	maxD := n + m
	if maxD > maxTextDiffEdits {
		maxD = maxTextDiffEdits
	}

	offset := maxD + 1
	v := make([]int, 2*maxD+3)
	var trace [][]int32
	found := false
	for d := 0; d <= maxD && !found; d++ {
		snap := make([]int32, 2*d+1)
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[offset+k] = x
			if x >= n && y >= m {
				found = true
			}
		}
		for k := -d; k <= d; k++ {
			snap[k+d] = int32(v[offset+k])
		}
		trace = append(trace, snap)
	}
	if !found {
		return textDiff{}, errTextDiffTooLarge
	}

	var rev []string
	var diff textDiff
	x, y := n, m
	for d := len(trace) - 1; d >= 0; d-- {
		var prevK int
		k := x - y
		if d > 0 {
			prev := trace[d-1]
			at := func(k int) int32 { return prev[k+d-1] }
			if k == -d || (k != d && at(k-1) < at(k+1)) {
				prevK = k + 1
			} else {
				prevK = k - 1
			}
		}
		prevX := 0
		if d > 0 {
			prevX = int(trace[d-1][prevK+d-1])
		}
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			x--
			y--
			rev = append(rev, " "+a[x])
			diff.unchanged++
		}
		if d == 0 {
			break
		}
		if x == prevX {
			y--
			rev = append(rev, "+"+b[y])
			diff.added++
		} else {
			x--
			rev = append(rev, "-"+a[x])
			diff.removed++
		}
	}

	diff.lines = make([]string, len(rev))
	for i := range rev {
		diff.lines[i] = rev[len(rev)-1-i]
	}
	return diff, nil
}

func cmdDIFFJSON(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	json1 := ctx.ArgString(0)
	json2 := ctx.ArgString(1)

	diff := computeJSONDiff(json1, json2)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("added"),
		resp.IntegerValue(diff.added),
		resp.BulkString("removed"),
		resp.IntegerValue(diff.removed),
		resp.BulkString("changed"),
		resp.IntegerValue(diff.changed),
	})
}

type jsonDiff struct {
	added   int64
	removed int64
	changed int64
}

func computeJSONDiff(json1, json2 string) jsonDiff {
	obj1 := parseJSONObject(json1)
	obj2 := parseJSONObject(json2)

	var diff jsonDiff

	for k := range obj2 {
		if _, exists := obj1[k]; !exists {
			diff.added++
		} else if obj1[k] != obj2[k] {
			diff.changed++
		}
	}

	for k := range obj1 {
		if _, exists := obj2[k]; !exists {
			diff.removed++
		}
	}

	return diff
}

var (
	pools   = make(map[string]*ResourcePool)
	poolsMu sync.RWMutex
)

type ResourcePool struct {
	Name    string
	Items   []string
	MaxSize int
	mu      sync.RWMutex
}

func cmdPOOLCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	maxSize := int(parseInt64(ctx.ArgString(1)))

	poolsMu.Lock()
	pools[name] = &ResourcePool{
		Name:    name,
		Items:   make([]string, 0),
		MaxSize: maxSize,
	}
	poolsMu.Unlock()

	return ctx.WriteOK()
}

func cmdPOOLGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	poolsMu.RLock()
	pool, exists := pools[name]
	poolsMu.RUnlock()

	if !exists {
		return ctx.WriteNull()
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()

	if len(pool.Items) == 0 {
		return ctx.WriteNull()
	}

	item := pool.Items[0]
	pool.Items = pool.Items[1:]

	return ctx.WriteBulkString(item)
}

func cmdPOOLPUT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	item := ctx.ArgString(1)

	poolsMu.RLock()
	pool, exists := pools[name]
	poolsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR pool not found"))
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()

	if len(pool.Items) >= pool.MaxSize {
		return ctx.WriteInteger(0)
	}

	pool.Items = append(pool.Items, item)
	return ctx.WriteInteger(1)
}

func cmdPOOLCLEAR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	poolsMu.RLock()
	pool, exists := pools[name]
	poolsMu.RUnlock()

	if !exists {
		return ctx.WriteOK()
	}

	pool.mu.Lock()
	pool.Items = make([]string, 0)
	pool.mu.Unlock()

	return ctx.WriteOK()
}

func cmdPOOLSTATS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	poolsMu.RLock()
	pool, exists := pools[name]
	poolsMu.RUnlock()

	if !exists {
		return ctx.WriteNull()
	}

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"),
		resp.BulkString(pool.Name),
		resp.BulkString("size"),
		resp.IntegerValue(int64(len(pool.Items))),
		resp.BulkString("max_size"),
		resp.IntegerValue(int64(pool.MaxSize)),
		resp.BulkString("available"),
		resp.IntegerValue(int64(pool.MaxSize - len(pool.Items))),
	})
}
