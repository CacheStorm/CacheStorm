package resp

import "strconv"

type Type byte

const (
	TypeSimpleString Type = '+'
	TypeError        Type = '-'
	TypeInteger      Type = ':'
	TypeBulkString   Type = '$'
	TypeArray        Type = '*'
	TypeMap          Type = '%'
	TypeNull         Type = '_'
	// TypePush is a RESP3 out-of-band message (pub/sub, invalidation). A
	// RESP2 writer encodes it as an array.
	TypePush Type = '>'
	// TypeSet is a RESP3 set. A RESP2 writer encodes it as an array.
	TypeSet Type = '~'
	// TypeDouble is a RESP3 double; Str holds its text. A RESP2 writer
	// encodes it as a bulk string with the same text.
	TypeDouble Type = ','
	// TypeVerbatim is a RESP3 verbatim string; Str holds its three-letter
	// format and Bulk its text. A RESP2 writer encodes it as a bulk string.
	TypeVerbatim Type = '='
	// TypeBigNumber is a RESP3 big number; Str holds its digits. A RESP2
	// writer encodes it as a bulk string.
	TypeBigNumber Type = '('
	// TypeBoolean is a RESP3 boolean held in Int (1 or 0). A RESP2 writer
	// encodes it as that integer.
	TypeBoolean Type = '#'
)

type Value struct {
	Type   Type
	Str    string
	Int    int64
	Bulk   []byte
	Array  []*Value
	Map    map[string]*Value
	IsNull bool
	Err    string
}

func SimpleString(s string) *Value {
	return &Value{Type: TypeSimpleString, Str: s}
}

func ErrorValue(s string) *Value {
	return &Value{Type: TypeError, Err: s}
}

func IntegerValue(n int64) *Value {
	return &Value{Type: TypeInteger, Int: n}
}

func BulkBytes(b []byte) *Value {
	if b == nil {
		return &Value{Type: TypeBulkString, IsNull: true}
	}
	return &Value{Type: TypeBulkString, Bulk: b}
}

func BulkString(s string) *Value {
	if s == "" {
		return &Value{Type: TypeBulkString, Bulk: []byte{}}
	}
	return &Value{Type: TypeBulkString, Bulk: []byte(s)}
}

func NullValue() *Value {
	return &Value{Type: TypeNull, IsNull: true}
}

func NullBulkString() *Value {
	return &Value{Type: TypeBulkString, IsNull: true}
}

func NullArray() *Value {
	return &Value{Type: TypeArray, IsNull: true}
}

func ArrayValue(items []*Value) *Value {
	return &Value{Type: TypeArray, Array: items}
}

func MapValue(m map[string]*Value) *Value {
	return &Value{Type: TypeMap, Map: m}
}

// PairsValue is a map reply that keeps its order: pairs holds key, value,
// key, value, ... A RESP3 writer encodes it as a map and a RESP2 writer as
// the flat array Redis sends.
func PairsValue(pairs []*Value) *Value {
	return &Value{Type: TypeMap, Array: pairs}
}

// DoubleValue is a RESP3 double whose text is s, as Redis formats it.
// Field returns the value of a map reply's string key, whether the map holds
// a Go map or ordered pairs, or nil.
func (v *Value) Field(key string) *Value {
	if v == nil || v.Type != TypeMap {
		return nil
	}
	if v.Map != nil {
		return v.Map[key]
	}
	for i := 0; i+1 < len(v.Array); i += 2 {
		k := v.Array[i]
		if k.Type == TypeBulkString && string(k.Bulk) == key || k.Type == TypeSimpleString && k.Str == key {
			return v.Array[i+1]
		}
	}
	return nil
}

// BooleanValue is a RESP3 boolean reply, the integer 1 or 0 for RESP2.
func BooleanValue(b bool) *Value {
	v := &Value{Type: TypeBoolean}
	if b {
		v.Int = 1
	}
	return v
}

// BigNumberValue is a RESP3 big number reply of digits.
func BigNumberValue(digits string) *Value {
	return &Value{Type: TypeBigNumber, Str: digits}
}

func DoubleValue(s string) *Value {
	return &Value{Type: TypeDouble, Str: s}
}

// PushValue is a RESP3 push message.
// VerbatimValue returns a verbatim string of format ("txt" or "mkd").
func VerbatimValue(format, text string) *Value {
	return &Value{Type: TypeVerbatim, Str: format, Bulk: []byte(text)}
}

// SetValue is a RESP3 set reply of items, an array for a RESP2 client.
func SetValue(items []*Value) *Value {
	return &Value{Type: TypeSet, Array: items}
}

func PushValue(items []*Value) *Value {
	return &Value{Type: TypePush, Array: items}
}

func OK() *Value {
	return SimpleString("OK")
}

func PONG() *Value {
	return SimpleString("PONG")
}

func Queued() *Value {
	return SimpleString("QUEUED")
}

func (v *Value) String() string {
	switch v.Type {
	case TypeSimpleString:
		return v.Str
	case TypeError:
		return v.Err
	case TypeInteger:
		return strconv.FormatInt(v.Int, 10)
	case TypeBulkString:
		if v.IsNull {
			return "(nil)"
		}
		return string(v.Bulk)
	case TypeArray:
		if v.IsNull {
			return "(nil)"
		}
		return "(array)"
	case TypeNull:
		return "(nil)"
	default:
		return "(unknown)"
	}
}
