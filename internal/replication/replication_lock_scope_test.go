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
