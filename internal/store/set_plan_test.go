package store

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestSetPlanIdentityDuplicatesAndKeys(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
	set := &SetValue{Members: map[string]struct{}{"kept": {}}}
	if err := s.Set("s", set, SetOptions{Tags: []string{"tag"}, ExpireAtMS: 4102444800000}); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Get("s")
	version := s.WatchKey("s")
	defer s.UnwatchKey("s")
	if n, err := s.AddSetMembers("s", []string{"kept", "kept"}); n != 0 || err != nil || s.GetVersion("s") != version {
		t.Fatalf("duplicate: %d %v", n, err)
	}
	if n, err := s.AddSetMembers("s", []string{"new", "new", ""}); n != 2 || err != nil {
		t.Fatalf("add: %d %v", n, err)
	}
	after, _ := s.Get("s")
	if after != before || after.Value != set || after.ExpiresAt.Load() != 4102444800000 || len(after.TagList()) != 1 {
		t.Fatal("identity/metadata changed")
	}
	if n, err := s.AddSetMembers("missing", nil); n != 0 || err != nil || s.Exists("missing") {
		t.Fatalf("empty: %d %v", n, err)
	}
	for _, key := range []string{"", strings.Repeat("k", MaxKeySize+1)} {
		if _, err := s.AddSetMembers(key, []string{"m"}); err == nil {
			t.Fatal("invalid key accepted")
		}
	}
	if err := s.SetEntry("", NewEntry(&SetValue{Members: map[string]struct{}{"old": {}}})); err != nil {
		t.Fatal(err)
	}
	if n, err := s.AddSetMembers("", []string{"new"}); n != 1 || err != nil {
		t.Fatalf("existing empty key: %d %v", n, err)
	}
	requireBatchAccountingX(t, s)
}

func TestSetPlanSharedBudgetAndLoading(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(2048, EvictionNoEviction, 70, 90, 5)
	dbs := NewDatabases(s, 2)
	start := make(chan struct{})
	results := make(chan error, 16)
	for i := 0; i < 16; i++ {
		go func(i int) {
			<-start
			_, err := dbs.DB(i%2).AddSetMembers(fmt.Sprintf("s%d", i), []string{strings.Repeat("m", 600)})
			results <- err
		}(i)
	}
	close(start)
	wins := 0
	for i := 0; i < 16; i++ {
		if err := <-results; err == nil {
			wins++
		} else if !errors.Is(err, ErrMemoryLimit) {
			t.Fatal(err)
		}
	}
	total := dbs.DB(0).MemUsage() + dbs.DB(1).MemUsage()
	if wins != 2 || total != s.MemoryTracker().Usage() || !s.MemoryTracker().withinLimit(total) {
		t.Fatalf("wins=%d total=%d tracker=%d", wins, total, s.MemoryTracker().Usage())
	}
	s = NewStore()
	s.ConfigureMemory(1, EvictionNoEviction, 70, 90, 5)
	s.BeginLoading()
	if n, err := s.AddSetMembers("s", []string{strings.Repeat("m", 1000)}); n != 1 || err != nil {
		t.Fatalf("history: %d %v", n, err)
	}
	s.EndLoading()
	requireBatchAccountingX(t, s)
}

