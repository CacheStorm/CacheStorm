package store

import (
	"testing"
	"time"
)

func TestNewEntry(t *testing.T) {
	entry := NewEntry(&StringValue{Data: []byte("test")})

	if entry == nil {
		t.Fatal("expected entry, got nil")
	}

	if entry.CreatedAt == 0 {
		t.Error("expected CreatedAt to be set")
	}

	if entry.LastAccess == 0 {
		t.Error("expected LastAccess to be set")
	}
}

func TestEntryIsExpired(t *testing.T) {
	entry := NewEntry(&StringValue{Data: []byte("test")})

	if entry.IsExpired() {
		t.Error("new entry should not be expired")
	}

	entry.ExpiresAt = time.Now().Add(-1 * time.Second).UnixNano()
	if !entry.IsExpired() {
		t.Error("entry should be expired")
	}
}

func TestEntrySetTTL(t *testing.T) {
	entry := NewEntry(&StringValue{Data: []byte("test")})

	entry.SetTTL(10 * time.Second)

	if entry.ExpiresAt == 0 {
		t.Error("expected ExpiresAt to be set")
	}

	ttl := entry.TTL()
	if ttl < 9*time.Second || ttl > 10*time.Second {
		t.Errorf("expected TTL ~10s, got %v", ttl)
	}
}

func TestEntryTouch(t *testing.T) {
	entry := NewEntry(&StringValue{Data: []byte("test")})
	oldAccess := entry.LastAccess
	oldCount := entry.AccessCount

	time.Sleep(1 * time.Millisecond)
	entry.Touch()

	if entry.LastAccess <= oldAccess {
		t.Error("expected LastAccess to be updated")
	}

	if entry.AccessCount != oldCount+1 {
		t.Errorf("expected AccessCount to increment")
	}
}

func TestEntryMemoryUsage(t *testing.T) {
	entry := NewEntry(&StringValue{Data: []byte("test")})
	entry.Tags = []string{"tag1", "tag2"}

	usage := entry.MemoryUsage()
	if usage <= 0 {
		t.Error("expected positive memory usage")
	}
}

func TestStringValue(t *testing.T) {
	v := &StringValue{Data: []byte("hello")}

	if v.Type() != DataTypeString {
		t.Errorf("expected DataTypeString, got %v", v.Type())
	}

	size := v.SizeOf()
	if size <= 0 {
		t.Error("expected positive size")
	}

	cloned := v.Clone()
	if string(cloned.(*StringValue).Data) != "hello" {
		t.Error("clone should have same data")
	}
}

func TestHashValue(t *testing.T) {
	v := &HashValue{Fields: map[string][]byte{
		"field1": []byte("value1"),
		"field2": []byte("value2"),
	}}

	if v.Type() != DataTypeHash {
		t.Errorf("expected DataTypeHash, got %v", v.Type())
	}

	cloned := v.Clone().(*HashValue)
	if len(cloned.Fields) != 2 {
		t.Error("clone should have same fields")
	}
}

func TestListValue(t *testing.T) {
	v := &ListValue{Elements: [][]byte{
		[]byte("elem1"),
		[]byte("elem2"),
	}}

	if v.Type() != DataTypeList {
		t.Errorf("expected DataTypeList, got %v", v.Type())
	}

	cloned := v.Clone().(*ListValue)
	if len(cloned.Elements) != 2 {
		t.Error("clone should have same elements")
	}
}

func TestSetValue(t *testing.T) {
	v := &SetValue{Members: map[string]struct{}{
		"member1": {},
		"member2": {},
	}}

	if v.Type() != DataTypeSet {
		t.Errorf("expected DataTypeSet, got %v", v.Type())
	}

	cloned := v.Clone().(*SetValue)
	if len(cloned.Members) != 2 {
		t.Error("clone should have same members")
	}
}
