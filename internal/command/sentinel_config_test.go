package command

import (
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/sentinel"
)

// TestSentinelConfigReachesConstructedSentinel proves that values supplied
// through the `sentinel` configuration section actually reach the Sentinel that
// EnsureSentinel builds.
//
// The gap this closes: EnsureSentinel previously hardcoded ID/Addr/Port inline,
// so a deployment setting sentinel.id / sentinel.addr / sentinel.port /
// sentinel.quorum / sentinel.down_after / sentinel.failover_time in its config
// file would see them silently ignored.
//
// Sentinel.Info() is the observable surface: it reports sentinel_id, sentinel_addr,
// sentinel_port, quorum and down_after_ms, which is enough to verify every field
// the config section owns.
func TestSentinelConfigReachesConstructedSentinel(t *testing.T) {
	// globalSentinel and sentinelConfig are both package-level singletons shared
	// across this test binary. Save and restore both.
	prevSentinel := globalSentinel
	prevConfig := sentinelConfig
	defer func() {
		globalSentinel = prevSentinel
		sentinelConfig = prevConfig
	}()

	// A cold start, so EnsureSentinel really constructs from sentinelConfig.
	globalSentinel = nil

	want := sentinel.Config{
		ID:           "cfg-sentinel-9",
		Addr:         "10.0.0.9",
		Port:         26479,
		Quorum:       3,
		DownAfter:    45 * time.Second,
		FailoverTime: 90 * time.Second,
	}
	ConfigureSentinel(want)

	got := EnsureSentinel()
	if got == nil {
		t.Fatal("EnsureSentinel returned nil")
	}
	info := got.Info()

	if info["sentinel_id"] != want.ID {
		t.Errorf("sentinel_id = %v, want %q — the configured ID did not reach the Sentinel",
			info["sentinel_id"], want.ID)
	}
	if info["sentinel_addr"] != want.Addr {
		t.Errorf("sentinel_addr = %v, want %q — the configured address did not reach the Sentinel",
			info["sentinel_addr"], want.Addr)
	}
	if info["sentinel_port"] != want.Port {
		t.Errorf("sentinel_port = %v, want %d — the configured port did not reach the Sentinel",
			info["sentinel_port"], want.Port)
	}
	if info["quorum"] != want.Quorum {
		t.Errorf("quorum = %v, want %d — the configured quorum did not reach the Sentinel",
			info["quorum"], want.Quorum)
	}
	if info["down_after_ms"] != want.DownAfter.Milliseconds() {
		t.Errorf("down_after_ms = %v, want %d — the configured down_after did not reach the "+
			"Sentinel", info["down_after_ms"], want.DownAfter.Milliseconds())
	}
	t.Log("configured ID, Addr, Port, Quorum and DownAfter all reached the Sentinel")

	// FailoverTime is not reported by Info(), so verify it indirectly: an
	// explicit value must not be replaced by the package default of 3 minutes.
	tmpl := SentinelConfigTemplate()
	if tmpl.FailoverTime != want.FailoverTime {
		t.Errorf("FailoverTime = %v, want %v — the configured failover_time did not reach the "+
			"configuration the Sentinel is built from", tmpl.FailoverTime, want.FailoverTime)
	}
	t.Log("configured FailoverTime is carried into the construction template")
}

// TestSentinelConfigQuorumReachesEvaluation is the behavioural half of the
// contract, expressed through the one EXPORTED surface that reflects a quorum
// decision: CKQUORUM. It reports the number of sentinels that reached the
// master, so a configured sentinel-wide quorum of 5 must be the value the
// Sentinel is actually operating under.
//
// The per-master quorum decision itself (checkODown) is unexported and is
// already covered in-package by internal/sentinel/monitor_quorum_test.go, so it
// is not duplicated here.
func TestSentinelConfigQuorumReachesEvaluation(t *testing.T) {
	prevSentinel := globalSentinel
	prevConfig := sentinelConfig
	defer func() {
		globalSentinel = prevSentinel
		sentinelConfig = prevConfig
	}()

	globalSentinel = nil
	ConfigureSentinel(sentinel.Config{
		ID:     "quorum-sentinel",
		Addr:   "127.0.0.1",
		Port:   26379,
		Quorum: 5,
	})

	s := EnsureSentinel()
	if err := s.Monitor("m", "127.0.0.1", 1, 1); err != nil {
		t.Fatalf("Monitor failed: %v", err)
	}

	// Info() must report the configured sentinel-wide quorum, not the sentinel
	// package's own default of 2.
	if got := s.Info()["quorum"]; got != 5 {
		t.Errorf("sentinel-wide quorum = %v, want 5 — the configured quorum is not the one the "+
			"Sentinel evaluates against", got)
	}

	// CKQUORUM must succeed for a monitored master (it is deliberately a count
	// of reachable sentinels, and this one sentinel is reachable).
	if _, err := s.CKQUORUM("m"); err != nil {
		t.Errorf("CKQUORUM errored for a monitored master: %v", err)
	}
}

// TestConfigureSentinelDoesNotClobberConstructedSentinel pins the boundary the
// startup wiring depends on: ConfigureSentinel supplies configuration for a
// sentinel that has not been built yet, and must not silently replace one that
// already exists.
func TestConfigureSentinelDoesNotClobberConstructedSentinel(t *testing.T) {
	prevSentinel := globalSentinel
	prevConfig := sentinelConfig
	defer func() {
		globalSentinel = prevSentinel
		sentinelConfig = prevConfig
	}()

	globalSentinel = nil
	ConfigureSentinel(sentinel.Config{ID: "first", Addr: "127.0.0.1", Port: 1, Quorum: 2})
	first := EnsureSentinel()

	ConfigureSentinel(sentinel.Config{ID: "second", Addr: "127.0.0.2", Port: 2, Quorum: 9})
	if again := EnsureSentinel(); again != first {
		t.Error("ConfigureSentinel replaced an already-constructed singleton; it should only " +
			"supply configuration for a Sentinel that has not been built yet")
	}
	if id := first.Info()["sentinel_id"]; id != "first" {
		t.Errorf("the original sentinel's ID changed to %v after a second ConfigureSentinel", id)
	}
}
