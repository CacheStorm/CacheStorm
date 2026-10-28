package store

import (
	"testing"
	"time"
)

func strVal(s string) *StringValue { return &StringValue{Data: []byte(s)} }

func TestDatabasesLazySiblingsShareGlobalState(t *testing.T) {
	base := NewStore()
	base.ConfigureMemory(1<<30, EvictionAllKeysLRU, 70, 85, 5)
	dbs := NewDatabases(base, 4)

	if dbs.Count() != 4 || dbs.DB(0) != base {
		t.Fatal("database 0 must be the base store")
	}
	if dbs.Existing(2) != nil {
		t.Fatal("databases must be created on first use only")
	}
	db2 := dbs.DB(2)
	if db2 == nil || db2 == base || dbs.DB(2) != db2 {
		t.Fatal("DB(2) must create one store and keep returning it")
	}
	if db2.GetPubSub() != base.GetPubSub() {
		t.Fatal("Pub/Sub is not scoped to a database and must be shared")
	}
	if db2.MemoryTracker() != base.MemoryTracker() || db2.Evictor() == nil || db2.Evictor() == base.Evictor() {
		t.Fatal("a sibling must share the memory tracker and own its evictor")
	}
	if dbs.DB(4) != nil || dbs.DB(-1) != nil {
		t.Fatal("indexes out of range must return nil")
	}

	before := base.MemoryTracker().Usage()
	if err := db2.Set("k", strVal("value"), SetOptions{}); err != nil {
		t.Fatal(err)
	}
	if base.MemoryTracker().Usage() <= before {
		t.Fatal("writes in another database must count against the global memory limit")
	}
	if base.Exists("k") {
		t.Fatal("databases must not share keys")
	}
}

func TestDatabasesSwapAndFlushAll(t *testing.T) {
	dbs := NewDatabases(NewStore(), 16)
	_ = dbs.DB(0).Set("a", strVal("0"), SetOptions{})
	_ = dbs.DB(1).Set("b", strVal("1"), SetOptions{})

	if err := dbs.Swap(0, 1); err != nil {
		t.Fatal(err)
	}
	if !dbs.DB(0).Exists("b") || !dbs.DB(1).Exists("a") || dbs.DB(0).Exists("a") {
		t.Fatal("SWAPDB must exchange the contents of the two databases")
	}
	if err := dbs.Swap(0, 16); err != ErrDBIndexOutOfRange {
		t.Fatalf("swap out of range: got %v", err)
	}
	if dbs.KeyCount() != 2 {
		t.Fatalf("KeyCount over all databases: got %d", dbs.KeyCount())
	}
	dbs.FlushAll()
	if dbs.KeyCount() != 0 {
		t.Fatal("FlushAll must empty every database")
	}
}

func TestDatabasesExpiryRunsInLaterDatabases(t *testing.T) {
	dbs := NewDatabases(NewStore(), 16)
	dbs.StartExpiry()
	defer dbs.StopExpiry()

	db5 := dbs.DB(5)
	if err := db5.Set("k", strVal("v"), SetOptions{TTL: 50 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for db5.KeyCount() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("active expiry must run in a database created after StartExpiry")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestWatchVersionChangesOnEveryModification(t *testing.T) {
	s := NewStore()
	_ = s.Set("k", strVal("v"), SetOptions{})

	check := func(name string, mutate func()) {
		t.Helper()
		v := s.WatchKey("k")
		mutate()
		if s.GetVersion("k") == v {
			t.Fatalf("%s must change the watched key's version", name)
		}
		s.UnwatchKey("k")
	}
	check("EXPIRE", func() { s.SetTTL("k", time.Hour) })
	check("PERSIST", func() { s.Persist("k") })
	check("DEL", func() { s.Delete("k") })
	check("create", func() { _ = s.Set("k", strVal("v"), SetOptions{}) })
	check("delete and recreate", func() {
		s.Delete("k")
		_ = s.Set("k", strVal("v"), SetOptions{})
	})
	check("FLUSHDB", func() { s.Flush() })
	_ = s.Set("k", strVal("v"), SetOptions{})
	check("DeleteBatch", func() { s.DeleteBatch([]string{"k"}) })
	_ = s.Set("k", strVal("v"), SetOptions{})
	check("in-place write", func() { s.TouchKey("k") })

	v := s.WatchKey("other")
	s.Flush()
	if s.GetVersion("other") != v {
		t.Fatal("flushing a database must not touch a watched key it did not hold")
	}
	s.UnwatchKey("other")

	if s.HasWatchedKeys() || len(s.versions) != 0 || len(s.watchRefs) != 0 {
		t.Fatal("unwatching every key must release all watch state")
	}
}

func TestWatchVersionChangesWhenKeyExpires(t *testing.T) {
	s := NewStore()
	_ = s.Set("k", strVal("v"), SetOptions{TTL: 10 * time.Millisecond})
	v := s.WatchKey("k")
	time.Sleep(20 * time.Millisecond)
	if _, ok := s.Get("k"); ok {
		t.Fatal("key should have expired")
	}
	if s.GetVersion("k") == v {
		t.Fatal("the expiry of a watched key must change its version")
	}
}

func TestSwapTouchesWatchersOfBothDatabases(t *testing.T) {
	dbs := NewDatabases(NewStore(), 16)
	db0, db1 := dbs.DB(0), dbs.DB(1)
	_ = db0.Set("a", strVal("v"), SetOptions{})
	_ = db0.Set("b", strVal("v"), SetOptions{})
	v0 := db0.WatchKey("a")
	v1 := db1.WatchKey("b")
	vc := db0.WatchKey("c")
	if err := dbs.Swap(0, 1); err != nil {
		t.Fatal(err)
	}
	if db0.GetVersion("a") == v0 || db1.GetVersion("b") == v1 {
		t.Fatal("SWAPDB must abort transactions watching keys of either database")
	}
	if db0.GetVersion("c") != vc {
		t.Fatal("SWAPDB must not touch a watched key missing from both databases")
	}
}

func TestExpiryStats(t *testing.T) {
	s := NewStore()
	_ = s.Set("a", strVal("v"), SetOptions{})
	_ = s.Set("b", strVal("v"), SetOptions{TTL: 100 * time.Second})
	n, avg := s.ExpiryStats()
	if n != 1 || avg <= 90000 || avg > 100000 {
		t.Fatalf("ExpiryStats: got %d keys, avg %d ms", n, avg)
	}
}
