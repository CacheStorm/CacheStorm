package store

import (
	"testing"
	"time"
)

func TestStoreType(t *testing.T) {
	s := NewStore()

	s.Set("str", &StringValue{Data: []byte("value")}, SetOptions{})
	if s.Type("str") != DataTypeString {
		t.Errorf("expected DataTypeString, got %v", s.Type("str"))
	}

	s.Set("hash", &HashValue{Fields: map[string][]byte{"f": []byte("v")}}, SetOptions{})
	if s.Type("hash") != DataTypeHash {
		t.Errorf("expected DataTypeHash, got %v", s.Type("hash"))
	}

	s.Set("list", &ListValue{Elements: [][]byte{[]byte("a")}}, SetOptions{})
	if s.Type("list") != DataTypeList {
		t.Errorf("expected DataTypeList, got %v", s.Type("list"))
	}

	s.Set("set", &SetValue{Members: map[string]struct{}{"a": {}}}, SetOptions{})
	if s.Type("set") != DataTypeSet {
		t.Errorf("expected DataTypeSet, got %v", s.Type("set"))
	}

	if s.Type("nonexistent") != DataType(0) {
		t.Errorf("expected DataType(0) for nonexistent key, got %v", s.Type("nonexistent"))
	}
}

func TestStoreKeys(t *testing.T) {
	s := NewStore()

	s.Set("key1", &StringValue{Data: []byte("v1")}, SetOptions{})
	s.Set("key2", &StringValue{Data: []byte("v2")}, SetOptions{})
	s.Set("key3", &StringValue{Data: []byte("v3")}, SetOptions{})

	keys := s.Keys()
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}
}

func TestStoreGetShard(t *testing.T) {
	s := NewStore()

	shard := s.GetShard("testkey")
	if shard == nil {
		t.Error("expected shard")
	}
}

func TestStoreGetTTL(t *testing.T) {
	s := NewStore()

	ttl := s.GetTTL("nonexistent")
	if ttl != -2*time.Second {
		t.Errorf("expected -2s for nonexistent key, got %v", ttl)
	}

	s.Set("notimeout", &StringValue{Data: []byte("v")}, SetOptions{})
	ttl = s.GetTTL("notimeout")
	if ttl != -1*time.Second {
		t.Errorf("expected -1s for key without timeout, got %v", ttl)
	}

	s.Set("withtimeout", &StringValue{Data: []byte("v")}, SetOptions{TTL: 10 * time.Second})
	ttl = s.GetTTL("withtimeout")
	if ttl <= 0 || ttl > 10*time.Second {
		t.Errorf("expected TTL between 0 and 10s, got %v", ttl)
	}
}

func TestStoreSetTTLNonexistent(t *testing.T) {
	s := NewStore()

	if s.SetTTL("nonexistent", 10*time.Second) {
		t.Error("SetTTL should return false for nonexistent key")
	}
}

func TestStoreSetExpiresAtNonexistent(t *testing.T) {
	s := NewStore()

	if s.SetExpiresAt("nonexistent", time.Now().Add(10*time.Second).UnixNano()) {
		t.Error("SetExpiresAt should return false for nonexistent key")
	}
}

func TestStorePersistNonexistent(t *testing.T) {
	s := NewStore()

	if s.Persist("nonexistent") {
		t.Error("Persist should return false for nonexistent key")
	}
}

func TestStoreGetTagIndex(t *testing.T) {
	s := NewStore()

	ti := s.GetTagIndex()
	if ti == nil {
		t.Error("expected tag index")
	}
}

func TestStoreGetPubSub(t *testing.T) {
	s := NewStore()

	ps := s.GetPubSub()
	if ps == nil {
		t.Error("expected pubsub")
	}
}

func TestStoreGetNamespaceManager(t *testing.T) {
	s := NewStoreWithNamespaces()

	nm := s.GetNamespaceManager()
	if nm == nil {
		t.Error("expected namespace manager")
	}
}

func TestStoreGetNamespaceManagerNil(t *testing.T) {
	s := NewStore()

	nm := s.GetNamespaceManager()
	if nm != nil {
		t.Error("expected nil namespace manager for store without namespaces")
	}
}

func TestStoreVersion(t *testing.T) {
	s := NewStore()

	if s.GetVersion("key") != 0 {
		t.Error("initial version should be 0")
	}

	s.Set("key", &StringValue{Data: []byte("v")}, SetOptions{})
	if s.GetVersion("key") != 1 {
		t.Errorf("version should be 1 after set, got %d", s.GetVersion("key"))
	}

	s.Set("key", &StringValue{Data: []byte("v2")}, SetOptions{})
	if s.GetVersion("key") != 2 {
		t.Errorf("version should be 2 after second set, got %d", s.GetVersion("key"))
	}
}

