package store

import (
	"testing"
	"time"
)

// lockWaiterOutcome is one waiter's outcome from a blocking Lock call.
type lockWaiterOutcome struct {
	holder string
	got    bool
}

// startLockWaiter runs a blocking Lock in the background and reports its
// outcome. It signals `entered` immediately before calling Lock, so the
// caller knows the goroutine is about to contend for the lock.
func startLockWaiter(dl *DistributedLock, key, holder, token string, ttl, timeout time.Duration, entered chan<- struct{}) <-chan lockWaiterOutcome {
	out := make(chan lockWaiterOutcome, 1)
	go func() {
		entered <- struct{}{}
		got := dl.Lock(key, holder, token, ttl, timeout)
		out <- lockWaiterOutcome{holder: holder, got: got}
	}()
	return out
}

// TestDistributedLockHandsOffToASecondWaiter pins the contract that a
// release transfers ownership to a caller that is already waiting.
//
// The bug this pins: Unlock deleted the whole wait queue for the key and
// signalled every waiter, so all of them woke and raced for the released
// lock. Only one could win. The losers fell back to waiting on their
// channel — but Unlock had already removed them from the queue, so no later
// release ever signalled them again. They were orphaned: each blocked until
// its own timeout expired and then reported failure, although the lock had
// been free for that entire time.
//
// A distributed lock whose waiting callers never receive the lock defeats
// its only purpose — mutual exclusion by serialising the holders.
func TestDistributedLockHandsOffToASecondWaiter(t *testing.T) {
	const (
		key     = "handoff"
		ttl     = 10 * time.Second
		timeout = 5 * time.Second
	)

	dl := NewDistributedLock()

	// The holder owns the lock, so both waiters must block on it.
	if !dl.TryLock(key, "holder-0", "token-0", ttl) {
		t.Fatalf("the initial holder could not take the lock")
	}

	entered1 := make(chan struct{}, 1)
	entered2 := make(chan struct{}, 1)
	res1 := startLockWaiter(dl, key, "waiter-1", "token-1", ttl, timeout, entered1)
	res2 := startLockWaiter(dl, key, "waiter-2", "token-2", ttl, timeout, entered2)

	// Both waiters are about to contend; give them a moment to register.
	<-entered1
	<-entered2
	time.Sleep(150 * time.Millisecond)

	// Release the lock: both waiters wake and race for it.
	if !dl.Unlock(key, "holder-0", "token-0") {
		t.Fatalf("the holder could not release the lock")
	}

	// Exactly one waiter wins the release.
	var winner lockWaiterOutcome
	select {
	case winner = <-res1:
	case winner = <-res2:
	case <-time.After(2 * time.Second):
		t.Fatalf("no waiter acquired the lock within 2s of the release")
	}
	if !winner.got {
		t.Fatalf("waiter %s reported failure right after the lock was released", winner.holder)
	}

	// The loser is the other waiter. It must still be blocked, waiting for
	// the lock to become free.
	loserCh := res2
	if winner.holder == "waiter-2" {
		loserCh = res1
	}
	select {
	case r := <-loserCh:
		t.Fatalf("the losing waiter %s returned %v before the lock was free; it must "+
			"stay blocked until it acquires the lock or its timeout expires",
			r.holder, r.got)
	case <-time.After(200 * time.Millisecond):
	}

	// The winner now releases the lock. Ownership must transfer to the
	// waiter that is still blocked: the lock is free, so the blocked waiter
	// must acquire it rather than sit until its timeout.
	winnerToken := "token-" + winner.holder[len("waiter-"):]
	if !dl.Unlock(key, winner.holder, winnerToken) {
		t.Fatalf("the winner could not release the lock")
	}
	if dl.IsLocked(key) {
		t.Fatalf("the lock is still held after the winner released it")
	}

	select {
	case r := <-loserCh:
		if !r.got {
			t.Fatalf("the waiting waiter %s failed to acquire the lock although the lock "+
				"was free and its %s timeout had not expired", r.holder, timeout)
		}
	case <-time.After(3 * time.Second):
		// Collect the waiter's eventual answer so the report names the
		// exact failure mode.
		select {
		case r := <-loserCh:
			t.Fatalf("the waiting waiter %s did not acquire the released lock; it returned %v "+
				"only after its timeout expired, while the lock was free", r.holder, r.got)
		case <-time.After(2 * time.Second):
			t.Fatalf("the waiting waiter never acquired the released lock; it stayed blocked " +
				"although the lock was free")
		}
	}
}

// TestDistributedLockSingleWaiterAcquires is the control: with only ONE
// waiter there is no race, so the waiter must acquire the lock as soon as it
// is released. This path works before and after the fix and proves the
// harness is sound — a failure in the test above is the contended case, not
// the harness.
func TestDistributedLockSingleWaiterAcquires(t *testing.T) {
	const key = "single"

	dl := NewDistributedLock()
	if !dl.TryLock(key, "holder", "token", 10*time.Second) {
		t.Fatalf("the initial holder could not take the lock")
	}

	entered := make(chan struct{}, 1)
	res := startLockWaiter(dl, key, "waiter", "token-w", 10*time.Second, 5*time.Second, entered)
	<-entered
	time.Sleep(150 * time.Millisecond)

	if !dl.Unlock(key, "holder", "token") {
		t.Fatalf("the holder could not release the lock")
	}

	select {
	case r := <-res:
		if !r.got {
			t.Fatalf("the single waiter failed to acquire the released lock")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("the single waiter never acquired the released lock")
	}
}

// TestDistributedLockWaiterTimesOutWhileLockIsHeld covers the other end of
// the contract: a waiter whose timeout expires while the lock is still held
// must report failure, and it must give up its place in the queue — a waiter
// that gave up must not keep the next waiter from acquiring the lock.
func TestDistributedLockWaiterTimesOutWhileLockIsHeld(t *testing.T) {
	const key = "timeout"

	dl := NewDistributedLock()
	if !dl.TryLock(key, "holder", "token", 10*time.Second) {
		t.Fatalf("the initial holder could not take the lock")
	}

	entered := make(chan struct{}, 1)
	res := startLockWaiter(dl, key, "waiter", "token-w", 10*time.Second, 300*time.Millisecond, entered)
	<-entered

	select {
	case r := <-res:
		if r.got {
			t.Fatalf("the waiter acquired the lock although it was held throughout")
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("the waiter neither acquired the lock nor gave up at its timeout")
	}

	// The abandoned waiter must not block the queue: a fresh waiter must
	// still acquire the lock once it is released.
	if !dl.Unlock(key, "holder", "token") {
		t.Fatalf("the holder could not release the lock")
	}

	entered2 := make(chan struct{}, 1)
	res2 := startLockWaiter(dl, key, "waiter-2", "token-2", 10*time.Second, 3*time.Second, entered2)
	<-entered2

	select {
	case r := <-res2:
		if !r.got {
			t.Fatalf("a fresh waiter could not acquire the released lock after an earlier " +
				"waiter had given up")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("a fresh waiter never acquired the released lock after an earlier waiter " +
			"had given up")
	}
}
