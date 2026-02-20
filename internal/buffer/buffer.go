package buffer

import (
	"bytes"
	"sync"
)

const (
	minBufferSize = 64
	maxBufferSize = 65536
)

var (
	smallPool  = newSyncPool(minBufferSize, 1024)
	mediumPool = newSyncPool(1024, 16384)
	largePool  = newSyncPool(16384, maxBufferSize)
)

type bufferPool struct {
	min int
	max int
	p   sync.Pool
}

func newSyncPool(min, max int) *bufferPool {
	return &bufferPool{
		min: min,
		max: max,
		p: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, min))
			},
		},
	}
}

func (bp *bufferPool) Get() *bytes.Buffer {
	buf := bp.p.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func (bp *bufferPool) Put(buf *bytes.Buffer) {
	if buf == nil {
		return
	}

	cap := buf.Cap()
	if cap < bp.min || cap > bp.max {
		return
	}

	bp.p.Put(buf)
}

func GetBuffer(size int) *bytes.Buffer {
	switch {
	case size <= 1024:
		return smallPool.Get()
	case size <= 16384:
		return mediumPool.Get()
	default:
		return largePool.Get()
	}
}

func PutBuffer(buf *bytes.Buffer) {
	switch {
	case buf.Cap() <= 1024:
		smallPool.Put(buf)
	case buf.Cap() <= 16384:
		mediumPool.Put(buf)
	default:
		largePool.Put(buf)
	}
}

type ByteBuffer struct {
	b []byte
}

func NewByteBuffer(cap int) *ByteBuffer {
	return &ByteBuffer{b: make([]byte, 0, cap)}
}

func (b *ByteBuffer) Write(p []byte) (int, error) {
	b.b = append(b.b, p...)
	return len(p), nil
}

func (b *ByteBuffer) WriteByte(c byte) error {
	b.b = append(b.b, c)
	return nil
}

func (b *ByteBuffer) WriteString(s string) (int, error) {
	b.b = append(b.b, s...)
	return len(s), nil
}

func (b *ByteBuffer) Bytes() []byte {
	return b.b
}

func (b *ByteBuffer) String() string {
	return string(b.b)
}

func (b *ByteBuffer) Len() int {
	return len(b.b)
}

func (b *ByteBuffer) Cap() int {
	return cap(b.b)
}

func (b *ByteBuffer) Reset() {
	b.b = b.b[:0]
}

func (b *ByteBuffer) Grow(n int) {
	if cap(b.b)-len(b.b) < n {
		newCap := cap(b.b) * 2
		if newCap < len(b.b)+n {
			newCap = len(b.b) + n
		}
		newBuf := make([]byte, len(b.b), newCap)
		copy(newBuf, b.b)
		b.b = newBuf
	}
}

var byteBufferPool = sync.Pool{
	New: func() interface{} {
		return NewByteBuffer(1024)
	},
}

func GetByteBuffer() *ByteBuffer {
	return byteBufferPool.Get().(*ByteBuffer)
}

func PutByteBuffer(buf *ByteBuffer) {
	if buf == nil || buf.Cap() > maxBufferSize {
		return
	}
	buf.Reset()
	byteBufferPool.Put(buf)
}

type SlicePool[T any] struct {
	p sync.Pool
}

func NewSlicePool[T any](initialCap int) *SlicePool[T] {
	return &SlicePool[T]{
		p: sync.Pool{
			New: func() interface{} {
				slice := make([]T, 0, initialCap)
				return &slice
			},
		},
	}
}

func (sp *SlicePool[T]) Get() *[]T {
	slice := sp.p.Get().(*[]T)
	*slice = (*slice)[:0]
	return slice
}

func (sp *SlicePool[T]) Put(slice *[]T) {
	if slice == nil || cap(*slice) > 10000 {
		return
	}
	sp.p.Put(slice)
}

type MapPool[K comparable, V any] struct {
	p sync.Pool
}

func NewMapPool[K comparable, V any](initialSize int) *MapPool[K, V] {
	return &MapPool[K, V]{
		p: sync.Pool{
			New: func() interface{} {
				m := make(map[K]V, initialSize)
				return &m
			},
		},
	}
}

func (mp *MapPool[K, V]) Get() *map[K]V {
	m := mp.p.Get().(*map[K]V)
	for k := range *m {
		delete(*m, k)
	}
	return m
}

func (mp *MapPool[K, V]) Put(m *map[K]V) {
	if m == nil || len(*m) > 1000 {
		return
	}
	mp.p.Put(m)
}
