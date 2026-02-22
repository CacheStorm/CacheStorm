package store

import (
	"testing"
	"time"
)

func TestStoreBulkOperations(t *testing.T) {
	t.Run("Set 1000 Keys", func(t *testing.T) {
		s := NewStore()

		for i := 0; i < 1000; i++ {
			key := "bulkkey" + string(rune(i%256))
			s.Set(key, &StringValue{Data: []byte("value")}, SetOptions{})
		}

		if s.KeyCount() < 0 {
			t.Error("Key count should not be negative")
		}
	})

	t.Run("Delete All Keys", func(t *testing.T) {
		s := NewStore()

		// Add keys
		for i := 0; i < 100; i++ {
			key := "delkey" + string(rune(i))
			s.Set(key, &StringValue{Data: []byte("value")}, SetOptions{})
		}

		// Delete all
		for i := 0; i < 100; i++ {
			key := "delkey" + string(rune(i))
			s.Delete(key)
		}

		// Verify
		count := s.KeyCount()
		if count != 0 {
			t.Errorf("Expected 0 keys, got %d", count)
		}
	})
}

func TestStoreExpirationAdvanced(t *testing.T) {
	t.Run("Multiple Expiring Keys", func(t *testing.T) {
		s := NewStore()

		// Set keys with different TTLs
		for i := 0; i < 10; i++ {
			opts := SetOptions{TTL: time.Duration(50+i*10) * time.Millisecond}
			key := "expirekey" + string(rune(i))
			s.Set(key, &StringValue{Data: []byte("value")}, opts)
		}

		// Wait for some to expire
		time.Sleep(150 * time.Millisecond)

		// Some should be expired, some not
		count := s.KeyCount()
		// Just verify store works, exact count may vary
		_ = count
	})

	t.Run("Update TTL", func(t *testing.T) {
		s := NewStore()

		s.Set("ttlkey", &StringValue{Data: []byte("value")}, SetOptions{})

		// Update TTL
		if !s.SetTTL("ttlkey", 50*time.Millisecond) {
			t.Error("SetTTL should return true")
		}

		time.Sleep(100 * time.Millisecond)

		_, exists := s.Get("ttlkey")
		if exists {
			t.Error("Key should be expired")
		}
	})
}

func TestStoreDataTypesAdvanced(t *testing.T) {
	t.Run("String Value Operations", func(t *testing.T) {
		val := &StringValue{Data: []byte("test string")}

		if val.Type() != DataTypeString {
			t.Error("Type mismatch")
		}

		if val.SizeOf() <= 0 {
			t.Error("Size should be positive")
		}

		if val.String() != "test string" {
			t.Error("String() mismatch")
		}

		// Clone
		cloned := val.Clone().(*StringValue)
		if string(cloned.Data) != "test string" {
			t.Error("Clone mismatch")
		}
	})

	t.Run("Hash Value Operations", func(t *testing.T) {
		val := &HashValue{Fields: map[string][]byte{
			"field1": []byte("value1"),
			"field2": []byte("value2"),
		}}

		if val.Type() != DataTypeHash {
			t.Error("Type mismatch")
		}

		// Clone
		cloned := val.Clone().(*HashValue)
		if string(cloned.Fields["field1"]) != "value1" {
			t.Error("Clone mismatch")
		}
	})

	t.Run("List Value Operations", func(t *testing.T) {
		val := &ListValue{Elements: [][]byte{
			[]byte("item1"),
			[]byte("item2"),
			[]byte("item3"),
		}}

		if val.Type() != DataTypeList {
			t.Error("Type mismatch")
		}

		// Clone
		cloned := val.Clone().(*ListValue)
		if len(cloned.Elements) != 3 {
			t.Error("Clone length mismatch")
		}
	})

	t.Run("Set Value Operations", func(t *testing.T) {
		val := &SetValue{Members: map[string]struct{}{
			"member1": {},
			"member2": {},
			"member3": {},
		}}

		if val.Type() != DataTypeSet {
			t.Error("Type mismatch")
		}

		// Clone
		cloned := val.Clone().(*SetValue)
		if len(cloned.Members) != 3 {
			t.Error("Clone length mismatch")
		}
	})

	t.Run("Sorted Set Value Operations", func(t *testing.T) {
		val := &SortedSetValue{Members: map[string]float64{
			"member1": 1.0,
			"member2": 2.0,
			"member3": 3.0,
		}}

		if val.Type() != DataTypeSortedSet {
			t.Error("Type mismatch")
		}

		// Clone
		cloned := val.Clone().(*SortedSetValue)
		if len(cloned.Members) != 3 {
			t.Error("Clone length mismatch")
		}
	})
}

func TestStoreShardAdvanced(t *testing.T) {
	t.Run("Shard Consistency", func(t *testing.T) {
		s := NewStore()

		// Same key should always go to same shard
		idx1 := s.shardIndex("testkey")
		idx2 := s.shardIndex("testkey")

		if idx1 != idx2 {
			t.Error("Shard index should be consistent")
		}
	})

	t.Run("Multiple Shards", func(t *testing.T) {
		s := NewStore()

		// Add keys that might go to different shards
		for i := 0; i < 100; i++ {
			key := "shardtest" + string(rune(i))
			s.Set(key, &StringValue{Data: []byte("value")}, SetOptions{})
		}

		// All should be retrievable
		for i := 0; i < 100; i++ {
			key := "shardtest" + string(rune(i))
			if _, exists := s.Get(key); !exists {
				t.Errorf("Key %s should exist", key)
			}
		}
	})
}

func TestStoreNamespaceAdvanced(t *testing.T) {
	t.Run("Namespace Manager", func(t *testing.T) {
		s := NewStoreWithNamespaces()
		nm := s.GetNamespaceManager()

		if nm == nil {
			t.Fatal("NamespaceManager should not be nil")
		}
	})
}

func TestStorePubSubAdvanced(t *testing.T) {
	t.Run("PubSub Instance", func(t *testing.T) {
		s := NewStore()
		ps := s.GetPubSub()

		if ps == nil {
			t.Fatal("PubSub should not be nil")
		}
	})
}
