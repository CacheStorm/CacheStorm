package store

import (
	"runtime"
	"testing"
	"time"
)

func TestNotifierImmediateWakeRetainsKey(t *testing.T) {
	previous := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(previous)
	for i := 0; i < 10000; i++ {
		kn := NewKeyNotifier()
		done := make(chan bool, 1)
		go func() {
			key, ok := kn.WaitForKeysOpts([]string{"q"}, time.Second, WaitOptions{Consume: true})
			done <- ok && key == "q"
		}()
		deadline := time.Now().Add(2 * time.Second)
		for kn.WaiterCount("q") == 0 {
			if time.Now().After(deadline) {
				t.Fatal("waiter did not register")
			}
			runtime.Gosched()
		}
		kn.NotifyKey("q")
		if !<-done {
			t.Fatalf("notification lost its key at iteration %d", i)
		}
	}
}

func TestNotifierReadyCheckRetainsNotificationOwnership(t *testing.T) {
	kn := NewKeyNotifier()
	notified := false
	key, ok := kn.WaitForKeysOpts([]string{"q"}, 0, WaitOptions{Consume: true, Notified: &notified, Ready: func(key string) bool { kn.NotifyKey(key); return true }})
	if !ok || key != "q" || !notified {
		t.Fatalf("ready check lost notification ownership: key %q ready %v notified %v", key, ok, notified)
	}
	kn.Pass(key, func(string) bool { return false }, notified)
	kn.mu.Lock()
	defer kn.mu.Unlock()
	if len(kn.inflight) != 0 {
		t.Fatal("completed consumer retained its turn")
	}
}

func TestNotifierNewConsumerCannotOvertakeInflightOwner(t *testing.T) {
	kn := NewKeyNotifier()
	done := make(chan bool, 1)
	go func() {
		key, ok := kn.WaitForKeysOpts([]string{"q"}, time.Second, WaitOptions{Consume: true})
		done <- ok && key == "q"
	}()
	deadline := time.Now().Add(2 * time.Second)
	for kn.WaiterCount("q") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("waiter did not register")
		}
		runtime.Gosched()
	}
	kn.NotifyKey("q")
	if !<-done {
		t.Fatal("first consumer was not notified")
	}
	_, ok := kn.WaitForKeysOpts([]string{"q"}, 0, WaitOptions{Consume: true, Ready: func(string) bool { return true }})
	if ok {
		t.Fatal("new consumer overtook the notified consumer before Pass")
	}
	kn.Pass("q", func(string) bool { return false }, true)
}

func TestNotifierRetryReleasesPriorTurn(t *testing.T) {
	kn := NewKeyNotifier()
	var token WakeToken
	done := make(chan bool, 1)
	start := func(front bool) {
		go func() {
			key, ok := kn.WaitForKeysOpts([]string{"q"}, time.Second, WaitOptions{Consume: true, Front: front, Token: &token})
			done <- ok && key == "q"
		}()
		deadline := time.Now().Add(2 * time.Second)
		for kn.WaiterCount("q") == 0 {
			if time.Now().After(deadline) {
				t.Fatal("waiter did not register")
			}
			runtime.Gosched()
		}
		kn.NotifyKey("q")
		if !<-done {
			t.Fatal("consumer did not receive its turn")
		}
	}
	start(false)
	start(true)
	stale := token
	newOwner := make(chan struct{})
	kn.mu.Lock()
	kn.inflight["q"] = inflightToken{ch: newOwner, since: time.Now()}
	kn.mu.Unlock()
	kn.PassToken("q", func(string) bool { return false }, stale)
	kn.mu.Lock()
	defer kn.mu.Unlock()
	if kn.inflight["q"].ch != newOwner {
		t.Fatal("stale consumer released a later consumer's turn")
	}
}
