package store

import (
	"sync"
	"testing"
	"time"
)

// Audit finding: Entry.ExpiresAt is a bare int64 sitting beside two fields that
// WERE converted to atomics in the same struct (entry.go:193-199):
//
//	type Entry struct {
//		Value       Value
//		Tags        []string
//		ExpiresAt   int64          <-- bare, no atomic
//		CreatedAt   int64
//		LastAccess  atomic.Int64
//		AccessCount atomic.Uint64
//	}
//
// Entry has NO lock of its own, and the shard lock cannot stand in for one,
// because Shard.Get (shard.go:25-30) returns a POINTER and releases the lock on
// return:
//
//	func (s *Shard) Get(key string) (*Entry, bool) {
//		s.mu.RLock()
//		defer s.mu.RUnlock()
//		entry, ok := s.data[key]
//		return entry, ok      <-- lock already released; caller holds a live pointer
//	}
//
// WRITER (holds the shard lock -- which the reader no longer holds):
//	shard.updateExpiry  shard.go:82   entry.ExpiresAt.Store(expiresAt)
//	Entry.SetTTL        entry.go:234  e.ExpiresAt = time.Now().Add(ttl).UnixNano()
//	Entry.SetExpiresAt  entry.go:238  e.ExpiresAt = expiresAt
//	                    reached via Store.SetTTL (store.go:471) and Store.SetExpiresAt
//
// READERS (hold no lock at all -- they only hold the raw pointer):
//	cmdTTL        server_commands.go:600  if entry.ExpiresAt.Load() == 0
//	cmdTTL        server_commands.go:603  entry.ExpiresAt.Load() / 1e6
//	cmdEXPIRETIME server_commands.go:615  entry.ExpiresAt.Load() == 0
//	cmdEXPIRETIME server_commands.go:618  entry.ExpiresAt.Load() / 1e9
//	debug_commands.go:130                entry.IsExpired()
//	rdb.go:172 / replication_commands.go:195-196   entry.TTL()
//
// So one client running EXPIRE (or PERSIST, or SETEX) against a key races
// another's TTL / EXPIRETIME on that same Entry. The lock ordering is correct;
// the problem is that no lock is held across the pointer's lifetime.
//
// Impact: a stale or torn 8-byte value read as a TTL. On 64-bit an aligned int64
// load does not tear in practice, so this is a formal data race and a detector
// finding rather than memory corruption -- unlike the map races, which abort the
// process. Fixing it means bringing ExpiresAt in line with the two fields
// beside it that were already converted.

// TestEntryExpiresAtRacesSetTTL is the primary case: Store.SetTTL writing
// ExpiresAt under the shard lock while a reader holds the pointer from Store.Get
// and reads it with no lock. Only meaningful under -race.
func TestEntryExpiresAtRacesSetTTL(t *testing.T) {
	s := NewStore()
	const key = "ttl-race-key"

	if err := s.Set(key, &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("seed Set: %v", err)
	}

	var wg sync.WaitGroup
	// Writer: EXPIRE / PERSIST path -> updateExpiry -> entry.ExpiresAt.Store(...)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 4000; i++ {
			s.SetTTL(key, time.Duration(i%500+1)*time.Second)
			s.SetExpiresAt(key, time.Now().Add(time.Duration(i%500+1)*time.Second).UnixMilli())
		}
	}()
	// Reader: TTL / EXPIRETIME path -- Get returns the pointer, then read bare.
	for i := 0; i < 4000; i++ {
		entry, ok := s.Get(key)
		if !ok {
			t.Fatalf("key vanished at iteration %d", i)
		}
		if entry.ExpiresAt.Load() == 0 {
			_ = entry.ExpiresAt.Load() / 1e6
		}
		_ = entry.IsExpired()
		_ = entry.TTL()
	}
	wg.Wait()
}

// TestEntryExpiresAtControls is the control: TTL and EXPIRETIME report the value
// SetTTL actually stored, and IsExpired/TTL agree with it. Passes before and
// after the fix, and proves the race test above actually reads the field rather
// than short-circuiting on a missing key.
func TestEntryExpiresAtControls(t *testing.T) {
	s := NewStore()
	const key = "ttl-ctl-key"

	if err := s.Set(key, &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("seed Set: %v", err)
	}

	entry, ok := s.Get(key)
	if !ok {
		t.Fatal("seed key missing")
	}
	if entry.ExpiresAt.Load() != 0 {
		t.Fatalf("CONTROL FAIL: fresh entry has ExpiresAt %d, want 0", entry.ExpiresAt.Load())
	}
	if entry.TTL() != -1 {
		t.Fatalf("CONTROL FAIL: TTL of a key with no expiry = %v, want -1", entry.TTL())
	}
	if entry.IsExpired() {
		t.Fatal("CONTROL FAIL: a key with no expiry reported expired")
	}

	want := 90 * time.Second
	if !s.SetTTL(key, want) {
		t.Fatal("CONTROL FAIL: SetTTL on an existing key returned false")
	}

	entry, _ = s.Get(key)
	if entry.ExpiresAt.Load() == 0 {
		t.Fatal("CONTROL FAIL: ExpiresAt still 0 after SetTTL")
	}
	if entry.IsExpired() {
		t.Fatal("CONTROL FAIL: entry reported expired immediately after SetTTL(90s)")
	}
	ttl := entry.TTL()
	if ttl <= 0 || ttl > want {
		t.Fatalf("CONTROL FAIL: TTL = %v, want (0, %v]", ttl, want)
	}
	t.Logf("CONTROL PASS: SetTTL stored ExpiresAt, IsExpired/TTL agree (ttl=%v)", ttl)
}
