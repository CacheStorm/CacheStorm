package store

import (
	"testing"
	"time"
)

func TestPriorityQueueBasic(t *testing.T) {
	pq := NewPriorityQueue()

	if pq.Len() != 0 {
		t.Errorf("expected empty queue, got %d", pq.Len())
	}

	pq.PushItem("item1", 5)
	pq.PushItem("item2", 3)
	pq.PushItem("item3", 7)

	if pq.Size() != 3 {
		t.Errorf("expected size 3, got %d", pq.Size())
	}

	value, priority, ok := pq.Peek()
	if !ok {
		t.Fatal("expected to peek item")
	}
	if priority != 3 {
		t.Errorf("expected priority 3, got %d", priority)
	}
	if value != "item2" {
		t.Errorf("expected item2, got %s", value)
	}
}

func TestPriorityQueuePop(t *testing.T) {
	pq := NewPriorityQueue()
	pq.PushItem("low", 10)
	pq.PushItem("high", 1)
	pq.PushItem("medium", 5)

	val, pri, ok := pq.PopItem()
	if !ok || val != "high" || pri != 1 {
		t.Errorf("expected high/1, got %s/%d", val, pri)
	}

	val, pri, ok = pq.PopItem()
	if !ok || val != "medium" || pri != 5 {
		t.Errorf("expected medium/5, got %s/%d", val, pri)
	}

	val, pri, ok = pq.PopItem()
	if !ok || val != "low" || pri != 10 {
		t.Errorf("expected low/10, got %s/%d", val, pri)
	}

	_, _, ok = pq.PopItem()
	if ok {
		t.Error("expected empty queue")
	}
}

func TestPriorityQueueClear(t *testing.T) {
	pq := NewPriorityQueue()
	pq.PushItem("item1", 1)
	pq.PushItem("item2", 2)

	pq.Clear()

	if pq.Size() != 0 {
		t.Errorf("expected empty queue, got %d", pq.Size())
	}
}

