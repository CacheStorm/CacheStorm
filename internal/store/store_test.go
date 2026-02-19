package store

import (
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	s := NewStore()
	if s == nil {
		t.Fatal("expected store, got nil")
	}
}

func TestSetGet(t *testing.T) {
	s := NewStore()

	err := s.Set("foo", &StringValue{Data: []byte("bar")}, SetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry, exists := s.Get("foo")
	if !exists {
		t.Fatal("expected key to exist")
	}

	strVal, ok := entry.Value.(*StringValue)
	if !ok {
		t.Fatal("expected StringValue")
	}

	if string(strVal.Data) != "bar" {
		t.Errorf("expected 'bar', got '%s'", string(strVal.Data))
	}
}

func TestGetNonExistent(t *testing.T) {
	s := NewStore()

	_, exists := s.Get("nonexistent")
	if exists {
		t.Error("expected key not to exist")
	}
}

func TestDelete(t *testing.T) {
	s := NewStore()

	s.Set("foo", &StringValue{Data: []byte("bar")}, SetOptions{})

	if !s.Delete("foo") {
		t.Error("expected delete to return true")
	}

	if s.Exists("foo") {
		t.Error("expected key to be deleted")
	}
}

func TestExists(t *testing.T) {
	s := NewStore()

	if s.Exists("foo") {
		t.Error("expected key not to exist")
	}

	s.Set("foo", &StringValue{Data: []byte("bar")}, SetOptions{})

	if !s.Exists("foo") {
		t.Error("expected key to exist")
	}
}

func TestSetNX(t *testing.T) {
	s := NewStore()

	err := s.Set("foo", &StringValue{Data: []byte("bar")}, SetOptions{NX: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = s.Set("foo", &StringValue{Data: []byte("baz")}, SetOptions{NX: true})
	if err != ErrKeyExists {
		t.Errorf("expected ErrKeyExists, got %v", err)
	}
}

func TestSetXX(t *testing.T) {
	s := NewStore()

	err := s.Set("foo", &StringValue{Data: []byte("bar")}, SetOptions{XX: true})
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}

	s.Set("foo", &StringValue{Data: []byte("bar")}, SetOptions{})

	err = s.Set("foo", &StringValue{Data: []byte("baz")}, SetOptions{XX: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTTL(t *testing.T) {
	s := NewStore()

	ttl := s.TTL("nonexistent")
	if ttl != -2 {
		t.Errorf("expected -2 for nonexistent key, got %v", ttl)
	}

	s.Set("foo", &StringValue{Data: []byte("bar")}, SetOptions{})

	ttl = s.TTL("foo")
	if ttl != -1 {
		t.Errorf("expected -1 for key without TTL, got %v", ttl)
	}

	s.SetTTL("foo", 10*time.Second)

	ttl = s.TTL("foo")
	if ttl < 9*time.Second || ttl > 10*time.Second {
		t.Errorf("expected ~10s TTL, got %v", ttl)
	}
}

func TestSetWithTTL(t *testing.T) {
	s := NewStore()

	s.Set("foo", &StringValue{Data: []byte("bar")}, SetOptions{TTL: 100 * time.Millisecond})

	_, exists := s.Get("foo")
	if !exists {
		t.Fatal("expected key to exist")
	}

	time.Sleep(150 * time.Millisecond)

	_, exists = s.Get("foo")
	if exists {
		t.Error("expected key to be expired")
	}
}

func TestKeyCount(t *testing.T) {
	s := NewStore()

	if s.KeyCount() != 0 {
		t.Errorf("expected 0 keys, got %d", s.KeyCount())
	}

	s.Set("foo", &StringValue{Data: []byte("bar")}, SetOptions{})
	s.Set("baz", &StringValue{Data: []byte("qux")}, SetOptions{})

	if s.KeyCount() != 2 {
		t.Errorf("expected 2 keys, got %d", s.KeyCount())
	}
}

func TestFlush(t *testing.T) {
	s := NewStore()

	s.Set("foo", &StringValue{Data: []byte("bar")}, SetOptions{})
	s.Set("baz", &StringValue{Data: []byte("qux")}, SetOptions{})

	s.Flush()

	if s.KeyCount() != 0 {
		t.Errorf("expected 0 keys after flush, got %d", s.KeyCount())
	}
}

func TestShardDistribution(t *testing.T) {
	s := NewStore()

	keys := []string{"foo", "bar", "baz", "qux", "quux", "corge", "grault", "garply"}

	for _, key := range keys {
		s.Set(key, &StringValue{Data: []byte("value")}, SetOptions{})
	}

	for _, key := range keys {
		if !s.Exists(key) {
			t.Errorf("expected key '%s' to exist", key)
		}
	}
}
