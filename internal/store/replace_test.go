package store

import (
	"math"
	"testing"
	"time"
)

func TestReplaceFromPreservesOwnersAndAccounting(t *testing.T) {
	base := NewStore()
	base.ConfigureMemory(1<<20, EvictionAllKeysLRU, 80, 90, 5)
	dst := NewDatabases(base, 4)
	oldDB2 := dst.DB(2)
	if err := base.Set("old", &StringValue{Data: []byte("old")}, SetOptions{Tags: []string{"old-tag"}}); err != nil {
		t.Fatal(err)
	}
	if err := oldDB2.Set("gone", &StringValue{Data: []byte("old-db2")}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	tracker, notifier, hub := base.MemoryTracker(), base.KeyNotifier(), base.GetPubSub()
	version := base.WatchKey("old")
	defer base.UnwatchKey("old")
	invalidated := false
	base.SetHooks(StoreHooks{OnTagInvalidate: func(tag string, keys []string) {
		if tag == "old-tag" && len(keys) == 1 && keys[0] == "old" {
			invalidated = true
		}
	}})
	src := dst.NewStaging()
	if err := src.DB(0).Set("new", &StringValue{Data: []byte("new")}, SetOptions{Tags: []string{"new-tag"}, ExpireAtMS: math.MaxInt64}); err != nil {
		t.Fatal(err)
	}
	if err := src.DB(0).Set("expired", &StringValue{Data: []byte("expired")}, SetOptions{ExpireAtMS: 1}); err != nil {
		t.Fatal(err)
	}
	if err := src.DB(3).Set("third", &StringValue{Data: []byte("db3")}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := dst.ReplaceFrom(src); err != nil {
		t.Fatal(err)
	}
	if dst.DB(0) != base || dst.DB(2) != oldDB2 || base.MemoryTracker() != tracker || base.KeyNotifier() != notifier || base.GetPubSub() != hub {
		t.Fatal("replacement changed a live owner")
	}
	if base.Exists("old") || oldDB2.KeyCount() != 0 || !dst.DB(3).Exists("third") {
		t.Fatal("replacement did not replace all numbered databases")
	}
	if base.GetVersion("old") == version || !invalidated {
		t.Fatal("replacement did not invalidate old observers")
	}
	if keys := base.GetTagIndex().GetKeys("new-tag"); len(keys) != 1 || keys[0] != "new" {
		t.Fatalf("new tag index = %v", keys)
	}
	if tracker.Usage() != dst.MemUsage() || src.MemUsage() != 0 || src.DB(0).MemoryTracker().Usage() != 0 {
		t.Fatal("replacement charged memory to the wrong owner")
	}
	base.expiry.tick()
	if base.KeyCount() != 1 || !base.Exists("new") {
		t.Fatal("replacement did not schedule elapsed expiry")
	}
	base.expiry.farFuture.mu.Lock()
	at := base.expiry.farFuture.keys["new"]
	base.expiry.farFuture.mu.Unlock()
	if at != math.MaxInt64 || tracker.Usage() != dst.MemUsage() {
		t.Fatal("replacement lost expiry or memory accounting")
	}
}

func TestReplaceFromNotifiesBlockedReaders(t *testing.T) {
	dst := NewDatabases(NewStore(), 2)
	src := dst.NewStaging()
	if err := src.DB(0).Set("ready", &ListValue{Elements: [][]byte{[]byte("item")}}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	registered := make(chan struct{})
	result := make(chan bool, 1)
	go func() {
		_, ok := dst.DB(0).KeyNotifier().WaitForKeysOpts([]string{"ready"}, time.Second, WaitOptions{Ready: func(string) bool {
			close(registered)
			return false
		}})
		result <- ok
	}()
	<-registered
	if err := dst.ReplaceFrom(src); err != nil {
		t.Fatal(err)
	}
	if !<-result {
		t.Fatal("replacement left a reader blocked")
	}
}

func TestReplaceFromNotifiesRemovedKeys(t *testing.T) {
	dst := NewDatabases(NewStore(), 2)
	if err := dst.DB(0).Set("removed", &StringValue{Data: []byte("old")}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	registered := make(chan struct{})
	result := make(chan bool, 1)
	go func() {
		_, ok := dst.DB(0).KeyNotifier().WaitForKeysOpts([]string{"removed"}, time.Second, WaitOptions{Ready: func(string) bool {
			close(registered)
			return false
		}})
		result <- ok
	}()
	<-registered
	if err := dst.ReplaceFrom(dst.NewStaging()); err != nil {
		t.Fatal(err)
	}
	if !<-result {
		t.Fatal("replacement left a removed-key observer blocked")
	}
}
