package store

import (
	"encoding/binary"
	"errors"
	"hash/fnv"
	"math"
)

// ExtensionValue is a value of a type Redis has no RDB encoding for. It is
// saved through its binary form under a private type byte: in DUMP payloads,
// as a module value in RDB files and as RESTORE in the AOF.
type ExtensionValue interface {
	Value
	ExtensionType() byte
	MarshalBinary() ([]byte, error)
}

// The private type bytes of the extension values. 201 and 202 are JSON
// documents and time series, which persistence encodes itself.
const (
	ExtensionBloom   byte = 203
	ExtensionCuckoo  byte = 204
	ExtensionCMS     byte = 205
	ExtensionTopK    byte = 206
	ExtensionTDigest byte = 207
)

// ErrExtensionFormat reports a corrupt extension value encoding.
var ErrExtensionFormat = errors.New("ERR Bad data format")

var extensionDecoders = map[byte]func([]byte) (Value, error){}

// RegisterExtension registers the decoder of an extension type that lives
// outside this package. Call it from an init function.
func RegisterExtension(t byte, decode func([]byte) (Value, error)) {
	extensionDecoders[t] = decode
}

// DecodeExtension decodes the binary form of an extension value; ok is false
// when t is no extension type.
func DecodeExtension(t byte, data []byte) (v Value, ok bool, err error) {
	decode := extensionDecoders[t]
	if decode == nil {
		return nil, false, nil
	}
	v, err = decode(data)
	return v, true, err
}

// itemHashes returns two independent 64-bit hashes of item, which the
// probabilistic structures combine into as many as they need. They are
// stable across processes, so a saved structure answers the same after a
// restart.
func itemHashes(item []byte) (uint64, uint64) {
	h := fnv.New64a()
	h.Write(item) //nolint:errcheck // hash writes cannot fail
	h1 := h.Sum64()
	h2 := h1 + 0x9e3779b97f4a7c15
	h2 = (h2 ^ h2>>30) * 0xbf58476d1ce4e5b9
	h2 = (h2 ^ h2>>27) * 0x94d049bb133111eb
	h2 ^= h2 >> 31
	return h1, h2 | 1
}

// binWriter and binReader encode the extension values: little-endian fixed
// width numbers and length-prefixed byte strings.
type binWriter struct{ b []byte }

func (w *binWriter) u8(v uint8)   { w.b = append(w.b, v) }
func (w *binWriter) u32(v uint32) { w.b = binary.LittleEndian.AppendUint32(w.b, v) }
func (w *binWriter) u64(v uint64) { w.b = binary.LittleEndian.AppendUint64(w.b, v) }
func (w *binWriter) f64(v float64) {
	w.u64(math.Float64bits(v))
}
func (w *binWriter) bytes(v []byte) {
	w.u64(uint64(len(v)))
	w.b = append(w.b, v...)
}

type binReader struct {
	b   []byte
	err error
}

func (r *binReader) take(n uint64) []byte {
	if r.err != nil || n > uint64(len(r.b)) {
		r.err = ErrExtensionFormat
		return nil
	}
	v := r.b[:n]
	r.b = r.b[n:]
	return v
}
func (r *binReader) u8() uint8 {
	if v := r.take(1); v != nil {
		return v[0]
	}
	return 0
}
func (r *binReader) u32() uint32 {
	if v := r.take(4); v != nil {
		return binary.LittleEndian.Uint32(v)
	}
	return 0
}
func (r *binReader) u64() uint64 {
	if v := r.take(8); v != nil {
		return binary.LittleEndian.Uint64(v)
	}
	return 0
}
func (r *binReader) f64() float64 { return math.Float64frombits(r.u64()) }
func (r *binReader) bytes() []byte {
	n := r.u64()
	return append([]byte(nil), r.take(n)...)
}

// count reads a collection length, refusing one larger than what is left to
// read at minSize bytes per element.
func (r *binReader) count(minSize uint64) uint64 {
	n := r.u64()
	if r.err == nil && minSize > 0 && n > uint64(len(r.b))/minSize {
		r.err = ErrExtensionFormat
		return 0
	}
	return n
}

func (r *binReader) done() error {
	if r.err == nil && len(r.b) != 0 {
		return ErrExtensionFormat
	}
	return r.err
}
