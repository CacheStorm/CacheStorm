package server

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// newShutdownConn runs a real Connection over a loopback TCP socket. TCP is
// used rather than net.Pipe because a pipelined test must be able to write
// command N+1 before reading reply N; net.Pipe is unbuffered and synchronous,
// so the server blocking on its reply write deadlocks against the client
// blocking on its next command write.
func newShutdownConn(t *testing.T) net.Conn {
	t.Helper()

	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterServerCommands(router)
	command.RegisterStringCommands(router)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	handleDone := make(chan struct{})
	go func() {
		defer close(handleDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		NewConnection(1, conn, s, router, nil).Handle()
	}()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
		select {
		case <-handleDone:
		case <-time.After(2 * time.Second):
		}
	})
	return client
}

func sdSend(t *testing.T, c net.Conn, args ...string) {
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

func sdRecv(t *testing.T, c net.Conn, r *bufio.Reader) *resp.Value {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	v, err := resp.NewReader(r).ReadValue()
	if err != nil {
		t.Fatalf("reading reply: %v", err)
	}
	return v
}

// TestShutdownEmitsExactlyOneReply pins the RESP framing invariant: one command
// in, exactly one reply out. Router.Execute documents it at router.go
// ("returning the error would make the caller write a second reply").
//
// cmdSHUTDOWN wrote +OK AND returned errors.New("SHUTDOWN"), so the connection
// loop wrote that error as a second reply. A client that pipelines SHUTDOWN
// with the next command reads the stray -SHUTDOWN as the reply to its NEXT
// command, and the response stream desynchronizes from there on.
func TestShutdownEmitsExactlyOneReply(t *testing.T) {
	c := newShutdownConn(t)
	r := bufio.NewReader(c)

	// Control: an ordinary pipelined pair lines up. Passes before and after, so
	// a broken harness cannot masquerade as the defect.
	sdSend(t, c, "SET", "ctl", "v")
	sdSend(t, c, "GET", "ctl")
	if v := sdRecv(t, c, r); v.Type != resp.TypeSimpleString || v.Str != "OK" {
		t.Fatalf("control failed: SET replied %+v, want +OK", v)
	}
	if v := sdRecv(t, c, r); v.Type != resp.TypeBulkString || string(v.Bulk) != "v" {
		t.Fatalf("control failed: GET replied %+v, want the bulk value v", v)
	}

	// The contract: SHUTDOWN followed by another command must produce exactly
	// one reply each, in order.
	sdSend(t, c, "SHUTDOWN")
	sdSend(t, c, "PING")

	// This router has no server behind it, so SHUTDOWN is refused with a
	// single error reply.
	first := sdRecv(t, c, r)
	if first.Type != resp.TypeError {
		t.Fatalf("SHUTDOWN replied %+v, want a single error", first)
	}

	second := sdRecv(t, c, r)
	if second.Type != resp.TypeSimpleString || second.Str != "PONG" {
		t.Fatalf("FAIL: the reply after SHUTDOWN is %+v (type %v), want +PONG — "+
			"cmdSHUTDOWN writes +OK and also returns an error, so the connection loop "+
			"writes a stray second reply that desynchronizes every pipelined command "+
			"after it", second, second.Type)
	}
}

func TestShutdownStopsTheServer(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	if got := c.do("SHUTDOWN", "SAVE"); got.Type != resp.TypeError {
		t.Fatalf("SHUTDOWN SAVE without persistence = %+v, want an error", got)
	}
	select {
	case <-s.ShutdownRequested():
		t.Fatal("a refused SHUTDOWN still requested the stop")
	default:
	}
	if got := c.do("SHUTDOWN", "ABORT"); got.Type != resp.TypeError {
		t.Fatalf("SHUTDOWN ABORT = %+v", got)
	}
	if _, err := c.conn.Write(encodeArgs("SHUTDOWN", "NOSAVE")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.ShutdownRequested():
	case <-time.After(3 * time.Second):
		t.Fatal("SHUTDOWN did not request the stop")
	}
	if _, ok := c.next(2 * time.Second); ok {
		t.Fatal("SHUTDOWN sent a reply instead of closing the connection")
	}
}

func TestShutdownSavesWhenSnapshotsAreConfigured(t *testing.T) {
	dir := t.TempDir()
	s := startReplServerCfg(t, replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.AOF = false
		c.Persistence.DataDir = dir
		c.Persistence.SnapshotInterval = "1h"
	}))
	c := dialRepl(t, s)
	c.do("SET", "k", "v")
	if _, err := c.conn.Write(encodeArgs("SHUTDOWN")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.ShutdownRequested():
	case <-time.After(3 * time.Second):
		t.Fatal("SHUTDOWN did not request the stop")
	}
	if _, err := os.Stat(filepath.Join(dir, "dump.rdb")); err != nil {
		t.Fatalf("SHUTDOWN did not save the snapshot: %v", err)
	}
}
