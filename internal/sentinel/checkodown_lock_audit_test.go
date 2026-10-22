package sentinel

import (
	"sync"
	"testing"
	"time"
)

// Audit of checkODown / checkODownLocked / checkMasters / startFailover for
// unsynchronised reads of shared Sentinel state.
//
// RESULT of the lock census:
//
//	checkMasters       holds s.mu.Lock() for its WHOLE body. Every MasterInfo
//	                   mutation (State, LastOkPing, Flags) is under that lock, and
//	                   it deliberately calls checkODownLocked -- the
//	                   caller-holds-the-lock variant -- to avoid a nested RLock
//	                   deadlock. It spawns `go s.startFailover(name, master)`;
//	                   startFailover immediately takes s.mu.Lock(), so it blocks
//	                   until checkMasters returns. Correct.
//
//	startFailover      every MasterInfo access is guarded: FailoverState read
//	                   and write under Lock, the Replicas scan under RLock, and
//	                   the Addr/Port/State/Flags/Epoch commit under Lock.
//
//	checkODownLocked   reads peer.LastSeen and master.Quorum with NO lock. Its
//	                   sole production caller (checkMasters) happens to hold the
//	                   write lock, so it is safe BY CONVENTION, not by
//	                   construction.
//
//	checkODown         the unlocked wrapper: it takes RLock only to fetch
//	                   peers, RELEASES it, then calls checkODownLocked, which
//	                   then reads peer.LastSeen and master.Quorum with no lock
//	                   at all. Dead in production today (tests only), but it is
//	                   the one path where those reads are genuinely unguarded.
//
// checkODown is the defect this file pins. recordPeer writes p.LastSeen under
// the write lock (sentinel.go:366) from the gossip goroutine; checkODown reads
// the very same field with no lock held.

// newQuorumRaceSentinel builds a sentinel with one monitored master.
func newQuorumRaceSentinel(t *testing.T) (*Sentinel, *MasterInfo) {
	t.Helper()
	s := New(Config{
		ID: "s1", Addr: "127.0.0.1", Port: 26379,
		DownAfter: time.Minute, Quorum: 1,
	})
	if err := s.Monitor("mymaster", "127.0.0.1", 6379, 1); err != nil {
		t.Fatalf("Monitor: %v", err)
	}
	// recordPeer registers a peer against EVERY monitored master, so this
	// populates s.sentinels["mymaster"] -- without it checkODownLocked
	// iterates an empty slice and reads nothing.
	s.recordPeer("10.0.0.2", 26380)

	s.mu.RLock()
	master := s.masters["mymaster"]
	s.mu.RUnlock()
	return s, master
}

// TestCheckODownUnguardedPeerReadRaces is the primary case: the unlocked
// checkODown wrapper's peer.LastSeen read racing recordPeer's locked write.
// Only meaningful under -race.
func TestCheckODownUnguardedPeerReadRaces(t *testing.T) {
	s, master := newQuorumRaceSentinel(t)

	var wg sync.WaitGroup
	wg.Add(2)
	// Reader: checkODown reads peer.LastSeen with no lock held.
	go func() {
		defer wg.Done()
		deadline := time.Now().Add(300 * time.Millisecond)
		for time.Now().Before(deadline) {
			_ = s.checkODown(master)
		}
	}()
	// Writer: the gossip path writes p.LastSeen under the write lock.
	go func() {
		defer wg.Done()
		deadline := time.Now().Add(300 * time.Millisecond)
		for time.Now().Before(deadline) {
			s.recordPeer("10.0.0.2", 26380)
		}
	}()
	wg.Wait()
}

// TestCheckODownQuorumControls is the control: both call forms still compute the
// quorum verdict correctly, so the race test above is not passing because the
// path does nothing. Passes before and after the fix.
func TestCheckODownQuorumControls(t *testing.T) {
	s, master := newQuorumRaceSentinel(t)

	// The one registered peer is fresh (LastSeen=now, DownAfter=1m), so it counts
	// as alive; self is added by the "+1", so downCount+1 == 2 >= quorum 1.
	if !s.checkODown(master) {
		t.Fatal("CONTROL FAIL: checkODown = false; a live peer plus self should meet quorum 1")
	}

	// The caller-holds-the-lock variant (how checkMasters calls it) must agree.
	s.mu.RLock()
	got := s.checkODownLocked(master, s.sentinels["mymaster"])
	s.mu.RUnlock()
	if !got {
		t.Fatal("CONTROL FAIL: checkODownLocked = false; both forms must agree")
	}

	// A master with no registered peers: only self counts, quorum 2 cannot be met.
	s2 := New(Config{ID: "s2", Addr: "127.0.0.3", Port: 26379, DownAfter: time.Minute, Quorum: 2})
	if err := s2.Monitor("solo", "127.0.0.4", 6379, 2); err != nil {
		t.Fatalf("Monitor: %v", err)
	}
	s2.mu.RLock()
	solo := s2.masters["solo"]
	s2.mu.RUnlock()
	if s2.checkODown(solo) {
		t.Fatal("CONTROL FAIL: a lone sentinel must not meet quorum 2")
	}

	t.Log("CONTROL PASS: both checkODown forms compute the quorum verdict correctly")
}
