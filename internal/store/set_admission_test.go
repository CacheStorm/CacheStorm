package store

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSetAdmissionCreditsReplacement(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
	if err := s.Set("key", &StringValue{Data: make([]byte, 500)}, SetOptions{}); err != nil {
		t.Fatalf("control: %v", err)
	}
	before := s.MemUsage()
	for _, size := range []int{500, 450, 0} {
		if err := s.Set("key", &StringValue{Data: make([]byte, size)}, SetOptions{}); err != nil {
			t.Fatalf("EXPECTED: replacement admitted | ACTUAL: %v", err)
		}
		if s.MemUsage() > before || s.MemoryTracker().Usage() != s.MemUsage() {
			t.Fatal("replacement accounting mismatch")
		}
	}
}

func TestSetAdmissionIncludesKeysAndTags(t *testing.T) {
	for _, mode := range []string{"key", "tags", "create"} {
		t.Run(mode, func(t *testing.T) {
			s := NewStore()
			s.ConfigureMemory(1024, EvictionNoEviction, 70, 90, 5)
			if err := s.Set("control", &StringValue{Data: []byte("small")}, SetOptions{}); err != nil {
				t.Fatalf("control: %v", err)
			}
			before := s.MemUsage()
			key := strings.Repeat("k", 1024)
			var err error
			switch mode {
			case "tags":
				key = "tagged"
				err = s.Set(key, &StringValue{Data: []byte("small")}, SetOptions{Tags: []string{strings.Repeat("t", 1024)}})
			case "create":
				_, _, err = s.GetOrCreate(key, func() Value { return &HashValue{Fields: map[string][]byte{}} })
			default:
				err = s.Set(key, &StringValue{Data: []byte("small")}, SetOptions{})
			}
			if !errors.Is(err, ErrMemoryLimit) || s.Exists(key) || s.MemUsage() != before || s.MemoryTracker().Usage() != before {
				t.Fatalf("EXPECTED: rejected without mutation | ACTUAL: err=%v exists=%v before=%d after=%d", err, s.Exists(key), before, s.MemUsage())
			}
		})
	}
}

type admissionBarrierValue struct {
	tracker *MemoryTracker
	arrived chan<- struct{}
	release <-chan struct{}
	stop    <-chan struct{}
	calls   atomic.Int64
}

func (v *admissionBarrierValue) Type() DataType { return DataTypeString }
func (v *admissionBarrierValue) String() string { return "value" }
func (v *admissionBarrierValue) Clone() Value   { return &StringValue{Data: make([]byte, 2276)} }
func (v *admissionBarrierValue) SizeOf() int64 {
	if v.calls.Add(1) == 3 && v.tracker.Usage() == 0 {
		v.arrived <- struct{}{}
		select {
		case <-v.release:
		case <-v.stop:
		}
	}
	return 2300
}

func TestSetAndCreateReserveBeforePublication(t *testing.T) {
	for _, create := range []bool{false, true} {
		t.Run(fmt.Sprint(create), func(t *testing.T) {
			s := NewStore()
			s.ConfigureMemory(4096, EvictionNoEviction, 70, 90, 5)
			if s.shardIndex("a") == s.shardIndex("b") {
				t.Fatal("control: keys must use different shards")
			}
			arrived, release, stop, done := make(chan struct{}, 2), make(chan struct{}), make(chan struct{}), make(chan struct{})
			go func() {
				defer close(done)
				for i := 0; i < 2; i++ {
					select {
					case <-arrived:
					case <-stop:
						return
					}
				}
				close(release)
			}()
			defer func() { close(stop); <-done }()
			var accepted atomic.Int64
			var writers sync.WaitGroup
			for _, key := range []string{"a", "b"} {
				writers.Add(1)
				go func(key string) {
					defer writers.Done()
					value := &admissionBarrierValue{tracker: s.MemoryTracker(), arrived: arrived, release: release, stop: stop}
					var err error
					if create {
						_, _, err = s.GetOrCreate(key, func() Value { return value })
					} else {
						err = s.Set(key, value, SetOptions{})
					}
					if err == nil {
						accepted.Add(1)
					} else if !errors.Is(err, ErrMemoryLimit) {
						t.Errorf("unexpected admission error: %v", err)
					}
				}(key)
			}
			writers.Wait()
			if accepted.Load() != 1 || !s.MemoryTracker().withinLimit(s.MemUsage()) || s.MemoryTracker().Usage() != s.MemUsage() {
				t.Fatalf("EXPECTED: one admitted writer within budget | ACTUAL: accepted=%d usage=%d tracked=%d", accepted.Load(), s.MemUsage(), s.MemoryTracker().Usage())
			}
		})
	}
}

