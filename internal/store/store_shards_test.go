package store

import (
	"fmt"
	"testing"
)

func TestStoreShards(t *testing.T) {
	s := NewStore()

	t.Run("Shard Distribution", func(t *testing.T) {
		// Add keys to different shards
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key%d", i)
			s.Set(key, &StringValue{Data: []byte("value")}, SetOptions{})
		}

		// Verify all keys exist
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key%d", i)
			_, exists := s.Get(key)
			if !exists {
				t.Errorf("Key %s should exist", key)
			}
		}
	})

	t.Run("Shard Index Consistency", func(t *testing.T) {
		key := "testkey123"
		idx1 := s.shardIndex(key)
		idx2 := s.shardIndex(key)
		if idx1 != idx2 {
			t.Error("Shard index should be consistent for same key")
		}
	})
}

func TestStoreStats(t *testing.T) {
	s := NewStore()

	t.Run("Memory Stats", func(t *testing.T) {
		s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{})
		s.Set("key2", &StringValue{Data: []byte("value2")}, SetOptions{})
		// Stats functionality verified through KeyCount
		count := s.KeyCount()
		if count < 2 {
			t.Error("Should have at least 2 keys")
		}
	})

	t.Run("Key Count", func(t *testing.T) {
		initialCount := s.KeyCount()

		s.Set("countkey1", &StringValue{Data: []byte("v")}, SetOptions{})
		s.Set("countkey2", &StringValue{Data: []byte("v")}, SetOptions{})

		newCount := s.KeyCount()
		if newCount != initialCount+2 {
			t.Errorf("Expected key count %d, got %d", initialCount+2, newCount)
		}
	})
}

func TestStoreNamespace(t *testing.T) {
	s := NewStore()

	t.Run("Default Namespace", func(t *testing.T) {
		// Namespace manager test - just verify it doesn't panic
		_ = s.GetNamespaceManager()
	})
}

func TestStorePubSub(t *testing.T) {
	s := NewStore()

	t.Run("Get PubSub", func(t *testing.T) {
		ps := s.GetPubSub()
		if ps == nil {
			t.Fatal("PubSub should not be nil")
		}
	})
}

func TestStoreFlushAll(t *testing.T) {
	s := NewStore()

	t.Run("Flush All", func(t *testing.T) {
		// Add data
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("flushkey%d", i)
			s.Set(key, &StringValue{Data: []byte("value")}, SetOptions{})
		}

		// Verify data exists
		if s.KeyCount() == 0 {
			t.Error("Should have keys before flush")
		}

		// Flush
		s.Flush()

		// Verify all gone
		if s.KeyCount() != 0 {
			t.Errorf("Should have 0 keys after flush, got %d", s.KeyCount())
		}
	})
}
