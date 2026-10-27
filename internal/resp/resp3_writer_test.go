package resp

import (
	"bytes"
	"io"
	"testing"
)

// newRESP3Writer returns a writer that uses the RESP3 null and map encodings,
// for the tests that pin those encodings.
func newRESP3Writer(w io.Writer) *Writer {
	wr := NewWriter(w)
	wr.SetRESP3(true)
	return wr
}

func TestWriterDefaultsToRESP2(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WriteNull(); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteValue(ArrayValue([]*Value{NullValue()})); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteValue(MapValue(map[string]*Value{"k": IntegerValue(1)})); err != nil {
		t.Fatal(err)
	}
	want := "$-1\r\n*1\r\n$-1\r\n*2\r\n$1\r\nk\r\n:1\r\n"
	if got := buf.String(); got != want {
		t.Fatalf("RESP2 encoding %q, want %q", got, want)
	}
}

func TestRESP3PushAndPairs(t *testing.T) {
	push := PushValue([]*Value{BulkString("invalidate"), ArrayValue([]*Value{BulkString("k")})})
	pairs := PairsValue([]*Value{BulkString("b"), IntegerValue(2), BulkString("a"), IntegerValue(1)})
	for _, tc := range []struct {
		resp3 bool
		value *Value
		want  string
	}{
		{false, push, "*2\r\n$10\r\ninvalidate\r\n*1\r\n$1\r\nk\r\n"},
		{true, push, ">2\r\n$10\r\ninvalidate\r\n*1\r\n$1\r\nk\r\n"},
		{false, pairs, "*4\r\n$1\r\nb\r\n:2\r\n$1\r\na\r\n:1\r\n"},
		{true, pairs, "%2\r\n$1\r\nb\r\n:2\r\n$1\r\na\r\n:1\r\n"},
		{true, NullBulkString(), "_\r\n"},
		{false, NullBulkString(), "$-1\r\n"},
		{true, NullArray(), "_\r\n"},
	} {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		w.SetRESP3(tc.resp3)
		if err := w.WriteValue(tc.value); err != nil {
			t.Fatal(err)
		}
		if buf.String() != tc.want {
			t.Errorf("resp3=%v: wrote %q, want %q", tc.resp3, buf.String(), tc.want)
		}
	}
}

func TestVerbatimRoundTrip(t *testing.T) {
	for _, resp3 := range []bool{false, true} {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		w.SetRESP3(resp3)
		if err := w.WriteVerbatim("txt", "a:b\r\n"); err != nil {
			t.Fatal(err)
		}
		want := "$5\r\na:b\r\n\r\n"
		if resp3 {
			want = "=9\r\ntxt:a:b\r\n\r\n"
		}
		if buf.String() != want {
			t.Fatalf("resp3=%v: %q", resp3, buf.String())
		}
		v, err := NewReader(&buf).ReadValue()
		if err != nil || string(v.Bulk) != "a:b\r\n" {
			t.Fatalf("read back: %v %+v", err, v)
		}
	}
}
