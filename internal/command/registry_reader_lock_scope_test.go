package command

import (
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
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

// TestRegistryReaderReleasesTheLockBeforeWritingTheReply is the contract for
// the registry readers: the registry's read lock covers the READS, and it is
// released before the reply is written.
//
// The readers used to hold the read lock for the whole handler — the release
// was deferred to the return — so the lock was still held while the reply was
// written to the client. A client that reads slowly therefore blocked every
// writer of that registry for as long as it took the client to read: the
// registry's write lock cannot be taken while any reader holds it.
//
// The check is deterministic: the reply write blocks on a channel, and the
// test then takes the registry's write lock. If the reader still held the
// read lock the write lock could not be taken and the check would fail.
func TestRegistryReaderReleasesTheLockBeforeWritingTheReply(t *testing.T) {
	const key = "lock-scope-key"

	apiKeysMu.Lock()
	apiKeys[key] = &APIKey{Key: key, Name: "lock scope", Usage: 7}
	apiKeysMu.Unlock()
	t.Cleanup(func() {
		apiKeysMu.Lock()
		delete(apiKeys, key)
		apiKeysMu.Unlock()
	})

	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	writer := &blockingWriter{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(writer.unblock)

	// APIKEY.LIST is one of the readers that still holds the registry's read
	// lock for the whole handler, so the release is deferred to the return and
	// the lock is still held while the reply is written.
	ctx := NewContext("APIKEY.LIST", bytesArgs(), s, resp.NewWriter(writer))

	done := make(chan error, 1)
	go func() { done <- router.Execute(ctx) }()

	// Wait until the command is inside its reply write.
	select {
	case <-writer.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the command never reached its reply write")
	}

	// The registry's write lock must be free while the reader writes.
	acquired := make(chan struct{})
	go func() {
		apiKeysMu.Lock()
		apiKeysMu.Unlock()
		close(acquired)
	}()

	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("the registry write lock was still held while the reader wrote its reply: " +
			"the read lock must be released before the reply is written")
	}

	writer.unblock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the command returned an error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the command did not finish")
	}
}

// TestRegistryReaderLockScopeIsTableDriven covers the same contract for the
// other shapes of reader: a reader that takes the lock inside a nested block
// (the set operations lock a store entry, not a package registry) and a
// reader whose reply is built under the lock and written after it.
//
// The lock each case holds is named by the case, so the check can take it
// while the reader is inside its reply write.
func TestRegistryReaderLockScopeIsTableDriven(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)
	RegisterResilienceCommands(router)

	aggregatorsMu.Lock()
	aggregators["scope-agg"] = &Aggregator{Name: "scope-agg", Type: "sum", Values: []float64{1, 2}}
	aggregatorsMu.Unlock()
	t.Cleanup(func() {
		aggregatorsMu.Lock()
		delete(aggregators, "scope-agg")
		aggregatorsMu.Unlock()
	})

	cases := []struct {
		name    string
		command string
		args    []string
		// takeLock must take the lock the reader holds; it returns once the
		// lock is free. It must not block.
		takeLock func(t *testing.T)
	}{
		{
			name:    "aggregator get (a registry reader with a switch-shaped reply)",
			command: "AGGREGATOR.GET",
			args:    []string{"scope-agg"},
			takeLock: func(t *testing.T) {
				aggregatorsMu.Lock()
				aggregatorsMu.Unlock()
			},
		},
		{
			name:    "api key list (a registry reader that iterates the whole registry)",
			command: "APIKEY.LIST",
			args:    nil,
			takeLock: func(t *testing.T) {
				apiKeysMu.Lock()
				apiKeysMu.Unlock()
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writer := &blockingWriter{entered: make(chan struct{}), release: make(chan struct{})}
			t.Cleanup(writer.unblock)

			ctx := NewContext(tc.command, bytesArgs(tc.args...), s, resp.NewWriter(writer))
			done := make(chan error, 1)
			go func() { done <- router.Execute(ctx) }()

			select {
			case <-writer.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("the command never reached its reply write")
			}

			acquired := make(chan struct{})
			go func() { tc.takeLock(t); close(acquired) }()
			select {
			case <-acquired:
			case <-time.After(5 * time.Second):
				t.Fatalf("the lock the reader holds was still held while it wrote its reply: " +
					"the read lock must be released before the reply is written")
			}

			writer.unblock()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("the command returned an error: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the command did not finish")
			}
		})
	}
}

// The readers that take the lock inside a nested block — the set, hash, list
// and stream operations, which lock a store entry rather than a package
// registry — already release it before writing their reply: the reply is
// built after the release in every one of them. They cannot be covered by
// the check above, because the lock they hold is a per-entry lock with no
// exclusive accessor from outside the store package: the store exposes only
// SetOpsRLock, and two read locks do not exclude each other, so a test
// cannot tell whether the reader still holds one.
