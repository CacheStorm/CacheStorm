package command

import (
	"sync"
)

// blockingWriter blocks the first write until it is released, so a test can
// hold a command in the middle of writing its reply.
type blockingWriter struct {
	entered  chan struct{}
	release  chan struct{}
	once     sync.Once
	released sync.Once
}

func (w *blockingWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return len(p), nil
}

// unblock releases the writer; it is safe to call more than once.
func (w *blockingWriter) unblock() {
	w.released.Do(func() { close(w.release) })
}

// The readers that take the lock inside a nested block — the set, hash, list
// and stream operations, which lock a store entry rather than a package
// registry — already release it before writing their reply: the reply is
// built after the release in every one of them. They cannot be covered by
// the check above, because the lock they hold is a per-entry lock with no
// exclusive accessor from outside the store package: the store exposes only
// SetOpsRLock, and two read locks do not exclude each other, so a test
// cannot tell whether the reader still holds one.
