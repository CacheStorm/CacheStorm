package store

import (
	"testing"
)

func TestStoreEntryOperations(t *testing.T) {
	t.Run("Entry Creation", func(t *testing.T) {
		entry := NewEntry(&StringValue{Data: []byte("value")})
		if entry == nil {
			t.Fatal("NewEntry returned nil")
		}
		if entry.Value == nil {
			t.Error("Entry value should not be nil")
		}
	})

	t.Run("Entry Is Expired", func(t *testing.T) {
		entry := NewEntry(&StringValue{Data: []byte("value")})
		if entry.IsExpired() {
			t.Error("Entry with no expiry should not be expired")
		}
	})

	t.Run("Entry TTL", func(t *testing.T) {
		entry := NewEntry(&StringValue{Data: []byte("value")})
		ttl := entry.TTL()
		if ttl != -1 {
			t.Error("TTL should be -1 for non-expiring entry")
		}
	})

	t.Run("Entry Memory Usage", func(t *testing.T) {
		entry := NewEntry(&StringValue{Data: []byte("value")})
		usage := entry.MemoryUsage()
		if usage <= 0 {
			t.Error("MemoryUsage should be positive")
		}
	})
}

func TestStoreShardOperations(t *testing.T) {
	t.Run("Shard Creation", func(t *testing.T) {
		shard := NewShard()
		if shard == nil {
			t.Fatal("NewShard returned nil")
		}
	})

	t.Run("Shard Set and Get", func(t *testing.T) {
		shard := NewShard()
		entry := NewEntry(&StringValue{Data: []byte("value")})
		shard.Set("test", entry)

		retrieved, exists := shard.Get("test")
		if !exists {
			t.Error("Key should exist")
		}
		if retrieved == nil {
			t.Error("Retrieved entry should not be nil")
		}
	})

	t.Run("Shard Delete", func(t *testing.T) {
		shard := NewShard()
		entry := NewEntry(&StringValue{Data: []byte("value")})
		shard.Set("test", entry)
		shard.Delete("test")

		_, exists := shard.Get("test")
		if exists {
			t.Error("Key should not exist after delete")
		}
	})

	t.Run("Shard Get All", func(t *testing.T) {
		shard := NewShard()
		shard.Set("key1", NewEntry(&StringValue{Data: []byte("v1")}))
		shard.Set("key2", NewEntry(&StringValue{Data: []byte("v2")}))

		all := shard.GetAll()
		if len(all) != 2 {
			t.Errorf("Expected 2 entries, got %d", len(all))
		}
	})
}

func TestStoreDataTypes(t *testing.T) {
	t.Run("String Value", func(t *testing.T) {
		val := &StringValue{Data: []byte("test")}
		if val.Type() != DataTypeString {
			t.Error("Type should be DataTypeString")
		}
		if val.SizeOf() <= 0 {
			t.Error("SizeOf should be positive")
		}
		if val.String() != "test" {
			t.Error("String() mismatch")
		}
	})

	t.Run("Hash Value", func(t *testing.T) {
		val := &HashValue{Fields: map[string][]byte{"f1": []byte("v1")}}
		if val.Type() != DataTypeHash {
			t.Error("Type should be DataTypeHash")
		}
	})

	t.Run("List Value", func(t *testing.T) {
		val := &ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}
		if val.Type() != DataTypeList {
			t.Error("Type should be DataTypeList")
		}
	})

	t.Run("Set Value", func(t *testing.T) {
		val := &SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}
		if val.Type() != DataTypeSet {
			t.Error("Type should be DataTypeSet")
		}
	})

	t.Run("Sorted Set Value", func(t *testing.T) {
		val := &SortedSetValue{Members: map[string]float64{"a": 1.0}}
		if val.Type() != DataTypeSortedSet {
			t.Error("Type should be DataTypeSortedSet")
		}
	})
}

func TestStoreValueClone(t *testing.T) {
	t.Run("String Clone", func(t *testing.T) {
		original := &StringValue{Data: []byte("original")}
		cloned := original.Clone().(*StringValue)
		if string(cloned.Data) != "original" {
			t.Error("Clone data mismatch")
		}
	})

	t.Run("Hash Clone", func(t *testing.T) {
		original := &HashValue{Fields: map[string][]byte{"key": []byte("value")}}
		cloned := original.Clone().(*HashValue)
		if string(cloned.Fields["key"]) != "value" {
			t.Error("Clone field mismatch")
		}
	})

	t.Run("Set Clone", func(t *testing.T) {
		original := &SetValue{Members: map[string]struct{}{"member": {}}}
		cloned := original.Clone().(*SetValue)
		if _, exists := cloned.Members["member"]; !exists {
			t.Error("Clone member missing")
		}
	})
}
