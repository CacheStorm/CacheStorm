package command

import (
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// TestStoreEntryReaderReleasesTheEntryLockBeforeWritingTheReply is the same
// contract as the registry readers' check, for the readers that lock a store
// entry rather than a package registry: the set, hash, list and sorted-set
// operations.
//
// The reader took the entry's read lock with a deferred release, so the lock
// was still held while the reply went to the client. The entry's write lock
// cannot be taken while any reader holds its read lock, so a client that read
// slowly blocked every writer of that entry.
//
// The check is deterministic: the reply write blocks on a channel, and the
// test then takes the entry's write lock. If the reader still held the read
// lock the write lock could not be taken and the check would fail.
func TestStoreEntryReaderReleasesTheEntryLockBeforeWritingTheReply(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	seed, _ := bufCtx("HSET", bytesArgs("scope-hash", "field", "value"), s)
	if err := router.Execute(seed); err != nil {
		t.Fatalf("HSET: %v", err)
	}

	// The hash the reader locks.
	lookup, _ := bufCtx("HGET", bytesArgs("scope-hash", "field"), s)
	hash, err := getOrCreateHash(lookup, "scope-hash")
	if err != nil {
		t.Fatalf("getOrCreateHash: %v", err)
	}

	writer := &blockingWriter{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(writer.unblock)

	ctx := NewContext("HGET", bytesArgs("scope-hash", "field"), s, resp.NewWriter(writer))
	done := make(chan error, 1)
	go func() { done <- router.Execute(ctx) }()

	select {
	case <-writer.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the command never reached its reply write")
	}

	acquired := make(chan struct{})
	go func() {
		hash.Lock()
		hash.Unlock()
		close(acquired)
	}()

	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("the hash's write lock was still held while the reader wrote its reply: " +
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
