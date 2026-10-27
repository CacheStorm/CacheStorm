package server

import (
	"bytes"
	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
	"sync"
	"testing"
)

func tryGateReadWriteX(s *Server, key, value string) bool {
	if !s.gate.global.TryRLock() {
		return false
	}
	defer s.gate.global.RUnlock()
	stripe := &s.gate.stripes[stripeOf(key)]
	if !stripe.TryLock() {
		return false
	}
	defer stripe.Unlock()
	if err := s.store.Set(key, &store.StringValue{Data: []byte(value)}, store.SetOptions{}); err != nil {
		panic(err)
	}
	return true
}

func TestCommandReadSharesAndReleasesForWaitX(t *testing.T) {
	s := &Server{store: store.NewStore()}
	ctx := command.NewContext("XREAD", byteArgs([]string{"BLOCK", "0", "STREAMS", "stream", "0"}), s.store, nil)
	release := s.holdGate(ctx)
	stripe := &s.gate.stripes[stripeOf("stream")]
	if !stripe.TryRLock() {
		release()
		t.Fatal("readers cannot share")
	}
	stripe.RUnlock()
	if tryGateReadWriteX(s, "stream", "before") {
		release()
		t.Fatal("writer entered read")
	}
	ctx.BeforeWait()
	if !tryGateReadWriteX(s, "stream", "during") {
		t.Fatal("wait retained read lock")
	}
	ctx.AfterWait()
	if tryGateReadWriteX(s, "stream", "after") {
		release()
		t.Fatal("wait did not reacquire lock")
	}
	release()
	if !tryGateReadWriteX(s, "stream", "done") {
		t.Fatal("read lock leaked")
	}
}

func TestCommandReadSortWithoutLookupStaysConcurrentX(t *testing.T) {
	for _, args := range [][]string{{"items", "ALPHA"}, {"items", "BY", "nosort"}, {"items", "GET", "#"}} {
		s := &Server{store: store.NewStore()}
		ctx := command.NewContext("SORT_RO", byteArgs(args), s.store, nil)
		release := s.holdGate(ctx)
		other := "other"
		for stripeOf(other) == stripeOf("items") {
			other += "x"
		}
		if !tryGateReadWriteX(s, other, "control") {
			release()
			t.Fatalf("non-lookup SORT serialized unrelated key: %v", args)
		}
		if tryGateReadWriteX(s, "items", "blocked") {
			release()
			t.Fatal("source writer admitted")
		}
		release()
	}
}

func TestCommandReadBlockingStreamProducerX(t *testing.T) {
	s := startReplServer(t, nil)
	var out bytes.Buffer
	ctx := command.NewContext("XREAD", byteArgs([]string{"BLOCK", "0", "STREAMS", "stream", "$"}), s.store, resp.NewWriter(&out))
	cancel := make(chan struct{})
	ctx.Done = cancel
	t.Cleanup(func() { close(cancel) })
	release := s.holdGate(ctx)
	before := ctx.BeforeWait
	waiting := make(chan struct{})
	var once sync.Once
	ctx.BeforeWait = func() { before(); once.Do(func() { close(waiting) }) }
	done := make(chan error, 1)
	go func() { defer release(); done <- s.router.Execute(ctx) }()
	<-waiting
	var written bytes.Buffer
	producer := command.NewContext("XADD", byteArgs([]string{"stream", "1-0", "f", "v"}), s.store, resp.NewWriter(&written))
	unlock := s.holdGate(producer)
	err := s.router.Execute(producer)
	unlock()
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("1-0")) {
		t.Fatalf("reader lost producer: %q", out.String())
	}
}

func TestCommandReadTCPDuplicateMGETX(t *testing.T) {
	s := startReplServer(t, nil)
	writer := dialRepl(t, s)
	reader := dialRepl(t, s)
	if got := writer.do("SET", "k", "0"); !aclIsOK(got) {
		t.Fatal(got)
	}
	start := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-start
		for i := 0; i < 1000; i++ {
			if got := writer.do("INCR", "k"); got.Type != resp.TypeInteger {
				t.Errorf("writer=%+v", got)
				return
			}
		}
	}()
	t.Cleanup(func() { <-done })
	close(start)
	for i := 0; i < 1000; i++ {
		got := reader.do("MGET", "k", "k", "k")
		if got.Type != resp.TypeArray || len(got.Array) != 3 {
			t.Fatalf("MGET=%+v", got)
		}
		for j := 1; j < 3; j++ {
			if !bytes.Equal(got.Array[0].Bulk, got.Array[j].Bulk) {
				t.Fatalf("torn MGET: %+v", got)
			}
		}
	}
	<-done
}
func TestCommandReadIsolationX(t *testing.T) {
	s := &Server{store: store.NewStore()}
	if err := s.store.Set("k", &store.StringValue{Data: []byte("old")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	other := "other"
	for stripeOf(other) == stripeOf("k") {
		other += "x"
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan [2]string, 1)
	go func() {
		ctx := command.NewContext("MGET", byteArgs([]string{"k", "k"}), s.store, nil)
		unlock := s.holdGate(ctx)
		defer unlock()
		first, _ := s.store.Get("k")
		old := first.Value.String()
		close(entered)
		<-release
		second, _ := s.store.Get("k")
		done <- [2]string{old, second.Value.String()}
	}()
	<-entered
	if !tryGateReadWriteX(s, other, "control") {
		close(release)
		<-done
		t.Fatal("CONTROL unrelated writer was blocked")
	}
	admitted := tryGateReadWriteX(s, "k", "new")
	close(release)
	got := <-done
	t.Logf("EXPECTED: same-key writer excluded, [old old] | ACTUAL: admitted=%v values=%v", admitted, got)
	if admitted || got != [2]string{"old", "old"} {
		t.Error("MGET gate permits torn observation")
	}
	for _, name := range []string{"KEYS", "SCAN", "DBSIZE", "SORT_RO"} {
		t.Run(name, func(t *testing.T) {
			args := []string{"*"}
			if name == "SCAN" {
				args = []string{"0"}
			}
			if name == "DBSIZE" {
				args = nil
			}
			if name == "SORT_RO" {
				args = []string{"items", "BY", "weight_*"}
			}
			ctx := command.NewContext(name, byteArgs(args), s.store, nil)
			unlock := s.holdGate(ctx)
			ok := tryGateReadWriteX(s, "weight_1", "new")
			unlock()
			t.Logf("EXPECTED: global/dynamic snapshot excludes writer | ACTUAL: %s admitted=%v", name, ok)
			if ok {
				t.Error("writer admitted during keyspace/dynamic read")
			}
		})
	}
	for _, name := range []string{"PING", "ECHO", "TIME", "COMMAND"} {
		ctx := command.NewContext(name, nil, s.store, nil)
		unlock := s.holdGate(ctx)
		ok := tryGateReadWriteX(s, "k", "control")
		unlock()
		if !ok {
			t.Fatalf("CONTROL %s serialized unrelated data", name)
		}
	}
}
