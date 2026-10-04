package command

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/sentinel"
)

// waitUntilListening polls a loopback port until something accepts, so a bind
// failure surfaces as a clean test failure instead of a later timeout.
func waitUntilListening(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)), 200*time.Millisecond)
		if err == nil {
			c.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("nothing began listening on port %d", port)
}

// TestConfiguredSeedIsUsedForDiscovery proves the `sentinel.seeds` list is not
// merely stored: a sentinel configured with a seed must actually reach that peer
// and record it, with no manual registration anywhere in the test.
//
// The observable is CKQUORUM, which counts the peers recorded for a master
// (excluding self, which it adds as +1). The seed is the ONLY way this sentinel
// can learn a peer exists — gossip announces to seeds and nothing else — so
// reaching 2 proves the configured seed was used.
func TestConfiguredSeedIsUsedForDiscovery(t *testing.T) {
	// The sentinel config and singleton are process-wide; restore both so this
	// test cannot leak into the rest of the package.
	prevSentinel, prevConfig := globalSentinel, sentinelConfig
	defer func() {
		StopSentinel()
		globalSentinel, sentinelConfig = prevSentinel, prevConfig
	}()

	// A real peer sentinel, listening on its own port and monitoring the same
	// master, so it records us under that master and answers our HELLO.
	peerPort := sentinelPort(t)
	peer := sentinel.New(sentinel.Config{
		ID: "seed-peer", Addr: "127.0.0.1", Port: peerPort, DownAfter: 5 * time.Second,
	})
	if err := peer.Monitor("m", "127.0.0.1", 1, 2); err != nil {
		t.Fatalf("peer Monitor failed: %v", err)
	}
	peerCtx, peerCancel := context.WithCancel(context.Background())
	defer peerCancel()
	go func() { _ = peer.Serve(peerCtx, peerPort) }()
	waitUntilListening(t, peerPort)

	// Configure through the same entry point server.New uses, with the seed
	// spelled the way an operator would put it in YAML.
	seed := net.JoinHostPort("127.0.0.1", fmt.Sprint(peerPort))
	ConfigureSentinel(sentinel.Config{
		ID: "seed-local", Addr: "127.0.0.1", Port: sentinelPort(t),
		Quorum: 2, DownAfter: 5 * time.Second,
		Seeds: []string{seed},
	})
	globalSentinel = nil // force a fresh build from the configured value

	s := EnsureSentinel()
	if err := s.Monitor("m", "127.0.0.1", 1, 2); err != nil {
		t.Fatalf("Monitor failed: %v", err)
	}

	// Guard: before the gossip loop runs, the only sentinel it knows is itself.
	if n, err := GetSentinel().CKQUORUM("m"); err != nil || n != 1 {
		t.Fatalf("harness broken: CKQUORUM = %d (err %v) before gossip, want 1", n, err)
	}

	if err := StartSentinel(); err != nil {
		t.Fatalf("StartSentinel failed: %v", err)
	}

	// The gossip loop ticks every 2s, so allow comfortably more than one tick.
	deadline := time.Now().Add(8 * time.Second)
	last := 0
	for time.Now().Before(deadline) {
		if n, err := GetSentinel().CKQUORUM("m"); err == nil {
			last = n
			if n >= 2 {
				t.Logf("sentinel configured with seed %s discovered it (CKQUORUM=%d)", seed, n)
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("FAIL: a sentinel configured with seed %s never discovered it "+
		"(CKQUORUM=%d, want 2) — the configured seed did not reach the gossip loop", seed, last)
}

// TestConfiguredSeedReachesConstructedSentinel covers the wiring half quickly:
// without starting any loop, the value handed to ConfigureSentinel must be what
// EnsureSentinel builds from. SentinelConfigTemplate is the observable, and it
// is the same value EnsureSentinel passes to sentinel.New.
func TestConfiguredSeedReachesConstructedSentinel(t *testing.T) {
	prevSentinel, prevConfig := globalSentinel, sentinelConfig
	defer func() {
		StopSentinel()
		globalSentinel, sentinelConfig = prevSentinel, prevConfig
	}()

	seeds := []string{"10.0.0.1:26379", "10.0.0.2:26379"}
	ConfigureSentinel(sentinel.Config{
		ID: "wired", Addr: "127.0.0.1", Port: 26379, Seeds: seeds,
	})

	got := SentinelConfigTemplate().Seeds
	if len(got) != len(seeds) {
		t.Fatalf("template seeds = %v, want %v", got, seeds)
	}
	for i := range seeds {
		if got[i] != seeds[i] {
			t.Errorf("template seeds[%d] = %q, want %q", i, got[i], seeds[i])
		}
	}

	// And the singleton built from that template must be non-nil, i.e. the
	// seeds-bearing config is usable rather than rejected.
	if s := EnsureSentinel(); s == nil {
		t.Fatal("EnsureSentinel returned nil for a seeds-bearing config")
	}
}

// TestConfigureSentinelReplacesSeeds proves a second ConfigureSentinel replaces
// the seed list rather than appending to it — otherwise a restart with changed
// config would keep announcing to peers that were removed.
func TestConfigureSentinelReplacesSeeds(t *testing.T) {
	prevSentinel, prevConfig := globalSentinel, sentinelConfig
	defer func() {
		StopSentinel()
		globalSentinel, sentinelConfig = prevSentinel, prevConfig
	}()

	ConfigureSentinel(sentinel.Config{Seeds: []string{"10.0.0.9:26379"}})
	ConfigureSentinel(sentinel.Config{Seeds: []string{"10.0.0.1:26379"}})

	got := SentinelConfigTemplate().Seeds
	if len(got) != 1 || got[0] != "10.0.0.1:26379" {
		t.Errorf("seeds = %v, want [10.0.0.1:26379] — the second ConfigureSentinel "+
			"must replace, not accumulate", got)
	}
}
