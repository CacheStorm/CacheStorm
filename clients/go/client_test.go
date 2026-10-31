package cachestorm_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cachestorm "github.com/cachestorm/cachestorm/clients/go"
)

type fakeServer struct {
	t        *testing.T
	ln       net.Listener
	mu       sync.Mutex
	data     map[string]string
	commands [][]string
	conns    atomic.Int64
	batch    int
	wg       sync.WaitGroup
	pubsub   chan string
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeServer{t: t, ln: ln, data: map[string]string{}, pubsub: make(chan string, 16)}
	s.wg.Add(1)
	go s.serve()
	t.Cleanup(func() {
		ln.Close()
		s.wg.Wait()
	})
	return s
}

func (s *fakeServer) addr() string { return s.ln.Addr().String() }

func (s *fakeServer) serve() {
	defer s.wg.Done()
	var conns sync.WaitGroup
	defer conns.Wait()
	var open []net.Conn
	var openMu sync.Mutex
	defer func() {
		openMu.Lock()
		for _, c := range open {
			c.Close()
		}
		openMu.Unlock()
	}()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			openMu.Lock()
			for _, c := range open {
				c.Close()
			}
			open = nil
			openMu.Unlock()
			return
		}
		s.conns.Add(1)
		openMu.Lock()
		open = append(open, c)
		openMu.Unlock()
		conns.Add(1)
		go func() {
			defer conns.Done()
			defer c.Close()
			s.handle(c)
		}()
	}
}

func readCommand(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(line, "*") {
		return nil, fmt.Errorf("bad header %q", line)
	}
	n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil {
		return nil, err
	}
	args := make([]string, n)
	for i := range args {
		hdr, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(strings.TrimSpace(hdr[1:]))
		if err != nil {
			return nil, err
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		args[i] = string(buf[:size])
	}
	return args, nil
}

func bulk(s string) string { return "$" + strconv.Itoa(len(s)) + "\r\n" + s + "\r\n" }

func (s *fakeServer) handle(c net.Conn) {
	r := bufio.NewReader(c)
	var pending []string
	for {
		args, err := readCommand(r)
		if err != nil {
			return
		}
		s.mu.Lock()
		s.commands = append(s.commands, args)
		batch := s.batch
		s.mu.Unlock()
		reply, keep := s.reply(c, args)
		if !keep {
			return
		}
		if batch > 0 {
			pending = append(pending, reply)
			if len(pending) < batch {
				continue
			}
			reply = strings.Join(pending, "")
			pending = nil
		}
		if _, err := io.WriteString(c, reply); err != nil {
			return
		}
	}
}

func (s *fakeServer) reply(c net.Conn, args []string) (string, bool) {
	switch strings.ToUpper(args[0]) {
	case "PING":
		return "+PONG\r\n", true
	case "SET":
		s.mu.Lock()
		s.data[args[1]] = args[2]
		s.mu.Unlock()
		return "+OK\r\n", true
	case "GET":
		s.mu.Lock()
		v, ok := s.data[args[1]]
		s.mu.Unlock()
		if !ok {
			return "$-1\r\n", true
		}
		return bulk(v), true
	case "BYE":
		// Replies, then closes the connection as an idle timeout would.
		_, _ = io.WriteString(c, "+BYE\r\n")
		return "", false
	case "BADCMD":
		return "-ERR unknown command 'BADCMD'\r\n", true
	case "MIXED":
		return "*3\r\n" + bulk("a") + "-ERR inner\r\n$-1\r\n", true
	case "BINARY":
		return bulk("a\r\nb\x00c"), true
	case "BADBULK":
		return "$-5\r\n", true
	case "SLOW":
		time.Sleep(300 * time.Millisecond)
		return "+SLOW\r\n", true
	case "HANG":
		time.Sleep(2 * time.Second)
		return "+LATE\r\n", true
	case "SUBSCRIBE":
		var b strings.Builder
		for i, ch := range args[1:] {
			fmt.Fprintf(&b, "*3\r\n$9\r\nsubscribe\r\n%s:%d\r\n", bulk(ch), i+1)
		}
		go func() {
			for p := range s.pubsub {
				if p == "" {
					return
				}
				if _, err := io.WriteString(c, "*3\r\n"+bulk("message")+bulk(args[1])+bulk(p)); err != nil {
					return
				}
			}
		}()
		return b.String(), true
	default:
		return "-ERR unknown command\r\n", true
	}
}

func newClient(t *testing.T, addr string, opts ...cachestorm.Option) *cachestorm.Client {
	t.Helper()
	c, err := cachestorm.NewClient(addr, opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestCommandAfterCloseReturnsErrClosed(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr())
	ctx := context.Background()
	if _, err := c.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := c.Ping(ctx); !errors.Is(err, cachestorm.ErrClosed) {
		t.Fatalf("Ping after Close: %v", err)
	}
	if _, err := c.Get(ctx, "k"); !errors.Is(err, cachestorm.ErrClosed) {
		t.Fatalf("Get after Close: %v", err)
	}
	pipe := c.Pipeline()
	pipe.Get(ctx, "k")
	if _, err := pipe.Exec(ctx); !errors.Is(err, cachestorm.ErrClosed) {
		t.Fatalf("pipeline after Close: %v", err)
	}
}

func TestCloseWithoutConnecting(t *testing.T) {
	c, err := cachestorm.NewClient("127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Ping(context.Background()); !errors.Is(err, cachestorm.ErrClosed) {
		t.Fatalf("got %v", err)
	}
}

func TestConcurrentDoAndClose(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr(), cachestorm.WithPoolSize(4))
	ctx := context.Background()
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			for j := 0; j < 50; j++ {
				key := fmt.Sprintf("k%d", i)
				err := c.Set(ctx, key, j, 0)
				if err != nil && !errors.Is(err, cachestorm.ErrClosed) {
					t.Errorf("Set: %v", err)
					return
				}
				if errors.Is(err, cachestorm.ErrClosed) {
					return
				}
			}
		}(i)
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			time.Sleep(2 * time.Millisecond)
			if err := c.Close(); err != nil {
				t.Errorf("Close: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if _, err := c.Ping(ctx); !errors.Is(err, cachestorm.ErrClosed) {
		t.Fatalf("Ping after concurrent Close: %v", err)
	}
}

func TestCloseInterruptsInFlightCommand(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr())
	done := make(chan error, 1)
	go func() {
		_, err := c.Do(context.Background(), "HANG")
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)
	c.Close()
	select {
	case err := <-done:
		if !errors.Is(err, cachestorm.ErrClosed) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("in-flight command not interrupted by Close")
	}
}

