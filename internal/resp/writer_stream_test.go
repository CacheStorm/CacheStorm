package resp

import (
	"bytes"
	"strings"
	"testing"
)

// A reply streamed as a header and items encodes as the same reply built
// from Values, in RESP2 and RESP3, and lengths of 100 and up allocate
// nothing.
func TestStreamedReplyMatchesValues(t *testing.T) {
	long := strings.Repeat("x", 1000)
	for _, resp3 := range []bool{false, true} {
		var streamed, built bytes.Buffer
		sw, bw := NewWriter(&streamed), NewWriter(&built)
		sw.SetRESP3(resp3)
		bw.SetRESP3(resp3)

		sw.WriteMapHeader(2)
		sw.WriteBulkStringItem("f")
		sw.WriteBulkItem([]byte(long))
		sw.WriteBulkStringItem("")
		sw.WriteDoubleItem("1.5")
		sw.WriteSetHeader(1)
		sw.WriteBulkItem(nil)
		sw.WriteArrayHeader(0)
		if err := sw.Flush(); err != nil {
			t.Fatal(err)
		}

		bw.WriteValue(PairsValue([]*Value{BulkString("f"), BulkString(long), BulkString(""), DoubleValue("1.5")}))
		bw.WriteValue(SetValue([]*Value{BulkBytes([]byte{})}))
		bw.WriteArray([]*Value{})
		if streamed.String() != built.String() {
			t.Fatalf("RESP3 %v: streamed %q, built %q", resp3, streamed.String(), built.String())
		}
	}

	w := NewWriter(&bytes.Buffer{})
	b := []byte(long)
	if n := testing.AllocsPerRun(100, func() { w.WriteBulkItem(b); w.WriteArrayHeader(12345) }); n != 0 {
		t.Fatalf("streaming items allocates %v times", n)
	}
}