func TestSetMoveAdmissionMetadataAndDuplicateDestination(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(2048, EvictionNoEviction, 70, 90, 5)
	a := &SetValue{Members: map[string]struct{}{"m": {}, "stay": {}}}
	b := &SetValue{Members: map[string]struct{}{"other": {}}}
	for key, value := range map[string]*SetValue{"s": a, "d": b} {
		if err := s.Set(key, value, SetOptions{Tags: []string{key}, ExpireAtMS: 4102444800000}); err != nil {
			t.Fatal(err)
		}
	}
	before := s.MemUsage()
	src, _ := s.Get("s")
	dst, _ := s.Get("d")
	if ok, err := s.MoveSetMember("s", "d", "m"); !ok || err != nil {
		t.Fatalf("move: %v %v", ok, err)
	}
	afterS, _ := s.Get("s")
	afterD, _ := s.Get("d")
	if afterS != src || afterD != dst || afterS.Value != a || afterD.Value != b || s.MemUsage() != before || afterD.ExpiresAt.Load() != 4102444800000 || len(afterD.TagList()) != 1 {
		t.Fatal("net accounting/identity/metadata mismatch")
	}
	if _, err := s.AddSetMembers("s", []string{"m"}); err != nil {
		t.Fatal(err)
	}
	version := s.WatchKey("d")
	defer s.UnwatchKey("d")
	if ok, err := s.MoveSetMember("s", "d", "m"); !ok || err != nil || s.GetVersion("d") != version {
		t.Fatalf("duplicate dest: %v %v", ok, err)
	}
	if ok, err := s.MoveSetMember("d", "d", "m"); !ok || err != nil || s.GetVersion("d") != version {
		t.Fatalf("self: %v %v", ok, err)
	}
	if ok, err := s.MoveSetMember("s", "missing", "absent"); ok || err != nil || s.Exists("missing") {
		t.Fatalf("absent: %v %v", ok, err)
	}
	requireBatchAccountingX(t, s)
}

func TestSetMoveRejectedGrowthAndSourceCredit(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
	member := strings.Repeat("m", 700)
	if err := s.Set("s", &SetValue{Members: map[string]struct{}{member: {}}}, SetOptions{Tags: []string{"tag"}, ExpireAtMS: 4102444800000}); err != nil {
		t.Fatal(err)
	}
	before := s.MemUsage()
	version := s.WatchKey("s")
	defer s.UnwatchKey("s")
	if ok, err := s.MoveSetMember("s", strings.Repeat("d", 200), member); ok || !errors.Is(err, ErrMemoryLimit) || !s.Exists("s") || s.MemUsage() != before || s.GetVersion("s") != version {
		t.Fatalf("reject: %v %v usage=%d", ok, err, s.MemUsage())
	}
	if ok, err := s.MoveSetMember("s", "d", member); !ok || err != nil {
		t.Fatalf("credit: %v %v", ok, err)
	}
	e, _ := s.Get("d")
	if s.Exists("s") || e.ExpiresAt.Load() != 0 || len(e.TagList()) != 0 || len(s.GetTagIndex().GetKeys("tag")) != 0 {
		t.Fatal("source metadata carried/dangling")
	}
	requireBatchAccountingX(t, s)
}

func TestSetMoveEvictionReleasesLocksAndReselects(t *testing.T) {
	for _, remove := range []bool{false, true} {
		s := NewStore()
		s.ConfigureMemory(1100, EvictionAllKeysLRU, 70, 90, 128)
		member := strings.Repeat("m", 500)
		if _, err := s.AddSetMembers("s", []string{member, "n"}); err != nil {
			t.Fatal(err)
		}
		if err := s.Set("victim", &StringValue{Data: make([]byte, 100)}, SetOptions{}); err != nil {
			t.Fatal(err)
		}
		victim, _ := s.Get("victim")
		victim.LastAccess.Store(0)
		calls := 0
		s.SetHooks(StoreHooks{OnEvict: func(key string, _ interface{}) {
			calls++
			if key != "victim" {
				t.Errorf("victim=%s", key)
			}
			if ok, err := s.MoveSetMember("absent", "unused", "m"); ok || err != nil {
				t.Errorf("reentry=%v %v", ok, err)
			}
			if remove {
				s.Delete("s")
			} else if _, err := s.AddSetMembers("s", []string{"fromcallback"}); err != nil {
				t.Error(err)
			}
		}})
		ok, err := s.MoveSetMember("s", "destination-123456789", member)
		if err != nil || ok == remove || calls != 1 || s.Exists("victim") {
			t.Fatalf("move=%v error=%v evictions=%d", ok, err, calls)
		}
		if remove && s.Exists("destination-123456789") {
			t.Fatal("stale source published")
		}
		if !remove {
			e, _ := s.Get("s")
			if _, found := e.Value.(*SetValue).Members["fromcallback"]; !found {
				t.Fatal("callback field lost")
			}
		}
		requireBatchAccountingX(t, s)
	}
}

