package store

import (
	"testing"
	"time"
)

func TestGeoValue(t *testing.T) {
	gv := NewGeoValue()

	gv.Add("point1", 13.361389, 38.115556)
	gv.Add("point2", 15.087269, 37.502669)

	if gv.Type() != DataTypeGeo {
		t.Errorf("expected DataTypeGeo, got %v", gv.Type())
	}

	if gv.SizeOf() <= 0 {
		t.Errorf("size should be positive, got %d", gv.SizeOf())
	}

	p, ok := gv.Get("point1")
	if !ok {
		t.Error("point1 should exist")
	}
	if p.Lon != 13.361389 || p.Lat != 38.115556 {
		t.Errorf("unexpected coordinates: (%f, %f)", p.Lon, p.Lat)
	}

	_, ok = gv.Get("nonexistent")
	if ok {
		t.Error("nonexistent point should not exist")
	}

	removed := gv.Remove("point1")
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	removed = gv.Remove("nonexistent")
	if removed != 0 {
		t.Errorf("expected 0 removed for nonexistent, got %d", removed)
	}
}

func TestGeoValueDistance(t *testing.T) {
	gv := NewGeoValue()
	gv.Add("p1", 13.361389, 38.115556)
	gv.Add("p2", 15.087269, 37.502669)

	dist := gv.Distance("p1", "p2")
	if dist <= 0 {
		t.Errorf("distance should be positive, got %f", dist)
	}

	dist = gv.Distance("p1", "nonexistent")
	if dist != -1 {
		t.Errorf("distance with nonexistent should be -1, got %f", dist)
	}
}

func TestGeoValueString(t *testing.T) {
	gv := NewGeoValue()
	gv.Add("p1", 13.361389, 38.115556)

	s := gv.String()
	if s == "" {
		t.Error("string should not be empty")
	}
}

func TestGeoValueClone(t *testing.T) {
	gv := NewGeoValue()
	gv.Add("p1", 13.361389, 38.115556)

	cloned := gv.Clone().(*GeoValue)
	if len(cloned.Points) != len(gv.Points) {
		t.Error("clone should have same number of points")
	}
}

func TestHaversine(t *testing.T) {
	dist := Haversine(13.361389, 38.115556, 15.087269, 37.502669)
	if dist <= 0 {
		t.Errorf("haversine distance should be positive, got %f", dist)
	}

	dist = Haversine(0, 0, 0, 0)
	if dist != 0 {
		t.Errorf("same point distance should be 0, got %f", dist)
	}
}

func TestEncodeGeohash(t *testing.T) {
	hash := EncodeGeohash(13.361389, 38.115556)
	if len(hash) != 12 {
		t.Errorf("expected 12 char geohash, got %d", len(hash))
	}
}

func TestSortedSetValueMethods(t *testing.T) {
	sv := &SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}

	if sv.Type() != DataTypeSortedSet {
		t.Errorf("expected DataTypeSortedSet, got %v", sv.Type())
	}

	if sv.SizeOf() <= 0 {
		t.Errorf("size should be positive, got %d", sv.SizeOf())
	}

	str := sv.String()
	if str == "" {
		t.Error("string should not be empty")
	}

	cloned := sv.Clone().(*SortedSetValue)
	if len(cloned.Members) != len(sv.Members) {
		t.Error("clone should have same number of members")
	}

	sv.Lock()
	sv.Unlock()

	sv.RLock()
	sv.RUnlock()
}

func TestSortedSetValueGetSortedRange(t *testing.T) {
	sv := &SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}

	entries := sv.GetSortedRange(0, -1, false, false)
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	entries = sv.GetSortedRange(0, 1, false, false)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	entries = sv.GetSortedRange(0, -1, false, true)
	if len(entries) != 3 {
		t.Errorf("expected 3 entries in reverse, got %d", len(entries))
	}
	if entries[0].Score != 3.0 {
		t.Errorf("first entry in reverse should have score 3.0, got %f", entries[0].Score)
	}

	entries = sv.GetSortedRange(10, 20, false, false)
	if entries != nil {
		t.Error("out of range should return nil")
	}
}

