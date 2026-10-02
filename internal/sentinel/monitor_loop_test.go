package sentinel

import (
	"net"
	"testing"
	"time"
)

// deadPort binds then immediately releases a loopback port, so a dial to it is
// refused immediately. A master on such a port is unreachable, which is what
// drives the subjectively-down transition.
func deadPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve a port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// waitForState polls until the master reaches want, or fails the test.
func waitForState(t *testing.T, s *Sentinel, name string, want MasterState, limit time.Duration) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if m, ok := s.GetMaster(name); ok && m.State == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	m, _ := s.GetMaster(name)
	got := -1
	if m != nil {
		got = int(m.State)
	}
	t.Fatalf("master %q did not reach %v within %v (last state %v)", name, want, limit, got)
}

// TestMonitorLoopMarksUnreachableMasterSdown proves that STARTING the Sentinel
// actually runs the monitor loop, by observing the state transition the loop
// causes. Before StartSentinel was wired into Server.Start, monitorLoop never
// ran in production and no master could ever leave its initial state.
//
// The transition is driven by isReachable: checkMasters dials the master on
// every tick and marks it subjectively down as soon as the dial fails. Note that
// down_after governs PEER liveness (how long a fellow sentinel may go unseen
// before it stops counting toward quorum) — it is deliberately short here so a
// real quorum could be reached, and is NOT what gates this master's own sdown.
func TestMonitorLoopMarksUnreachableMasterSdown(t *testing.T) {
	s := New(Config{
		ID:           "loop-test",
		Addr:         "127.0.0.1",
		Port:         26379,
		Quorum:       2,               // unmet by one sentinel: stays SDown, not ODown
		DownAfter:    50 * time.Millisecond,
		FailoverTime: 50 * time.Millisecond,
	})

	port := deadPort(t)
	if s.isReachable("127.0.0.1", port) {
		t.Fatalf("harness broken: 127.0.0.1:%d accepted a dial, so the unreachable branch "+
			"would never run", port)
	}
	if err := s.Monitor("watched", "127.0.0.1", port, 2); err != nil {
		t.Fatalf("Monitor failed: %v", err)
	}

	// Guard: before Start the master is untouched. If this ever fails, the loop
	// is running without Start and the test would prove nothing.
	if m, _ := s.GetMaster("watched"); m.State != MasterStateNone {
		t.Fatalf("harness broken: master state is %v before Start, want None", m.State)
	}

	if err := s.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop() // joins monitorLoop and gossipLoop via wg.Wait

	// The monitor loop ticks every second, so allow comfortably more than one
	// tick while staying well inside a normal test timeout.
	waitForState(t, s, "watched", MasterStateSDown, 5*time.Second)

	m, _ := s.GetMaster("watched")
	if m.State != MasterStateSDown {
		t.Errorf("state = %v, want SDown", m.State)
	}
	if len(m.Flags) != 2 || m.Flags[0] != "master" || m.Flags[1] != "s_down" {
		t.Errorf("flags = %v, want [master s_down]", m.Flags)
	}
	// Quorum 2 with a single sentinel is unmet, so it must NOT have escalated.
	if m.State == MasterStateODown {
		t.Error("state escalated to ODown, but quorum 2 is unmet by one sentinel")
	}
	t.Log("the running monitor loop marked the unreachable master SDown")
}

// TestMonitorLoopMarksMasterBackUpWhenReachable is the converse: once a master
// becomes reachable again the same loop must return it to OK. Without it a
// sentinel that had flagged a master down could never recover it.
func TestMonitorLoopMarksMasterBackUpWhenReachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port

	s := New(Config{ID: "recover-test", Quorum: 2, DownAfter: 50 * time.Millisecond})
	if err := s.Monitor("flapping", "127.0.0.1", port, 2); err != nil {
		t.Fatalf("Monitor failed: %v", err)
	}
	if err := s.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop()

	waitForState(t, s, "flapping", MasterStateOK, 5*time.Second)

	m, _ := s.GetMaster("flapping")
	if m.LastOkPing.IsZero() {
		t.Error("expected LastOkPing to be recorded for a reachable master")
	}
	if len(m.Flags) != 1 || m.Flags[0] != "master" {
		t.Errorf("flags = %v, want [master] once the master is reachable", m.Flags)
	}
	t.Log("the running monitor loop returned the reachable master to OK")
}