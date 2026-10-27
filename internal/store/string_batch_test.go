package store

import (
	"errors"
	"strings"
	"testing"
)

func requireBatchAccountingX(t *testing.T, s *Store) {
	t.Helper()
	if s.memTracker != nil && s.memTracker.Usage() != s.MemUsage() {
		t.Fatalf("tracker=%d store=%d", s.memTracker.Usage(), s.MemUsage())
	}
}

func TestStringBatchFinalFootprintX(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(2048, EvictionNoEviction, 70, 90, 5)
	for _, key := range []string{"a", "b"} {
		if err := s.Set(key, &StringValue{Data: make([]byte, 600)}, SetOptions{Tags: []string{"old"}, ExpireAtMS: 4102444800000}); err != nil {
			t.Fatal(err)
		}
	}
	before := s.MemUsage()
	if ok, err := s.SetStringBatch([]StringPair{{"a", make([]byte, 1200)}, {"b", nil}}, false); !ok || err != nil {
		t.Fatalf("neutral batch: %v %v", ok, err)
	}
	if s.MemUsage() >= before || len(s.GetTagIndex().GetKeys("old")) != 0 {
		t.Fatalf("tags/accounting not cleared: %d -> %d", before, s.MemUsage())
	}
	for _, key := range []string{"a", "b"} {
		e, ok := s.Get(key)
		if !ok || e.ExpiresAt.Load() != 0 || len(e.Tags) != 0 {
			t.Fatalf("replacement metadata: %s %+v", key, e)
		}
	}
	requireBatchAccountingX(t, s)
}

func TestStringBatchBudgetBoundaryAndFailureX(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
	if ok, err := s.SetStringBatch([]StringPair{{"a", make([]byte, 762)}, {"b", nil}}, false); !ok || err != nil {
		t.Fatalf("972-byte control: %v %v", ok, err)
	}
	version := s.WatchKey("a")
	defer s.UnwatchKey("a")
	before, _ := s.Get("a")
	for i := 0; i < 3; i++ {
		if ok, err := s.SetStringBatch([]StringPair{{"a", make([]byte, 763)}, {"b", nil}}, false); ok || !errors.Is(err, ErrMemoryLimit) {
			t.Fatalf("973-byte rejection: %v %v", ok, err)
		}
		entry, _ := s.Get("a")
		if entry != before || s.MemUsage() != 972 || s.GetVersion("a") != version {
			t.Fatal("failed batch changed source or WATCH")
		}
	}
	requireBatchAccountingX(t, s)
}

func TestStringBatchDuplicatesValidationAndNX(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
	if ok, err := s.SetStringBatch([]StringPair{{"a", make([]byte, 2048)}, {"a", []byte("last")}}, true); !ok || err != nil {
		t.Fatalf("last duplicate controls final footprint: %v %v", ok, err)
	}
	e, _ := s.Get("a")
	if e.Value.String() != "last" || s.MemUsage() != 109 {
		t.Fatalf("duplicate result: %v usage=%d", e.Value, s.MemUsage())
	}
	version := s.WatchKey("a")
	defer s.UnwatchKey("a")
	if ok, err := s.SetStringBatch([]StringPair{{"b", []byte("new")}, {"a", []byte("overwrite")}}, true); ok || err != nil || s.Exists("b") || s.GetVersion("a") != version {
		t.Fatalf("NX no-op: %v %v", ok, err)
	}
	for _, key := range []string{"", strings.Repeat("x", MaxKeySize+1)} {
		if ok, err := s.SetStringBatch([]StringPair{{"b", []byte("new")}, {key, nil}}, false); ok || err == nil || s.Exists("b") {
			t.Fatalf("late invalid key: %v %v", ok, err)
		}
	}
	if ok, err := s.SetStringBatch(nil, false); ok || err != nil {
		t.Fatalf("empty batch: %v %v", ok, err)
	}
	requireBatchAccountingX(t, s)
}

func TestStringBatchExpiredAndLoadingX(t *testing.T) {
	for _, loading := range []bool{false, true} {
		s := NewStore()
		s.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
		if err := s.Set("a", &StringValue{Data: []byte("old")}, SetOptions{ExpireAtMS: 1, Tags: []string{"expired"}}); err != nil {
			t.Fatal(err)
		}
		if loading {
			s.BeginLoading()
		}
		ok, err := s.SetStringBatch([]StringPair{{"a", make([]byte, 1200)}, {"b", nil}}, false)
		if loading {
			s.EndLoading()
			if !ok || err != nil || s.MemUsage() != 1410 || len(s.GetTagIndex().GetKeys("expired")) != 0 {
				t.Fatalf("historical load: %v %v usage=%d", ok, err, s.MemUsage())
			}
		} else if ok || !errors.Is(err, ErrMemoryLimit) || s.Exists("a") || s.Exists("b") {
			t.Fatalf("expired cleanup and rejection: %v %v", ok, err)
		}
		requireBatchAccountingX(t, s)
	}
}

func TestStringBatchConcurrentAdmissionX(t *testing.T) {
	for _, nx := range []bool{false, true} {
		s := NewStore()
		s.ConfigureMemory(2048, EvictionNoEviction, 70, 90, 5)
		start := make(chan struct{})
		type result struct {
			ok  bool
			err error
		}
		results := make(chan result, 2)
		for _, prefix := range []string{"a", "b"} {
			go func(prefix string) {
				<-start
				if nx {
					prefix = "a"
				}
				ok, err := s.SetStringBatch([]StringPair{{prefix + "1", make([]byte, 600)}, {prefix + "2", make([]byte, 600)}}, nx)
				results <- result{ok, err}
			}(prefix)
		}
		close(start)
		wins := 0
		for i := 0; i < 2; i++ {
			r := <-results
			if r.ok {
				wins++
			} else if nx && r.err != nil || !nx && !errors.Is(r.err, ErrMemoryLimit) {
				t.Fatalf("loser: %+v", r)
			}
		}
		if wins != 1 || s.KeyCount() != 2 || s.MemUsage() != 1412 {
			t.Fatalf("wins=%d keys=%d usage=%d", wins, s.KeyCount(), s.MemUsage())
		}
		requireBatchAccountingX(t, s)
	}
}

func TestStringBatchEvictionRechecksNX(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionAllKeysLRU, 70, 90, 128)
	if err := s.Set("victim", &StringValue{Data: make([]byte, 600)}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	evictions := 0
	s.SetHooks(StoreHooks{OnEvict: func(key string, _ interface{}) {
		evictions++
		if key != "victim" {
			t.Errorf("victim=%s", key)
		}
		if err := s.Set("a", &StringValue{Data: []byte("from callback")}, SetOptions{}); err != nil {
			t.Error(err)
		}
	}})
	if ok, err := s.SetStringBatch([]StringPair{{"a", make([]byte, 300)}, {"b", make([]byte, 300)}}, true); ok || err != nil || evictions != 1 || s.Exists("b") {
		t.Fatalf("retry: ok=%v err=%v evictions=%d", ok, err, evictions)
	}
	e, _ := s.Get("a")
	if e.Value.String() != "from callback" {
		t.Fatal("overwrote concurrent NX destination")
	}
	requireBatchAccountingX(t, s)
}
