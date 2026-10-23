package replication

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/store"
)

// blockingConn is a net.Conn whose Close blocks until it is released, so a
// check can hold a handler inside its connection close.
type blockingConn struct {
	net.Conn
	closing   chan struct{}
	release   chan struct{}
	closeOnce sync.Once
	relOnce   sync.Once
}

func newBlockingConn() *blockingConn {
	return &blockingConn{closing: make(chan struct{}), release: make(chan struct{})}
}

func (c *blockingConn) Close() error {
	c.closeOnce.Do(func() { close(c.closing) })
	<-c.release
	return nil
}

func (c *blockingConn) unblock() { c.relOnce.Do(func() { close(c.release) }) }

// TestRemoveReplicaDoesNotHoldTheManagerLockWhileClosingTheConnection is the
// same contract as the command layer's lock-scope checks: a handler must not
// hold the manager's lock while it performs I/O, because the lock is the
// manager's only serialisation point and a slow connection would block every
// other operation on it.
//
// RemoveReplica closed the replica's connection while holding the write lock.
func TestRemoveReplicaDoesNotHoldTheManagerLockWhileClosingTheConnection(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	conn := newBlockingConn()
	t.Cleanup(conn.unblock)
	replica := m.AddReplica(conn, "127.0.0.1", 6380, nil)

	done := make(chan struct{})
	go func() {
		m.RemoveReplica(replica.ID)
		close(done)
	}()

	select {
	case <-conn.closing:
	case <-time.After(3 * time.Second):
		t.Fatal("RemoveReplica never reached the connection close")
	}

	acquired := make(chan struct{})
	go func() {
		m.mu.Lock()
		m.mu.Unlock()
		close(acquired)
	}()

	select {
	case <-acquired:
	case <-time.After(3 * time.Second):
		t.Fatal("the manager's lock was still held while the replica's connection was closed: " +
			"the lock must be released before the connection is closed")
	}

	conn.unblock()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("RemoveReplica did not finish")
	}
}

// blockingWriteConn is a net.Conn whose Write blocks until it is released, so
// a check can hold a propagation inside a replica write.
type blockingWriteConn struct {
	net.Conn
	writing  chan struct{}
	release  chan struct{}
	writeOne sync.Once
	relOne   sync.Once
}

func newBlockingWriteConn() *blockingWriteConn {
	return &blockingWriteConn{writing: make(chan struct{}), release: make(chan struct{})}
}

func (c *blockingWriteConn) Write(p []byte) (int, error) {
	c.writeOne.Do(func() { close(c.writing) })
	<-c.release
	return len(p), nil
}

func (c *blockingWriteConn) unblock() { c.relOne.Do(func() { close(c.release) }) }

// TestSlowReplicaWriteDoesNotBlockTheManager is the lock-scope contract for
// PropagateCommand: the manager's lock must not be held while a replica's
// socket is written to, or one slow replica stalls every other operation on
// the manager.
//
// Each replica's writer is serialised by the replica's own lock, so the
// manager's lock is only needed for the shared backlog and the offsets.
func TestSlowReplicaWriteDoesNotBlockTheManager(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	conn := newBlockingWriteConn()
	t.Cleanup(conn.unblock)
	m.AddReplica(conn, "127.0.0.1", 6380, nil)

	// Bigger than a bufio.Writer's buffer, so the write reaches the socket.
	cmd := make([]byte, 8192)

	done := make(chan struct{})
	go func() {
		m.PropagateCommand(cmd)
		close(done)
	}()

	select {
	case <-conn.writing:
	case <-time.After(3 * time.Second):
		t.Fatal("the propagation never reached the replica's socket")
	}

	acquired := make(chan struct{})
	go func() {
		m.mu.Lock()
		m.mu.Unlock()
		close(acquired)
	}()

	select {
	case <-acquired:
	case <-time.After(3 * time.Second):
		t.Fatal("the manager's lock was still held while a replica was being written to: " +
			"a slow replica must not block the manager")
	}

	conn.unblock()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the propagation did not finish")
	}
}
