package benchmarks

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func BenchmarkReadBulkString(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = 'x'
	}

	input := "$1024\r\n" + string(data) + "\r\n"
	reader := bytes.NewReader([]byte(input))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset([]byte(input))
		r := resp.NewReader(reader)
		r.ReadValue()
	}
}

func BenchmarkReadArray(b *testing.B) {
	input := "*10\r\n$3\r\nfoo\r\n$3\r\nbar\r\n$3\r\nbaz\r\n$3\r\nqux\r\n$4\r\nquux\r\n$5\r\ncorge\r\n$6\r\ngrault\r\n$6\r\ngarply\r\n$5\r\nwaldo\r\n$4\r\nfred\r\n"
	reader := bytes.NewReader([]byte(input))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset([]byte(input))
		r := resp.NewReader(reader)
		r.ReadValue()
	}
}

func BenchmarkWriteBulkString(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = 'x'
	}

	var buf bytes.Buffer
	buf.Grow(2048)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		w := resp.NewWriter(&buf)
		w.WriteBulkBytes(data)
	}
}

func BenchmarkWriteArray(b *testing.B) {
	items := make([]*resp.Value, 10)
	for i := 0; i < 10; i++ {
		items[i] = resp.BulkString("value")
	}

	var buf bytes.Buffer
	buf.Grow(1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		w := resp.NewWriter(&buf)
		w.WriteArray(items)
	}
}

func BenchmarkReadCommand(b *testing.B) {
	input := "*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n"
	reader := bytes.NewReader([]byte(input))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset([]byte(input))
		r := resp.NewReader(reader)
		r.ReadCommand()
	}
}
