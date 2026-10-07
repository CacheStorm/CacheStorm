package cluster

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func TestTagMessageConcurrentDuplicatesDispatchOnce(t *testing.T) {
	prior := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(prior)
	c := New("self", "local", 1, 2, nil)
	message := []byte(`{"type":"TAG_INVALIDATE","origin_node":"peer","timestamp":1,"tag":"tag","keys":["k"]}`)
	for round := 0; round < 20; round++ {
		tb := NewTagBroadcaster(c)
		var count atomic.Int64
		tb.RegisterHandler(func(string, []string) { count.Add(1) })
		start, ready := make(chan struct{}), make(chan struct{}, 64)
		errors := make(chan error, 64)
		var done sync.WaitGroup
		for i := 0; i < 64; i++ {
			done.Add(1)
			go func() { defer done.Done(); ready <- struct{}{}; <-start; errors <- tb.HandleMessage(message) }()
		}
		for i := 0; i < 64; i++ {
			<-ready
		}
		close(start)
		done.Wait()
		close(errors)
		for err := range errors {
			if err != nil {
				t.Fatal(err)
			}
		}
		if count.Load() != 1 {
			t.Fatalf("round %d: expected one callback, got %d", round, count.Load())
		}
	}
	tb := NewTagBroadcaster(c)
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	var count atomic.Int64
	tb.RegisterHandler(func(string, []string) { count.Add(1); close(entered); <-release })
	go func() { finished <- tb.HandleMessage(message) }()
	<-entered
	if err := tb.HandleMessage(message); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 1 {
		t.Fatal("duplicate dispatched while first callback blocked")
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if err := tb.HandleMessage(message); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 1 {
		t.Fatal("completed duplicate dispatched")
	}
	independent := NewTagBroadcaster(c)
	count.Store(0)
	independent.RegisterHandler(func(string, []string) { count.Add(1) })
	for _, data := range [][]byte{message, []byte(`{"origin_node":"peer","timestamp":2}`), []byte(`{"origin_node":"other","timestamp":1}`), []byte(`{"origin_node":"self","timestamp":3}`)} {
		if err := independent.HandleMessage(data); err != nil {
			t.Fatal(err)
		}
	}
	if count.Load() != 3 {
		t.Fatal("distinct messages or self-origin handling changed")
	}
	if err := independent.HandleMessage([]byte(`{`)); err == nil {
		t.Fatal("malformed JSON accepted")
	}
	t.Log("EXPECTED: callback count=1 for concurrent and gated in-flight duplicates ACTUAL: 1")
	fmt.Println("FIX VERIFIED")
}