func TestSetGetRoundTrip(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr())
	ctx := context.Background()
	if err := c.Set(ctx, "greeting", "hello world", 0); err != nil {
		t.Fatal(err)
	}
	got, err := c.Get(ctx, "greeting")
	if err != nil || got != "hello world" {
		t.Fatalf("got %q, %v", got, err)
	}
	if err := c.Set(ctx, "bin", []byte{'x', 0, 'y'}, 0); err != nil {
		t.Fatal(err)
	}
	if got, _ := c.Get(ctx, "bin"); got != "x\x00y" {
		t.Fatalf("binary value encoded as %q", got)
	}
	if err := c.Set(ctx, "f", 0.5, 0); err != nil {
		t.Fatal(err)
	}
	if got, _ := c.Get(ctx, "f"); got != "0.5" {
		t.Fatalf("float encoded as %q", got)
	}
}

func TestIdleConnectionClosedByServerIsReplaced(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr())
	ctx := context.Background()
	if err := c.Set(ctx, "k", "v", 0); err != nil {
		t.Fatal(err)
	}
	if got, err := c.Do(ctx, "BYE"); err != nil || got != "BYE" {
		t.Fatalf("BYE: %v %v", got, err)
	}
	time.Sleep(50 * time.Millisecond)
	if got, err := c.Get(ctx, "k"); err != nil || got != "v" {
		t.Fatalf("GET after the server closed the idle connection: %q, %v", got, err)
	}
	if n := s.conns.Load(); n != 2 {
		t.Fatalf("%d connections, want 2", n)
	}
}

func TestBulkDecodedByLength(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr())
	v, err := c.Do(context.Background(), "BINARY")
	if err != nil || v != "a\r\nb\x00c" {
		t.Fatalf("got %q, %v", v, err)
	}
	if p, err := c.Ping(context.Background()); err != nil || p != "PONG" {
		t.Fatalf("stream desynchronized: %q, %v", p, err)
	}
}

func TestSubSecondExpirationUsesPX(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr())
	if err := c.Set(context.Background(), "k", "v", 1500*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	last := s.commands[len(s.commands)-1]
	s.mu.Unlock()
	if strings.Join(last, " ") != "SET k v PX 1500" {
		t.Fatalf("sent %q", last)
	}
}

func TestErrorReply(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr())
	ctx := context.Background()
	_, err := c.Do(ctx, "BADCMD")
	var re cachestorm.RedisError
	if !errors.As(err, &re) || !strings.HasPrefix(string(re), "ERR unknown command") {
		t.Fatalf("got %v", err)
	}
	if p, err := c.Ping(ctx); err != nil || p != "PONG" {
		t.Fatalf("connection unusable after error reply: %q, %v", p, err)
	}
	if n := s.conns.Load(); n != 1 {
		t.Fatalf("error reply discarded the connection: %d dials", n)
	}
}

