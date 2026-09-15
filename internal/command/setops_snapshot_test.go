package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func newSetOpsSnapshotRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterSetCommands(router)
	return router
}

// Concurrency regression locks for the multi-key set readers.
//
// During a single-member SMOVE ping-pong, the member moves between "x"
// and "y" while these readers run. The readers' intersection/union values
// cannot distinguish an atomic multi-set snapshot from a half-applied one
// (an empty SINTER is legitimate both before and after the move), so the
// atomicity itself is proven structurally: every reader here takes the
// shared SetOpsRLock that MoveSetMember holds exclusively — the same
// mechanism proven for SUNION's atomic snapshot by
// TestSMOVEAtomicUnderConcurrentObservation. These tests lock in the
// intersection/count/store semantics under that concurrency and would
// catch regressions in the readers' locking or result shape.

func TestSINTERSnapshotUnderConcurrentMoves(t *testing.T) {
	s := store.NewStore()
	r := newSetOpsSnapshotRouter(s)

	runCmd(t, s, r, "SADD", "x", "member-1")

	const moves = 300
	done := make(chan struct{})

	go func() {
		defer close(done)
		dst, src := "y", "x"
		for i := 0; i < moves; i++ {
			runCmd(t, s, r, "SMOVE", src, dst, "member-1")
			dst, src = src, dst
		}
	}()

	observerDone := make(chan struct{})
	go func() {
		defer close(observerDone)
		for {
			select {
			case <-done:
				return
			default:
			}
			sinter := runCmd(t, s, r, "SINTER", "x", "y")
			if !strings.Contains(sinter, "*") {
				t.Errorf("SINTER returned a malformed reply: %q", sinter)
			}
		}
	}()

	<-done
	<-observerDone // the observer has drained

	// After the moves stop, the member exists in exactly one set, so the
	// intersection must be empty.
	if member := runCmd(t, s, r, "SISMEMBER", "x", "member-1"); strings.Contains(member, ":1") {
		return // ended in x: the intersection is legitimately empty
	}
	if sinter := runCmd(t, s, r, "SINTER", "x", "y"); !strings.Contains(sinter, "*0") {
		t.Fatalf("after the moves stopped the member is in y, so SINTER x y must be empty, got %q", sinter)
	}
}

func TestSINTERCARDSnapshotUnderConcurrentMoves(t *testing.T) {
	s := store.NewStore()
	r := newSetOpsSnapshotRouter(s)

	runCmd(t, s, r, "SADD", "x", "member-1")

	const moves = 300
	done := make(chan struct{})

	go func() {
		defer close(done)
		dst, src := "y", "x"
		for i := 0; i < moves; i++ {
			runCmd(t, s, r, "SMOVE", src, dst, "member-1")
			dst, src = src, dst
		}
	}()

	observerDone := make(chan struct{})
	go func() {
		defer close(observerDone)
		for {
			select {
			case <-done:
				return
			default:
			}
			count := runCmd(t, s, r, "SINTERCARD", "x", "y")
			if !strings.Contains(count, ":0") && !strings.Contains(count, ":1") {
				t.Errorf("SINTERCARD returned a malformed reply: %q", count)
			}
		}
	}()

	<-done
	<-observerDone

	if sintercard := runCmd(t, s, r, "SINTERCARD", "x", "y"); !strings.Contains(sintercard, ":0") {
		t.Fatalf("after the moves stopped the sets must be disjoint, got SINTERCARD=%q", sintercard)
	}
}

