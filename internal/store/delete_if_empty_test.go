package store

import (
	"sync"
	"testing"
	"time"
)

func TestDeleteIfEmptyOnlyRemovesTheSameEmptyValue(t *testing.T) {
	s := NewStore()
	l := &ListValue{Elements: [][]byte{[]byte("a")}}
	if err := s.Set("k", l, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	if s.DeleteIfEmpty("k", l) {
		t.Fatal("a non-empty value was deleted")
	}

	l.Lock()
	l.Elements = nil
	l.Unlock()
	other := &ListValue{Elements: [][]byte{[]byte("b")}}
	if err := s.Set("k", other, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	if s.DeleteIfEmpty("k", l) {
		t.Fatal("deleted a key that no longer holds the emptied value")
	}
	if e, ok := s.Get("k"); !ok || e.Value != other {
		t.Fatal("the replacing value is gone")
	}

	other.Lock()
	other.Elements = nil
	other.Unlock()
	watched := s.WatchKey("k")
	if !s.DeleteIfEmpty("k", other) {
		t.Fatal("the emptied value was not deleted")
	}
	other.RLock()
	dead := other.IsDead()
	other.RUnlock()
	if !dead {
		t.Fatal("the deleted value is not marked dead")
	}
	if _, ok := s.Get("k"); ok {
		t.Fatal("the key survived")
	}
	if s.KeyCount() != 0 {
		t.Fatalf("keyCount %d after the only key was deleted", s.KeyCount())
	}
	if s.GetVersion("k") == watched {
		t.Fatal("the deletion did not touch the watched key")
	}
	if s.DeleteIfEmpty("k", other) {
		t.Fatal("deleted twice")
	}
	if s.DeleteIfEmpty("missing", &StringValue{}) {
		t.Fatal("a non-collection was deleted")
	}
}

func TestGetOrCreate(t *testing.T) {
	s := NewStore()
	watched := s.WatchKey("k")
	v, created, err := s.GetOrCreate("k", func() Value { return &HashValue{Fields: map[string][]byte{}} })
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if s.GetVersion("k") == watched {
		t.Fatal("creation did not touch the watched key")
	}
	again, created, err := s.GetOrCreate("k", func() Value { t.Fatal("create called for an existing key"); return nil })
	if err != nil || created || again != v {
		t.Fatalf("second call: created=%v err=%v same=%v", created, err, again == v)
	}
	if s.KeyCount() != 1 {
		t.Fatalf("keyCount %d", s.KeyCount())
	}

	if err := s.Set("exp", &StringValue{Data: []byte("x")}, SetOptions{TTL: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	fresh, created, err := s.GetOrCreate("exp", func() Value { return &SetValue{Members: map[string]struct{}{}} })
	if err != nil || !created {
		t.Fatalf("an expired key was not treated as absent: created=%v err=%v", created, err)
	}
	if _, ok := fresh.(*SetValue); !ok {
		t.Fatalf("got %T", fresh)
	}
	if s.KeyCount() != 2 {
		t.Fatalf("keyCount %d", s.KeyCount())
	}

	if _, _, err := s.GetOrCreate("", func() Value { return &SetValue{Members: map[string]struct{}{}} }); err != ErrInvalidKey {
		t.Fatalf("empty key: %v", err)
	}
}

func TestGetOrCreateConcurrentCreatorsShareOneValue(t *testing.T) {
	for round := 0; round < 100; round++ {
		s := NewStore()
		const n = 8
		got := make([]Value, n)
		var created sync.WaitGroup
		start := make(chan struct{})
		var mu sync.Mutex
		creations := 0
		for i := 0; i < n; i++ {
			created.Add(1)
			go func(i int) {
				defer created.Done()
				<-start
				v, c, err := s.GetOrCreate("k", func() Value { return &ListValue{} })
				if err != nil {
					t.Error(err)
				}
				if c {
					mu.Lock()
					creations++
					mu.Unlock()
				}
				got[i] = v
			}(i)
		}
		close(start)
		created.Wait()
		if creations != 1 {
			t.Fatalf("round %d: %d creations", round, creations)
		}
		for i := 1; i < n; i++ {
			if got[i] != got[0] {
				t.Fatalf("round %d: callers got different values", round)
			}
		}
	}
}
