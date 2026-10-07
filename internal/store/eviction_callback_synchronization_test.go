package store

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func TestEvictionCallbackReplacementIsSynchronized(t *testing.T) {
	s := NewStore()
	ec := NewEvictionController(EvictionAllKeysLRU, 1000, s, NewMemoryTracker(1000, 80, 90), 1)
	put := func(key string) {
		if err := s.Set(key, &StringValue{Data: []byte("value")}, SetOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	put("control")
	if ec.ForceEvict(1) != 1 {
		t.Fatal("no-callback control failed")
	}
	put("concurrent")
	var calls atomic.Int64
	handler := func(string, *Entry) { calls.Add(1) }
	ec.SetOnEvict(handler)
	start, ready, changed, done := make(chan struct{}), make(chan struct{}, 2), make(chan struct{}), make(chan int, 1)
	go func() { ready <- struct{}{}; <-start; ec.SetOnEvict(handler); close(changed) }()
	go func() { ready <- struct{}{}; <-start; done <- ec.ForceEvict(1) }()
	<-ready
	<-ready
	close(start)
	<-changed
	if <-done != 1 || calls.Load() != 1 {
		t.Fatal("concurrent callback delivery changed")
	}
	put("old")
	entered, release := make(chan struct{}), make(chan struct{})
	ec.SetOnEvict(func(string, *Entry) { close(entered); <-release; calls.Add(1) })
	go func() { done <- ec.ForceEvict(1) }()
	<-entered
	var fresh atomic.Int64
	ec.SetOnEvict(func(string, *Entry) { fresh.Add(1) })
	close(release)
	if <-done != 1 || calls.Load() != 2 || fresh.Load() != 0 {
		t.Fatal("in-flight callback snapshot changed")
	}
	put("new")
	if ec.ForceEvict(1) != 1 || fresh.Load() != 1 {
		t.Fatal("replacement callback not used")
	}
	ec.SetOnEvict(nil)
	put("nil")
	if ec.ForceEvict(1) != 1 || fresh.Load() != 1 {
		t.Fatal("nil callback not respected")
	}
	if ec.ForceEvict(1) != 0 {
		t.Fatal("empty store eviction reported success")
	}
	t.Logf("EXPECTED: old callbacks=2 new callbacks=1 ACTUAL: old=%d new=%d", calls.Load(), fresh.Load())
	fmt.Println("FIX VERIFIED")
}
