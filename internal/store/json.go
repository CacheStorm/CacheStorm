package store

import (
	"errors"
	"sync"
)

// JSONValue is a JSON document, RedisJSON's ReJSON-RL type.
type JSONValue struct {
	mu   sync.RWMutex
	Root *JNode
}

// ErrJSONIndexRange is JSON.SET's error for an index past an array's end.
var ErrJSONIndexRange = errors.New("ERR array index out of range")

func (v *JSONValue) Lock()    { v.mu.Lock() }
func (v *JSONValue) Unlock()  { v.mu.Unlock() }
func (v *JSONValue) RLock()   { v.mu.RLock() }
func (v *JSONValue) RUnlock() { v.mu.RUnlock() }

// ParseJSONValue parses a document.
func ParseJSONValue(data []byte) (*JSONValue, error) {
	root, err := ParseJSON(string(data))
	if err != nil {
		return nil, err
	}
	return &JSONValue{Root: root}, nil
}

// MustParseJSONValue parses a document known to be valid.
func MustParseJSONValue(data string) *JSONValue {
	v, err := ParseJSONValue([]byte(data))
	if err != nil {
		panic(err)
	}
	return v
}

// Bytes returns the document serialized.
func (v *JSONValue) Bytes() []byte {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return AppendJSON(nil, v.Root)
}

func (v *JSONValue) Type() DataType { return DataTypeJSON }

func (v *JSONValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Root.MemorySize()
}

func (v *JSONValue) String() string { return string(v.Bytes()) }

func (v *JSONValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return &JSONValue{Root: v.Root.Clone()}
}
