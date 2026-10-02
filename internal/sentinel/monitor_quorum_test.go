package sentinel

import (
	"testing"
	"time"
)

// TestProofMonitorQuorumIsEnforced is the round proof.
//
// CONTRACT: Monitor(name, addr, port, quorum) accepts a PER-MASTER quorum. That
// value is stored in MasterInfo.Quorum, so a caller setting quorum=1 for one
// master is asserting "this master needs only one sentinel to be considered
// objective-down", while a different master in the same Sentinel may need five.
// The parameter exists to be used; storing it and never reading it means the
// caller's instruction is silently discarded.
//
// CONTRACT-INDEPENDENT ANCHOR: this needs no reference Redis. "An argument that
// is accepted and stored but never read cannot influence behaviour" is a
// property of this package alone — verified by a repo-wide grep for `.Quorum`
// whose only hits are cfg.Quorum at sentinel.go:97/98/111, never
// MasterInfo.Quorum.
//
// DEFECT: Sentinel.checkODown (line 226) evaluates
//
//     return downCount+1 >= s.quorum
//
// using the Sentinel-WIDE quorum (Config.Quorum, default 2) rather than the
// master-specific one. So every master in the process is governed by one global
// number, and Monitor's quorum argument is dead configuration.
//
// Test shape: a Sentinel with a HIGH global quorum (5) monitoring a master whose
// OWN quorum is 1. With no peers recorded, downCount is 0, so downCount+1 == 1.
// Against the master's own quorum of 1 that satisfies the check (1 >= 1) and the
// master IS objective-down; against the global 5 it does not (1 >= 5). The
// current code takes the global path and reports NOT down, silently overruling
// the caller.
func TestProofMonitorQuorumIsEnforced(t *testing.T) {
	// Global quorum deliberately HIGH, per-master quorum deliberately LOW.
	s := New(Config{Quorum: 5, DownAfter: time.Second})

	if err := s.Monitor("m-low-quorum", "127.0.0.1", 6379, 1); err != nil {
		t.Fatalf("harness: Monitor failed: %v", err)
	}
	if err := s.Monitor("m-high-quorum", "127.0.0.1", 6380, 4); err != nil {
		t.Fatalf("harness: Monitor failed: %v", err)
	}
	high, ok := s.GetMaster("m-high-quorum")
	if !ok {
		t.Fatalf("harness: m-high-quorum not found after Monitor")
	}

	// ---- CONTROL 1: Monitor really does store the per-master value.
	low, ok := s.GetMaster("m-low-quorum")
	if !ok {
		t.Fatalf("CONTROL broken harness: master not found after Monitor")
	}
	if low.Quorum != 1 {
		t.Fatalf("CONTROL broken harness: stored Quorum = %d, want 1", low.Quorum)
	}
	t.Log("CONTROL 1 ok: Monitor stores the per-master quorum (m-low-quorum = 1)")

	// ---- CONTROL 2: a master whose quorum is NOT met must not be down.
	// With zero peers, downCount+1 == 1. A quorum of 4 is not met, so this must
	// be false regardless of which quorum value the code consults. This proves
	// the harness can distinguish true from false.
	if s.checkODown(high) {
		t.Fatalf("CONTROL 2 broken harness: m-high-quorum (quorum 4, 1 sentinel) was reported "+
			"objectively down; the harness cannot tell satisfied from unsatisfied quorums")
	}
	t.Log("CONTROL 2 ok: an unmet quorum is correctly reported as not-down")

	// ---- THE DEFECT: the master whose OWN quorum (1) IS met must be down.
	// The code compares 1 >= s.quorum (the global 5) and reports false,
	// overruling the caller's explicit quorum=1.
	if !s.checkODown(low) {
		t.Fatalf("FAIL: master \"m-low-quorum\" was configured with quorum 1 and is served by "+
			"exactly one sentinel, so 1 >= 1 satisfies its quorum — but checkODown reported it "+
			"as NOT objectively down.\n"+
			"checkODown evaluates `downCount+1 >= s.quorum` with the Sentinel-WIDE quorum from "+
			"Config (5 here) instead of the master-specific MasterInfo.Quorum that Monitor "+
			"stored. A repo-wide grep for `.Quorum` returns only cfg.Quorum "+
			"(sentinel.go:97/98/111) and never MasterInfo.Quorum, so the per-master argument is "+
			"dead configuration that can never influence behaviour.")
	}
	t.Log("PASS: the master's own quorum is honoured")
}