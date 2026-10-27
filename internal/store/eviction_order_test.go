package store

import (
	"sync"
	"testing"
	"time"
)

func TestEvictionRemovalIsOrderedBeforeReplacement(t *testing.T) {
	s := NewStore()
	if err := s.Set("k", &StringValue{Data: []byte("old")}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	old, _ := s.Get("k")
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	s.SetHooks(StoreHooks{OnEvictLocked: func(_ *Store, _ string) { close(entered); <-release }})
	evicted := make(chan bool, 1)
	go func() { evicted <- s.evictObserved("k", old) }()
	<-entered
	written := make(chan error, 1)
	go func() { written <- s.Set("k", &StringValue{Data: []byte("new")}, SetOptions{}) }()
	select {
	case err := <-written:
		t.Fatalf("replacement passed unrecorded removal: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	unblock()
	if !<-evicted {
		t.Fatal("victim not evicted")
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	if s.evictObserved("k", old) {
		t.Fatal("stale victim removed replacement")
	}
	got, ok := s.Get("k")
	if !ok || got.Value.String() != "new" {
		t.Fatal("replacement lost")
	}
}
