package store

import (
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
}

type StringValue struct {
	Data []byte
}

func (v *StringValue) Type() DataType { return DataTypeString }
func (v *StringValue) SizeOf() int64  { return int64(len(v.Data)) + 24 }
func (v *StringValue) Clone() Value {
	cloned := make([]byte, len(v.Data))
	copy(cloned, v.Data)
	return &StringValue{Data: cloned}
}

type HashValue struct {
	Fields map[string][]byte
}

func (v *HashValue) Type() DataType { return DataTypeHash }
func (v *HashValue) SizeOf() int64 {
	var size int64 = 48
	for k, val := range v.Fields {
		size += int64(len(k)) + int64(len(val)) + 80
	}
	return size
}
func (v *HashValue) Clone() Value {
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
}

func (v *ListValue) Type() DataType { return DataTypeList }
func (v *ListValue) SizeOf() int64 {
	var size int64 = 24
	for _, el := range v.Elements {
		size += int64(len(el)) + 24
	}
	return size
}
func (v *ListValue) Clone() Value {
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
}

func (v *SetValue) Type() DataType { return DataTypeSet }
func (v *SetValue) SizeOf() int64 {
	var size int64 = 48
	for k := range v.Members {
		size += int64(len(k)) + 48
	}
	return size
}
func (v *SetValue) Clone() Value {
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
