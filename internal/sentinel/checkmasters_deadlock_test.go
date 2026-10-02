package sentinel

import (
	"runtime"
	"testing"
	"time"
)

// TestCheckMastersDoesNotSelfDeadlock is the round proof.
//
// CONTRACT: Sentinel.checkMasters must return. It is driven from monitorLoop on
// a ticker, and an operator's only way to observe the sentinel is the SENTINEL
// command surface (and this package's Serve loop). A function that can never
// return is not a subtle miscalculation: it stops the master monitor
// permanently, so master state silently freezes while the process still answers
// queries as if it were healthy.
//
// CONTRACT-INDEPENDENT ANCHOR: "checkMasters returns within a bounded time" is a
// property of this package alone. No reference Redis is needed, and no contract
// about what a correct quorum verdict is.
//
// DEFECT: checkMasters opens with
//
//     s.mu.Lock()
//     defer s.mu.Unlock()
//
// and then, inside the unreachable branch, calls s.checkODown(master) — which
// begins with
//
//     s.mu.RLock()
//     peers := s.sentinels[master.Name]
//     s.mu.RUnlock()
//
// sync.RWMutex is NOT reentrant, and Go's RWMutex additionally forbids a new
// RLock while a writer holds the lock. checkMasters already holds the exclusive
// lock for the whole function, so checkODown's very first statement blocks
// forever. Nothing can release it: the deferred Unlock in checkMasters is
// downstream of the call that is stuck. The monitor goroutine wedges on its
// first unreachable master, permanently.
//
// The path is reachable: any master that is unreachable AND whose state is OK
// or None sets SDown and then calls checkODown — i.e. the first master that
// goes down is enough.
//
// The test runs checkMasters on its own goroutine behind a watchdog so a hang is
// reported as a named failure WITH the blocked goroutine's stack (which is the
// evidence that it is parked in RLock under a held writer), rather than
// silently burning the test runner's own timeout.
func TestCheckMastersDoesNotSelfDeadlock(t *testing.T) {
	// ---- CONTROL 1: an UNREACHABLE master is actually detected as unreachable
	// by isReachable, so the deadlock path is genuinely entered. A closed port
	// on loopback refuses the dial immediately.
	s := New(Config{ID: "s1", Quorum: 2, DownAfter: time.Second})
	if err := s.Monitor("downmaster", "127.0.0.1", 1, 2); err != nil {
		t.Fatalf("CONTROL broken harness: Monitor failed: %v", err)
	}
	if s.isReachable("127.0.0.1", 1) {
		t.Fatalf("CONTROL broken harness: 127.0.0.1:1 was reachable; the deadlock path would " +
			"never be entered and the test could not observe the defect")
	}
	m, ok := s.GetMaster("downmaster")
	if !ok {
		t.Fatalf("CONTROL broken harness: master not found after Monitor")
	}
	if m.State != MasterStateNone {
		t.Fatalf("CONTROL broken harness: fresh master state = %v, want None (the state the "+
			"unreachable branch requires before it calls checkODown)", m.State)
	}
	t.Log("CONTROL 1 ok: the master is unreachable and in state None — checkODown will be called")

	// ---- THE DEFECT: checkMasters must return; today it cannot.
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.checkMasters()
	}()

	select {
	case <-done:
		t.Log("PASS: checkMasters returned instead of parking on the nested RLock")
	case <-time.After(10 * time.Second):
		// Capture the stack of the parked goroutine as evidence.
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		stack := string(buf[:n])

		t.Fatalf("FAIL: checkMasters deadlocked — it did not return within 10s.\n"+
			"checkMasters takes s.mu.Lock() with `defer s.mu.Unlock()` for the whole function, "+
			"then calls s.checkODown(master) inside the unreachable branch. checkODown begins "+
			"with s.mu.RLock(). sync.RWMutex is not reentrant and forbids a new RLock while a "+
			"writer holds the lock, so checkODown parks forever on its first statement and the "+
			"deferred Unlock in checkMasters — the only thing that could release it — is "+
			"downstream of the stuck call. Any master going down wedges the monitor for good.\n"+
			"--- goroutine stack evidence ---\n%s\n"+
			"--- expected in the stack: a goroutine blocked at checkODown's s.mu.RLock(), "+
			"called from checkMasters which already holds s.mu.Lock() ---",
			stack)
	}
}

// TestCheckODownReturnsWhenLockNotHeld is the control: it calls checkODown
// DIRECTLY, without a write lock already held. It passes both before and after
// the fix, so it can never be the assertion that goes red — it only proves the
// harness distinguishes "checkODown called under a held lock" (which parks) from
// "checkODown called normally" (which returns).
func TestCheckODownReturnsWhenLockNotHeld(t *testing.T) {
	s := New(Config{ID: "s1", Quorum: 2, DownAfter: time.Second})
	if err := s.Monitor("m", "127.0.0.1", 1, 1); err != nil {
		t.Fatalf("harness: Monitor failed: %v", err)
	}
	m, _ := s.GetMaster("m")

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.checkODown(m)
	}()
	select {
	case <-done:
		t.Log("CONTROL ok: checkODown returns promptly when the lock is not already held")
	case <-time.After(5 * time.Second):
		t.Fatalf("harness is broken in a different way: checkODown hung even WITHOUT a held lock")
	}
}