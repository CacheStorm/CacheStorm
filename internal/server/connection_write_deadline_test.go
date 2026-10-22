package server

import (
	"bufio"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// newDeadlineConnServer runs a real Connection over an in-memory pipe with the
// list commands registered, so BLPOP takes its production blocking path.
func newDeadlineConnServer(t *testing.T, writeTimeout time.Duration) (net.Conn, *Connection) {
	t.Helper()

	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterListCommands(router)

	clientSide, serverSide := net.Pipe()
	conn := NewConnection(1, serverSide, s, router, nil)
	// Shrink the production 5-minute guard so the block outlasts it. The
	// relationship being tested (block duration > write deadline) is the same
	// one a 10-minute BLPOP hits against the shipped default.
	conn.writeTimeout = writeTimeout

	handleDone := make(chan struct{})
	go func() {
		defer close(handleDone)
		conn.Handle()
	}()

	t.Cleanup(func() {
		_ = clientSide.Close()
		select {
		case <-handleDone:
		case <-time.After(2 * time.Second):
		}
	})
	return clientSide, conn
}

func dlSend(t *testing.T, c net.Conn, args ...string) {
	t.Helper()
	var out []byte
	out = append(out, fmt.Sprintf("*%d\r\n", len(args))...)
	for _, a := range args {
		out = append(out, fmt.Sprintf("$%d\r\n%s\r\n", len(a), a)...)
	}
	if _, err := c.Write(out); err != nil {
		t.Fatalf("send %v: %v", args, err)
	}
}

// TestBlockingCommandReplySurvivesWriteDeadline pins the contract that every
// command the server accepts must get its reply written.
//
// Connection.Handle armed the socket write deadline once, before
// router.Execute. Blocking commands such as BLPOP sleep INSIDE Execute, for as
// long as the client asked for, so the deadline expires while the command is
// still running and the reply — the result of a completed, successful pop — is
// dropped. The client is then disconnected having received nothing at all.
func TestBlockingCommandReplySurvivesWriteDeadline(t *testing.T) {
	const writeTimeout = 150 * time.Millisecond

	c, _ := newDeadlineConnServer(t, writeTimeout)
	r := bufio.NewReader(c)

	// Control: a command that returns immediately is unaffected, both before and
	// after the fix, so a broken harness cannot masquerade as the defect.
	dlSend(t, c, "BLPOP", "ctlkey", "0")
	if v, err := resp.NewReader(r).ReadValue(); err != nil {
		t.Fatalf("control failed: immediate BLPOP returned no value: %v", err)
	} else if !v.IsNull {
		t.Fatalf("control failed: BLPOP on a missing key = %+v, want a null array", v)
	}

	// The contract: a blocking pop that blocks longer than the write deadline
	// must still deliver its null (timed-out) reply.
	dlSend(t, c, "BLPOP", "slowkey", "2")

	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	v, err := resp.NewReader(r).ReadValue()
	if err != nil {
		t.Fatalf("FAIL: BLPOP blocked for 2s with a %v write deadline and never got a reply "+
			"(read error: %v) — Handle arms the write deadline BEFORE Execute, so it expires "+
			"while the command blocks and the completed reply is discarded",
			writeTimeout, err)
	}
	if !v.IsNull {
		t.Fatalf("BLPOP on a missing key after timing out = %+v, want a null array", v)
	}
}
