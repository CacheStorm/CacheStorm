package store

import (
	"strings"
	"testing"
)

// SizeOf of a collection is maintained incrementally, not recounted: after
// the first count, a change made behind the helpers' back is not seen.
func TestCollectionSizeIsNotRecounted(t *testing.T) {
	l := &ListValue{}
	for i := 0; i < 200_000; i++ {
		l.PushBack([]byte("x"))
	}
	want := int64(listBaseSize + 200_000*(1+listElemOverhead))
	if got := l.SizeOf(); got != want {
		t.Fatalf("SizeOf=%d, want %d", got, want)
	}
	l.Elements = append(l.Elements, []byte("hidden"))
	if got := l.SizeOf(); got != want {
		t.Fatalf("SizeOf walked the list: %d", got)
	}
}

func TestCollectionHelpersKeepSize(t *testing.T) {
	l := &ListValue{Elements: [][]byte{[]byte("a"), []byte("bb")}}
	l.SizeOf()
	l.PushFront([]byte("ccc"))
	l.PushBack([]byte("dddd"), nil)
	l.SetAt(1, []byte("eeeeee"))
	l.PopFront(1)
	l.PopBack(1)
	l.Trim(0, 1)
	if got, want := l.SizeOf(), listBaseSize+sumListElems(l.Elements); got != want {
		t.Fatalf("list SizeOf=%d, recount=%d", got, want)
	}

	h := &HashValue{Fields: map[string][]byte{"f": []byte("v")}}
	h.SizeOf()
	h.SetField("f", []byte("longer"))
	h.SetField("g", []byte("1"))
	h.DeleteField("f")
	h.DeleteField("missing")
	if got, want := h.SizeOf(), int64(hashBaseSize)+hashFieldSize("g", []byte("1")); got != want {
		t.Fatalf("hash SizeOf=%d, want %d", got, want)
	}

	s := &SetValue{Members: map[string]struct{}{"a": {}}}
	s.SizeOf()
	s.AddMember("bb")
	s.AddMember("bb")
	s.RemoveMember("a")
	if got, want := s.SizeOf(), int64(setBaseSize)+setMemberSize("bb"); got != want {
		t.Fatalf("set SizeOf=%d, want %d", got, want)
	}

	z := &SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}
	z.SizeOf()
	z.Add("a", 5)
	z.Add("dd", 1)
	z.RemoveRangeByScore(2, 3)
	if got, want := z.SizeOf(), int64(zsetBaseSize)+zsetMemberSize("a")+zsetMemberSize("dd"); got != want {
		t.Fatalf("zset SizeOf=%d, want %d", got, want)
	}
}

func footprintOf(key string, v Value) int64 { return 64 + v.SizeOf() + int64(len(key)) + 16 }

// Reaccount charges in-place growth to the shard and the tracker; Delete then
// credits exactly what was charged.
func TestReaccountChargesInPlaceGrowth(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1<<30, EvictionNoEviction, 70, 85, 5)
	l := &ListValue{}
	if err := s.Set("k", l, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	l.Lock()
	l.PushBack([]byte(strings.Repeat("x", 1000)))
	l.Unlock()
	if s.MemUsage() == footprintOf("k", l) {
		t.Fatal("growth was charged without Reaccount; the test proves nothing")
	}
	s.Reaccount("k", "missing")
	if got, want := s.MemUsage(), footprintOf("k", l); got != want {
		t.Fatalf("MemUsage=%d, want %d", got, want)
	}
	if got := s.MemoryTracker().Usage(); got != s.MemUsage() {
		t.Fatalf("tracker=%d, shards=%d", got, s.MemUsage())
	}

	// Growth not yet reaccounted when the key goes is not credited twice.
	l.Lock()
	l.PushBack([]byte("more"))
	l.Unlock()
	s.Delete("k")
	if s.MemUsage() != 0 || s.MemoryTracker().Usage() != 0 {
		t.Fatalf("after Delete: shards=%d tracker=%d", s.MemUsage(), s.MemoryTracker().Usage())
	}
}

// RENAME and MOVE install the entry still stored under the old key; each key
// must keep its own accounted footprint.
func TestSetEntryKeepsAccountingPerKey(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1<<30, EvictionNoEviction, 70, 85, 5)
	if err := s.Set("short", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	entry, _ := s.Get("short")
	if err := s.SetEntry("a-much-longer-key-name", entry); err != nil {
		t.Errorf("SetEntry: %v", err)
	}
	s.Delete("short")
	if got, want := s.MemUsage(), footprintOf("a-much-longer-key-name", entry.Value); got != want {
		t.Fatalf("MemUsage=%d, want %d", got, want)
	}
	if got := s.MemoryTracker().Usage(); got != s.MemUsage() {
		t.Fatalf("tracker=%d, shards=%d", got, s.MemUsage())
	}
}

func TestCheckWriteMemory(t *testing.T) {
	s := NewStore()
	if err := s.CheckWriteMemory(); err != nil {
		t.Fatalf("no limit: %v", err)
	}
	s.ConfigureMemory(4096, EvictionNoEviction, 70, 85, 5)
	l := &ListValue{}
	_ = s.Set("k", l, SetOptions{})
	l.Lock()
	l.PushBack([]byte(strings.Repeat("x", 8192)))
	l.Unlock()
	if err := s.CheckWriteMemory(); err != nil {
		t.Fatalf("growth not yet reaccounted is invisible: %v", err)
	}
	s.Reaccount("k")
	if err := s.CheckWriteMemory(); err != ErrMemoryLimit {
		t.Fatalf("over the limit: %v", err)
	}

	e := NewStore()
	e.ConfigureMemory(4096, EvictionAllKeysLRU, 70, 85, 5)
	big := &ListValue{}
	_ = e.Set("k", big, SetOptions{})
	big.Lock()
	big.PushBack([]byte(strings.Repeat("x", 8192)))
	big.Unlock()
	e.Reaccount("k")
	if err := e.CheckWriteMemory(); err != nil {
		t.Fatalf("eviction should have made room: %v", err)
	}
	if e.Exists("k") {
		t.Fatal("the key over the limit was not evicted")
	}
}
