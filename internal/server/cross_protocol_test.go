package server

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A browser made to POST to the RESP port sends HTTP lines; Redis drops the
// connection on POST or Host: so commands in the body never run.
func TestCrossProtocolHTTPRequestIsDropped(t *testing.T) {
	for _, req := range []string{
		"POST / HTTP/1.1\r\nHost: 127.0.0.1\r\nContent-Length: 13\r\n\r\nSET k pwned\r\n",
		"host: 127.0.0.1\r\nSET k pwned\r\n",
		"*2\r\n$4\r\npost\r\n$1\r\n/\r\n*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$5\r\npwned\r\n",
	} {
		serverSide, clientSide := net.Pipe()
		s := store.NewStore()
		router := command.NewRouter()
		command.RegisterStringCommands(router)
		conn := newConnection(1, serverSide, s, router, nil, connectionOptions{})
		done := make(chan struct{})
		go func() { defer close(done); conn.Handle() }()
		go func() { _, _ = clientSide.Write([]byte(req)) }()
		clientSide.SetReadDeadline(time.Now().Add(2 * time.Second))
		got, err := io.ReadAll(clientSide)
		if err != nil || len(got) != 0 {
			t.Errorf("%q: reply %q, err %v; want the connection closed silently", req, got, err)
		}
		clientSide.Close()
		<-done
		if _, ok := s.Get("k"); ok {
			t.Errorf("%q: the body's SET ran", req)
		}
	}
}
