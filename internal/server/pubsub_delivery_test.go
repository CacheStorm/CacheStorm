package server

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

type psTrackedConn struct {
	c    *Connection
	done chan struct{}
}

// psServer runs real Connection handlers behind a loopback TCP listener.
type psServer struct {
	t      *testing.T
	ln     net.Listener
	st     *store.Store
	router *command.Router
	mu     sync.Mutex
	conns  []*psTrackedConn
	accept chan *psTrackedConn
	nextID int64
}

func newPSServer(t *testing.T) *psServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("loopback listener unavailable: %v", err)
	}
	router := command.NewRouter()
	command.RegisterStringCommands(router)
	command.RegisterServerCommands(router)
	command.RegisterPubSubCommands(router)
	command.RegisterTransactionCommands(router)
	srv := &psServer{t: t, ln: ln, st: store.NewStore(), router: router, accept: make(chan *psTrackedConn, 16)}
	go func() {
		for {
			nc, err := ln.Accept()
			if err != nil {
				return
			}
			srv.mu.Lock()
			srv.nextID++
			tc := &psTrackedConn{c: NewConnection(srv.nextID+1000, nc, srv.st, srv.router, nil), done: make(chan struct{})}
			srv.conns = append(srv.conns, tc)
			srv.mu.Unlock()
			go func() {
				defer close(tc.done)
				tc.c.Handle()
			}()
			srv.accept <- tc
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		srv.mu.Lock()
		conns := append([]*psTrackedConn(nil), srv.conns...)
		srv.mu.Unlock()
		for _, tc := range conns {
			tc.c.conn.Close()
			select {
			case <-tc.done:
			case <-time.After(5 * time.Second):
				t.Errorf("connection %d handler did not exit", tc.c.ID)
			}
		}
	})
	return srv
}

type psClient struct {
	t    *testing.T
	conn net.Conn
	r    *bufio.Reader
	srv  *psTrackedConn
}

func (s *psServer) dial() *psClient {
	s.t.Helper()
	nc, err := net.Dial("tcp", s.ln.Addr().String())
	if err != nil {
		s.t.Fatalf("dial: %v", err)
	}
	s.t.Cleanup(func() { nc.Close() })
	select {
	case tc := <-s.accept:
		return &psClient{t: s.t, conn: nc, r: bufio.NewReader(nc), srv: tc}
	case <-time.After(5 * time.Second):
		s.t.Fatal("server did not accept")
	}
	return nil
}

func encodeCmd(args ...string) []byte {
	var b bytes.Buffer
	b.WriteString("*" + strconv.Itoa(len(args)) + "\r\n")
	for _, a := range args {
		b.WriteString("$" + strconv.Itoa(len(a)) + "\r\n" + a + "\r\n")
	}
	return b.Bytes()
}

func (c *psClient) send(args ...string) {
	c.t.Helper()
	if _, err := c.conn.Write(encodeCmd(args...)); err != nil {
		c.t.Fatalf("write %v: %v", args, err)
	}
}

// expect reads exactly len(want) bytes and compares them verbatim.
func (c *psClient) expect(want string) {
	c.t.Helper()
	_ = c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := make([]byte, len(want))
	n, err := io.ReadFull(c.r, got)
	if err != nil {
		c.t.Fatalf("reading %q: got %q, err %v", want, got[:n], err)
	}
	if string(got) != want {
		c.t.Fatalf("frame mismatch:\n got  %q\n want %q", got, want)
	}
}

func (c *psClient) do(want string, args ...string) {
	c.t.Helper()
	c.send(args...)
	c.expect(want)
}

func (c *psClient) expectSilence(d time.Duration) {
	c.t.Helper()
	_ = c.conn.SetReadDeadline(time.Now().Add(d))
	b, err := c.r.ReadByte()
	if err == nil {
		rest, _ := c.r.Peek(c.r.Buffered())
		c.t.Fatalf("unexpected data %q", append([]byte{b}, rest...))
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		c.t.Fatalf("expected timeout, got %v", err)
	}
}

func bulk(s string) string { return "$" + strconv.Itoa(len(s)) + "\r\n" + s + "\r\n" }

func ack(kind, name string, n int) string {
	return "*3\r\n" + bulk(kind) + bulk(name) + ":" + strconv.Itoa(n) + "\r\n"
}

func msgFrame(ch, payload string) string {
	return "*3\r\n" + bulk("message") + bulk(ch) + bulk(payload)
}

func TestPubSubDeliveryOverTCP(t *testing.T) {
	srv := newPSServer(t)
	a, b := srv.dial(), srv.dial()

	a.do(ack("subscribe", "ch", 1), "SUBSCRIBE", "ch")
	b.do(":1\r\n", "PUBLISH", "ch", "hello")
	a.expect(msgFrame("ch", "hello"))
	b.do(":0\r\n", "PUBLISH", "other", "x")
	a.expectSilence(100 * time.Millisecond)
}

