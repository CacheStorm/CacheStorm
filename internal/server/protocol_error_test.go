package server

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

func protocolConn(t *testing.T) net.Conn {
	t.Helper()
	router := command.NewRouter()
	command.RegisterServerCommands(router)
	command.RegisterStringCommands(router)
	client, server := net.Pipe()
	conn := NewConnection(1, server, store.NewStore(), router, nil)
	done := make(chan struct{})
	go func() { defer close(done); conn.Handle() }()
	t.Cleanup(func() {
		client.Close()
		<-done
	})
	return client
}

func TestInlineCommandsAreServed(t *testing.T) {
	c := protocolConn(t)
	r := bufio.NewReader(c)
	go func() { _, _ = c.Write([]byte("PING\r\nSET k \"a b\"\nGET k\r\n")) }()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	for _, want := range []string{"+PONG\r\n", "+OK\r\n", "$3\r\n", "a b\r\n"} {
		line, err := r.ReadString('\n')
		if err != nil || line != want {
			t.Fatalf("got %q %v, want %q", line, err, want)
		}
	}
}

func TestProtocolErrorIsReportedBeforeClose(t *testing.T) {
	c := protocolConn(t)
	go func() { _, _ = c.Write([]byte("*1\r\n:5\r\n")) }()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	r := bufio.NewReader(c)
	line, err := r.ReadString('\n')
	if err != nil || !strings.HasPrefix(line, "-ERR Protocol error") {
		t.Fatalf("got %q %v, want a protocol error reply", line, err)
	}
	if _, err := r.ReadByte(); err != io.EOF {
		t.Fatalf("connection stayed open after a protocol error: %v", err)
	}
}
