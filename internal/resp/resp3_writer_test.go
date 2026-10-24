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
