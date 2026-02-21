package command

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

var encodingRandMu sync.Mutex
var encodingRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func RegisterEncodingCommands(router *Router) {
	router.Register(&CommandDef{Name: "MSGPACK.ENCODE", Handler: cmdMSGPACKENCODE})
	router.Register(&CommandDef{Name: "MSGPACK.DECODE", Handler: cmdMSGPACKDECODE})

	router.Register(&CommandDef{Name: "BSON.ENCODE", Handler: cmdBSONENCODE})
	router.Register(&CommandDef{Name: "BSON.DECODE", Handler: cmdBSONDECODE})

	router.Register(&CommandDef{Name: "URL.ENCODE", Handler: cmdURLENCODE})
	router.Register(&CommandDef{Name: "URL.DECODE", Handler: cmdURLDECODE})

	router.Register(&CommandDef{Name: "XML.ENCODE", Handler: cmdXMLENCODE})
	router.Register(&CommandDef{Name: "XML.DECODE", Handler: cmdXMLDECODE})

	router.Register(&CommandDef{Name: "YAML.ENCODE", Handler: cmdYAMLENCODE})
	router.Register(&CommandDef{Name: "YAML.DECODE", Handler: cmdYAMLDECODE})

	router.Register(&CommandDef{Name: "TOML.ENCODE", Handler: cmdTOMLENCODE})
	router.Register(&CommandDef{Name: "TOML.DECODE", Handler: cmdTOMLDECODE})

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
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	encoded := msgpackEncode(data)
	return ctx.WriteBulkBytes(encoded)
}

func msgpackEncode(data []byte) []byte {
	result := make([]byte, 0)
	result = append(result, 0xDA)
	result = append(result, byte(len(data)>>8), byte(len(data)))
	result = append(result, data...)
	return result
}

func cmdMSGPACKDECODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	decoded, err := msgpackDecode(data)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkBytes(decoded)
}

func msgpackDecode(data []byte) ([]byte, error) {
	if len(data) < 3 || data[0] != 0xDA {
		return nil, fmt.Errorf("invalid msgpack format")
	}
	length := int(data[1])<<8 | int(data[2])
	if len(data) < 3+length {
		return nil, fmt.Errorf("incomplete msgpack data")
	}
	return data[3 : 3+length], nil
}

func cmdBSONENCODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	encoded := bsonEncode(data)
	return ctx.WriteBulkBytes(encoded)
}

func bsonEncode(data []byte) []byte {
	length := int32(len(data) + 5)
	result := make([]byte, 4)
	result[0] = byte(length)
	result[1] = byte(length >> 8)
	result[2] = byte(length >> 16)
	result[3] = byte(length >> 24)
	result = append(result, 0x02)
	result = append(result, data...)
	result = append(result, 0x00)
	return result
}

func cmdBSONDECODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	decoded, err := bsonDecode(data)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkBytes(decoded)
}

func bsonDecode(data []byte) ([]byte, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("invalid bson format")
	}
	length := int32(data[0]) | int32(data[1])<<8 | int32(data[2])<<16 | int32(data[3])<<24
	if len(data) < int(length) {
		return nil, fmt.Errorf("incomplete bson data")
	}
	return data[5 : length-1], nil
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

	for _, c := range s {
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
	result := ""
	i := 0

	for i < len(s) {
		if s[i] == '%' && i+2 < len(s) {
			hex := s[i+1 : i+3]
			val := hexToByte(hex)
			result += string(rune(val))
			i += 3
		} else if s[i] == '+' {
			result += " "
			i++
		} else {
			result += string(s[i])
			i++
		}
	}

	return result, nil
}

func hexToByte(hex string) byte {
	var result byte
	for _, c := range hex {
		result <<= 4
		if c >= '0' && c <= '9' {
			result |= byte(c - '0')
		} else if c >= 'A' && c <= 'F' {
			result |= byte(c - 'A' + 10)
		} else if c >= 'a' && c <= 'f' {
			result |= byte(c - 'a' + 10)
		}
	}
	return result
}

func cmdXMLENCODE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	tag := ctx.ArgString(0)
	value := ctx.ArgString(1)

	encoded := "<" + tag + ">" + escapeXML(value) + "</" + tag + ">"
	return ctx.WriteBulkString(encoded)
}

func escapeXML(s string) string {
	s = replaceAll(s, "&", "&amp;")
	s = replaceAll(s, "<", "&lt;")
	s = replaceAll(s, ">", "&gt;")
	s = replaceAll(s, "\"", "&quot;")
	s = replaceAll(s, "'", "&apos;")
	return s
}

func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old) - 1
		} else {
			result += string(s[i])
		}
	}
	return result
}

func cmdXMLDECODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.ArgString(0)
	tag, value, err := parseSimpleXML(data)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(tag),
		resp.BulkString(value),
	})
}

func parseSimpleXML(s string) (string, string, error) {
	if len(s) < 3 || s[0] != '<' {
		return "", "", fmt.Errorf("invalid XML format")
	}

	endTag := strings.Index(s, ">")
	if endTag == -1 {
		return "", "", fmt.Errorf("invalid XML format")
	}

	tag := s[1:endTag]
	closeTag := "</" + tag + ">"
	closeIdx := strings.Index(s, closeTag)

	if closeIdx == -1 {
		return "", "", fmt.Errorf("invalid XML format")
	}

	value := s[endTag+1 : closeIdx]
	value = unescapeXML(value)

	return tag, value, nil
}

func unescapeXML(s string) string {
	s = replaceAll(s, "&lt;", "<")
	s = replaceAll(s, "&gt;", ">")
	s = replaceAll(s, "&quot;", "\"")
	s = replaceAll(s, "&apos;", "'")
	s = replaceAll(s, "&amp;", "&")
	return s
}

func cmdYAMLENCODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	value := ctx.ArgString(1)

	encoded := key + ": " + value + "\n"
	return ctx.WriteBulkString(encoded)
}

func cmdYAMLDECODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.ArgString(0)
	key, value := parseSimpleYAML(data)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(key),
		resp.BulkString(value),
	})
}

func parseSimpleYAML(s string) (string, string) {
	colonIdx := strings.Index(s, ":")
	if colonIdx == -1 {
		return s, ""
	}

	key := strings.TrimSpace(s[:colonIdx])
	value := strings.TrimSpace(s[colonIdx+1:])

	return key, value
}

func cmdTOMLENCODE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	value := ctx.ArgString(1)

	encoded := key + " = \"" + value + "\"\n"
	return ctx.WriteBulkString(encoded)
}

func cmdTOMLDECODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.ArgString(0)
	key, value := parseSimpleTOML(data)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(key),
		resp.BulkString(value),
	})
}

func parseSimpleTOML(s string) (string, string) {
	eqIdx := strings.Index(s, "=")
	if eqIdx == -1 {
		return s, ""
	}

	key := strings.TrimSpace(s[:eqIdx])
	value := strings.TrimSpace(s[eqIdx+1:])

	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}

	return key, value
}

func cmdCBORENCODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	encoded := cborEncode(data)
	return ctx.WriteBulkBytes(encoded)
}

func cborEncode(data []byte) []byte {
	result := make([]byte, 0)
	result = append(result, 0x78)
	result = append(result, byte(len(data)))
	result = append(result, data...)
	return result
}

func cmdCBORDECODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	decoded, err := cborDecode(data)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkBytes(decoded)
}

func cborDecode(data []byte) ([]byte, error) {
	if len(data) < 2 || data[0] != 0x78 {
		return nil, fmt.Errorf("invalid CBOR format")
	}
	length := int(data[1])
	if len(data) < 2+length {
		return nil, fmt.Errorf("incomplete CBOR data")
	}
	return data[2 : 2+length], nil
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
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
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

	version := uuid[14]
	return ctx.WriteInteger(int64(version - '0'))
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

func cmdTIMESTAMPPARSE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	input := ctx.ArgString(0)

	var t time.Time
	var err error

	if len(input) == 10 {
		var unix int64
		for _, c := range input {
			if c >= '0' && c <= '9' {
				unix = unix*10 + int64(c-'0')
			}
		}
		t = time.Unix(unix, 0)
	} else if len(input) == 13 {
		var unixMilli int64
		for _, c := range input {
			if c >= '0' && c <= '9' {
				unixMilli = unixMilli*10 + int64(c-'0')
			}
		}
		t = time.UnixMilli(unixMilli)
	} else {
		t, err = time.Parse(time.RFC3339, input)
		if err != nil {
			return ctx.WriteError(err)
		}
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

func cmdTIMESTAMPSTARTOF(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	unix := parseInt64(ctx.ArgString(0))
	unit := strings.ToLower(ctx.ArgString(1))

	t := time.Unix(unix, 0)

	switch unit {
	case "second":
		t = time.Unix(t.Unix(), 0)
	case "minute":
		t = time.Unix(t.Unix()-int64(t.Second()), 0)
	case "hour":
		t = time.Unix(t.Unix()-int64(t.Minute()*60+t.Second()), 0)
	case "day":
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case "week":
		weekday := int(t.Weekday())
		t = time.Date(t.Year(), t.Month(), t.Day()-weekday, 0, 0, 0, 0, t.Location())
	case "month":
		t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	case "year":
		t = time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown unit: %s", unit))
	}

	return ctx.WriteInteger(t.Unix())
}

func cmdTIMESTAMPENDOF(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	unix := parseInt64(ctx.ArgString(0))
	unit := strings.ToLower(ctx.ArgString(1))

	t := time.Unix(unix, 0)

	switch unit {
	case "second":
		t = time.Unix(t.Unix(), 0).Add(time.Second - time.Nanosecond)
	case "minute":
		t = time.Unix(t.Unix()-int64(t.Second()), 0).Add(time.Minute - time.Nanosecond)
	case "hour":
		t = time.Unix(t.Unix()-int64(t.Minute()*60+t.Second()), 0).Add(time.Hour - time.Nanosecond)
	case "day":
		t = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
	case "month":
		t = time.Date(t.Year(), t.Month()+1, 0, 23, 59, 59, 999999999, t.Location())
	case "year":
		t = time.Date(t.Year(), 12, 31, 23, 59, 59, 999999999, t.Location())
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown unit: %s", unit))
	}

	return ctx.WriteInteger(t.Unix())
}

func cmdDIFFTEXT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	text1 := ctx.ArgString(0)
	text2 := ctx.ArgString(1)

	diff := computeTextDiff(text1, text2)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("added"),
		resp.IntegerValue(diff.added),
		resp.BulkString("removed"),
		resp.IntegerValue(diff.removed),
		resp.BulkString("unchanged"),
		resp.IntegerValue(diff.unchanged),
	})
}

type textDiff struct {
	added     int64
	removed   int64
	unchanged int64
}

func computeTextDiff(text1, text2 string) textDiff {
	lines1 := strings.Split(text1, "\n")
	lines2 := strings.Split(text2, "\n")

	set1 := make(map[string]bool)
	set2 := make(map[string]bool)

	for _, l := range lines1 {
		set1[l] = true
	}
	for _, l := range lines2 {
		set2[l] = true
	}

	var diff textDiff

	for l := range set2 {
		if !set1[l] {
			diff.added++
		} else {
			diff.unchanged++
		}
	}

	for l := range set1 {
		if !set2[l] {
			diff.removed++
		}
	}

	return diff
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
