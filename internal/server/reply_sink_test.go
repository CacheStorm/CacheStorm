package server

import (
	"bytes"
	"io"
	"net"
	"testing"
)

// Output held past maxKeptReplyBuffer moves to a lent buffer and reaches the
// socket whole and in order; afterwards the connection keeps only its own
// small buffer.
func TestReplySinkLendsBigBuffers(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	got := make(chan []byte)
	go func() { b, _ := io.ReadAll(client); got <- b }()

	s := &replySink{conn: server}
	var want bytes.Buffer
	for round := range 3 {
		s.hold()
		for i := range 400 {
			chunk := bytes.Repeat([]byte{byte('a' + (round+i)%26)}, 1+i%300)
			want.Write(chunk)
			if _, err := s.Write(chunk); err != nil {
				t.Fatal(err)
			}
		}
		if s.big == nil {
			t.Fatalf("round %d: %d bytes held without a lent buffer", round, s.pending())
		}
		if s.buf.Cap() > maxKeptReplyBuffer {
			t.Fatalf("round %d: own buffer grew to %d", round, s.buf.Cap())
		}
		if err := s.release(); err != nil {
			t.Fatal(err)
		}
		if s.big != nil || s.pending() != 0 {
			t.Fatalf("round %d: release kept %v, %d bytes", round, s.big != nil, s.pending())
		}
	}
	s.hold()
	s.Write([]byte("tail"))
	want.WriteString("tail")
	s.release()
	server.Close()
	if b := <-got; !bytes.Equal(b, want.Bytes()) {
		t.Fatalf("socket got %d bytes, want %d in order", len(b), want.Len())
	}
}
