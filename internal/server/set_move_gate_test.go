package server

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestSetMoveReaderGateX(t *testing.T) {
	for _, name := range []string{"EXEC", "SMOVE"} {
		t.Run(name, func(t *testing.T) {
			s := &Server{store: store.NewStore()}
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			go func() {
				defer close(done)
				ctx := command.NewContext(name, [][]byte{[]byte("a"), []byte("b"), []byte("m")}, s.store, nil)
				unlock := s.holdGate(ctx)
				defer unlock()
				if err := s.store.Set("a", &store.StringValue{Data: []byte("1")}, store.SetOptions{}); err != nil {
					panic(err)
				}
				close(entered)
				<-release
				if err := s.store.Set("b", &store.StringValue{Data: []byte("2")}, store.SetOptions{}); err != nil {
					panic(err)
				}
			}()
			<-entered
			admitted := s.gate.global.TryRLock()
			partial := false
			if admitted {
				partial = s.store.Exists("a") && !s.store.Exists("b")
				s.gate.global.RUnlock()
			}
			close(release)
			<-done
			t.Logf("EXPECTED: reader excluded during batch | ACTUAL: admitted=%v partial=%v", admitted, partial)
			if admitted {
				t.Error("reader can observe partially applied batch")
			}
		})
	}
}

func TestSetMoveGateHeldThroughCommitX(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	if got := c.do("SADD", "s", "m"); got.Type != resp.TypeInteger || got.Int != 1 {
		t.Fatalf("seed=%+v", got)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	s.router.SetPostExecute(func(cmd string, _ [][]byte) {
		if cmd == "SMOVE" {
			close(entered)
			<-release
		}
	})
	done := make(chan *resp.Value, 1)
	go func() { done <- c.do("SMOVE", "s", "d", "m") }()
	select {
	case <-entered:
	case got := <-done:
		t.Fatalf("returned before commit: %+v", got)
	}
	if s.gate.global.TryRLock() {
		s.gate.global.RUnlock()
		t.Error("reader admitted before commit")
	}
	close(release)
	if got := <-done; got.Type != resp.TypeInteger || got.Int != 1 {
		t.Fatalf("move=%+v", got)
	}
}
