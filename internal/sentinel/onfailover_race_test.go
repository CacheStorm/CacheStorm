package sentinel

import (
	"sync"
	"testing"
	"time"
)

// Regression: Sentinel.onFailover is written without the mutex and read without
// the mutex, so registering a failover callback concurrently with a completing
// failover is a data race on a two-word func value.
//
//	writer — OnFailover, NO lock:
//	  func (s *Sentinel) OnFailover(fn func(string, string, int)) {
//	  	s.onFailover = fn
//
//	reader — startFailover, NO lock (s.mu released two lines earlier):
//	  s.mu.Unlock()
//	  ...
//	  if s.onFailover != nil {
//	  	s.onFailover(name, bestReplica.Addr, bestReplica.Port)
//	  }
//
// Every other field on Sentinel is either mutex-guarded (masters, sentinels) or
// atomic (running); onFailover is neither. The reader is genuinely concurrent:
// startFailover is spawned as a goroutine from two production paths —
// checkODown's `go func(){ s.startFailover(...) }(name, master)` and Failover's
// `go s.startFailover(name, m)`.
//
// This test is IN-PACKAGE deliberately. GetMaster() returns
// snapshotMasterInfoX(m) — a COPY — so the external API cannot attach the
// replica that startFailover requires before its `bestReplica == nil` early
// return; without it the race test silently exercises nothing.

// armReplica gives a monitored master one replica so startFailover reaches the
// callback invocation instead of returning early.
func armReplica(t *testing.T, s *Sentinel, name string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.masters[name]
	if !ok {
		t.Fatalf("master %q is not monitored", name)
	}
	m.Replicas = append(m.Replicas, &ReplicaInfo{Addr: "127.0.0.2", Port: 6380, Offset: 1})
}

func newFailoverRaceSentinel() *Sentinel {
	return New(Config{
		ID: "s1", Addr: "127.0.0.1", Port: 26379,
		FailoverTime: time.Millisecond, Quorum: 1,
	})
}

// TestOnFailoverRegistrationRacesFailoverCallback is the primary case: the
// exported registration API (writer) concurrent with Failover -> startFailover
// (reader). Only meaningful under -race.
//
// The overlap here is TIME-based on purpose. An earlier version drove both sides
// with fixed iteration counts and reported zero races while the control passed —
// i.e. the read really was reached, but never concurrently with the write:
// Failover() returns in microseconds (most calls short-circuit on
// "failover already in progress"), while each startFailover sleeps FailoverTime
// before it reads. The writer therefore finished long before the first reader
// touched the field. A wall-clock window guarantees genuine overlap.
func TestOnFailoverRegistrationRacesFailoverCallback(t *testing.T) {
	s := newFailoverRaceSentinel()
	if err := s.Monitor("mymaster", "127.0.0.1", 6379, 1); err != nil {
		t.Fatalf("Monitor: %v", err)
	}
	armReplica(t, s, "mymaster")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // writer: the exported OnFailover API, hammering for the whole window
		defer wg.Done()
		deadline := time.Now().Add(300 * time.Millisecond)
		for time.Now().Before(deadline) {
			s.OnFailover(func(string, string, int) {})
		}
	}()

	// Reader: repeatedly trigger failovers so startFailover goroutines execute
	// their read inside the writer's window. Failover clears FailoverState when
	// it completes, so pacing these slower than FailoverTime lets each one run.
	for i := 0; i < 50; i++ {
		_ = s.Failover("mymaster")
		time.Sleep(2 * time.Millisecond)
	}
	wg.Wait()
}

// TestOnFailoverCallbackStillFires is the control: a registered callback must
// still be invoked when a failover completes, with the promoted replica's
// address. It passes before and after the fix, and it is what proves the race
// test above actually reaches the unguarded read rather than exiting early.
func TestOnFailoverCallbackStillFires(t *testing.T) {
	s := newFailoverRaceSentinel()
	if err := s.Monitor("mymaster", "127.0.0.1", 6379, 1); err != nil {
		t.Fatalf("Monitor: %v", err)
	}
	armReplica(t, s, "mymaster")

	fired := make(chan string, 4)
	s.OnFailover(func(master, newAddr string, newPort int) { fired <- newAddr })
	if err := s.Failover("mymaster"); err != nil {
		t.Fatalf("Failover: %v", err)
	}

	select {
	case addr := <-fired:
		if addr != "127.0.0.2" {
			t.Fatalf("callback got addr %q, want the promoted replica 127.0.0.2", addr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("CONTROL FAIL: the registered failover callback never fired")
	}
	t.Log("CONTROL PASS: a registered callback is invoked after failover")
}
