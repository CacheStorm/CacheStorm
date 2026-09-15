package command

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

func timeoutAfter(seconds int) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		time.Sleep(time.Duration(seconds) * time.Second)
		close(ch)
	}()
	return ch
}

func newSMOVERouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterSetCommands(router)
	return router
}

// SMOVE must not lose the member when the destination holds the wrong
// type: Redis validates both keys before touching anything, so the member
// stays in the source and the command answers WRONGTYPE.
func TestSMOVEDoesNotLoseMemberOnWrongTypeDestination(t *testing.T) {
	s := store.NewStore()
	r := newSMOVERouter(s)

	runCmd(t, s, r, "SADD", "src", "m")
	runCmd(t, s, r, "SET", "dst", "plain-string")

	reply := runCmd(t, s, r, "SMOVE", "src", "dst", "member-1")
	if !strings.Contains(reply, "WRONGTYPE") {
		t.Fatalf("SMOVE to a wrong-type destination must answer WRONGTYPE, got %q", reply)
	}
	if member := runCmd(t, s, r, "SISMEMBER", "src", "m"); !strings.Contains(member, ":1") {
		t.Fatalf("member must stay in the source on WRONGTYPE, got SISMEMBER=%q", member)
	}
}

// While SMOVE runs, every observer must see the member in exactly one of
// the two sets — never in neither. The current sequential lock/unlock
// leaves a window where the member has left the source but has not yet
// arrived at the destination.
func TestSMOVEAtomicUnderConcurrentObservation(t *testing.T) {
	s := store.NewStore()
	r := newSMOVERouter(s)

	runCmd(t, s, r, "SADD", "a", "member-1")

	const moves = 300
	done := make(chan struct{})
	var violations int

	go func() {
		defer close(done)
		for i := 0; i < moves; i++ {
			dst := "b"
			if i%2 == 1 {
				dst = "a"
			}
			src := "a"
			if i%2 == 1 {
				src = "b"
			}
			runCmd(t, s, r, "SMOVE", src, dst, "member-1")
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
			union := runCmd(t, s, r, "SUNION", "a", "b")
			if !strings.Contains(union, "member-1") {
				violations++
			}
		}
	}()

	<-done
	<-observerDone // the observer has drained: no further violations writes
	if violations > 0 {
		t.Fatalf("member absent from the SUNION snapshot %d times during SMOVE", violations)
	}
	if member := runCmd(t, s, r, "SISMEMBER", "a", "member-1"); strings.Contains(member, ":1") {
		return // ended in a
	}
	if member := runCmd(t, s, r, "SISMEMBER", "b", "member-1"); !strings.Contains(member, ":1") {
		t.Fatalf("after the moves stopped the member must exist in exactly one set")
	}
}

// SMOVE where source and destination are the same key must be a no-op
// reporting membership, never a self-deadlock.
func TestSMOVESameKeyIsNoOp(t *testing.T) {
	s := store.NewStore()
	r := newSMOVERouter(s)

	runCmd(t, s, r, "SADD", "k", "m")
	if reply := runCmd(t, s, r, "SMOVE", "k", "k", "m"); !strings.Contains(reply, ":1") {
		t.Fatalf("SMOVE to the same key with a present member must answer 1, got %q", reply)
	}
	if reply := runCmd(t, s, r, "SMOVE", "k", "k", "absent"); !strings.Contains(reply, ":0") {
		t.Fatalf("SMOVE to the same key with an absent member must answer 0, got %q", reply)
	}
	if member := runCmd(t, s, r, "SISMEMBER", "k", "m"); !strings.Contains(member, ":1") {
		t.Fatalf("member must survive a same-key SMOVE, got %q", member)
	}
}

// Concurrent opposite-direction moves must complete: the store-level move
// acquires both set locks in deterministic key order, so a→b and b→a can
// never deadlock.
func TestSMOVEOppositeDirectionDeadlockFree(t *testing.T) {
	s := store.NewStore()
	r := newSMOVERouter(s)

	runCmd(t, s, r, "SADD", "x", "m")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			runCmd(t, s, r, "SMOVE", "x", "y", "m")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			runCmd(t, s, r, "SMOVE", "y", "x", "m")
		}
	}()

	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-timeoutAfter(10):
		t.Fatal("opposite-direction SMOVEs deadlocked")
	}
}
