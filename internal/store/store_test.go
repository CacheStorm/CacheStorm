package store

import (
	"testing"
	"time"
)

func TestStoreSetGet(t *testing.T) {
	s := NewStore()

	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{})

	entry, exists := s.Get("key1")
	if !exists {
		t.Fatal("key1 should exist")
	}

	if string(entry.Value.(*StringValue).Data) != "value1" {
		t.Errorf("expected value1, got %s", entry.Value.(*StringValue).Data)
	}
}

func TestStoreDelete(t *testing.T) {
	s := NewStore()

	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{})

	if !s.Delete("key1") {
		t.Error("delete should return true")
	}

	if _, exists := s.Get("key1"); exists {
		t.Error("key1 should not exist after delete")
	}

	if s.Delete("nonexistent") {
		t.Error("deleting nonexistent key should return false")
	}
}

func TestStoreExists(t *testing.T) {
	s := NewStore()

	if s.Exists("key1") {
		t.Error("nonexistent key should not exist")
	}

	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{})

	if !s.Exists("key1") {
		t.Error("key1 should exist")
	}
}

func TestStoreTTL(t *testing.T) {
	s := NewStore()

	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{TTL: 5 * time.Second})

	entry, exists := s.Get("key1")
	if !exists {
		t.Fatal("key1 should exist")
	}

	if entry.TTL() <= 0 || entry.TTL() > 5*time.Second {
		t.Errorf("TTL should be between 0 and 5 seconds, got %v", entry.TTL())
	}
}

func TestStoreExpiration(t *testing.T) {
	s := NewStore()

	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{TTL: 100 * time.Millisecond})

	time.Sleep(150 * time.Millisecond)

	_, exists := s.Get("key1")
	if exists {
		t.Error("key1 should be expired")
	}
}

func TestStoreSetNX(t *testing.T) {
	s := NewStore()

	err := s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{NX: true})
	if err != nil {
		t.Errorf("NX set should succeed: %v", err)
	}

	err = s.Set("key1", &StringValue{Data: []byte("value2")}, SetOptions{NX: true})
	if err != ErrKeyExists {
		t.Errorf("NX set on existing key should fail, got: %v", err)
	}
}

func TestStoreSetXX(t *testing.T) {
	s := NewStore()

	err := s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{XX: true})
	if err != ErrKeyNotFound {
		t.Errorf("XX set on nonexistent key should fail, got: %v", err)
	}

	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{})

	err = s.Set("key1", &StringValue{Data: []byte("value2")}, SetOptions{XX: true})
	if err != nil {
		t.Errorf("XX set on existing key should succeed: %v", err)
	}
}

func TestStoreFlush(t *testing.T) {
	s := NewStore()

	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{})
	s.Set("key2", &StringValue{Data: []byte("value2")}, SetOptions{})

	s.Flush()

	if s.KeyCount() != 0 {
		t.Errorf("key count should be 0 after flush, got %d", s.KeyCount())
	}
}

func TestHashOperations(t *testing.T) {
	s := NewStore()

	h := &HashValue{Fields: make(map[string][]byte)}
	h.Fields["field1"] = []byte("value1")
	s.Set("hash1", h, SetOptions{})

	entry, exists := s.Get("hash1")
	if !exists {
		t.Fatal("hash1 should exist")
	}

	hv := entry.Value.(*HashValue)
	if string(hv.Fields["field1"]) != "value1" {
		t.Errorf("expected value1, got %s", hv.Fields["field1"])
	}
}

func TestListOperations(t *testing.T) {
	s := NewStore()

	l := &ListValue{Elements: [][]byte{[]byte("item1"), []byte("item2")}}
	s.Set("list1", l, SetOptions{})

	entry, exists := s.Get("list1")
	if !exists {
		t.Fatal("list1 should exist")
	}

	lv := entry.Value.(*ListValue)
	if len(lv.Elements) != 2 {
		t.Errorf("expected 2 elements, got %d", len(lv.Elements))
	}
}

func TestSetOperations(t *testing.T) {
	s := NewStore()

	set := &SetValue{Members: make(map[string]struct{})}
	set.Members["member1"] = struct{}{}
	set.Members["member2"] = struct{}{}
	s.Set("set1", set, SetOptions{})

	entry, exists := s.Get("set1")
	if !exists {
		t.Fatal("set1 should exist")
	}

	sv := entry.Value.(*SetValue)
	if len(sv.Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(sv.Members))
	}
}

func TestSortedSetOperations(t *testing.T) {
	s := NewStore()

	zset := &SortedSetValue{Members: make(map[string]float64)}
	zset.Members["member1"] = 1.0
	zset.Members["member2"] = 2.0
	s.Set("zset1", zset, SetOptions{})

	entry, exists := s.Get("zset1")
	if !exists {
		t.Fatal("zset1 should exist")
	}

	zv := entry.Value.(*SortedSetValue)
	if len(zv.Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(zv.Members))
	}
}

func TestStoreMemoryUsage(t *testing.T) {
	s := NewStore()

	initial := s.MemUsage()

	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{})

	after := s.MemUsage()
	if after <= initial {
		t.Error("memory usage should increase after adding data")
	}
}

func TestStoreKeyCount(t *testing.T) {
	s := NewStore()

	if s.KeyCount() != 0 {
		t.Errorf("initial key count should be 0, got %d", s.KeyCount())
	}

	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{})
	s.Set("key2", &StringValue{Data: []byte("value2")}, SetOptions{})

	if s.KeyCount() != 2 {
		t.Errorf("key count should be 2, got %d", s.KeyCount())
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	s := NewStore()

	done := make(chan bool)

	for i := 0; i < 100; i++ {
		go func(n int) {
			key := string(rune('a' + n%10))
			s.Set(key, &StringValue{Data: []byte("value")}, SetOptions{})
			s.Get(key)
			s.Exists(key)
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestStoreGetAll(t *testing.T) {
	s := NewStore()

	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{})
	s.Set("key2", &StringValue{Data: []byte("value2")}, SetOptions{})
	s.Set("key3", &StringValue{Data: []byte("value3")}, SetOptions{})

	entries := s.GetAll()
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestStoreSetTTL(t *testing.T) {
	s := NewStore()

	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{})

	if !s.SetTTL("key1", 10*time.Second) {
		t.Error("SetTTL should return true for existing key")
	}

	entry, _ := s.Get("key1")
	if entry.TTL() <= 0 {
		t.Error("TTL should be set")
	}
}

func TestStorePersist(t *testing.T) {
	s := NewStore()

	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{TTL: 10 * time.Second})

	if !s.Persist("key1") {
		t.Error("Persist should return true for existing key with TTL")
	}

	entry, _ := s.Get("key1")
	if entry.TTL() != -1 {
		t.Error("TTL should be -1 after persist")
	}
}

func TestStoreSetExpiresAt(t *testing.T) {
	s := NewStore()

	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{})

	expiresAt := time.Now().Add(10 * time.Second).UnixNano()
	if !s.SetExpiresAt("key1", expiresAt) {
		t.Error("SetExpiresAt should return true")
	}

	entry, _ := s.Get("key1")
	if entry.TTL() <= 0 {
		t.Error("TTL should be set")
	}
}
