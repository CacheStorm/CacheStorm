package store

import (
	"testing"
	"time"
)

func TestStoreAdvancedSetOptions(t *testing.T) {
	s := NewStore()

	t.Run("SetWithTTL", func(t *testing.T) {
		opts := SetOptions{TTL: 100 * time.Millisecond}
		err := s.Set("key1", &StringValue{Data: []byte("value1")}, opts)
		if err != nil {
			t.Errorf("Set failed: %v", err)
		}

		// Verify key exists
		_, exists := s.Get("key1")
		if !exists {
			t.Error("Key should exist immediately after set")
		}

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)
		_, exists = s.Get("key1")
		if exists {
			t.Error("Key should have expired")
		}
	})

	t.Run("SetWithNX", func(t *testing.T) {
		s.Set("nxkey", &StringValue{Data: []byte("original")}, SetOptions{})

		// Try to set with NX when key exists
		opts := SetOptions{NX: true}
		err := s.Set("nxkey", &StringValue{Data: []byte("new")}, opts)
		if err != ErrKeyExists {
			t.Errorf("Expected ErrKeyExists, got: %v", err)
		}

		// Try to set with NX when key doesn't exist
		err = s.Set("nxkey2", &StringValue{Data: []byte("value")}, opts)
		if err != nil {
			t.Errorf("Set with NX on new key failed: %v", err)
		}
	})

	t.Run("SetWithXX", func(t *testing.T) {
		// Try to set with XX when key doesn't exist
		opts := SetOptions{XX: true}
		err := s.Set("xxkey", &StringValue{Data: []byte("value")}, opts)
		if err != ErrKeyNotFound {
			t.Errorf("Expected ErrKeyNotFound, got: %v", err)
		}

		// Set key first
		s.Set("xxkey2", &StringValue{Data: []byte("original")}, SetOptions{})

		// Now update with XX
		err = s.Set("xxkey2", &StringValue{Data: []byte("updated")}, opts)
		if err != nil {
			t.Errorf("Set with XX on existing key failed: %v", err)
		}
	})

	t.Run("SetTTL", func(t *testing.T) {
		s.Set("ttlkey", &StringValue{Data: []byte("value")}, SetOptions{})

		// Set TTL
		if !s.SetTTL("ttlkey", 100*time.Millisecond) {
			t.Error("SetTTL should return true for existing key")
		}

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)
		_, exists := s.Get("ttlkey")
		if exists {
			t.Error("Key should have expired after SetTTL")
		}

		// Try SetTTL on non-existent key
		if s.SetTTL("nonexistent", time.Second) {
			t.Error("SetTTL should return false for non-existent key")
		}
	})

	t.Run("SetExpiresAt", func(t *testing.T) {
		s.Set("expirekey", &StringValue{Data: []byte("value")}, SetOptions{})

		// Set expiration time in future
		future := time.Now().Add(100 * time.Millisecond).UnixMilli()
		if !s.SetExpiresAt("expirekey", future) {
			t.Error("SetExpiresAt should return true for existing key")
		}

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)
		_, exists := s.Get("expirekey")
		if exists {
			t.Error("Key should have expired")
		}

		// Try SetExpiresAt on non-existent key
		if s.SetExpiresAt("nonexistent", future) {
			t.Error("SetExpiresAt should return false for non-existent key")
		}
	})

	t.Run("Persist", func(t *testing.T) {
		opts := SetOptions{TTL: 100 * time.Millisecond}
		s.Set("persistkey", &StringValue{Data: []byte("value")}, opts)

		// Persist the key
		if !s.Persist("persistkey") {
			t.Error("Persist should return true for existing key with TTL")
		}

		// Wait and verify it still exists
		time.Sleep(150 * time.Millisecond)
		_, exists := s.Get("persistkey")
		if !exists {
			t.Error("Key should still exist after persist")
		}

		// Try Persist on non-existent key
		if s.Persist("nonexistent") {
			t.Error("Persist should return false for non-existent key")
		}
	})

	t.Run("Type", func(t *testing.T) {
		s.Set("strkey", &StringValue{Data: []byte("value")}, SetOptions{})
		s.Set("hashkey", &HashValue{Fields: map[string][]byte{"f": []byte("v")}}, SetOptions{})
		s.Set("setkey", &SetValue{Members: map[string]struct{}{"m": {}}}, SetOptions{})

		if s.Type("strkey") != DataTypeString {
			t.Errorf("Expected type DataTypeString, got: %v", s.Type("strkey"))
		}
		if s.Type("hashkey") != DataTypeHash {
			t.Errorf("Expected type DataTypeHash, got: %v", s.Type("hashkey"))
		}
		if s.Type("setkey") != DataTypeSet {
			t.Errorf("Expected type DataTypeSet, got: %v", s.Type("setkey"))
		}
		if s.Type("nonexistent") != DataType(0) {
			t.Errorf("Expected type 0 for nonexistent, got: %v", s.Type("nonexistent"))
		}
	})

	t.Run("Keys", func(t *testing.T) {
		s.Set("keys1", &StringValue{Data: []byte("v1")}, SetOptions{})
		s.Set("keys2", &StringValue{Data: []byte("v2")}, SetOptions{})
		s.Set("other", &StringValue{Data: []byte("v3")}, SetOptions{})

		keys := s.Keys()
		if len(keys) < 3 {
			t.Errorf("Expected at least 3 keys, got: %d", len(keys))
		}
	})

	t.Run("Delete", func(t *testing.T) {
		s.Set("delkey", &StringValue{Data: []byte("value")}, SetOptions{})

		if !s.Delete("delkey") {
			t.Error("Delete should return true for existing key")
		}

		_, exists := s.Get("delkey")
		if exists {
			t.Error("Key should not exist after delete")
		}

		// Try to delete non-existent key
		if s.Delete("nonexistent") {
			t.Error("Delete should return false for non-existent key")
		}
	})
}

func TestStoreTTLFunctions(t *testing.T) {
	s := NewStore()

	t.Run("TTL function", func(t *testing.T) {
		opts := SetOptions{TTL: 5 * time.Second}
		s.Set("ttltest", &StringValue{Data: []byte("value")}, opts)

		ttl := s.TTL("ttltest")
		if ttl <= 0 || ttl > 5*time.Second {
			t.Errorf("Expected TTL around 5s, got: %v", ttl)
		}

		// Non-existent key - returns -2 as time.Duration (nanoseconds)
		if s.TTL("nonexistent") != -2 {
			t.Error("TTL should return -2 for non-existent key")
		}
	})

	t.Run("GetTTL", func(t *testing.T) {
		opts := SetOptions{TTL: 5 * time.Second}
		s.Set("getttltest", &StringValue{Data: []byte("value")}, opts)

		ttl := s.GetTTL("getttltest")
		if ttl <= 0 || ttl > 5*time.Second {
			t.Errorf("Expected TTL around 5s, got: %v", ttl)
		}

		// Non-existent key
		if s.GetTTL("nonexistent") != -2*time.Second {
			t.Error("GetTTL should return -2 for non-existent key")
		}
	})
}