func TestStoreSetEntry(t *testing.T) {
	s := NewStore()

	entry := NewEntry(&StringValue{Data: []byte("value")})
	s.SetEntry("key", entry)

	e, exists := s.Get("key")
	if !exists {
		t.Fatal("key should exist")
	}
	if string(e.Value.(*StringValue).Data) != "value" {
		t.Errorf("expected 'value', got '%s'", e.Value.(*StringValue).Data)
	}
}

func TestStoreDeleteWithTags(t *testing.T) {
	s := NewStore()

	s.Set("key", &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"tag1"}})

	s.Delete("key")

	keys := s.GetTagIndex().GetKeys("tag1")
	if len(keys) != 0 {
		t.Errorf("tag should have no keys after delete, got %d", len(keys))
	}
}

func TestFNV32a(t *testing.T) {
	hash1 := fnv32a("test")
	hash2 := fnv32a("test")
	if hash1 != hash2 {
		t.Error("same string should produce same hash")
	}

	hash3 := fnv32a("other")
	if hash1 == hash3 {
		t.Error("different strings should produce different hashes")
	}
}

func TestStringValueMethods(t *testing.T) {
	v := &StringValue{Data: []byte("hello")}

	if v.Type() != DataTypeString {
		t.Errorf("expected DataTypeString, got %v", v.Type())
	}

	if v.SizeOf() != int64(len("hello"))+24 {
		t.Errorf("unexpected size: %d", v.SizeOf())
	}

	if v.String() != "hello" {
		t.Errorf("expected 'hello', got '%s'", v.String())
	}

	cloned := v.Clone()
	if cloned.String() != v.String() {
		t.Error("clone should have same value")
	}

	cloned.(*StringValue).Data[0] = 'x'
	if v.String() == "xello" {
		t.Error("clone should be independent")
	}
}

func TestHashValueMethods(t *testing.T) {
	v := &HashValue{Fields: map[string][]byte{"f1": []byte("v1"), "f2": []byte("v2")}}

	if v.Type() != DataTypeHash {
		t.Errorf("expected DataTypeHash, got %v", v.Type())
	}

	if v.SizeOf() <= 0 {
		t.Errorf("size should be positive, got %d", v.SizeOf())
	}

	str := v.String()
	if str == "" {
		t.Error("string should not be empty")
	}

	cloned := v.Clone().(*HashValue)
	if len(cloned.Fields) != len(v.Fields) {
		t.Error("clone should have same number of fields")
	}

	v.Lock()
	v.Unlock()

	v.RLock()
	v.RUnlock()
}

func TestListValueMethods(t *testing.T) {
	v := &ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}

	if v.Type() != DataTypeList {
		t.Errorf("expected DataTypeList, got %v", v.Type())
	}

	if v.SizeOf() <= 0 {
		t.Errorf("size should be positive, got %d", v.SizeOf())
	}

	if v.String() != "a, b, c" {
		t.Errorf("expected 'a, b, c', got '%s'", v.String())
	}

	cloned := v.Clone().(*ListValue)
	if len(cloned.Elements) != len(v.Elements) {
		t.Error("clone should have same number of elements")
	}

	v.Lock()
	v.Unlock()

	v.RLock()
	v.RUnlock()
}

func TestSetValueMethods(t *testing.T) {
	v := &SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}

	if v.Type() != DataTypeSet {
		t.Errorf("expected DataTypeSet, got %v", v.Type())
	}

	if v.SizeOf() <= 0 {
		t.Errorf("size should be positive, got %d", v.SizeOf())
	}

	cloned := v.Clone().(*SetValue)
	if len(cloned.Members) != len(v.Members) {
		t.Error("clone should have same number of members")
	}

	v.Lock()
	v.Unlock()

	v.RLock()
	v.RUnlock()
}

func TestEntryTouchMethod(t *testing.T) {
	e := NewEntry(&StringValue{Data: []byte("v")})
	time.Sleep(time.Millisecond)
	e.Touch()

	if e.LastAccess <= e.CreatedAt {
		t.Error("LastAccess should be updated")
	}
	if e.AccessCount != 1 {
		t.Errorf("AccessCount should be 1, got %d", e.AccessCount)
	}
}