func TestSetKeepTTLDoesNotRetainExpiredDeadline(t *testing.T) {
	s := NewStore()
	if err := s.Set("key", &StringValue{Data: []byte("old")}, SetOptions{ExpireAtMS: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("key", &StringValue{Data: []byte("new")}, SetOptions{KeepTTL: true}); err != nil {
		t.Fatal(err)
	}
	entry, ok := s.Get("key")
	if !ok || entry.Value.String() != "new" || entry.ExpiresAt.Load() != 0 {
		t.Fatalf("EXPECTED: persistent replacement of logically absent key | ACTUAL: entry=%+v present=%v", entry, ok)
	}
}

func TestSetAdmissionEvictsForFinalDeltaAndKeepsMetadata(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionAllKeysLRU, 70, 90, 128)
	if err := s.Set("victim", &StringValue{Data: make([]byte, 200)}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Hour).UnixMilli()
	if err := s.Set("target", &StringValue{Data: []byte("old")}, SetOptions{ExpireAtMS: deadline, Tags: []string{"old"}}); err != nil {
		t.Fatal(err)
	}
	victim, _ := s.Get("victim")
	victim.LastAccess.Store(0)
	if err := s.Set("target", &StringValue{Data: make([]byte, 700)}, SetOptions{KeepTTL: true, Tags: []string{"new"}}); err != nil {
		t.Fatal(err)
	}
	entry, ok := s.Get("target")
	if !ok || s.Exists("victim") || entry.ExpiresAt.Load() != deadline || len(s.GetTagIndex().GetKeys("old")) != 0 || len(s.GetTagIndex().GetKeys("new")) != 1 || !s.MemoryTracker().withinLimit(s.MemUsage()) || s.MemoryTracker().Usage() != s.MemUsage() {
		t.Fatal("eviction/replacement lost metadata or memory accounting")
	}
}

func TestSetRejectedConditionsDoNotEvictOrChangeMetadata(t *testing.T) {
	s := NewStore()
	s.ConfigureMemory(1024, EvictionAllKeysLRU, 70, 90, 128)
	if err := s.Set("key", &StringValue{Data: []byte("original")}, SetOptions{Tags: []string{"kept"}}); err != nil {
		t.Fatal(err)
	}
	version := s.WatchKey("key")
	defer s.UnwatchKey("key")
	before := s.MemUsage()
	for _, tc := range []struct {
		key  string
		opts SetOptions
		err  error
	}{{"key", SetOptions{NX: true}, ErrKeyExists}, {"missing", SetOptions{XX: true}, ErrKeyNotFound}, {"key", SetOptions{Tags: []string{"rejected"}}, ErrMemoryLimit}} {
		if err := s.Set(tc.key, &StringValue{Data: make([]byte, 8192)}, tc.opts); !errors.Is(err, tc.err) {
			t.Fatalf("key=%s error=%v want=%v", tc.key, err, tc.err)
		}
	}
	entry, ok := s.Get("key")
	if !ok || entry.Value.String() != "original" || s.GetVersion("key") != version || s.MemUsage() != before || s.MemoryTracker().Usage() != before || len(s.GetTagIndex().GetKeys("kept")) != 1 || len(s.GetTagIndex().GetKeys("rejected")) != 0 {
		t.Fatal("rejection changed data, metadata, WATCH or accounting")
	}
}

func TestSetOwnsInputTags(t *testing.T) {
	s := NewStore()
	tags := []string{"original"}
	if err := s.Set("key", &StringValue{Data: []byte("value")}, SetOptions{Tags: tags}); err != nil {
		t.Fatal(err)
	}
	tags[0] = "caller-changed"
	entry, ok := s.Get("key")
	if !ok || len(entry.Tags) != 1 || entry.Tags[0] != "original" || len(s.GetTagIndex().GetKeys("original")) != 1 || len(s.GetTagIndex().GetKeys("caller-changed")) != 0 {
		t.Fatal("caller tag mutation changed stored metadata")
	}
}

func TestSetAdmissionSharesBudgetAcrossDatabasesAndPlans(t *testing.T) {
	base := NewStore()
	base.ConfigureMemory(4096, EvictionNoEviction, 70, 90, 5)
	dbs := NewDatabases(base, 8)
	var writers sync.WaitGroup
	var accepted atomic.Int64
	start := make(chan struct{})
	for i := 0; i < 64; i++ {
		writers.Add(1)
		go func(i int) {
			defer writers.Done()
			<-start
			s := dbs.DB(i % dbs.Count())
			key := fmt.Sprintf("key:%d", i)
			var err error
			switch i % 3 {
			case 0:
				err = s.Set(key, &StringValue{Data: make([]byte, 256)}, SetOptions{})
			case 1:
				_, _, err = s.GetOrCreate(key, func() Value { return &StringValue{Data: make([]byte, 256)} })
			default:
				err = s.ApplyValuePlan(key, func(Value) (ValuePlan, error) { return stringValuePlan(nil, 256), nil })
			}
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, ErrMemoryLimit) {
				t.Errorf("writer=%d error=%v", i, err)
			}
		}(i)
	}
	close(start)
	writers.Wait()
	if accepted.Load() == 0 || accepted.Load() == 64 || !base.MemoryTracker().withinLimit(dbs.MemUsage()) || base.MemoryTracker().Usage() != dbs.MemUsage() {
		t.Fatalf("accepted=%d dataset=%d tracked=%d", accepted.Load(), dbs.MemUsage(), base.MemoryTracker().Usage())
	}
	dbs.FlushAll()
	if base.MemoryTracker().Usage() != 0 {
		t.Fatal("reservation leaked across database flush")
	}
}

func TestGetOrCreateRacedPublicationTouchesExistingEntry(t *testing.T) {
	s := NewStore()
	entered, release := make(chan struct{}), make(chan struct{})
	result := make(chan struct {
		value   Value
		created bool
		err     error
	}, 1)
	go func() {
		v, created, err := s.GetOrCreate("key", func() Value {
			close(entered)
			<-release
			return &StringValue{Data: []byte("candidate")}
		})
		result <- struct {
			value   Value
			created bool
			err     error
		}{v, created, err}
	}()
	<-entered
	actual := &StringValue{Data: []byte("winner")}
	if err := s.Set("key", actual, SetOptions{}); err != nil {
		close(release)
		t.Fatal(err)
	}
	entry, _ := s.GetShard("key").Get("key")
	before := entry.AccessCount.Load()
	close(release)
	got := <-result
	if got.err != nil || got.created || got.value != actual || entry.AccessCount.Load() != before+1 {
		t.Fatalf("raced result=%+v accesses=%d want=%d", got, entry.AccessCount.Load(), before+1)
	}
}