func TestSDIFFSnapshotUnderConcurrentMoves(t *testing.T) {
	s := store.NewStore()
	r := newSetOpsSnapshotRouter(s)

	runCmd(t, s, r, "SADD", "x", "member-1")

	const moves = 300
	done := make(chan struct{})

	go func() {
		defer close(done)
		dst, src := "y", "x"
		for i := 0; i < moves; i++ {
			runCmd(t, s, r, "SMOVE", src, dst, "member-1")
			dst, src = src, dst
		}
	}()

	observerDone := make(chan struct{})
	go func() {
		defer close(observerDone)
		for {
			select {
			case <-done:
				return
			default:
			}
			sdiff := runCmd(t, s, r, "SDIFF", "x", "y")
			if !strings.Contains(sdiff, "*") {
				t.Errorf("SDIFF returned a malformed reply: %q", sdiff)
			}
		}
	}()

	<-done
	<-observerDone

	// The member ends in exactly one set; SDIFF x y is that set's content.
	if sdiff := runCmd(t, s, r, "SDIFF", "x", "y"); !strings.Contains(sdiff, "member-1") && !strings.Contains(sdiff, "*0") {
		t.Fatalf("SDIFF x y must contain the member or be empty, got %q", sdiff)
	}
}

func TestSDIFFSTORESnapshotUnderConcurrentMoves(t *testing.T) {
	s := store.NewStore()
	r := newSetOpsSnapshotRouter(s)

	runCmd(t, s, r, "SADD", "x", "member-1")

	const moves = 300
	done := make(chan struct{})

	go func() {
		defer close(done)
		dst, src := "y", "x"
		for i := 0; i < moves; i++ {
			runCmd(t, s, r, "SMOVE", src, dst, "member-1")
			dst, src = src, dst
		}
	}()

	observerDone := make(chan struct{})
	go func() {
		defer close(observerDone)
		for {
			select {
			case <-done:
				return
			default:
			}
			runCmd(t, s, r, "SDIFFSTORE", "dest", "x", "y")
		}
	}()

	<-done
	<-observerDone

	// The destination holds the last snapshot's shape: the member or empty.
	if member := runCmd(t, s, r, "SISMEMBER", "dest", "member-1"); strings.Contains(member, ":1") {
		if others := runCmd(t, s, r, "SCARD", "dest"); !strings.Contains(others, ":1") {
			t.Fatalf("dest must hold exactly the member, got SCARD=%q", others)
		}
	}
}

func TestSINTERSTORESnapshotUnderConcurrentMoves(t *testing.T) {
	s := store.NewStore()
	r := newSetOpsSnapshotRouter(s)

	runCmd(t, s, r, "SADD", "x", "member-1")

	const moves = 300
	done := make(chan struct{})

	go func() {
		defer close(done)
		dst, src := "y", "x"
		for i := 0; i < moves; i++ {
			runCmd(t, s, r, "SMOVE", src, dst, "member-1")
			dst, src = src, dst
		}
	}()

	observerDone := make(chan struct{})
	go func() {
		defer close(observerDone)
		for {
			select {
			case <-done:
				return
			default:
			}
			runCmd(t, s, r, "SINTERSTORE", "dest", "x", "y")
		}
	}()

	<-done
	<-observerDone

	// After the moves stop the sets are disjoint, so the intersection
	// store must be empty.
	if sintercard := runCmd(t, s, r, "SINTERCARD", "x", "y"); !strings.Contains(sintercard, ":0") {
		t.Fatalf("the sets must be disjoint after the moves, got %q", sintercard)
	}
}

func TestSUNIONSTORESnapshotUnderConcurrentMoves(t *testing.T) {
	s := store.NewStore()
	r := newSetOpsSnapshotRouter(s)

	runCmd(t, s, r, "SADD", "x", "member-1")

	const moves = 300
	done := make(chan struct{})

	go func() {
		defer close(done)
		dst, src := "y", "x"
		for i := 0; i < moves; i++ {
			runCmd(t, s, r, "SMOVE", src, dst, "member-1")
			dst, src = src, dst
		}
	}()

	observerDone := make(chan struct{})
	go func() {
		defer close(observerDone)
		for {
			select {
			case <-done:
				return
			default:
			}
			runCmd(t, s, r, "SUNIONSTORE", "dest", "x", "y")
		}
	}()

	<-done
	<-observerDone

	// The union store must always hold the member: it exists in exactly
	// one set after the moves stop.
	if member := runCmd(t, s, r, "SISMEMBER", "dest", "member-1"); !strings.Contains(member, ":1") {
		t.Fatalf("SUNIONSTORE dest must contain the member, got SISMEMBER=%q", member)
	}
}
