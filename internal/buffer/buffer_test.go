package buffer

import (
	"testing"
)

func TestBufferPool(t *testing.T) {
	pool := newSyncPool(64, 1024)

	buf := pool.Get()
	if buf == nil {
		t.Fatal("expected buffer")
	}

	buf.WriteString("hello")
	pool.Put(buf)

	buf2 := pool.Get()
	if buf2.Len() != 0 {
		t.Error("buffer should be reset")
	}
	pool.Put(buf2)
}

func TestBufferPoolPutNil(t *testing.T) {
	pool := newSyncPool(64, 1024)
	pool.Put(nil)
}

func TestBufferPoolPutWrongSize(t *testing.T) {
	pool := newSyncPool(64, 1024)
	buf := pool.Get()
	buf.Grow(20000)
	pool.Put(buf)
}

func TestGetBuffer(t *testing.T) {
	tests := []struct {
		size     int
		expected string
	}{
		{100, "small"},
		{1024, "small"},
		{2000, "medium"},
		{16384, "medium"},
		{20000, "large"},
	}

	for _, tt := range tests {
		buf := GetBuffer(tt.size)
		if buf == nil {
			t.Errorf("expected buffer for size %d", tt.size)
		}
		PutBuffer(buf)
	}
}

func TestPutBuffer(t *testing.T) {
	buf := GetBuffer(100)
	buf.WriteString("test")
	PutBuffer(buf)
}

func TestByteBufferWrite(t *testing.T) {
	b := NewByteBuffer(64)

	n, err := b.Write([]byte("hello"))
	if err != nil || n != 5 {
		t.Errorf("expected 5, got %d, err: %v", n, err)
	}

	if string(b.Bytes()) != "hello" {
		t.Errorf("expected hello, got %s", b.Bytes())
	}
}

func TestByteBufferWriteByte(t *testing.T) {
	b := NewByteBuffer(64)

	if err := b.WriteByte('a'); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if b.String() != "a" {
		t.Errorf("expected a, got %s", b.String())
	}
}

func TestByteBufferWriteString(t *testing.T) {
	b := NewByteBuffer(64)

	n, err := b.WriteString("hello world")
	if err != nil || n != 11 {
		t.Errorf("expected 11, got %d, err: %v", n, err)
	}

	if b.String() != "hello world" {
		t.Errorf("expected hello world, got %s", b.String())
	}
}

func TestByteBufferLen(t *testing.T) {
	b := NewByteBuffer(64)
	b.WriteString("hello")

	if b.Len() != 5 {
		t.Errorf("expected 5, got %d", b.Len())
	}
}

func TestByteBufferCap(t *testing.T) {
	b := NewByteBuffer(64)

	if b.Cap() < 64 {
		t.Errorf("expected cap >= 64, got %d", b.Cap())
	}
}

func TestByteBufferReset(t *testing.T) {
	b := NewByteBuffer(64)
	b.WriteString("hello")
	b.Reset()

	if b.Len() != 0 {
		t.Errorf("expected 0 after reset, got %d", b.Len())
	}
}

func TestByteBufferGrow(t *testing.T) {
	b := NewByteBuffer(10)
	b.WriteString("hello")

	b.Grow(100)

	if b.Cap() < 105 {
		t.Errorf("expected cap >= 105, got %d", b.Cap())
	}
}

func TestByteBufferGrowNoop(t *testing.T) {
	b := NewByteBuffer(100)
	b.WriteString("hello")

	oldCap := b.Cap()
	b.Grow(10)

	if b.Cap() != oldCap {
		t.Error("grow should be noop when enough capacity")
	}
}

func TestGetPutByteBuffer(t *testing.T) {
	b := GetByteBuffer()
	if b == nil {
		t.Fatal("expected byte buffer")
	}

	b.WriteString("test")
	PutByteBuffer(b)

	b2 := GetByteBuffer()
	if b2.Len() != 0 {
		t.Error("byte buffer should be reset")
	}
	PutByteBuffer(b2)
}

func TestPutByteBufferNil(t *testing.T) {
	PutByteBuffer(nil)
}

func TestPutByteBufferTooLarge(t *testing.T) {
	b := NewByteBuffer(100000)
	PutByteBuffer(b)
}

func TestSlicePool(t *testing.T) {
	pool := NewSlicePool[int](10)

	slice := pool.Get()
	if slice == nil {
		t.Fatal("expected slice")
	}

	*slice = append(*slice, 1, 2, 3)
	pool.Put(slice)

	slice2 := pool.Get()
	if len(*slice2) != 0 {
		t.Error("slice should be reset")
	}
	pool.Put(slice2)
}

func TestSlicePoolPutNil(t *testing.T) {
	pool := NewSlicePool[int](10)
	pool.Put(nil)
}

func TestSlicePoolPutTooLarge(t *testing.T) {
	pool := NewSlicePool[int](10)
	slice := make([]int, 20000)
	pool.Put(&slice)
}

func TestMapPool(t *testing.T) {
	pool := NewMapPool[string, int](10)

	m := pool.Get()
	if m == nil {
		t.Fatal("expected map")
	}

	(*m)["key"] = 1
	pool.Put(m)

	m2 := pool.Get()
	if len(*m2) != 0 {
		t.Error("map should be cleared")
	}
	pool.Put(m2)
}

func TestMapPoolPutNil(t *testing.T) {
	pool := NewMapPool[string, int](10)
	pool.Put(nil)
}

func TestMapPoolPutTooLarge(t *testing.T) {
	pool := NewMapPool[string, int](10)
	m := make(map[string]int)
	for i := 0; i < 2000; i++ {
		m["key"+string(rune(i))] = i
	}
	pool.Put(&m)
}

func TestByteBufferBytes(t *testing.T) {
	b := NewByteBuffer(64)
	b.WriteString("test")

	bytes := b.Bytes()
	if string(bytes) != "test" {
		t.Errorf("expected test, got %s", bytes)
	}
}
