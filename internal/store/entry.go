package store

import (
	"sync"
	"time"
)

type DataType uint8

const (
	DataTypeString DataType = iota + 1
	DataTypeHash
	DataTypeList
	DataTypeSet
	DataTypeSortedSet
	DataTypeStream
	DataTypeGeo
)

func (dt DataType) String() string {
	switch dt {
	case DataTypeString:
		return "string"
	case DataTypeHash:
		return "hash"
	case DataTypeList:
		return "list"
	case DataTypeSet:
		return "set"
	case DataTypeSortedSet:
		return "zset"
	case DataTypeStream:
		return "stream"
	case DataTypeGeo:
		return "geo"
	default:
		return "unknown"
	}
}

type Value interface {
	Type() DataType
	SizeOf() int64
	Clone() Value
	String() string
}

type StringValue struct {
	Data []byte
}

func (v *StringValue) Type() DataType { return DataTypeString }
func (v *StringValue) SizeOf() int64  { return int64(len(v.Data)) + 24 }
func (v *StringValue) String() string { return string(v.Data) }
func (v *StringValue) Clone() Value {
	cloned := make([]byte, len(v.Data))
	copy(cloned, v.Data)
	return &StringValue{Data: cloned}
}

type HashValue struct {
	Fields map[string][]byte
	mu     sync.RWMutex
}

func (v *HashValue) Lock()    { v.mu.Lock() }
func (v *HashValue) Unlock()  { v.mu.Unlock() }
func (v *HashValue) RLock()   { v.mu.RLock() }
func (v *HashValue) RUnlock() { v.mu.RUnlock() }

func (v *HashValue) Type() DataType { return DataTypeHash }
func (v *HashValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	var size int64 = 48
	for k, val := range v.Fields {
		size += int64(len(k)) + int64(len(val)) + 80
	}
	return size
}
func (v *HashValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := ""
	for k, val := range v.Fields {
		if result != "" {
			result += ", "
		}
		result += k + ": " + string(val)
	}
	return result
}
func (v *HashValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	cloned := &HashValue{Fields: make(map[string][]byte, len(v.Fields))}
	for k, val := range v.Fields {
		cv := make([]byte, len(val))
		copy(cv, val)
		cloned.Fields[k] = cv
	}
	return cloned
}

type ListValue struct {
	Elements [][]byte
	mu       sync.RWMutex
}

func (v *ListValue) Lock()    { v.mu.Lock() }
func (v *ListValue) Unlock()  { v.mu.Unlock() }
func (v *ListValue) RLock()   { v.mu.RLock() }
func (v *ListValue) RUnlock() { v.mu.RUnlock() }

func (v *ListValue) Type() DataType { return DataTypeList }
func (v *ListValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	var size int64 = 24
	for _, el := range v.Elements {
		size += int64(len(el)) + 24
	}
	return size
}
func (v *ListValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := ""
	for _, el := range v.Elements {
		if result != "" {
			result += ", "
		}
		result += string(el)
	}
	return result
}
func (v *ListValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	cloned := &ListValue{Elements: make([][]byte, len(v.Elements))}
	for i, el := range v.Elements {
		cel := make([]byte, len(el))
		copy(cel, el)
		cloned.Elements[i] = cel
	}
	return cloned
}

type SetValue struct {
	Members map[string]struct{}
	mu      sync.RWMutex
}

func (v *SetValue) Lock()    { v.mu.Lock() }
func (v *SetValue) Unlock()  { v.mu.Unlock() }
func (v *SetValue) RLock()   { v.mu.RLock() }
func (v *SetValue) RUnlock() { v.mu.RUnlock() }

func (v *SetValue) Type() DataType { return DataTypeSet }
func (v *SetValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	var size int64 = 48
	for k := range v.Members {
		size += int64(len(k)) + 48
	}
	return size
}
func (v *SetValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := ""
	for k := range v.Members {
		if result != "" {
			result += ", "
		}
		result += k
	}
	return result
}
func (v *SetValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	cloned := &SetValue{Members: make(map[string]struct{}, len(v.Members))}
	for k := range v.Members {
		cloned.Members[k] = struct{}{}
	}
	return cloned
}

type Entry struct {
	Value       Value
	Tags        []string
	ExpiresAt   int64
	CreatedAt   int64
	LastAccess  int64
	AccessCount uint64
}

func NewEntry(value Value) *Entry {
	now := time.Now().UnixNano()
	return &Entry{
		Value:      value,
		CreatedAt:  now,
		LastAccess: now,
	}
}

func (e *Entry) IsExpired() bool {
	if e.ExpiresAt == 0 {
		return false
	}
	return time.Now().UnixNano() > e.ExpiresAt
}

func (e *Entry) MemoryUsage() int64 {
	var size int64 = 64
	size += e.Value.SizeOf()
	for _, tag := range e.Tags {
		size += int64(len(tag)) + 16
	}
	return size
}

func (e *Entry) Touch() {
	e.LastAccess = time.Now().UnixNano()
	e.AccessCount++
}

func (e *Entry) SetTTL(ttl time.Duration) {
	e.ExpiresAt = time.Now().Add(ttl).UnixNano()
}

func (e *Entry) SetExpiresAt(expiresAt int64) {
	e.ExpiresAt = expiresAt
}

func (e *Entry) TTL() time.Duration {
	if e.ExpiresAt == 0 {
		return -1
	}
	remaining := time.Until(time.Unix(0, e.ExpiresAt))
	if remaining < 0 {
		return -2
	}
	return remaining
}
