package store

import (
	"sort"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/hashslot"
)

func slotKeysOf(s *Store, key string) []string {
	keys := s.KeysInSlot(hashslot.Key(key), 100)
	sort.Strings(keys)
	return keys
}

func TestSlotIndexTracksKeys(t *testing.T) {
	s := NewStore()
	if err := s.Set("pre", &StringValue{Data: []byte("x")}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	s.EnableSlotIndex()
	if !s.SlotIndexed() || s.CountKeysInSlot(hashslot.Key("pre")) != 1 {
		t.Fatal("existing keys are not indexed")
	}
	v := func(d string) Value { return &StringValue{Data: []byte(d)} }
	a, b := "{u1}.a", "{u1}.b"
	slot := hashslot.Key(a)
	_ = s.Set(a, v("1"), SetOptions{})
	_ = s.Set(b, v("2"), SetOptions{})
	_ = s.Set(a, v("3"), SetOptions{})
	if n := s.CountKeysInSlot(slot); n != 2 {
		t.Fatalf("count after set = %d", n)
	}
	if got := slotKeysOf(s, a); len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("keys = %v", got)
	}
	if got := s.KeysInSlot(slot, 1); len(got) != 1 {
		t.Fatalf("max not honoured: %v", got)
	}
	s.Delete(b)
	if n := s.CountKeysInSlot(slot); n != 1 {
		t.Fatalf("count after delete = %d", n)
	}

	if _, err := s.Transfer(a, s, "other", TransferOptions{}); err != nil {
		t.Fatal(err)
	}
	if s.CountKeysInSlot(slot) != 0 || s.CountKeysInSlot(hashslot.Key("other")) != 1 {
		t.Fatal("rename did not move the key between slots")
	}

	_ = s.Set("exp", v("x"), SetOptions{TTL: time.Millisecond})
	time.Sleep(5 * time.Millisecond)
	if !s.DeleteIfExpired("exp") || s.CountKeysInSlot(hashslot.Key("exp")) != 0 {
		t.Fatal("expiry left the key in the index")
	}

	_ = s.Set(b, v("x"), SetOptions{})
	s.Flush()
	for _, k := range []string{"pre", "other", b} {
		if n := s.CountKeysInSlot(hashslot.Key(k)); n != 0 {
			t.Fatalf("%s: count after flush = %d", k, n)
		}
	}
}

func TestSlotIndexEviction(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1<<30, EvictionAllKeysRandom, 80, 90, 5)
	s.EnableSlotIndex()
	_ = s.Set("victim", &StringValue{Data: []byte("x")}, SetOptions{})
	if s.Evictor().ForceEvict(1) != 1 {
		t.Fatal("nothing evicted")
	}
	if n := s.CountKeysInSlot(hashslot.Key("victim")); n != 0 {
		t.Fatalf("count after eviction = %d", n)
	}
}

func TestSlotIndexReplaceFrom(t *testing.T) {
	base := NewStore()
	base.EnableSlotIndex()
	dbs := NewDatabases(base, 1)
	_ = base.Set("old", &StringValue{Data: []byte("x")}, SetOptions{})
	staging := dbs.NewStaging()
	if !staging.DB(0).SlotIndexed() {
		t.Fatal("staging store has no slot index")
	}
	_ = staging.DB(0).Set("new", &StringValue{Data: []byte("y")}, SetOptions{})
	if err := dbs.ReplaceFrom(staging); err != nil {
		t.Fatal(err)
	}
	if base.CountKeysInSlot(hashslot.Key("old")) != 0 || base.CountKeysInSlot(hashslot.Key("new")) != 1 {
		t.Fatal("full-sync load left the index stale")
	}
	if staging.DB(0).CountKeysInSlot(hashslot.Key("new")) != 0 {
		t.Fatal("the drained staging store still indexes the key")
	}
	if NewStore().CountKeysInSlot(hashslot.Key("new")) != 0 {
		t.Fatal("unindexed store")
	}
}
