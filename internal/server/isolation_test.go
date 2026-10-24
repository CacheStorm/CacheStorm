package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

func isolationServer(t *testing.T) string {
	t.Helper()
	cfg := config.Default()
	cfg.Server.Bind = "127.0.0.1"
	cfg.Server.Port = 0
	cfg.HTTP.Enabled = false
	cfg.Plugins.Metrics.Enabled = false
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Stop(context.Background()) })
	return s.listener.Addr().String()
}

type isoClient struct {
	c net.Conn
	r *bufio.Reader
}

func isoDial(t *testing.T, addr string) *isoClient {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return &isoClient{c: c, r: bufio.NewReader(c)}
}

func isoFrame(args ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	return b.String()
}

// do sends one command and returns the first line of its reply, plus the
// bulk payload when the reply is a bulk string.
func (ic *isoClient) do(args ...string) string {
	_ = ic.c.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := ic.c.Write([]byte(isoFrame(args...))); err != nil {
		return "write error: " + err.Error()
	}
	line, err := ic.r.ReadString('\n')
	if err != nil {
		return "read error: " + err.Error()
	}
	if strings.HasPrefix(line, "$") && line != "$-1\r\n" {
		body, _ := ic.r.ReadString('\n')
		return strings.TrimSuffix(body, "\r\n")
	}
	if strings.HasPrefix(line, "*") {
		n, _ := strconv.Atoi(strings.TrimSpace(line[1:]))
		for i := 0; i < n; i++ {
			ic.r.ReadString('\n')
		}
	}
	return strings.TrimSuffix(line, "\r\n")
}

func countOddReads(t *testing.T, addr string, write func(*isoClient)) int64 {
	t.Helper()
	var odd atomic.Int64
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		reader := isoDial(t, addr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if n, err := strconv.Atoi(reader.do("GET", "x")); err == nil && n%2 == 1 {
					odd.Add(1)
				}
			}
		}()
	}
	writer := isoDial(t, addr)
	for i := 0; i < 1000; i++ {
		write(writer)
	}
	close(stop)
	wg.Wait()
	return odd.Load()
}

// Each transaction adds 2 to x; a reader that sees an odd value saw one INCR
// of a transaction without the other.
func TestExecIsIsolatedFromReaders(t *testing.T) {
	addr := isolationServer(t)
	odd := countOddReads(t, addr, func(w *isoClient) {
		w.do("MULTI")
		w.do("INCR", "x")
		w.do("INCR", "x")
		w.do("EXEC")
	})
	if odd > 0 {
		t.Fatalf("readers saw a transaction half applied %d times", odd)
	}
}

func TestScriptIsIsolatedFromReaders(t *testing.T) {
	addr := isolationServer(t)
	odd := countOddReads(t, addr, func(w *isoClient) {
		w.do("EVAL", "redis.call('INCR', KEYS[1]); return redis.call('INCR', KEYS[1])", "1", "x")
	})
	if odd > 0 {
		t.Fatalf("readers saw a script half applied %d times", odd)
	}
}

// A client that sends commands and never reads the replies must not hold the
// write gate on a full socket buffer: EXEC needs the gate exclusively, and
// every other client would wait behind it.
func TestStalledReaderDoesNotBlockOthers(t *testing.T) {
	addr := isolationServer(t)
	setup := isoDial(t, addr)
	if got := setup.do("SET", "big", strings.Repeat("v", 1<<20)); got != "+OK" {
		t.Fatalf("SET: %q", got)
	}

	stalled, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer stalled.Close()
	go func() {
		frame := []byte(isoFrame("GET", "big"))
		for i := 0; i < 200; i++ {
			if _, err := stalled.Write(frame); err != nil {
				return
			}
		}
	}()
	time.Sleep(300 * time.Millisecond)

	other := isoDial(t, addr)
	done := make(chan string, 1)
	go func() {
		other.do("MULTI")
		other.do("INCR", "n")
		done <- other.do("EXEC")
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("EXEC stalled behind a client that does not read its replies")
	}
	if got := isoDial(t, addr).do("GET", "n"); got != "1" {
		t.Fatalf("GET n = %q", got)
	}
}

// A pipeline is answered in order, and the replies of a long one are not
// lost when they outgrow the reply buffer.
func TestPipelinedRepliesArriveInOrder(t *testing.T) {
	addr := isolationServer(t)
	c := isoDial(t, addr)
	var b strings.Builder
	const n = 5000
	for i := 0; i < n; i++ {
		b.WriteString(isoFrame("INCR", "p"))
	}
	go func() { _, _ = c.c.Write([]byte(b.String())) }()
	_ = c.c.SetDeadline(time.Now().Add(10 * time.Second))
	for i := 1; i <= n; i++ {
		line, err := c.r.ReadString('\n')
		if err != nil || line != ":"+strconv.Itoa(i)+"\r\n" {
			t.Fatalf("reply %d: %q %v", i, line, err)
		}
	}
}

func TestAppendStripeKeepsSortedUnique(t *testing.T) {
	var s []int
	for _, x := range []int{7, 3, 7, 200, 0, 3} {
		s = appendStripe(s, x)
	}
	if fmt.Sprint(s) != "[0 3 7 200]" {
		t.Fatalf("stripes %v", s)
	}
}

// A pipeline that ends with QUIT gets every reply before the connection
// closes; the replies kept for the pipeline used to be dropped.
func TestPipelineEndingInQuitDeliversAllReplies(t *testing.T) {
	addr := isolationServer(t)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write([]byte(isoFrame("SET", "k", "v") + isoFrame("GET", "k") + isoFrame("QUIT"))); err != nil {
		t.Fatal(err)
	}
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	var got strings.Builder
	buf := make([]byte, 256)
	for {
		n, err := c.Read(buf)
		got.Write(buf[:n])
		if err != nil {
			break
		}
	}
	if got.String() != "+OK\r\n$1\r\nv\r\n+OK\r\n" {
		t.Fatalf("replies %q", got.String())
	}
}
