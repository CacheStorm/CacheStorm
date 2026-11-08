package store

import (
	"bytes"
	"testing"
)

// A CompactString returns exactly the bytes and encoding it was built from,
// across the uvarint length boundaries, and a string that is not raw is
// stored in this form while a raw one keeps its *StringValue.
func TestCompactStringRoundTrip(t *testing.T) {
	for _, n := range []int{0, 1, 15, 16, 127, 128, 129, 16383, 16384, 1 << 20} {
		data := bytes.Repeat([]byte{'a' + byte(n%26)}, n)
		for _, enc := range []StringEncoding{StringEncAuto, StringEncNoInt, StringEncRaw} {
			c := NewCompactString(data, enc)
			got, gotEnc, ok := StringOf(c)
			if !ok || !bytes.Equal(got, data) || gotEnc != enc || c.Encoding() != enc {
				t.Fatalf("len %d enc %d: got %d bytes enc %d", n, enc, len(got), gotEnc)
			}
			if c.SizeOf() != (&StringValue{Data: data}).SizeOf() {
				t.Fatalf("len %d: SizeOf %d differs from *StringValue's", n, c.SizeOf())
			}
		}
	}

	c := NewCompactString([]byte("hello"), StringEncAuto)
	clone := c.Clone().(*StringValue)
	clone.Data[0] = 'j'
	if c.String() != "hello" {
		t.Fatal("changing a clone changed the CompactString")
	}
	c.MarkRaw()
	if c.Encoding() != StringEncRaw || c.String() != "hello" {
		t.Fatal("MarkRaw changed the bytes or left the encoding")
	}

	s := NewStore()
	if err := s.Set("plain", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	raw := &StringValue{Data: []byte("r"), Enc: StringEncRaw}
	if err := s.Set("raw", raw, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	if e, _ := s.Get("plain"); e == nil {
		t.Fatal("plain missing")
	} else if _, ok := e.Value.(CompactString); !ok {
		t.Fatalf("plain stored as %T, want CompactString", e.Value)
	}
	if e, _ := s.Get("raw"); e == nil || e.Value != Value(raw) {
		t.Fatal("a raw string was not kept as its *StringValue")
	}
}

// A small string's value costs one allocation.
func TestCompactStringAllocations(t *testing.T) {
	data := []byte("0123456789abcdef")
	if n := testing.AllocsPerRun(100, func() {
		var v Value = NewCompactString(data, StringEncAuto)
		_ = v
	}); n != 1 {
		t.Fatalf("NewCompactString made %.0f allocations, want 1", n)
	}
}