func TestPubSubPatternDeliveryOverTCP(t *testing.T) {
	srv := newPSServer(t)
	a, b := srv.dial(), srv.dial()

	a.do(ack("subscribe", "news.a", 1), "SUBSCRIBE", "news.a")
	a.do(ack("psubscribe", "news.*", 2), "PSUBSCRIBE", "news.*")
	b.do(":2\r\n", "PUBLISH", "news.a", "x")
	a.expect(msgFrame("news.a", "x"))
	a.expect("*4\r\n" + bulk("pmessage") + bulk("news.*") + bulk("news.a") + bulk("x"))
	b.do(":1\r\n", "PUBLISH", "news.tech", "hi")
	a.expect("*4\r\n" + bulk("pmessage") + bulk("news.*") + bulk("news.tech") + bulk("hi"))
}

func TestPubSubShardDeliveryOverTCP(t *testing.T) {
	srv := newPSServer(t)
	a, b := srv.dial(), srv.dial()

	a.do(ack("ssubscribe", "sh", 1), "SSUBSCRIBE", "sh")
	b.do(":0\r\n", "PUBLISH", "sh", "plain")
	b.do(":1\r\n", "SPUBLISH", "sh", "x")
	a.expect("*3\r\n" + bulk("smessage") + bulk("sh") + bulk("x"))
	b.do("*1\r\n"+bulk("sh"), "PUBSUB", "SHARDCHANNELS")
	b.do("*2\r\n"+bulk("sh")+":1\r\n", "PUBSUB", "SHARDNUMSUB", "sh")
	a.do(ack("sunsubscribe", "sh", 0), "SUNSUBSCRIBE")
	b.do(":0\r\n", "SPUBLISH", "sh", "y")
	a.do("$-1\r\n", "GET", "k")
}

func TestPubSubBinaryPayloadOverTCP(t *testing.T) {
	srv := newPSServer(t)
	a, b := srv.dial(), srv.dial()
	payload := "a\x00b\r\nc\xff"
	a.do(ack("subscribe", "ch", 1), "SUBSCRIBE", "ch")
	b.do(":1\r\n", "PUBLISH", "ch", payload)
	a.expect(msgFrame("ch", payload))
}

func TestPubSubMultipleSubscribersOverTCP(t *testing.T) {
	srv := newPSServer(t)
	a, c, b := srv.dial(), srv.dial(), srv.dial()
	a.do(ack("subscribe", "ch", 1), "SUBSCRIBE", "ch")
	c.do(ack("subscribe", "ch", 1), "SUBSCRIBE", "ch")
	b.do(":2\r\n", "PUBLISH", "ch", "x")
	a.expect(msgFrame("ch", "x"))
	c.expect(msgFrame("ch", "x"))

	for i := 0; i < 200; i++ {
		b.send("PUBLISH", "ch", "m"+strconv.Itoa(i))
	}
	for i := 0; i < 200; i++ {
		b.expect(":2\r\n")
	}
	for i := 0; i < 200; i++ {
		a.expect(msgFrame("ch", "m"+strconv.Itoa(i)))
		c.expect(msgFrame("ch", "m"+strconv.Itoa(i)))
	}
}

func TestPubSubPublishFromMultiOverTCP(t *testing.T) {
	srv := newPSServer(t)
	a, b := srv.dial(), srv.dial()
	a.do(ack("subscribe", "ch", 1), "SUBSCRIBE", "ch")
	b.do("+OK\r\n", "MULTI")
	b.do("+QUEUED\r\n", "PUBLISH", "ch", "tx")
	b.do("*1\r\n:1\r\n", "EXEC")
	a.expect(msgFrame("ch", "tx"))
}

func TestPubSubUnsubscribeStopsDeliveryOverTCP(t *testing.T) {
	srv := newPSServer(t)
	a, b := srv.dial(), srv.dial()

	a.send("SUBSCRIBE", "a", "b")
	a.expect(ack("subscribe", "a", 1) + ack("subscribe", "b", 2))
	a.do(ack("unsubscribe", "a", 1), "UNSUBSCRIBE", "a")
	b.do(":0\r\n", "PUBLISH", "a", "x")
	b.do(":1\r\n", "PUBLISH", "b", "y")
	a.expect(msgFrame("b", "y"))
	a.do(ack("unsubscribe", "b", 0), "UNSUBSCRIBE")
	b.do(":0\r\n", "PUBLISH", "b", "z")
	a.expectSilence(100 * time.Millisecond)
	a.do("*3\r\n"+bulk("unsubscribe")+"$-1\r\n:0\r\n", "UNSUBSCRIBE")

	// Back in normal mode.
	a.do("+OK\r\n", "SET", "k", "v")
	a.do(bulk("v"), "GET", "k")
	a.do("+PONG\r\n", "PING")
}

