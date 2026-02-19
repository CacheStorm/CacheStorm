package resp

type Type byte

const (
	TypeSimpleString Type = '+'
	TypeError        Type = '-'
	TypeInteger      Type = ':'
	TypeBulkString   Type = '$'
	TypeArray        Type = '*'
	TypeMap          Type = '%'
	TypeNull         Type = '_'
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
		return string(rune(v.Int))
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
