package server

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

func idleReadConn(t *testing.T, readBuf int) (net.Conn, *bufio.Reader, func()) {
	t.Helper()
	serverSide, clientSide := net.Pipe()
	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterServerCommands(router)
	command.RegisterStringCommands(router)
	conn := newConnection(1, serverSide, s, router, nil, connectionOptions{readBufferSize: readBuf})
	done := make(chan struct{})
	go func() { defer close(done); conn.Handle() }()
	clientSide.SetDeadline(time.Now().Add(10 * time.Second))
	return clientSide, bufio.NewReader(clientSide), func() { clientSide.Close(); <-done }
}

// A connection waiting for its next command holds no read buffer, so the
// command is read partly into the small wait array and partly through a
// buffer taken afterwards: commands split anywhere, and pipelines larger
// than both, are read whole and in order.
func TestIdleConnectionReadsCommandsAcrossTheWait(t *testing.T) {
	t.Run("byte by byte", func(t *testing.T) {
		client, replies, done := idleReadConn(t, 16)
		defer done()
		go func() {
			for _, b := range []byte("*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$5\r\nhello\r\nGET k\r\n") {
				if _, err := client.Write([]byte{b}); err != nil {
					return
				}
			}
		}()
		for _, want := range []string{"+OK\r\n", "$5\r\n", "hello\r\n"} {
			if line, err := replies.ReadString('\n'); err != nil || line != want {
				t.Fatalf("reply %q, %v; want %q", line, err, want)
			}
		}
	})
	t.Run("buffer emptied with bytes still waiting", func(t *testing.T) {
		// Commands of exactly the buffer's 16 bytes leave it empty after
		// each one while the rest of what the wait read is still pending.
		client, replies, done := idleReadConn(t, 16)
		defer done()
		var pipeline strings.Builder
		for i := range 40 {
			fmt.Fprintf(&pipeline, "SET k%02d 123456\r\n", i)
		}
		pipeline.WriteString("GET k39\r\n")
		go client.Write([]byte(pipeline.String()))
		for i := range 40 {
			if line, err := replies.ReadString('\n'); err != nil || line != "+OK\r\n" {
				t.Fatalf("SET %d: reply %q, %v", i, line, err)
			}
		}
		for _, want := range []string{"$6\r\n", "123456\r\n"} {
			if line, err := replies.ReadString('\n'); err != nil || line != want {
				t.Fatalf("GET: reply %q, %v; want %q", line, err, want)
			}
		}
	})
	for _, readBuf := range []int{16, 4096} {
		t.Run(fmt.Sprintf("pipeline over a %d byte buffer", readBuf), func(t *testing.T) {
			client, replies, done := idleReadConn(t, readBuf)
			defer done()
			const n = 300
			var pipeline strings.Builder
			for i := range n {
				v := strings.Repeat(string(rune('a'+i%26)), 1+i*7%300)
				fmt.Fprintf(&pipeline, "*3\r\n$3\r\nSET\r\n$%d\r\nk%d\r\n$%d\r\n%s\r\n", len(fmt.Sprint("k", i)), i, len(v), v)
				fmt.Fprintf(&pipeline, "*2\r\n$3\r\nGET\r\n$%d\r\nk%d\r\n", len(fmt.Sprint("k", i)), i)
			}
			go client.Write([]byte(pipeline.String()))
			for i := range n {
				v := strings.Repeat(string(rune('a'+i%26)), 1+i*7%300)
				for _, want := range []string{"+OK\r\n", fmt.Sprintf("$%d\r\n", len(v)), v + "\r\n"} {
					if line, err := replies.ReadString('\n'); err != nil || line != want {
						t.Fatalf("command %d: reply %q, %v; want %q", i, line, err, want)
					}
				}
			}
		})
	}
}
