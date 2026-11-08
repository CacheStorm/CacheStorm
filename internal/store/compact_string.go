package store

import (
	"encoding/binary"
	"unsafe"
)

// CompactString is a string value that is not changed in place, held in a
// single allocation: one encoding byte, the length as a uvarint, then the
// bytes. Being one pointer it converts to a Value without allocating, so a
// key set with SET costs one small pointer-free object for its value, where
// a *StringValue costs a 32-byte header plus a second allocation for the
// bytes. setLocked stores every string that is not raw in this form; a
// command that changes a string builds a new *StringValue, as it already
// must for one that is not raw.
type CompactString struct{ p *byte }

// NewCompactString copies data into a CompactString with encoding enc.
func NewCompactString(data []byte, enc StringEncoding) CompactString {
	var hdr [1 + binary.MaxVarintLen64]byte
	hdr[0] = byte(enc)
	n := 1 + binary.PutUvarint(hdr[1:], uint64(len(data)))
	buf := make([]byte, n+len(data))
	copy(buf, hdr[:n])
	copy(buf[n:], data)
	return CompactString{p: &buf[0]}
}

func (c CompactString) at(i int) byte { return *(*byte)(unsafe.Add(unsafe.Pointer(c.p), i)) }

// parts returns the encoding and the bytes. The bytes are shared: callers
// must not modify them.
func (c CompactString) parts() (StringEncoding, []byte) {
	var n uint64
	var shift uint
	i := 1
	for {
		b := c.at(i)
		i++
		n |= uint64(b&0x7f) << shift
		if b < 0x80 {
			break
		}
		shift += 7
	}
	if n == 0 {
		return StringEncoding(c.at(0)), []byte{}
	}
	return StringEncoding(c.at(0)), unsafe.Slice((*byte)(unsafe.Add(unsafe.Pointer(c.p), i)), int(n))
}

// Bytes returns the string's bytes, which the caller must not modify.
func (c CompactString) Bytes() []byte { _, b := c.parts(); return b }

// Encoding returns how the string was built (see StringEncoding).
func (c CompactString) Encoding() StringEncoding { return StringEncoding(c.at(0)) }

func (c CompactString) Type() DataType { return DataTypeString }

// SizeOf matches *StringValue's for the same bytes, so a string is charged
// the same in either form.
func (c CompactString) SizeOf() int64  { return int64(len(c.Bytes())) + 24 }
func (c CompactString) String() string { return string(c.Bytes()) }

// Clone returns a *StringValue copy, which the caller may change.
func (c CompactString) Clone() Value {
	enc, b := c.parts()
	return &StringValue{Data: append([]byte(nil), b...), Enc: enc}
}

// StringOf returns the bytes and encoding of a value held in either string
// form. The bytes of a CompactString are shared and must not be modified.
func StringOf(v Value) ([]byte, StringEncoding, bool) {
	switch s := v.(type) {
	case *StringValue:
		return s.Data, s.Enc, true
	case CompactString:
		enc, b := s.parts()
		return b, enc, true
	}
	return nil, 0, false
}

// compactValue returns v in the form setLocked stores: a string that is not
// raw becomes a CompactString.
func compactValue(v Value) Value {
	if sv, ok := v.(*StringValue); ok && sv.Enc != StringEncRaw {
		return NewCompactString(sv.Data, sv.Enc)
	}
	return v
}

// MarkRaw gives the string the raw encoding in place, as Redis' bit writes
// do to a string they leave unchanged. The caller holds the shard lock.
func (c CompactString) MarkRaw() { *c.p = byte(StringEncRaw) }

// AsStringValue returns a string value in either form as a *StringValue.
// For a CompactString it is a new header over the shared bytes, which the
// caller must not modify; it is for readers that want one shape.
func AsStringValue(v Value) (*StringValue, bool) {
	switch s := v.(type) {
	case *StringValue:
		return s, true
	case CompactString:
		enc, b := s.parts()
		return &StringValue{Data: b, Enc: enc}, true
	}
	return nil, false
}
