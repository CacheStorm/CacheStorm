package store

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func stringValuePlan(data []byte, length int) ValuePlan {
	return ValuePlan{Size: int64(length) + 24, Apply: func() Value { next := make([]byte, length); copy(next, data); return &StringValue{Data: next} }}
}

func TestValuePlanRefusesBeforeAllocationAndPreservesMetadata(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(4096, EvictionNoEviction, 70, 90, 5)
	deadline := time.Now().Add(time.Minute).UnixMilli()
	if err := s.Set("k", &StringValue{Data: []byte("kept")}, SetOptions{ExpireAtMS: deadline, Tags: []string{"tag"}}); err != nil {
		t.Fatal(err)
	}
	usage := s.MemoryTracker().Usage()
	version := s.WatchKey("k")
	defer s.UnwatchKey("k")
	applied := false
	err := s.ApplyValuePlan("k", func(Value) (ValuePlan, error) {
		return ValuePlan{Size: 8192, Apply: func() Value { applied = true; return &StringValue{Data: make([]byte, 8168)} }}, nil
	})
	if !errors.Is(err, ErrMemoryLimit) || applied {
		t.Fatalf("error=%v allocation=%v", err, applied)
	}
	got, _ := s.Get("k")
	if got.Value.String() != "kept" || got.ExpiresAt.Load() != deadline || len(got.TagList()) != 1 || s.GetVersion("k") != version || s.MemoryTracker().Usage() != usage || s.MemUsage() != usage {
		t.Fatal("refused plan changed key or accounting")
	}
}

func TestConcurrentValuePlansReserveSharedBudget(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(4096, EvictionNoEviction, 70, 90, 5)
	var wg sync.WaitGroup
	var accepted, applied atomic.Int64
	start := make(chan struct{})
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			err := s.ApplyValuePlan(fmt.Sprintf("k%d", i), func(Value) (ValuePlan, error) {
				return ValuePlan{Size: 280, Apply: func() Value { applied.Add(1); return &StringValue{Data: make([]byte, 256)} }}, nil
			})
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, ErrMemoryLimit) {
				t.Errorf("plan: %v", err)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	if accepted.Load() == 0 || accepted.Load() == 32 || applied.Load() != accepted.Load() {
		t.Fatalf("accepted=%d applied=%d", accepted.Load(), applied.Load())
	}
	if s.MemoryTracker().Usage() != s.MemUsage() || !s.MemoryTracker().withinLimit(s.MemUsage()) {
		t.Fatalf("usage=%d shards=%d", s.MemoryTracker().Usage(), s.MemUsage())
	}
	s.Flush()
	if s.MemoryTracker().Usage() != 0 {
		t.Fatal("reservation leaked")
	}
}

func TestValuePlanEvictionRetriesPreservingLiveMetadata(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionAllKeysLRU, 70, 90, 128)
	if err := s.Set("victim", &StringValue{Data: make([]byte, 200)}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Minute).UnixMilli()
	if err := s.Set("target", &StringValue{Data: []byte("kept")}, SetOptions{ExpireAtMS: deadline, Tags: []string{"t"}}); err != nil {
		t.Fatal(err)
	}
	victim, _ := s.Get("victim")
	victim.SetIdleSeconds(1 << 30)
	var preparations int
	if err := s.ApplyValuePlan("target", func(value Value) (ValuePlan, error) {
		preparations++
		return stringValuePlan([]byte(value.String()), 700), nil
	}); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Get("target")
	if !ok || !bytes.HasPrefix([]byte(got.Value.String()), []byte("kept")) || got.ExpiresAt.Load() != deadline || len(got.TagList()) != 1 || s.Exists("victim") || preparations < 2 {
		t.Fatal("eviction/retry lost live state")
	}
	if s.MemoryTracker().Usage() != s.MemUsage() {
		t.Fatal("retry accounted reservation twice")
	}
}

func TestValuePlanLoadingBypassesAdmissionWithoutLosingAccounting(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1, EvictionNoEviction, 70, 90, 5)
	s.BeginLoading()
	if err := s.ApplyValuePlan("k", func(Value) (ValuePlan, error) { return stringValuePlan(nil, 128), nil }); err != nil {
		t.Fatal(err)
	}
	s.EndLoading()
	if !s.Exists("k") || s.MemoryTracker().Usage() != s.MemUsage() || s.MemUsage() <= 1 {
		t.Fatal("loading did not account restored data")
	}
}
