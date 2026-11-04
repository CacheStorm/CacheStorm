package server

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A write_buffer_size below bufio's 4096-byte default once left every reply
// in a second buffer that was never flushed, so the client got nothing.
func TestSmallWriteBufferStillReplies(t *testing.T) {
	for _, size := range []int{64, 1024, 4095} {
		serverSide, clientSide := net.Pipe()
		s := store.NewStore()
		router := command.NewRouter()
		command.RegisterServerCommands(router)
		conn := newConnection(1, serverSide, s, router, nil, connectionOptions{writeBufferSize: size})
		done := make(chan struct{})
		go func() { defer close(done); conn.Handle() }()

		clientSide.SetDeadline(time.Now().Add(2 * time.Second))
		if _, err := clientSide.Write([]byte("*1\r\n$4\r\nPING\r\n")); err != nil {
			t.Fatal(err)
		}
		line, err := bufio.NewReader(clientSide).ReadString('\n')
		if err != nil || line != "+PONG\r\n" {
			t.Fatalf("write buffer %d: reply %q, %v", size, line, err)
		}
		clientSide.Close()
		<-done
	}
}
