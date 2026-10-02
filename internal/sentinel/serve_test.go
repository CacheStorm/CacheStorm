package sentinel

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// serveSentinel starts a Sentinel's peer listener on an ephemeral port and
// returns the bound port plus a shutdown func. Port 0 lets the OS choose, so
// two sentinels in one test never collide.
func serveSentinel(t *testing.T, s *Sentinel) (int, func()) {
	t.Helper()

	// Serve binds s.addr:port. Grab a free port first, then close the probe so
	// the real listener can take it.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve a port: %v", err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	probe.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan error, 1)
	go func() {
		ready <- s.Serve(ctx, port)
	}()

	// Wait for the listener to actually be accepting before the test proceeds,
	// so a bind failure is reported as a clean FAIL instead of a later timeout.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", net.JoinHostPort(s.addr, fmt.Sprint(port)), 200*time.Millisecond)
		if err == nil {
			c.Close()
			return port, cancel
		}
		select {
		case serveErr := <-ready:
			cancel()
			t.Fatalf("Serve returned before accepting: %v", serveErr)
		case <-time.After(20 * time.Millisecond):
		}
	}
	cancel()
	t.Fatalf("sentinel listener never became ready on port %d", port)
	return 0, func() {}
}

// TestTwoSentinelsReachEachOtherOverTCP is the round proof: two Sentinel
// instances, each running the real Serve listener, must be able to reach each
// other over TCP using the same wire protocol their gossip loop uses.
//
// It closes the loop that none of the earlier sentinel tests did. gossip only
// ever PINGed a peer's recorded address, and nothing ever proved that address
// actually had a listener behind it. Here both sides really listen, so
// pingPeer is exercised against a live Serve.
func TestTwoSentinelsReachEachOtherOverTCP(t *testing.T) {
	a := New(Config{ID: "sentinel-a", Addr: "127.0.0.1", Quorum: 2, DownAfter: 5 * time.Second})
	b := New(Config{ID: "sentinel-b", Addr: "127.0.0.1", Quorum: 2, DownAfter: 5 * time.Second})

	portA, stopA := serveSentinel(t, a)
	defer stopA()
	portB, stopB := serveSentinel(t, b)
	defer stopB()

	// Each sentinel monitors the same master and learns the other as a peer.
	for _, s := range []*Sentinel{a, b} {
		if err := s.Monitor("m", "127.0.0.1", 1, 2); err != nil {
			t.Fatalf("Monitor failed: %v", err)
		}
	}
	a.sentinels["m"] = append(a.sentinels["m"], &SentinelPeer{
		ID: "sentinel-b", Addr: "127.0.0.1", Port: portB, LastSeen: time.Now(),
	})
	b.sentinels["m"] = append(b.sentinels["m"], &SentinelPeer{
		ID: "sentinel-a", Addr: "127.0.0.1", Port: portA, LastSeen: time.Now(),
	})

	// A reaches B and B reaches A.
	if !a.pingPeer(a.sentinels["m"][0]) {
		t.Error("FAIL: sentinel-a could not reach sentinel-b over TCP")
	}
	if !b.pingPeer(b.sentinels["m"][0]) {
		t.Error("FAIL: sentinel-b could not reach sentinel-a over TCP")
	}

	// And gossip keeps the peer alive rather than pruning it.
	a.gossipSentinels()
	if n, err := a.CKQUORUM("m"); err != nil || n != 2 {
		t.Errorf("FAIL: after gossip, sentinel-a reports %d reachable sentinels (err %v), "+
			"want 2 — the live peer did not survive the gossip tick", n, err)
	}
	t.Logf("sentinels reached each other over TCP (ports %d and %d) and stayed in quorum", portA, portB)
}

// TestServeRespondsToSentinelCommandsOverTCP proves the listener speaks the
// peer protocol, not just PING: a real client connection gets +PONG, +OK for a
// SENTINEL subcommand, and an error for an unknown command.
func TestServeRespondsToSentinelCommandsOverTCP(t *testing.T) {
	s := New(Config{ID: "sentinel-c", Addr: "127.0.0.1", Quorum: 2, DownAfter: time.Second})
	port, stop := serveSentinel(t, s)
	defer stop()

	if err := s.Monitor("watched", "127.0.0.1", 1, 2); err != nil {
		t.Fatalf("Monitor failed: %v", err)
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to the sentinel listener: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	r := bufio.NewReader(conn)

	exchange := func(req, wantPrefix string) {
		t.Helper()
		if _, err := conn.Write([]byte(req + "\r\n")); err != nil {
			t.Fatalf("write %q failed: %v", req, err)
		}
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("read reply to %q failed: %v", req, err)
		}
		if len(line) < len(wantPrefix) || line[:len(wantPrefix)] != wantPrefix {
			t.Errorf("reply to %q = %q, want prefix %q", req, line, wantPrefix)
		}
	}

	exchange("PING", "+PONG")
	// RESET answers a single-line integer; MASTERS returns a multi-line listing,
	// so it is not usable here without draining the whole payload first.
	exchange("SENTINEL RESET *", ":")
	exchange("BOGUS", "-ERR")
	t.Log("the Serve listener answered PING, SENTINEL RESET and an unknown command")
}

// TestServeStopsOnContextCancel proves the listener does not outlive its owner.
// Serve must return and release the port once ctx is cancelled, which is what
// lets Server.Stop() free the port during shutdown.
func TestServeStopsOnContextCancel(t *testing.T) {
	s := New(Config{ID: "sentinel-d", Addr: "127.0.0.1", Quorum: 2, DownAfter: time.Second})
	port, stop := serveSentinel(t, s)

	// Before cancel: reachable.
	if c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)), time.Second); err != nil {
		t.Fatalf("harness broken: listener not reachable before cancel: %v", err)
	} else {
		c.Close()
	}

	stop()

	// After cancel the port must be released. Retry briefly: closing a listener
	// frees the port immediately, but the OS may still be tearing the accept
	// loop down, so a short bounded poll avoids a flaky assertion.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)), 200*time.Millisecond)
		if err != nil {
			t.Log("Serve released the port on context cancel")
			return
		}
		c.Close()
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("FAIL: the sentinel listener still accepted connections after its context was cancelled")
}

// TestConcurrentServeIsSafe guards the accept loop: many simultaneous peer
// connections must all be served without racing or deadlocking.
func TestConcurrentServeIsSafe(t *testing.T) {
	s := New(Config{ID: "sentinel-e", Addr: "127.0.0.1", Quorum: 2, DownAfter: time.Second})
	port, stop := serveSentinel(t, s)
	defer stop()

	const n = 12
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)), 2*time.Second)
			if err != nil {
				errs <- err
				return
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
			if _, err := conn.Write([]byte("PING\r\n")); err != nil {
				errs <- err
				return
			}
			line, err := bufio.NewReader(conn).ReadString('\n')
			if err != nil {
				errs <- err
				return
			}
			if line != "+PONG\r\n" {
				errs <- fmt.Errorf("got %q, want +PONG", line)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent peer connection failed: %v", err)
	}
}
