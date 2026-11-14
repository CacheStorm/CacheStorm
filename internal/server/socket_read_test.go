package server

import (
	"bufio"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A connection waits for input with no read buffer and then reads the
// socket directly: commands that arrive a few bytes at a time, a pipeline
// several times the read buffer, and a client that leaves while the
// connection waits all behave as with plain reads.
func TestDirectSocketReads(t *testing.T) {
	s := startToggleServer(t, aofTxConfig(t))
	defer stopReplServer(s)
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(s.testPort()), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	rd := bufio.NewReader(conn)
	expect := func(want string) {
		t.Helper()
		line, err := rd.ReadString('\n')
		if err != nil || line != want {
			t.Fatalf("reply %q (%v), want %q", line, err, want)
		}
	}

	msg := []byte("*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$5\r\nhello\r\n*2\r\n$3\r\nGET\r\n$1\r\nk\r\n")
	for i := 0; i < len(msg); i += 3 {
		if _, err := conn.Write(msg[i:min(i+3, len(msg))]); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	expect("+OK\r\n")
	expect("$5\r\n")
	expect("hello\r\n")

	const n = 5000
	var big strings.Builder
	for range n {
		big.WriteString("*2\r\n$4\r\nINCR\r\n$1\r\nc\r\n")
	}
	go func() { _, _ = io.WriteString(conn, big.String()) }()
	for i := 1; i <= n; i++ {
		expect(":" + strconv.Itoa(i) + "\r\n")
	}

	time.Sleep(20 * time.Millisecond)
	if _, err := conn.Write([]byte("*1\r\n$4\r\nPING\r\n")); err != nil {
		t.Fatal(err)
	}
	expect("+PONG\r\n")

	before := s.connectedClients()
	_ = conn.(*net.TCPConn).CloseWrite()
	if _, err := rd.ReadByte(); err != io.EOF {
		t.Fatalf("after the client closed its side: %v, want EOF", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for s.connectedClients() >= before {
		if time.Now().After(deadline) {
			t.Fatalf("the connection stayed open after the client left")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (s *Server) connectedClients() int {
	n := 0
	s.conns.Range(func(_, _ any) bool { n++; return true })
	return n
}

// Under appendfsync always a pipeline's writes share the fsync made before
// its replies go out, and every write a reply acknowledges is in the file
// by then, with the server still running.
func TestAlwaysFsyncCoversPipelinedReplies(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	defer stopReplServer(s)
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(s.testPort()), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	const n = 2000
	var pipe strings.Builder
	for range n {
		pipe.WriteString("*2\r\n$4\r\nINCR\r\n$3\r\nctr\r\n")
	}
	go func() { _, _ = io.WriteString(conn, pipe.String()) }()
	rd := bufio.NewReader(conn)
	for i := 1; i <= n; i++ {
		line, err := rd.ReadString('\n')
		if err != nil || line != ":"+strconv.Itoa(i)+"\r\n" {
			t.Fatalf("reply %d: %q (%v)", i, line, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "INCR\r\n$3\r\nctr\r\n"); got != n {
		t.Fatalf("the AOF holds %d of the %d acknowledged INCRs", got, n)
	}
}