func TestSortedSetValueRank(t *testing.T) {
	sv := &SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}

	rank := sv.Rank("a", false)
	if rank != 0 {
		t.Errorf("rank of 'a' should be 0, got %d", rank)
	}

	rank = sv.Rank("a", true)
	if rank != 2 {
		t.Errorf("reverse rank of 'a' should be 2, got %d", rank)
	}

	rank = sv.Rank("nonexistent", false)
	if rank != -1 {
		t.Errorf("rank of nonexistent should be -1, got %d", rank)
	}
}

func TestConsumerGroup(t *testing.T) {
	cg := NewConsumerGroup("test-group")

	if cg.Name != "test-group" {
		t.Errorf("expected name 'test-group', got '%s'", cg.Name)
	}

	if cg.LastID != "0-0" {
		t.Errorf("expected LastID '0-0', got '%s'", cg.LastID)
	}
}

func TestConsumerGroupGetOrCreateConsumer(t *testing.T) {
	cg := NewConsumerGroup("test")

	c1 := cg.GetOrCreateConsumer("consumer1")
	if c1 == nil {
		t.Fatal("consumer should be created")
	}
	if c1.Name != "consumer1" {
		t.Errorf("expected name 'consumer1', got '%s'", c1.Name)
	}

	c2 := cg.GetOrCreateConsumer("consumer1")
	if c1 != c2 {
		t.Error("should return same consumer")
	}
}

func TestConsumerGroupAddPending(t *testing.T) {
	cg := NewConsumerGroup("test")
	cg.GetOrCreateConsumer("consumer1")

	cg.AddPending("entry1", "consumer1")

	if len(cg.Pending) != 1 {
		t.Errorf("expected 1 pending entry, got %d", len(cg.Pending))
	}

	c := cg.Consumers["consumer1"]
	if c.Pending != 1 {
		t.Errorf("expected consumer pending 1, got %d", c.Pending)
	}
}

func TestConsumerGroupAck(t *testing.T) {
	cg := NewConsumerGroup("test")
	cg.GetOrCreateConsumer("consumer1")
	cg.AddPending("entry1", "consumer1")

	if !cg.Ack("entry1") {
		t.Error("ack should return true")
	}

	if len(cg.Pending) != 0 {
		t.Errorf("expected 0 pending after ack, got %d", len(cg.Pending))
	}

	if cg.Ack("nonexistent") {
		t.Error("ack nonexistent should return false")
	}
}

func TestConsumerGroupClaim(t *testing.T) {
	cg := NewConsumerGroup("test")
	cg.GetOrCreateConsumer("consumer1")
	cg.GetOrCreateConsumer("consumer2")
	cg.AddPending("entry1", "consumer1")
	cg.AddPending("entry2", "consumer1")

	claimed := cg.Claim([]string{"entry1"}, "consumer2")
	if len(claimed) != 1 {
		t.Errorf("expected 1 claimed, got %d", len(claimed))
	}

	p := cg.Pending["entry1"]
	if p.Consumer != "consumer2" {
		t.Errorf("entry1 should now belong to consumer2, got %s", p.Consumer)
	}
}

func TestStreamEntry(t *testing.T) {
	entry := &StreamEntry{
		ID:        "1-0",
		Fields:    map[string][]byte{"field1": []byte("value1")},
		CreatedAt: time.Now(),
	}

	if entry.ID != "1-0" {
		t.Errorf("expected ID '1-0', got '%s'", entry.ID)
	}

	if string(entry.Fields["field1"]) != "value1" {
		t.Errorf("expected field1 'value1', got '%s'", entry.Fields["field1"])
	}
}

func TestStreamValue(t *testing.T) {
	sv := NewStreamValue(1000)

	if sv.Type() != DataTypeStream {
		t.Errorf("expected DataTypeStream, got %v", sv.Type())
	}

	if sv.SizeOf() <= 0 {
		t.Errorf("size should be positive, got %d", sv.SizeOf())
	}
}

func TestSortedEntries(t *testing.T) {
	entries := sortedEntries{
		{Member: "a", Score: 3.0},
		{Member: "b", Score: 1.0},
		{Member: "c", Score: 2.0},
	}

	if entries.Len() != 3 {
		t.Errorf("expected len 3, got %d", entries.Len())
	}

	if !entries.Less(1, 2) {
		t.Error("b (1.0) should be less than c (2.0)")
	}

	entries.Swap(0, 1)
	if entries[0].Member != "b" {
		t.Error("swap should work")
	}
}
