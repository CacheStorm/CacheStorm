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

// newAuthConnServer starts a real Connection over an in-memory pipe with
// requirepass configured, exercising the actual per-command loop.
func newAuthConnServer(t *testing.T, requirePass string) (net.Conn, *store.Store) {
	t.Helper()

	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterStringCommands(router)
	command.RegisterKeyCommands(router)
	command.RegisterServerCommands(router)
	router.SetRequirePass(requirePass)

	clientSide, serverSide := net.Pipe()
	conn := NewConnection(1, serverSide, s, router, nil)
	handleDone := make(chan struct{})
	go func() {
		defer close(handleDone)
		conn.Handle()
	}()

	t.Cleanup(func() {
		_ = clientSide.Close()
		// Same rationale as newACLSession: the Handle goroutine's final
		// metrics/slow-log writes must land before later tests run.
		select {
		case <-handleDone:
		case <-time.After(2 * time.Second):
		}
	})
	return clientSide, s
}

func authSend(c net.Conn, args ...string) {
	var out []byte
	out = append(out, fmt.Sprintf("*%d\r\n", len(args))...)
	for _, a := range args {
		out = append(out, fmt.Sprintf("$%d\r\n%s\r\n", len(a), a)...)
	}
	if _, err := c.Write(out); err != nil {
		panic(err)
	}
}

func authRecv(t *testing.T, c net.Conn, r *bufio.Reader) *resp.Value {
	t.Helper()
	if err := c.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	v, err := resp.NewReader(r).ReadValue()
	if err != nil {
		t.Fatalf("reading reply: %v", err)
	}
	return v
}

// The authenticated flag must survive from the AUTH command to the next
// command. It used to live only on the per-command Context, which Handle()
// rebuilds for every command, so AUTH replied +OK and then the very next
// command was rejected with NOAUTH — requirepass made the server unusable.
func TestAuthStatePersistsToNextCommand(t *testing.T) {
	c, s := newAuthConnServer(t, "s3cret")
	r := bufio.NewReader(c)

	authSend(c, "AUTH", "s3cret")
	if got := authRecv(t, c, r); got.Type != resp.TypeSimpleString || got.Str != "OK" {
		t.Fatalf("AUTH returned type %v %q, want +OK", got.Type, got.Str)
	}

	s.Set("k", &store.StringValue{Data: []byte("v")}, store.SetOptions{})

	authSend(c, "GET", "k")
	got := authRecv(t, c, r)
	if got.Type == resp.TypeError {
		t.Fatalf("after a successful AUTH, GET returned %q", got.Err)
	}
	// resp.BulkString populates Bulk; Str stays empty.
	if got.Type != resp.TypeBulkString || string(got.Bulk) != "v" {
		t.Fatalf("GET returned type %v %q, want bulk \"v\"", got.Type, got.Bulk)
	}
}

// The state must persist across MANY commands, not just the next one.
func TestAuthStatePersistsAcrossManyCommands(t *testing.T) {
	c, s := newAuthConnServer(t, "s3cret")
	r := bufio.NewReader(c)

	authSend(c, "AUTH", "s3cret")
	if got := authRecv(t, c, r); got.Type != resp.TypeSimpleString {
		t.Fatalf("AUTH returned type %v, want +OK", got.Type)
	}

	s.Set("k", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	for i := 0; i < 25; i++ {
		authSend(c, "GET", "k")
		got := authRecv(t, c, r)
		if got.Type == resp.TypeError {
			t.Fatalf("command %d after AUTH returned %q", i, got.Err)
		}
	}
}

// A command issued BEFORE authenticating must still be refused; the fix must
// not make the gate permissive.
func TestUnauthenticatedCommandRefusedControl(t *testing.T) {
	c, s := newAuthConnServer(t, "s3cret")
	r := bufio.NewReader(c)

	s.Set("k", &store.StringValue{Data: []byte("v")}, store.SetOptions{})

	authSend(c, "GET", "k")
	got := authRecv(t, c, r)
	if got.Type != resp.TypeError {
		t.Fatalf("unauthenticated GET returned type %v, want an error", got.Type)
	}
}

// A wrong password must be rejected and must not authenticate.
func TestWrongPasswordDoesNotAuthenticateControl(t *testing.T) {
	c, s := newAuthConnServer(t, "s3cret")
	r := bufio.NewReader(c)

	authSend(c, "AUTH", "wrong")
	if got := authRecv(t, c, r); got.Type != resp.TypeError {
		t.Fatalf("AUTH with a wrong password returned type %v, want an error", got.Type)
	}

	s.Set("k", &store.StringValue{Data: []byte("v")}, store.SetOptions{})

	authSend(c, "GET", "k")
	if got := authRecv(t, c, r); got.Type != resp.TypeError {
		t.Fatalf("GET after a failed AUTH returned type %v, want an error", got.Type)
	}
}

// After a successful AUTH, a subsequent wrong password must de-authenticate
// the connection rather than leaving the earlier grant in force.
func TestFailedReauthClearsStateControl(t *testing.T) {
	c, s := newAuthConnServer(t, "s3cret")
	r := bufio.NewReader(c)

	authSend(c, "AUTH", "s3cret")
	if got := authRecv(t, c, r); got.Type != resp.TypeSimpleString {
		t.Fatalf("AUTH returned type %v, want +OK", got.Type)
	}

	authSend(c, "AUTH", "wrong")
	if got := authRecv(t, c, r); got.Type != resp.TypeError {
		t.Fatalf("re-AUTH with a wrong password returned type %v, want an error", got.Type)
	}

	s.Set("k", &store.StringValue{Data: []byte("v")}, store.SetOptions{})

	authSend(c, "GET", "k")
	if got := authRecv(t, c, r); got.Type != resp.TypeError {
		t.Fatalf("GET after a failed re-AUTH returned type %v, want an error", got.Type)
	}
}

// With no requirepass configured, commands must work without AUTH; the fix
// must not gate a password-less server.
func TestNoRequirePassNeedsNoAuthControl(t *testing.T) {
	c, s := newAuthConnServer(t, "")
	r := bufio.NewReader(c)

	s.Set("k", &store.StringValue{Data: []byte("v")}, store.SetOptions{})

	authSend(c, "GET", "k")
	got := authRecv(t, c, r)
	if got.Type != resp.TypeBulkString || string(got.Bulk) != "v" {
		t.Fatalf("with no requirepass, GET returned type %v %q, want bulk \"v\"", got.Type, got.Bulk)
	}
}

// PING is in noAuthCommands and must stay reachable before authenticating.
func TestPingBypassesAuthControl(t *testing.T) {
	c, _ := newAuthConnServer(t, "s3cret")
	r := bufio.NewReader(c)

	authSend(c, "PING")
	if got := authRecv(t, c, r); got.Type == resp.TypeError {
		t.Fatalf("PING before AUTH returned %q, want +PONG", got.Err)
	}
}
