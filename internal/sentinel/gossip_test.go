package sentinel

import (
	"net"
	"sync"
	"testing"
	"time"
)

// pongServer is a stand-in for a peer sentinel: it accepts connections and
// answers PING with +PONG, which is exactly what Serve/handleCommand speaks.
type pongServer struct {
	ln   net.Listener
	mu   sync.Mutex
	seen int
}

func startPongServer(t *testing.T) *pongServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	p := &pongServer{ln: ln}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			p.mu.Lock()
			p.seen++
			p.mu.Unlock()
			_, _ = c.Write([]byte("+PONG\r\n"))
			c.Close()
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return p
}

func (p *pongServer) port() int { return p.ln.Addr().(*net.TCPAddr).Port }

func (p *pongServer) pings() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.seen
}

// TestGossipPopulatesPeerTableAndSelf proves gossipSentinels now populates
// s.sentinels, which was previously an empty body — so the table stayed empty
// forever, CKQUORUM always answered 1, and checkODown's downCount was always 0.
//
// Self must be registered as a peer (that is what makes the table non-empty),
// and it must NOT be counted twice by the quorum math.
func TestGossipPopulatesPeerTableAndSelf(t *testing.T) {
	s := New(Config{ID: "me", Addr: "127.0.0.1", Port: 26379, Quorum: 2, DownAfter: time.Second})
	if err := s.Monitor("m", "127.0.0.1", 1, 2); err != nil {
		t.Fatalf("Monitor failed: %v", err)
	}

	// Guard: the table must genuinely be empty before gossip runs.
	s.mu.RLock()
	before := len(s.sentinels["m"])
	s.mu.RUnlock()
	if before != 0 {
		t.Fatalf("harness broken: peer table already has %d entries before gossip", before)
	}

	s.gossipSentinels()

	s.mu.RLock()
	peers := s.sentinels["m"]
	s.mu.RUnlock()

	if len(peers) != 1 {
		t.Fatalf("peer table has %d entries, want 1 (self): gossipSentinels did not register self",
			len(peers))
	}
	if peers[0].ID != "me" {
		t.Errorf("peer ID = %q, want \"me\"", peers[0].ID)
	}
	if time.Since(peers[0].LastSeen) > time.Second {
		t.Error("self peer LastSeen was not refreshed by gossip")
	}
	t.Log("gossipSentinels registered self in the peer table")
}

// TestGossipPeerWithinDownAfterContributesToQuorum is the round proof of the
// contract the task asks for: a peer seen within down_after must contribute to
// the quorum, so quorum 2 is reached by this sentinel plus that peer.
//
// Without this, checkODownLocked's downCount was permanently 0 and a quorum of
// 2 could never be met by a cluster that actually had a live peer.
func TestGossipPeerWithinDownAfterContributesToQuorum(t *testing.T) {
	peer := startPongServer(t)

	s := New(Config{ID: "me", Addr: "127.0.0.1", Port: 26379, DownAfter: 5 * time.Second})
	// Master quorum 2: this sentinel alone is NOT enough.
	if err := s.Monitor("m", "127.0.0.1", 1, 2); err != nil {
		t.Fatalf("Monitor failed: %v", err)
	}

	s.gossipSentinels()

	// With only self registered, quorum 2 is unmet (self counts once, via +1).
	s.mu.RLock()
	selfOnly := s.checkODownLocked(s.masters["m"], s.sentinels["m"])
	s.mu.RUnlock()
	if selfOnly {
		t.Fatal("harness broken: a lone sentinel already met quorum 2; the test cannot show " +
			"the peer contributing anything")
	}

	// Register a live peer and let gossip discover it is alive.
	s.mu.Lock()
	s.sentinels["m"] = append(s.sentinels["m"], &SentinelPeer{
		ID:       "peer-2",
		Addr:     "127.0.0.1",
		Port:     peer.port(),
		LastSeen: time.Now(),
	})
	s.mu.Unlock()

	s.gossipSentinels()

	if peer.pings() == 0 {
		t.Error("gossip did not PING the registered peer")
	}

	s.mu.RLock()
	quorumMet := s.checkODownLocked(s.masters["m"], s.sentinels["m"])
	s.mu.RUnlock()

	if !quorumMet {
		t.Fatal("FAIL: a peer seen within down_after did not contribute to the quorum — " +
			"quorum 2 still unmet with this sentinel plus one live peer")
	}
	t.Log("a peer seen within down_after contributed to the quorum")
}

