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

// MSET takes the stripes of its keys rather than the whole gate; a reader
// still never sees half of one: MGET of the keys always finds them equal
// while writers keep setting them all to one value.
func TestMSETIsAtomicToReaders(t *testing.T) {
	addr := isolationServer(t)
	keys := []string{"ma", "mb", "mc", "md"}
	var stop atomic.Bool
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			c := isoDial(t, addr)
			for i := 0; !stop.Load(); i++ {
				v := strconv.Itoa(w*1000000 + i)
				args := []string{"MSET"}
				for _, k := range keys {
					args = append(args, k, v)
				}
				c.do(args...)
			}
		}(w)
	}
	r := isoDial(t, addr)
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		_ = r.c.SetDeadline(time.Now().Add(5 * time.Second))
		if _, err := r.c.Write([]byte(isoFrame(append([]string{"MGET"}, keys...)...))); err != nil {
			t.Fatal(err)
		}
		head, _ := r.r.ReadString('\n')
		if head != "*4\r\n" {
			t.Fatalf("MGET reply %q", head)
		}
		var vals []string
		for range keys {
			l, _ := r.r.ReadString('\n')
			if l == "$-1\r\n" {
				vals = append(vals, "")
				continue
			}
			b, _ := r.r.ReadString('\n')
			vals = append(vals, b)
		}
		for _, v := range vals[1:] {
			if v != vals[0] {
				stop.Store(true)
				wg.Wait()
				t.Fatalf("MGET saw a partial MSET: %q", vals)
			}
		}
	}
	stop.Store(true)
	wg.Wait()
}

// rawExchange writes raw on a fresh connection and returns the replies that
// arrive within a short wait.
func rawExchange(t *testing.T, addr, raw string) string {
	t.Helper()
	c := isoDial(t, addr)
	if _, err := c.c.Write([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	buf := make([]byte, 4096)
	for {
		_ = c.c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		n, err := c.c.Read(buf)
		out.Write(buf[:n])
		if err != nil {
			return out.String()
		}
	}
}

// CLIENT REPLY OFF and SKIP silence the error of an unknown command too, and
// HELLO inside a transaction switches the connection's protocol for the
// rest of the session, as in Redis.
func TestReplyModeAndProtocolSwitchInTransactions(t *testing.T) {
	addr := isolationServer(t)
	got := rawExchange(t, addr, isoFrame("CLIENT", "REPLY", "OFF")+isoFrame("NOSUCH")+isoFrame("CLIENT", "REPLY", "ON")+isoFrame("PING"))
	if got != "+OK\r\n+PONG\r\n" {
		t.Errorf("CLIENT REPLY OFF + unknown command: %q", got)
	}
	got = rawExchange(t, addr, isoFrame("CLIENT", "REPLY", "SKIP")+isoFrame("NOSUCH")+isoFrame("PING"))
	if got != "+PONG\r\n" {
		t.Errorf("CLIENT REPLY SKIP + unknown command: %q", got)
	}
	got = rawExchange(t, addr, isoFrame("MULTI")+isoFrame("HELLO", "3")+isoFrame("EXEC")+isoFrame("GET", "nokey"))
	if !strings.Contains(got, "*1\r\n%") || !strings.HasSuffix(got, "_\r\n") {
		t.Errorf("HELLO 3 inside MULTI: %q", got)
	}
}
