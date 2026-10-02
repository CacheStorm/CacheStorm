package sentinel

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

// helloExchange sends a SENTINEL HELLO to a served sentinel and returns the
// identity it replies with, proving the peer protocol is reachable.
func helloExchange(t *testing.T, port int, addr string) []string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to sentinel on port %d: %v", port, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	if _, err := conn.Write([]byte("SENTINEL HELLO " + addr + "\r\n")); err != nil {
		t.Fatalf("write HELLO failed: %v", err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read HELLO reply failed: %v", err)
	}
	return splitFields(line)
}

func splitFields(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ' ' || r == '\r' || r == '\n' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// peerIDs returns the peer table for a master, excluding this sentinel's own
// entry, which gossip always registers for liveness bookkeeping.
func peerIDs(s *Sentinel, master string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []string
	for _, p := range s.sentinels[master] {
		if p.ID == s.id {
			continue
		}
		out = append(out, p.ID)
	}
	return out
}

// TestTwoSentinelsLearnEachOtherWithoutManualRegistration is the round proof.
// Two sentinels both run the real Serve listener, and one is configured as the
// other's seed. A single gossip tick must make each aware of the other with
// NO entry ever being added to s.sentinels by hand.
func TestTwoSentinelsLearnEachOtherWithoutManualRegistration(t *testing.T) {
	// Reserve two ports, then hand them to the sentinels.
	portA := reservePort(t)
	portB := reservePort(t)

	a := New(Config{ID: "sentinel-a", Addr: "127.0.0.1", Port: portA, DownAfter: 5 * time.Second})
	b := New(Config{ID: "sentinel-b", Addr: "127.0.0.1", Port: portB, DownAfter: 5 * time.Second})

	stopA := serveOn(t, a, portA)
	defer stopA()
	stopB := serveOn(t, b, portB)
	defer stopB()

	// Both monitor the same master — this is the table the peer is recorded in.
	for _, s := range []*Sentinel{a, b} {
		if err := s.Monitor("m", "127.0.0.1", 1, 2); err != nil {
			t.Fatalf("Monitor failed: %v", err)
		}
	}

	// A seeds with B. B seeds with A. Nothing else is configured.
	a.seeds = []string{net.JoinHostPort("127.0.0.1", fmt.Sprint(portB))}
	b.seeds = []string{net.JoinHostPort("127.0.0.1", fmt.Sprint(portA))}

	// Guard: before any gossip, neither knows about the other.
	if ids := peerIDs(a, "m"); len(ids) != 0 {
		t.Fatalf("harness broken: sentinel-a already knows peers %v before gossip", ids)
	}

	// One gossip tick from each side.
	a.gossipSentinels()
	b.gossipSentinels()

	idsA := peerIDs(a, "m")
	idsB := peerIDs(b, "m")

	if len(idsA) == 0 {
		t.Fatal("FAIL: sentinel-a learned no peers from gossip; sentinels did not discover each other")
	}
	if len(idsB) == 0 {
		t.Fatal("FAIL: sentinel-b learned no peers from gossip; sentinels did not discover each other")
	}
	t.Logf("discovered without manual registration: A sees %v, B sees %v", idsA, idsB)

	// Each must know the OTHER, not just something.
	wantA := net.JoinHostPort("127.0.0.1", fmt.Sprint(portB))
	wantB := net.JoinHostPort("127.0.0.1", fmt.Sprint(portA))
	if !contains(idsA, wantA) {
		t.Errorf("FAIL: sentinel-a does not list sentinel-b (%s); got %v", wantA, idsA)
	}
	if !contains(idsB, wantB) {
		t.Errorf("FAIL: sentinel-b does not list sentinel-a (%s); got %v", wantB, idsB)
	}

	// And the discovered peer must count toward quorum: 2 sentinels, quorum 2.
	if n, err := a.CKQUORUM("m"); err != nil || n != 2 {
		t.Errorf("FAIL: sentinel-a reports %d reachable sentinels (err %v), want 2 — "+
			"the discovered peer is not counted toward quorum", n, err)
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// reservePort picks a free loopback port and releases it for immediate reuse.
func reservePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve a port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// serveOn starts s.Serve on the given port and returns a shutdown func.
//
// The port is passed in rather than chosen here because announceTo reports
// s.port from config, so the configured port and the listening port must be the
// same number or peers would be told to dial the wrong address.
func serveOn(t *testing.T, s *Sentinel, port int) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = s.Serve(ctx, port) }()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", net.JoinHostPort(s.addr, fmt.Sprint(port)), 200*time.Millisecond)
		if err == nil {
			c.Close()
			return cancel
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	t.Fatalf("sentinel %s never began listening on port %d", s.id, port)
	return func() {}
}

// TestHelloRecordsPeerOnEveryMonitoredMaster proves HELLO registers the sender
// against EVERY monitored master, not just one — otherwise a sentinel
// monitoring three masters would only count as a peer for one of them.
func TestHelloRecordsPeerOnEveryMonitoredMaster(t *testing.T) {
	portA := reservePort(t)
	portB := reservePort(t)

	a := New(Config{ID: "sentinel-a", Addr: "127.0.0.1", Port: portA, DownAfter: 5 * time.Second})
	b := New(Config{ID: "sentinel-b", Addr: "127.0.0.1", Port: portB, DownAfter: 5 * time.Second})

	stopA := serveOn(t, a, portA)
	defer stopA()
	stopB := serveOn(t, b, portB)
	defer stopB()

	for _, name := range []string{"m1", "m2", "m3"} {
		if err := b.Monitor(name, "127.0.0.1", 1, 2); err != nil {
			t.Fatalf("Monitor %s failed: %v", name, err)
		}
	}

	// A announces to B once.
	a.seeds = []string{net.JoinHostPort("127.0.0.1", fmt.Sprint(portB))}
	a.gossipSentinels()

	for _, name := range []string{"m1", "m2", "m3"} {
		ids := peerIDs(b, name)
		want := net.JoinHostPort("127.0.0.1", fmt.Sprint(portA))
		if !contains(ids, want) {
			t.Errorf("FAIL: sentinel-b did not record the announcer for master %q; got %v", name, ids)
		}
	}
	t.Log("HELLO registered the announcing peer on every monitored master")
}

// TestHelloIsIdempotent proves repeated gossip does not grow the peer table —
// a peer is refreshed, not re-appended, or the table would grow without bound.
func TestHelloIsIdempotent(t *testing.T) {
	portA := reservePort(t)
	portB := reservePort(t)

	a := New(Config{ID: "sentinel-a", Addr: "127.0.0.1", Port: portA, DownAfter: 5 * time.Second})
	b := New(Config{ID: "sentinel-b", Addr: "127.0.0.1", Port: portB, DownAfter: 5 * time.Second})

	stopA := serveOn(t, a, portA)
	defer stopA()
	stopB := serveOn(t, b, portB)
	defer stopB()

	for _, s := range []*Sentinel{a, b} {
		if err := s.Monitor("m", "127.0.0.1", 1, 2); err != nil {
			t.Fatalf("Monitor failed: %v", err)
		}
	}
	a.seeds = []string{net.JoinHostPort("127.0.0.1", fmt.Sprint(portB))}

	for i := 0; i < 5; i++ {
		a.gossipSentinels()
	}

	ids := peerIDs(a, "m")
	if len(ids) != 1 {
		t.Errorf("FAIL: after 5 gossip ticks sentinel-a knows %d peers (%v), want exactly 1 — "+
			"HELLO is appending duplicates instead of refreshing", len(ids), ids)
	}
	t.Logf("after 5 ticks sentinel-a knows exactly %v", ids)
}

// TestHelloRejectsMalformedInput pins the argument validation: HELLO with no
// port, a non-numeric port, or an out-of-range port must be refused rather than
// recorded as a bogus peer that then never answers a PING.
func TestHelloRejectsMalformedInput(t *testing.T) {
	portA := reservePort(t)
	portB := reservePort(t)

	a := New(Config{ID: "sentinel-a", Addr: "127.0.0.1", Port: portA, DownAfter: 5 * time.Second})
	b := New(Config{ID: "sentinel-b", Addr: "127.0.0.1", Port: portB, DownAfter: 5 * time.Second})

	stopA := serveOn(t, a, portA)
	defer stopA()
	stopB := serveOn(t, b, portB)
	defer stopB()

	for _, s := range []*Sentinel{a, b} {
		if err := s.Monitor("m", "127.0.0.1", 1, 2); err != nil {
			t.Fatalf("Monitor failed: %v", err)
		}
	}

	for _, bad := range []string{
		"SENTINEL HELLO",
		"SENTINEL HELLO 127.0.0.1",
		"SENTINEL HELLO 127.0.0.1 notaport",
		"SENTINEL HELLO 127.0.0.1 0",
		"SENTINEL HELLO 127.0.0.1 70000",
	} {
		fields := helloExchange(t, portB, bad)
		if len(fields) == 0 || fields[0] != "-ERR" {
			t.Errorf("reply to %q = %v, want an -ERR rejection", bad, fields)
		}
	}

	if ids := peerIDs(b, "m"); len(ids) != 0 {
		t.Errorf("malformed HELLOs recorded bogus peers: %v", ids)
	}
	t.Log("HELLO rejected missing, non-numeric and out-of-range ports")
}
