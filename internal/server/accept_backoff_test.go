package server

import (
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

// failingListener fails every Accept, the way a listener does once the
// process has run out of file descriptors.
type failingListener struct {
	calls atomic.Int64
}

func (l *failingListener) Accept() (net.Conn, error) {
	l.calls.Add(1)
	return nil, errors.New("accept: too many open files")
}
func (l *failingListener) Close() error   { return nil }
func (l *failingListener) Addr() net.Addr { return &net.TCPAddr{} }

func TestAcceptLoopBacksOffOnPersistentErrors(t *testing.T) {
	s := &Server{cfg: config.Default(), stopCh: make(chan struct{})}
	l := &failingListener{}
	s.listener = l

	done := make(chan struct{})
	go func() { defer close(done); s.acceptLoop() }()
	time.Sleep(300 * time.Millisecond)
	close(s.stopCh)
	<-done

	// 5, 10, 20, 40, 80, 160 ms: about seven attempts fit in 300 ms. Without
	// the backoff the loop made millions.
	if n := l.calls.Load(); n > 20 {
		t.Fatalf("acceptLoop retried %d times in 300ms; it must back off", n)
	}
}