func TestSetMoveOppositeDirectionsAccounting(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(4096, EvictionNoEviction, 70, 90, 5)
	for key, member := range map[string]string{"a": "m", "b": "n"} {
		if _, err := s.AddSetMembers(key, []string{member}); err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for member, source := range map[string]string{"m": "a", "n": "b"} {
		wg.Add(1)
		go func(member, source string) {
			defer wg.Done()
			target := "a"
			if source == "a" {
				target = "b"
			}
			<-start
			for i := 0; i < 100; i++ {
				if ok, err := s.MoveSetMember(source, target, member); !ok || err != nil {
					t.Errorf("move=%v %v", ok, err)
					return
				}
				source, target = target, source
			}
		}(member, source)
	}
	close(start)
	wg.Wait()
	total := 0
	for _, key := range []string{"a", "b"} {
		if e, ok := s.Get(key); ok {
			total += len(e.Value.(*SetValue).Members)
		}
	}
	if total != 2 {
		t.Fatalf("lost/duplicated token count=%d", total)
	}
	requireBatchAccountingX(t, s)
}

func TestSetMoveExpiredSourceInvalidatesWatch(t *testing.T) {
	s := NewStore()
	if _, err := s.AddSetMembers("s", []string{"m"}); err != nil {
		t.Fatal(err)
	}
	e, _ := s.Get("s")
	version := s.WatchKey("s")
	defer s.UnwatchKey("s")
	e.ExpiresAt.Store(1)
	if ok, err := s.MoveSetMember("s", "d", "m"); ok || err != nil {
		t.Fatalf("expired move=%v %v", ok, err)
	}
	if s.GetVersion("s") == version {
		t.Fatal("expired source lookup did not invalidate WATCH")
	}
	if _, ok := s.GetShard("s").Get("s"); ok {
		t.Fatal("expired source was not reaped")
	}
}

func TestSetMoveErrorsReleaseLocks(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
	if _, err := s.AddSetMembers("s", []string{"m"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("wrong", &StringValue{Data: []byte("value")}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		src, dst, member string
		want             error
	}{
		{"wrong", "d", "m", ErrWrongType}, {"s", "wrong", "m", ErrWrongType}, {"s", "wrong", "absent", ErrWrongType},
		{"missing", "wrong", "m", nil}, {"s", "", "m", ErrInvalidKey}, {"s", strings.Repeat("x", MaxKeySize+1), "m", ErrKeyTooLarge},
	} {
		if ok, err := s.MoveSetMember(tc.src, tc.dst, tc.member); ok || !errors.Is(err, tc.want) {
			t.Fatalf("%s -> %s: %v %v", tc.src, tc.dst, ok, err)
		}
	}
	e, _ := s.Get("s")
	set := e.Value.(*SetValue)
	if !set.mu.TryLock() {
		t.Fatal("set lock leaked")
	}
	set.mu.Unlock()
	if !s.setOpsMu.TryLock() {
		t.Fatal("set operation lock leaked")
	}
	s.setOpsMu.Unlock()
	if _, ok := set.Members["m"]; !ok {
		t.Fatal("source changed")
	}
	if err := s.Set("alias", set, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.MoveSetMember("s", "alias", "m"); ok || !errors.Is(err, ErrWrongType) {
		t.Fatalf("aliased ownership=%v %v", ok, err)
	}
	requireBatchAccountingX(t, s)
}

func TestSetMoveLoadingAndPassiveExpiry(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1, EvictionNoEviction, 70, 90, 5)
	s.BeginLoading()
	if _, err := s.AddSetMembers("s", []string{strings.Repeat("m", 700)}); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.MoveSetMember("s", "d", strings.Repeat("m", 700)); !ok || err != nil {
		t.Fatalf("history move=%v %v", ok, err)
	}
	s.EndLoading()
	requireBatchAccountingX(t, s)
	e, _ := s.Get("d")
	s.passiveExpiry.Store(true)
	version := s.WatchKey("d")
	defer s.UnwatchKey("d")
	e.ExpiresAt.Store(1)
	if ok, err := s.MoveSetMember("d", "e", strings.Repeat("m", 700)); ok || err != nil || s.GetVersion("d") != version {
		t.Fatalf("passive=%v %v", ok, err)
	}
	if _, ok := s.GetShard("d").Get("d"); !ok {
		t.Fatal("passive physical entry was removed")
	}
}