func TestPriorityQueueGetAll(t *testing.T) {
	pq := NewPriorityQueue()
	pq.PushItem("item1", 1)
	pq.PushItem("item2", 2)

	items := pq.GetAll()
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestLRUCacheBasic(t *testing.T) {
	lru := NewLRUCache(3)

	lru.Set("k1", "v1")
	lru.Set("k2", "v2")
	lru.Set("k3", "v3")

	if v, ok := lru.Get("k1"); !ok || v != "v1" {
		t.Errorf("expected v1, got %s", v)
	}

	lru.Set("k4", "v4")

	if _, ok := lru.Get("k2"); ok {
		t.Error("k2 should be evicted")
	}
}

func TestLRUCacheDelete(t *testing.T) {
	lru := NewLRUCache(3)
	lru.Set("k1", "v1")

	if !lru.Delete("k1") {
		t.Error("delete should return true")
	}

	if lru.Delete("nonexistent") {
		t.Error("deleting nonexistent should return false")
	}
}

func TestLRUCacheClear(t *testing.T) {
	lru := NewLRUCache(3)
	lru.Set("k1", "v1")
	lru.Set("k2", "v2")

	lru.Clear()

	if lru.Size != 0 {
		t.Errorf("expected size 0, got %d", lru.Size)
	}
}

func TestLRUCacheKeys(t *testing.T) {
	lru := NewLRUCache(3)
	lru.Set("k1", "v1")
	lru.Set("k2", "v2")

	keys := lru.Keys()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestLRUCacheStats(t *testing.T) {
	lru := NewLRUCache(3)
	lru.Set("k1", "v1")
	lru.Set("k2", "v2")
	lru.Get("k1")
	lru.Get("nonexistent")

	stats := lru.Stats()
	if stats["capacity"].(int) != 3 {
		t.Errorf("expected capacity 3, got %v", stats["capacity"])
	}
}

func TestLRUCacheUpdate(t *testing.T) {
	lru := NewLRUCache(3)
	lru.Set("k1", "v1")
	lru.Set("k1", "v2")

	if v, ok := lru.Get("k1"); !ok || v != "v2" {
		t.Errorf("expected v2, got %s", v)
	}

	if lru.Size != 1 {
		t.Errorf("expected size 1, got %d", lru.Size)
	}
}

func TestTokenBucketBasic(t *testing.T) {
	tb := NewTokenBucket(10, 2)

	if !tb.Consume(5) {
		t.Error("should be able to consume 5 tokens")
	}

	if tb.Available() < 4 || tb.Available() > 6 {
		t.Errorf("expected ~5 tokens, got %f", tb.Available())
	}

	tb.Reset()
	if tb.Available() != 10 {
		t.Errorf("expected 10 tokens after reset, got %f", tb.Available())
	}
}

func TestTokenBucketConsumeTooMany(t *testing.T) {
	tb := NewTokenBucket(5, 1)

	if tb.Consume(10) {
		t.Error("should not be able to consume more than available")
	}
}

func TestLeakyBucketBasic(t *testing.T) {
	lb := NewLeakyBucket(10, 5)

	if !lb.Add(5) {
		t.Error("should be able to add 5 drops")
	}

	if lb.Available() < 4 || lb.Available() > 6 {
		t.Errorf("expected ~5 available, got %d", lb.Available())
	}
}

func TestLeakyBucketAddTooMany(t *testing.T) {
	lb := NewLeakyBucket(5, 1)

	if lb.Add(10) {
		t.Error("should not be able to add more than capacity")
	}
}

func TestSlidingWindowCounterBasic(t *testing.T) {
	swc := NewSlidingWindowCounter(1000, 5)

	for i := 0; i < 5; i++ {
		if _, ok := swc.Increment("key"); !ok {
			t.Errorf("increment %d should succeed", i)
		}
	}

	if _, ok := swc.Increment("key"); ok {
		t.Error("6th increment should fail")
	}

	if swc.Count() < 5 {
		t.Errorf("expected count >= 5, got %d", swc.Count())
	}

	swc.Reset()
	if swc.Count() != 0 {
		t.Errorf("expected count 0 after reset, got %d", swc.Count())
	}
}

func TestPriorityQueueConcurrent(t *testing.T) {
	pq := NewPriorityQueue()
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(n int) {
			pq.PushItem("item", int64(n))
			pq.Size()
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestLRUCacheConcurrent(t *testing.T) {
	lru := NewLRUCache(100)
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(n int) {
			lru.Set("k", "v")
			lru.Get("k")
			lru.Keys()
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestDataTypeStrings(t *testing.T) {
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
		if got := tt.dt.String(); got != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, got)
		}
	}
}

func TestStringValueString(t *testing.T) {
	v := &StringValue{Data: []byte("hello")}
	if v.String() != "hello" {
		t.Errorf("expected hello, got %s", v.String())
	}
}

func TestHashValueString(t *testing.T) {
	v := &HashValue{Fields: map[string][]byte{
		"f1": []byte("v1"),
		"f2": []byte("v2"),
	}}
	s := v.String()
	if s == "" {
		t.Error("expected non-empty string")
	}
}

func TestListValueString(t *testing.T) {
	v := &ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}
	s := v.String()
	if s == "" {
		t.Error("expected non-empty string")
	}
}

func TestSetValueString(t *testing.T) {
	v := &SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}
	s := v.String()
	if s == "" {
		t.Error("expected non-empty string")
	}
}

func TestEntrySetExpiresAt(t *testing.T) {
	entry := NewEntry(&StringValue{Data: []byte("test")})
	expiresAt := time.Now().Add(10 * time.Second).UnixNano()
	entry.SetExpiresAt(expiresAt)

	if entry.ExpiresAt != expiresAt {
		t.Errorf("expected ExpiresAt %d, got %d", expiresAt, entry.ExpiresAt)
	}
}

func TestEntryTTLExpired(t *testing.T) {
	entry := NewEntry(&StringValue{Data: []byte("test")})
	entry.SetExpiresAt(time.Now().Add(-1 * time.Second).UnixNano())

	ttl := entry.TTL()
	if ttl != -2 {
		t.Errorf("expected TTL -2 for expired entry, got %v", ttl)
	}
}
