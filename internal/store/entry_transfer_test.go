package store

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type transferCloneCounter struct{ clones atomic.Int64 }

func (v *transferCloneCounter) Type() DataType { return DataTypeString }
func (v *transferCloneCounter) String() string { return "value" }
func (v *transferCloneCounter) SizeOf() int64  { return 524 }
func (v *transferCloneCounter) Clone() Value {
	v.clones.Add(1)
	return &StringValue{Data: make([]byte, 500)}
}

func TestTransferReservesBeforeCloning(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
	v := &transferCloneCounter{}
	if err := s.Set("src", v, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	before := s.MemUsage()
	if changed, err := s.Transfer("src", s, "dst", TransferOptions{Copy: true}); changed || !errors.Is(err, ErrMemoryLimit) || v.clones.Load() != 0 || s.MemUsage() != before || s.Exists("dst") {
		t.Fatalf("changed=%v error=%v clones=%d footprint=%d", changed, err, v.clones.Load(), s.MemUsage())
	}
}

func TestTransferCreditsRemovedSourceAndPreservesMetadata(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1100, EvictionNoEviction, 70, 90, 5)
	deadline := time.Now().Add(time.Hour).UnixMilli()
	if err := s.Set("a", &StringValue{Data: make([]byte, 700)}, SetOptions{ExpireAtMS: deadline, Tags: []string{"tag"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("filler", &StringValue{Data: make([]byte, 20)}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	source, _ := s.Get("a")
	before := s.MemUsage()
	if changed, err := s.Transfer("a", s, "b", TransferOptions{Replace: true}); !changed || err != nil {
		t.Fatalf("changed=%v error=%v", changed, err)
	}
	dest, ok := s.Get("b")
	if !ok || s.Exists("a") || dest.Value != source.Value || dest.ExpiresAt.Load() != deadline || len(dest.TagList()) != 1 || dest.TagList()[0] != "tag" || s.MemUsage() != before || s.MemoryTracker().Usage() != before {
		t.Fatal("net-zero move changed footprint or metadata")
	}
	if keys := s.GetTagIndex().GetKeys("tag"); len(keys) != 1 || keys[0] != "b" {
		t.Fatalf("moved tag index=%v", keys)
	}
}

func TestTransferAcrossIndependentBudgetsRejectsBeforeRemovingSource(t *testing.T) {
	src, dst := NewStore(), NewStore()
	src.ConfigureMemory(4096, EvictionNoEviction, 70, 90, 5)
	dst.ConfigureMemory(256, EvictionNoEviction, 70, 90, 5)
	if err := src.Set("key", &StringValue{Data: make([]byte, 500)}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	before := src.MemUsage()
	if changed, err := src.Transfer("key", dst, "key", TransferOptions{}); changed || !errors.Is(err, ErrMemoryLimit) || !src.Exists("key") || dst.Exists("key") || src.MemUsage() != before || dst.MemUsage() != 0 {
		t.Fatalf("changed=%v error=%v src=%d dst=%d", changed, err, src.MemUsage(), dst.MemUsage())
	}
}

func TestTransferOppositeDirections(t *testing.T) {
	for _, shared := range []bool{false, true} {
		left := NewStore()
		left.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
		right := NewStore()
		right.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
		if shared {
			right = NewDatabases(left, 2).DB(1)
		}
		if err := left.Set("a", &StringValue{Data: []byte("token")}, SetOptions{}); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		var writers sync.WaitGroup
		for _, direction := range []bool{false, true} {
			writers.Add(1)
			go func(reverse bool) {
				defer writers.Done()
				<-start
				from, to, src, dst := left, right, "a", "b"
				if reverse {
					from, to, src, dst = right, left, "b", "a"
				}
				for i := 0; i < 200; i++ {
					if _, err := from.Transfer(src, to, dst, TransferOptions{}); err != nil && !errors.Is(err, ErrKeyNotFound) {
						t.Errorf("transfer: %v", err)
					}
				}
			}(direction)
		}
		close(start)
		writers.Wait()
		footprint := footprintOf("a", &StringValue{Data: []byte("token")})
		if left.KeyCount()+right.KeyCount() != 1 || left.MemUsage()+right.MemUsage() != footprint {
			t.Fatal("opposite transfers lost or duplicated the token")
		}
		if shared {
			if left.MemoryTracker().Usage() != footprint {
				t.Fatal("shared accounting mismatch")
			}
		} else if left.MemoryTracker().Usage() != left.MemUsage() || right.MemoryTracker().Usage() != right.MemUsage() {
			t.Fatal("independent accounting mismatch")
		}
	}
}

func TestSetEntryAdmissionAndSnapshotOwnership(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
	e := NewEntry(&StringValue{Data: []byte("kept")})
	e.SetTags([]string{"tag"})
	e.ExpiresAt.Store(time.Now().Add(time.Hour).UnixMilli())
	if err := s.SetEntry("key", e); err != nil {
		t.Fatal(err)
	}
	e.TagList()[0] = "caller"
	e.ExpiresAt.Store(1)
	stored, _ := s.Get("key")
	before := s.MemUsage()
	if stored.TagList()[0] != "tag" || stored.ExpiresAt.Load() == 1 {
		t.Fatal("metadata aliases caller entry")
	}
	large := NewEntry(&StringValue{Data: make([]byte, 2048)})
	if err := s.SetEntry("key", large); !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("oversized entry: %v", err)
	}
	if current, ok := s.Get("key"); !ok || current != stored || s.MemUsage() != before || s.MemoryTracker().Usage() != before {
		t.Fatal("rejected entry changed destination")
	}
	if err := s.SetEntry(strings.Repeat("x", MaxKeySize+1), e); !errors.Is(err, ErrKeyTooLarge) {
		t.Fatalf("oversized key: %v", err)
	}
}

func TestTransferCopyIsolationAndExpiredDestinations(t *testing.T) {
	s := NewStore()
	value := &HashValue{Fields: map[string][]byte{"field": []byte("v")}}
	if err := s.Set("src", value, SetOptions{ExpireAtMS: 4102444800000, Tags: []string{"source"}}); err != nil {
		t.Fatal(err)
	}
	dead := NewEntry(&StringValue{Data: []byte("old")})
	dead.ExpiresAt.Store(1)
	dead.SetTags([]string{"expired"})
	if err := s.SetEntry("dst", dead); err != nil {
		t.Fatal(err)
	}
	if changed, err := s.Transfer("src", s, "dst", TransferOptions{Copy: true}); !changed || err != nil {
		t.Fatalf("copy=%v error=%v", changed, err)
	}
	dest, _ := s.Get("dst")
	copyValue := dest.Value.(*HashValue)
	copyValue.Lock()
	copyValue.SetField("field", []byte("d"))
	copyValue.Unlock()
	if string(value.Fields["field"]) != "v" || dest.ExpiresAt.Load() != 4102444800000 || len(s.GetTagIndex().GetKeys("expired")) != 0 {
		t.Fatal("copy aliases source or expired destination metadata")
	}
	if changed, err := s.Transfer("src", s, "dst", TransferOptions{Copy: true}); changed || err != nil {
		t.Fatalf("destination conflict=%v error=%v", changed, err)
	}
	if changed, err := s.Transfer("src", s, "src", TransferOptions{}); changed || err != nil {
		t.Fatalf("self transfer=%v error=%v", changed, err)
	}
	if changed, err := s.Transfer("missing", s, "dst", TransferOptions{}); changed || !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("missing source=%v error=%v", changed, err)
	}
}

func TestTransferDuringHistoryLoadingRetainsAccounting(t *testing.T) {
	base := NewStore()
	base.ConfigureMemory(1, EvictionNoEviction, 70, 90, 5)
	dbs := NewDatabases(base, 2)
	base.BeginLoading()
	if err := base.Set("src", &StringValue{Data: make([]byte, 500)}, SetOptions{ExpireAtMS: 1}); err != nil {
		t.Fatal(err)
	}
	dst := dbs.DB(1)
	if changed, err := base.Transfer("src", dst, "copy", TransferOptions{Copy: true}); !changed || err != nil {
		t.Fatalf("loading copy=%v error=%v", changed, err)
	}
	if changed, err := base.Transfer("src", dst, "moved", TransferOptions{}); !changed || err != nil {
		t.Fatalf("loading move=%v error=%v", changed, err)
	}
	if !dst.Persist("copy") || !dst.Persist("moved") {
		t.Fatal("loading lost expired physical history")
	}
	dbs.ForEach(func(_ int, st *Store) { st.EndLoading() })
	if base.KeyCount() != 0 || dst.KeyCount() != 2 || base.MemoryTracker().Usage() != dbs.MemUsage() {
		t.Fatal("loading transfer lost data or accounting")
	}
}

func TestTransferImpossibleCopyDoesNotEvictSource(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionAllKeysLRU, 70, 90, 128)
	if err := s.Set("src", &StringValue{Data: make([]byte, 500)}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	before := s.MemUsage()
	changed, err := s.Transfer("src", s, "dst", TransferOptions{Copy: true})
	if changed || !errors.Is(err, ErrMemoryLimit) || !s.Exists("src") || s.Exists("dst") || s.MemUsage() != before {
		t.Fatalf("impossible copy changed=%v error=%v src=%v usage=%d", changed, err, s.Exists("src"), s.MemUsage())
	}
}

func TestTransferEvictionRetriesOutsideEntryLocks(t *testing.T) {
	for _, dropSource := range []bool{false, true} {
		s := NewStore()
		s.ConfigureMemory(1024, EvictionAllKeysLRU, 70, 90, 128)
		for _, tc := range []struct {
			key  string
			size int
		}{{"src", 200}, {"victim", 300}} {
			if err := s.Set(tc.key, &StringValue{Data: make([]byte, tc.size)}, SetOptions{}); err != nil {
				t.Fatal(err)
			}
		}
		victim, _ := s.Get("victim")
		victim.SetIdleSeconds(1 << 30)
		var evictions int
		s.SetHooks(StoreHooks{OnEvict: func(key string, _ interface{}) {
			evictions++
			if key != "victim" {
				t.Errorf("unexpected eviction: %s", key)
			}
			if _, ok := s.Get("src"); !ok {
				t.Error("eviction lost source before callback")
			}
			if dropSource {
				s.Delete("src")
			}
		}})
		changed, err := s.Transfer("src", s, "dst", TransferOptions{Copy: true})
		if dropSource {
			if changed || !errors.Is(err, ErrKeyNotFound) || s.Exists("dst") {
				t.Fatalf("stale source published after callback: changed=%v error=%v", changed, err)
			}
		} else if !changed || err != nil || !s.Exists("src") || !s.Exists("dst") {
			t.Fatalf("eviction retry: changed=%v error=%v", changed, err)
		}
		if evictions != 1 || s.MemoryTracker().Usage() != s.MemUsage() || !s.MemoryTracker().withinLimit(s.MemUsage()) {
			t.Fatal("eviction retry leaked reservation or exceeded budget")
		}
	}
}
