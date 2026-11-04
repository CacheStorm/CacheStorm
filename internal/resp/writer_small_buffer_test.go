package resp

import (
	"bufio"
	"bytes"
	"testing"
)

// A buffered writer smaller than bufio's default is written through, so
// Flush delivers the reply.
func TestNewWriterOverSmallBufferedWriterFlushes(t *testing.T) {
	for _, size := range []int{16, 1024, 4095, 4096, 16384} {
		var out bytes.Buffer
		w := NewWriter(bufio.NewWriterSize(&out, size))
		w.WriteSimpleString("PONG")
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
		if out.String() != "+PONG\r\n" {
			t.Fatalf("size %d: wrote %q", size, out.String())
		}
	}
}
