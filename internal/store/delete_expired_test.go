package store

import (
	"strconv"
	"testing"
	"time"
)

// DEL and UNLINK of a key that has expired but is still stored count it as
// missing, as in Redis: the key is expired first, which is what its hooks
// announce and record. A replica (passive expiry) deletes it as its master
// says.
func TestDeleteExpiredKeyCountsNothing(t *testing.T) {
	s := NewStore()
	expired := 0
	s.SetHooks(StoreHooks{OnExpireLocked: func(*Store, string) { expired++ }})
	if err := s.Set("k", &StringValue{Data: []byte("v")}, SetOptions{ExpireAtMS: time.Now().UnixMilli() - 1}); err != nil {
		t.Fatal(err)
	}
	if s.Delete("k") {
		t.Fatal("Delete counted an expired key")
	}
	if expired != 1 {
		t.Fatalf("the key was expired %d times, want once", expired)
	}
	if _, ok := s.shards[s.shardIndex("k")].Get("k"); ok {
		t.Fatal("the expired key is still stored")
	}

	r := NewStore()
	r.passiveExpiry.Store(true)
	if err := r.Set("k", &StringValue{Data: []byte("v")}, SetOptions{ExpireAtMS: time.Now().UnixMilli() - 1}); err != nil {
		t.Fatal(err)
	}
	if !r.Delete("k") {
		t.Fatal("a replica did not delete the expired key its master deleted")
	}
}

// While a transaction or script holds the clock still, a key past its expiry
// time is alive until the clock runs again.
func TestFrozenExpiryClock(t *testing.T) {
	s := NewStore()
	dbs := NewDatabases(s, 2)
	now := time.Now().UnixMilli()
	if err := s.Set("k", &StringValue{Data: []byte("v")}, SetOptions{ExpireAtMS: now + 20}); err != nil {
		t.Fatal(err)
	}
	thaw := dbs.FreezeExpiry(now)
	if inner := dbs.FreezeExpiry(now + 1000); inner == nil {
		t.Fatal("nil thaw")
	} else {
		inner() // a nested freeze leaves the outer one in place
	}
	time.Sleep(40 * time.Millisecond)
	if _, ok := s.Get("k"); !ok {
		t.Fatal("the key expired while the clock was frozen")
	}
	if dbs.DB(1).clock.frozen.Load() != now {
		t.Fatal("a database created during the freeze runs on the clock")
	}
	thaw()
	if _, ok := s.Get("k"); ok {
		t.Fatal("the key outlived the freeze")
	}
}

// INVALIDATE deletes a tag's expired keys too, but counts only the live
// ones (listing the expired ones last), in the sequential and the parallel
// batch alike.
func TestDeleteInvalidatedCountsLiveKeys(t *testing.T) {
	for _, n := range []int{10, 2 * parallelDeleteMin} {
		s := NewStore()
		past := time.Now().UnixMilli() - 1
		for i := 0; i < n; i++ {
			opts := SetOptions{Tags: []string{"t"}}
			if i%2 == 1 {
				opts.ExpireAtMS = past
			}
			if err := s.Set("k"+strconv.Itoa(i), &StringValue{Data: []byte("v")}, opts); err != nil {
				t.Fatal(err)
			}
		}
		keys := s.tagIndex.Invalidate("t")
		gone, live := s.DeleteInvalidated(keys, []string{"t"})
		if len(gone) != n || live != n/2 {
			t.Fatalf("n=%d: %d deleted, %d alive; want %d and %d", n, len(gone), live, n, n/2)
		}
		for i, k := range gone {
			idx, _ := strconv.Atoi(k[1:])
			if expired := idx%2 == 1; expired != (i >= live) {
				t.Fatalf("n=%d: %s listed at %d with %d alive", n, k, i, live)
			}
		}
	}
}
