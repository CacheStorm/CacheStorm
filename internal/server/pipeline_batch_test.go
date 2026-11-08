package server

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

// pipelineReplies sends raw on a fresh connection and returns what arrives
// within a short wait, without closing the client side first.
func pipelineReplies(t *testing.T, raw string) string {
	t.Helper()
	serverSide, clientSide := net.Pipe()
	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterServerCommands(router)
	command.RegisterStringCommands(router)
	conn := newConnection(1, serverSide, s, router, nil, connectionOptions{})
	done := make(chan struct{})
	go func() { defer close(done); conn.Handle() }()
	defer func() { clientSide.Close(); <-done }()

	go func() { _, _ = clientSide.Write([]byte(raw)) }()
	var out strings.Builder
	buf := make([]byte, 4096)
	for {
		clientSide.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		n, err := clientSide.Read(buf)
		out.Write(buf[:n])
		if err != nil {
			if err != io.EOF && !isTimeout(err) {
				t.Fatal(err)
			}
			return out.String()
		}
	}
}

func isTimeout(err error) bool {
	ne, ok := err.(net.Error)
	return ok && ne.Timeout()
}

// Replies go out when the only thing left in the buffer is an empty command
// (a blank line, *0 or *-1), which the reader skips and then waits past:
// they used to wait with it until the client sent something else.
func TestRepliesNotHeldForTrailingEmptyCommand(t *testing.T) {
	cmds := "*3\r\n$3\r\nSET\r\n$1\r\na\r\n$1\r\n1\r\n*1\r\n$4\r\nPING\r\n"
	for _, tail := range []string{"*0\r\n", "\r\n", "*-1\r\n", ""} {
		if got := pipelineReplies(t, cmds+tail); got != "+OK\r\n+PONG\r\n" {
			t.Errorf("tail %q: replies %q", tail, got)
		}
	}
}

// A pipeline parsed as one batch runs in order, and a protocol error met
// while parsing ahead is reported after the replies of the commands before
// it, as Redis does.
func TestPipelineBatchKeepsOrderAndReportsErrorsLast(t *testing.T) {
	raw := "*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\n1\r\n" +
		"*2\r\n$4\r\nINCR\r\n$1\r\nk\r\n" +
		"*2\r\n$3\r\nGET\r\n$1\r\nk\r\n" +
		"*1\r\n$x\r\n"
	got := pipelineReplies(t, raw)
	if !strings.HasPrefix(got, "+OK\r\n:2\r\n$1\r\n2\r\n-ERR Protocol error") {
		t.Fatalf("replies %q", got)
	}
}