func TestPubSubSubscribedModeRestrictionsOverTCP(t *testing.T) {
	srv := newPSServer(t)
	a := srv.dial()

	a.do(ack("subscribe", "ch", 1), "SUBSCRIBE", "ch")
	a.do("*2\r\n"+bulk("pong")+bulk(""), "PING")
	a.do("*2\r\n"+bulk("pong")+bulk("hi"), "PING", "hi")
	a.do("-ERR Can't execute 'get': only (P|S)SUBSCRIBE / (P|S)UNSUBSCRIBE / PING / QUIT / RESET are allowed in this context\r\n", "get", "x")
	a.do("-ERR Can't execute 'set': only (P|S)SUBSCRIBE / (P|S)UNSUBSCRIBE / PING / QUIT / RESET are allowed in this context\r\n", "SET", "x", "1")
	a.do("-ERR wrong number of arguments for 'set' command\r\n", "SET", "x")
	a.do("-ERR wrong number of arguments for 'command|count' command\r\n", "COMMAND", "COUNT", "x")
	a.do("-ERR Can't execute 'command|count': only (P|S)SUBSCRIBE / (P|S)UNSUBSCRIBE / PING / QUIT / RESET are allowed in this context\r\n", "COMMAND", "count")
	a.do("-EXECABORT Transaction discarded because of: Can't execute 'exec': only (P|S)SUBSCRIBE / (P|S)UNSUBSCRIBE / PING / QUIT / RESET are allowed in this context\r\n", "EXEC")
	a.do(ack("psubscribe", "p*", 2), "PSUBSCRIBE", "p*")
	a.do(ack("unsubscribe", "ch", 1), "UNSUBSCRIBE")
	a.do("-ERR Can't execute 'get': only (P|S)SUBSCRIBE / (P|S)UNSUBSCRIBE / PING / QUIT / RESET are allowed in this context\r\n", "GET", "x")
	a.do(ack("punsubscribe", "p*", 0), "PUNSUBSCRIBE")
	a.do("$-1\r\n", "GET", "x")

	// A shard subscription alone also puts the client in subscribed mode.
	a.do(ack("ssubscribe", "s", 1), "SSUBSCRIBE", "s")
	a.do("-ERR Can't execute 'get': only (P|S)SUBSCRIBE / (P|S)UNSUBSCRIBE / PING / QUIT / RESET are allowed in this context\r\n", "GET", "x")
	a.do("+OK\r\n", "QUIT")
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestPubSubDisconnectCleansUpOverTCP(t *testing.T) {
	srv := newPSServer(t)
	ps := srv.st.GetPubSub()
	before := runtime.NumGoroutine()

	a := srv.dial()
	a.do(ack("subscribe", "ch", 1), "SUBSCRIBE", "ch")
	a.do(ack("psubscribe", "p*", 2), "PSUBSCRIBE", "p*")
	a.do(ack("ssubscribe", "s", 1), "SSUBSCRIBE", "s")
	if got := ps.NumSub("ch")["ch"]; got != 1 {
		t.Fatalf("NUMSUB before disconnect = %d", got)
	}
	done := a.srv.c.deliveryDone
	if done == nil {
		t.Fatal("delivery goroutine not started")
	}

	a.conn.Close()
	select {
	case <-a.srv.done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not exit after disconnect")
	}
	select {
	case <-done:
	default:
		t.Fatal("delivery goroutine still running after the handler exited")
	}
	if got := ps.NumSub("ch")["ch"]; got != 0 {
		t.Fatalf("NUMSUB after disconnect = %d, want 0", got)
	}
	if ps.NumPat() != 0 || ps.ShardNumSub("s")["s"] != 0 {
		t.Fatal("pattern or shard subscription survived disconnect")
	}
	if ps.Publish("ch", []byte("x")) != 0 {
		t.Fatal("publish delivered to a disconnected client")
	}
	waitFor(t, "goroutines to return to baseline", func() bool { return runtime.NumGoroutine() <= before })
}

func TestPubSubSlowSubscriberIsDisconnected(t *testing.T) {
	prevQ, prevB := store.SetSubscriberLimits(8, 0)
	t.Cleanup(func() { store.SetSubscriberLimits(prevQ, prevB) })

	st := store.NewStore()
	router := command.NewRouter()
	command.RegisterPubSubCommands(router)

	// net.Pipe has no buffering: once the client stops reading, the
	// delivery goroutine's write blocks and the queue fills up.
	clientSide, serverSide := net.Pipe()
	defer clientSide.Close()
	c := NewConnection(1, serverSide, st, router, nil)
	handled := make(chan struct{})
	go func() {
		defer close(handled)
		c.Handle()
	}()

	cl := &psClient{t: t, conn: clientSide, r: bufio.NewReader(clientSide)}
	cl.do(ack("subscribe", "ch", 1), "SUBSCRIBE", "ch")

	ps := st.GetPubSub()
	delivered := 0
	payload := []byte(strings.Repeat("x", 1024))
	for i := 0; i < 100; i++ {
		delivered += ps.Publish("ch", payload)
	}
	if delivered >= 100 {
		t.Fatalf("all %d publishes were queued; expected the bounded queue to overflow", delivered)
	}

	select {
	case <-handled:
	case <-time.After(5 * time.Second):
		t.Fatal("slow subscriber was not disconnected")
	}
	if got := ps.NumSub("ch")["ch"]; got != 0 {
		t.Fatalf("NUMSUB after overflow disconnect = %d, want 0", got)
	}
	// The client sees the connection close once it drains what was sent.
	_ = clientSide.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err := io.Copy(io.Discard, cl.r)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Logf("client read ended with %v", err)
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("client connection still open after overflow")
	}
}