// TestGossipSelfIsNotDoubleCounted pins the invariant that makes the previous
// test meaningful: the "+1" in checkODownLocked already counts this sentinel, so
// the self entry gossip registers must be skipped by the counting loop. Without
// the skip, a lone sentinel would satisfy quorum 2 on its own.
func TestGossipSelfIsNotDoubleCounted(t *testing.T) {
	s := New(Config{ID: "me", Addr: "127.0.0.1", Port: 26379, DownAfter: time.Second})
	if err := s.Monitor("m", "127.0.0.1", 1, 2); err != nil {
		t.Fatalf("Monitor failed: %v", err)
	}

	s.gossipSentinels()

	s.mu.RLock()
	met := s.checkODownLocked(s.masters["m"], s.sentinels["m"])
	s.mu.RUnlock()

	if met {
		t.Error("FAIL: self was counted twice (own peer entry plus the +1), so a lone sentinel " +
			"satisfied quorum 2")
	}
}

// TestCKQUORUMCountsDiscoveredPeer checks the user-visible command reports the
// discovered peer too — CKQUORUM is the surface a client actually calls.
func TestCKQUORUMCountsDiscoveredPeer(t *testing.T) {
	peer := startPongServer(t)

	s := New(Config{ID: "me", Addr: "127.0.0.1", Port: 26379, DownAfter: 5 * time.Second})
	if err := s.Monitor("m", "127.0.0.1", 1, 2); err != nil {
		t.Fatalf("Monitor failed: %v", err)
	}
	s.gossipSentinels()

	if got, err := s.CKQUORUM("m"); err != nil || got != 1 {
		t.Fatalf("harness broken: CKQUORUM with only self = %d (err %v), want 1", got, err)
	}

	s.mu.Lock()
	s.sentinels["m"] = append(s.sentinels["m"], &SentinelPeer{
		ID: "peer-2", Addr: "127.0.0.1", Port: peer.port(), LastSeen: time.Now(),
	})
	s.mu.Unlock()
	s.gossipSentinels()

	got, err := s.CKQUORUM("m")
	if err != nil {
		t.Fatalf("CKQUORUM errored: %v", err)
	}
	if got != 2 {
		t.Fatalf("FAIL: CKQUORUM reported %d sentinels, want 2 (this one plus the live peer)", got)
	}
	t.Log("CKQUORUM reported the discovered peer")
}

// TestGossipPrunesSilentPeers proves the table cannot grow without bound: a peer
// that stops answering is dropped once it has been quiet past the retention
// window, while self is retained.
func TestGossipPrunesSilentPeers(t *testing.T) {
	s := New(Config{ID: "me", Addr: "127.0.0.1", Port: 26379, DownAfter: 10 * time.Millisecond})
	if err := s.Monitor("m", "127.0.0.1", 1, 2); err != nil {
		t.Fatalf("Monitor failed: %v", err)
	}

	// A peer on a port that refuses connections: it can never answer a PING.
	dead := deadPort(t)
	s.mu.Lock()
	s.sentinels["m"] = append(s.sentinels["m"], &SentinelPeer{
		ID: "ghost", Addr: "127.0.0.1", Port: dead, LastSeen: time.Now(),
	})
	s.mu.Unlock()

	// Let its LastSeen fall outside the retention window (3 x downAfter).
	time.Sleep(60 * time.Millisecond)
	s.gossipSentinels()

	s.mu.RLock()
	var ids []string
	for _, p := range s.sentinels["m"] {
		ids = append(ids, p.ID)
	}
	s.mu.RUnlock()

	for _, id := range ids {
		if id == "ghost" {
			t.Errorf("silent peer was not pruned; peer table is now %v", ids)
		}
	}
	if len(ids) != 1 || ids[0] != "me" {
		t.Errorf("peer table = %v, want just [me]", ids)
	}
	t.Logf("pruned the silent peer; table is now %v", ids)
}

// TestGossipDoesNotDoubleCountUnderRepeatedTicks guards a subtle regression:
// gossip runs every 2s, so repeated calls must not append a new self entry each
// time.
func TestGossipDoesNotDoubleCountUnderRepeatedTicks(t *testing.T) {
	s := New(Config{ID: "me", Addr: "127.0.0.1", Port: 26379, DownAfter: time.Second})
	if err := s.Monitor("m", "127.0.0.1", 1, 2); err != nil {
		t.Fatalf("Monitor failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		s.gossipSentinels()
	}

	s.mu.RLock()
	n := len(s.sentinels["m"])
	s.mu.RUnlock()

	if n != 1 {
		t.Errorf("after 5 gossip ticks the peer table holds %d entries, want 1 — self is being "+
			"appended repeatedly", n)
	}
}