func TestEntryTTL(t *testing.T) {
	e := NewEntry(&StringValue{Data: []byte("v")})

	if e.TTL() != -1 {
		t.Errorf("TTL should be -1 for entry without expiry, got %v", e.TTL())
	}

	e.SetTTL(10 * time.Second)
	if e.TTL() <= 0 || e.TTL() > 10*time.Second {
		t.Errorf("TTL should be between 0 and 10s, got %v", e.TTL())
	}

	e.SetExpiresAt(time.Now().Add(-1 * time.Second).UnixNano())
	if e.TTL() != -2 {
		t.Errorf("TTL should be -2 for expired entry, got %v", e.TTL())
	}
}

func TestEntryMemoryUsageMethod(t *testing.T) {
	e := NewEntry(&StringValue{Data: []byte("test")})
	usage := e.MemoryUsage()
	if usage <= 0 {
		t.Errorf("memory usage should be positive, got %d", usage)
	}

	eWithTags := NewEntry(&StringValue{Data: []byte("test")})
	eWithTags.Tags = []string{"tag1", "tag2"}
	usageWithTags := eWithTags.MemoryUsage()
	if usageWithTags <= usage {
		t.Error("entry with tags should have higher memory usage")
	}
}

func TestDataTypeString(t *testing.T) {
	tests := []struct {
		dt       DataType
		expected string
	}{
		{DataTypeString, "string"},
		{DataTypeHash, "hash"},
		{DataTypeList, "list"},
		{DataTypeSet, "set"},
		{DataTypeSortedSet, "zset"},
		{DataTypeStream, "stream"},
		{DataTypeGeo, "geo"},
		{DataType(99), "unknown"},
	}

	for _, tt := range tests {
		if tt.dt.String() != tt.expected {
			t.Errorf("expected '%s', got '%s'", tt.expected, tt.dt.String())
		}
	}
}

func TestShardAll(t *testing.T) {
	s := NewShard()

	s.Set("key1", NewEntry(&StringValue{Data: []byte("v1")}))
	s.Set("key2", NewEntry(&StringValue{Data: []byte("v2")}))

	all := s.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}

	if !s.Exists("key1") {
		t.Error("key1 should exist")
	}

	if s.Exists("nonexistent") {
		t.Error("nonexistent should not exist")
	}
}

func TestShardFlush(t *testing.T) {
	s := NewShard()

	s.Set("key1", NewEntry(&StringValue{Data: []byte("v1")}))
	freed := s.Flush()

	if freed <= 0 {
		t.Errorf("freed should be positive, got %d", freed)
	}

	if s.Len() != 0 {
		t.Errorf("len should be 0 after flush, got %d", s.Len())
	}
}

func TestShardMemUsage(t *testing.T) {
	s := NewShard()

	initial := s.MemUsage()

	s.Set("key1", NewEntry(&StringValue{Data: []byte("value")}))

	after := s.MemUsage()
	if after <= initial {
		t.Error("memory usage should increase after set")
	}
}

func TestShardDelete(t *testing.T) {
	s := NewShard()

	s.Set("key1", NewEntry(&StringValue{Data: []byte("v1")}))

	mem, ok := s.Delete("key1")
	if !ok {
		t.Error("delete should return true")
	}
	if mem <= 0 {
		t.Error("memory freed should be positive")
	}

	mem, ok = s.Delete("nonexistent")
	if ok {
		t.Error("delete of nonexistent should return false")
	}
	if mem != 0 {
		t.Error("memory freed should be 0 for nonexistent")
	}
}

func TestShardLen(t *testing.T) {
	s := NewShard()

	if s.Len() != 0 {
		t.Errorf("initial len should be 0, got %d", s.Len())
	}

	s.Set("key1", NewEntry(&StringValue{Data: []byte("v1")}))
	if s.Len() != 1 {
		t.Errorf("len should be 1, got %d", s.Len())
	}

	s.Set("key1", NewEntry(&StringValue{Data: []byte("v2")}))
	if s.Len() != 1 {
		t.Errorf("len should still be 1 after update, got %d", s.Len())
	}

	s.Set("key2", NewEntry(&StringValue{Data: []byte("v2")}))
	if s.Len() != 2 {
		t.Errorf("len should be 2, got %d", s.Len())
	}
}

func TestShardKeys(t *testing.T) {
	s := NewShard()

	s.Set("key1", NewEntry(&StringValue{Data: []byte("v1")}))
	s.Set("key2", NewEntry(&StringValue{Data: []byte("v2")}))

	keys := s.Keys()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}