func TestNestedErrorKeepsStreamInSync(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr())
	v, err := c.Do(context.Background(), "MIXED")
	if err != nil {
		t.Fatal(err)
	}
	arr := v.([]interface{})
	if len(arr) != 3 || arr[0] != "a" || arr[2] != nil {
		t.Fatalf("got %#v", arr)
	}
	if re, ok := arr[1].(cachestorm.RedisError); !ok || string(re) != "ERR inner" {
		t.Fatalf("inner error %#v", arr[1])
	}
	if p, err := c.Ping(context.Background()); err != nil || p != "PONG" {
		t.Fatalf("stream desynchronized: %q, %v", p, err)
	}
}

func TestProtocolErrorDoesNotPanic(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr())
	_, err := c.Do(context.Background(), "BADBULK")
	if !errors.Is(err, cachestorm.ErrProtocol) {
		t.Fatalf("got %v", err)
	}
	if p, err := c.Ping(context.Background()); err != nil || p != "PONG" {
		t.Fatalf("got %q, %v", p, err)
	}
}

func TestNilBulk(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr())
	v, err := c.Do(context.Background(), "GET", "missing")
	if err != nil || v != nil {
		t.Fatalf("got %#v, %v", v, err)
	}
	got, err := c.Get(context.Background(), "missing")
	if err != nil || got != "" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestContextCancelDiscardsConnection(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr(), cachestorm.WithPoolSize(1))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Do(ctx, "SLOW")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
	if time.Since(start) > 250*time.Millisecond {
		t.Fatal("context deadline not honored")
	}
	if p, err := c.Ping(context.Background()); err != nil || p != "PONG" {
		t.Fatalf("late SLOW reply leaked into next command: %q, %v", p, err)
	}
}

func TestPoolSizeBoundsConnections(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr(), cachestorm.WithPoolSize(2))
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Do(context.Background(), "SLOW"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := s.conns.Load(); n > 2 {
		t.Fatalf("%d connections opened with PoolSize 2", n)
	}
}

func TestPipelineBatches(t *testing.T) {
	s := newFakeServer(t)
	s.batch = 4
	c := newClient(t, s.addr())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pipe := c.Pipeline()
	pipe.Set(ctx, "a", "1", 0)
	pipe.Get(ctx, "a")
	pipe.Do(ctx, "BADCMD")
	pipe.Get(ctx, "missing")
	cmds, err := pipe.Exec(ctx)
	var re cachestorm.RedisError
	if !errors.As(err, &re) {
		t.Fatalf("Exec error %v", err)
	}
	if len(cmds) != 4 {
		t.Fatalf("got %d cmds", len(cmds))
	}
	if cmds[0].Val != "OK" || cmds[1].Val != "1" || cmds[2].Err == nil || cmds[3].Val != nil || cmds[3].Err != nil {
		t.Fatalf("got %+v", cmds)
	}
	if n := s.conns.Load(); n != 1 {
		t.Fatalf("pipeline used %d connections", n)
	}
	if again, err := pipe.Exec(ctx); err != nil || len(again) != 0 {
		t.Fatalf("queue not cleared: %d, %v", len(again), err)
	}
}

func TestPubSubReceive(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr())
	ps := c.Subscribe(context.Background(), "news")
	defer ps.Close()

	short, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_, err := ps.ReceiveMessage(short)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}

	s.pubsub <- "hello"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	msg, err := ps.ReceiveMessage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Channel != "news" || msg.Payload != "hello" {
		t.Fatalf("got %+v", msg)
	}

	if err := ps.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ps.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ps.ReceiveMessage(ctx); !errors.Is(err, cachestorm.ErrClosed) {
		t.Fatalf("got %v", err)
	}
	s.pubsub <- ""
}

func TestPubSubEndsOnClientClose(t *testing.T) {
	s := newFakeServer(t)
	c := newClient(t, s.addr())
	ps := c.Subscribe(context.Background(), "news")
	done := make(chan error, 1)
	go func() {
		_, err := ps.ReceiveMessage(context.Background())
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	c.Close()
	select {
	case err := <-done:
		if !errors.Is(err, cachestorm.ErrClosed) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ReceiveMessage not interrupted by Client.Close")
	}
	if ps2 := c.Subscribe(context.Background(), "x"); ps2 != nil {
		if _, err := ps2.ReceiveMessage(context.Background()); !errors.Is(err, cachestorm.ErrClosed) {
			t.Fatalf("Subscribe after Close: %v", err)
		}
	}
	s.pubsub <- ""
}
