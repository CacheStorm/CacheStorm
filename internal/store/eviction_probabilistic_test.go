package store

import (
	"testing"
	"time"
)

func TestNewEvictionController(t *testing.T) {
	s := NewStore()
	mt := NewMemoryTracker(1024*1024, 80, 90)
	ec := NewEvictionController(EvictionAllKeysLRU, 1024*1024, s, mt, 10)
	if ec == nil {
		t.Fatal("expected EvictionController")
	}
}

func TestEvictionControllerSetOnEvict(t *testing.T) {
	s := NewStore()
	mt := NewMemoryTracker(1024*1024, 80, 90)
	ec := NewEvictionController(EvictionAllKeysLRU, 1024*1024, s, mt, 10)

	ec.SetOnEvict(func(key string, entry *Entry) {})

	if ec.onEvict == nil {
		t.Error("expected onEvict to be set")
	}
}

func TestEvictionControllerCheckAndEvictNoMemory(t *testing.T) {
	s := NewStore()
	mt := NewMemoryTracker(1024*1024, 80, 90)
	ec := NewEvictionController(EvictionAllKeysLRU, 0, s, mt, 10)

	err := ec.CheckAndEvict()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEvictionControllerEvictKeys(t *testing.T) {
	s := NewStore()
	for i := 0; i < 100; i++ {
		s.Set("key"+string(rune(i)), &StringValue{Data: []byte("value")}, SetOptions{})
	}

	mt := NewMemoryTracker(1024*1024, 80, 90)
	ec := NewEvictionController(EvictionAllKeysLRU, 1024, s, mt, 50)

	ec.evictKeys(10)

	keys := s.Keys()
	if len(keys) >= 100 {
		t.Errorf("expected keys to be evicted, got %d keys", len(keys))
	}
}

func TestEvictionControllerSelectVictimLRU(t *testing.T) {
	s := NewStore()
	for i := 0; i < 100; i++ {
		s.Set("key"+string(rune(i)), &StringValue{Data: []byte("value")}, SetOptions{})
	}

	mt := NewMemoryTracker(1024*1024, 80, 90)
	ec := NewEvictionController(EvictionAllKeysLRU, 1024, s, mt, 50)

	victim := ec.selectVictim()
	if victim == "" {
		t.Error("expected victim key")
	}
}

func TestEvictionControllerSelectVictimLFU(t *testing.T) {
	s := NewStore()
	for i := 0; i < 100; i++ {
		s.Set("key"+string(rune(i)), &StringValue{Data: []byte("value")}, SetOptions{})
	}

	mt := NewMemoryTracker(1024*1024, 80, 90)
	ec := NewEvictionController(EvictionAllKeysLFU, 1024, s, mt, 50)

	victim := ec.selectVictim()
	if victim == "" {
		t.Error("expected victim key")
	}
}

func TestEvictionControllerSelectVictimRandom(t *testing.T) {
	s := NewStore()
	for i := 0; i < 100; i++ {
		s.Set("key"+string(rune(i)), &StringValue{Data: []byte("value")}, SetOptions{})
	}

	mt := NewMemoryTracker(1024*1024, 80, 90)
	ec := NewEvictionController(EvictionAllKeysRandom, 1024, s, mt, 50)

	victim := ec.selectVictim()
	if victim == "" {
		t.Error("expected victim key")
	}
}

func TestEvictionControllerSelectVictimVolatileLRU(t *testing.T) {
	s := NewStore()
	for i := 0; i < 100; i++ {
		s.Set("key"+string(rune(i)), &StringValue{Data: []byte("value")}, SetOptions{TTL: time.Minute})
	}

	mt := NewMemoryTracker(1024*1024, 80, 90)
	ec := NewEvictionController(EvictionVolatileLRU, 1024, s, mt, 50)

	victim := ec.selectVictim()
	if victim == "" {
		t.Error("expected victim key")
	}
}

func TestEvictionControllerSelectVictimNoEviction(t *testing.T) {
	s := NewStore()
	s.Set("key1", &StringValue{Data: []byte("value1")}, SetOptions{})

	mt := NewMemoryTracker(1024*1024, 80, 90)
	ec := NewEvictionController(EvictionNoEviction, 1024, s, mt, 10)

	victim := ec.selectVictim()
	if victim != "" {
		t.Errorf("expected no victim for no-eviction policy, got %s", victim)
	}
}

func TestEvictionControllerSampleKeysEmpty(t *testing.T) {
	s := NewStore()
	mt := NewMemoryTracker(1024*1024, 80, 90)
	ec := NewEvictionController(EvictionAllKeysLRU, 1024, s, mt, 10)

	candidates := ec.sampleKeys()
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for empty store, got %d", len(candidates))
	}
}

func TestEvictionControllerForceEvict(t *testing.T) {
	s := NewStore()
	for i := 0; i < 100; i++ {
		s.Set("key"+string(rune(i)), &StringValue{Data: []byte("value")}, SetOptions{})
	}

	mt := NewMemoryTracker(1024*1024, 80, 90)
	ec := NewEvictionController(EvictionAllKeysLRU, 1024, s, mt, 50)

	evicted := ec.ForceEvict(1)
	if evicted < 0 {
		t.Errorf("expected non-negative evicted, got %d", evicted)
	}
}

func TestNewBloomFilter(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)
	if bf == nil {
		t.Fatal("expected BloomFilter")
	}
	if bf.size != 1000 {
		t.Errorf("expected size 1000, got %d", bf.size)
	}
}

func TestBloomFilterAdd(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)
	result := bf.Add([]byte("test"))
	if !result {
		t.Error("expected Add to return true")
	}
}

func TestBloomFilterExists(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)
	bf.Add([]byte("test"))

	if !bf.Exists([]byte("test")) {
		t.Error("expected Exists to return true for added item")
	}

	if bf.Exists([]byte("nonexistent")) {
		t.Error("expected Exists to return false for non-existent item")
	}
}

func TestBloomFilterCount(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)
	bf.Add([]byte("test1"))
	bf.Add([]byte("test2"))

	if bf.Count() != 2 {
		t.Errorf("expected count 2, got %d", bf.Count())
	}
}

func TestBloomFilterInfo(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)
	bf.Add([]byte("test"))

	info := bf.Info()
	if info == nil {
		t.Fatal("expected info")
	}
	if info["size"] != uint(1000) {
		t.Errorf("expected size 1000, got %v", info["size"])
	}
}

func TestBloomFilterClear(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)
	bf.Add([]byte("test"))
	bf.Clear()

	if bf.Count() != 0 {
		t.Errorf("expected count 0 after clear, got %d", bf.Count())
	}
}

func TestNewCountMinSketch(t *testing.T) {
	cms := NewCountMinSketch(5, 100)
	if cms == nil {
		t.Fatal("expected CountMinSketch")
	}
	if cms.depth != 5 {
		t.Errorf("expected depth 5, got %d", cms.depth)
	}
}

func TestCountMinSketchAdd(t *testing.T) {
	cms := NewCountMinSketch(5, 100)
	count := cms.Add([]byte("test"), 1)
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
}

func TestCountMinSketchCount(t *testing.T) {
	cms := NewCountMinSketch(5, 100)
	cms.Add([]byte("test"), 5)
	cms.Add([]byte("test"), 3)

	count := cms.Count([]byte("test"))
	if count < 8 {
		t.Errorf("expected count >= 8, got %d", count)
	}
}

func TestCountMinSketchInfo(t *testing.T) {
	cms := NewCountMinSketch(5, 100)
	cms.Add([]byte("test"), 1)

	info := cms.Info()
	if info == nil {
		t.Fatal("expected info")
	}
	if info["depth"] != uint(5) {
		t.Errorf("expected depth 5, got %v", info["depth"])
	}
}

func TestCountMinSketchClear(t *testing.T) {
	cms := NewCountMinSketch(5, 100)
	cms.Add([]byte("test"), 5)
	cms.Clear()

	count := cms.Count([]byte("test"))
	if count != 0 {
		t.Errorf("expected count 0 after clear, got %d", count)
	}
}

func TestNewTopK(t *testing.T) {
	tk := NewTopK(10)
	if tk == nil {
		t.Fatal("expected TopK")
	}
	if tk.k != 10 {
		t.Errorf("expected k 10, got %d", tk.k)
	}
}

func TestTopKAdd(t *testing.T) {
	tk := NewTopK(10)
	count := tk.Add("item1", 5)
	if count != 5 {
		t.Errorf("expected count 5, got %d", count)
	}
}

func TestTopKQuery(t *testing.T) {
	tk := NewTopK(10)
	tk.Add("item1", 5)
	tk.Add("item1", 3)

	count := tk.Query("item1")
	if count != 8 {
		t.Errorf("expected count 8, got %d", count)
	}
}

func TestTopKQueryNonExistent(t *testing.T) {
	tk := NewTopK(10)
	count := tk.Query("nonexistent")
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

func TestTopKList(t *testing.T) {
	tk := NewTopK(3)
	tk.Add("item1", 10)
	tk.Add("item2", 5)
	tk.Add("item3", 8)
	tk.Add("item4", 3)

	list := tk.List()
	if len(list) != 3 {
		t.Errorf("expected 3 items, got %d", len(list))
	}
	if list[0] != "item1" {
		t.Errorf("expected first item to be 'item1', got %s", list[0])
	}
}

func TestTopKListWithCount(t *testing.T) {
	tk := NewTopK(3)
	tk.Add("item1", 10)
	tk.Add("item2", 5)

	list := tk.ListWithCount()
	if len(list) != 2 {
		t.Errorf("expected 2 items, got %d", len(list))
	}
	if list[0]["item"] != "item1" {
		t.Errorf("expected first item to be 'item1', got %v", list[0]["item"])
	}
	if list[0]["count"] != uint64(10) {
		t.Errorf("expected count 10, got %v", list[0]["count"])
	}
}

func TestTopKInfo(t *testing.T) {
	tk := NewTopK(10)
	tk.Add("item1", 5)

	info := tk.Info()
	if info == nil {
		t.Fatal("expected info")
	}
	if info["k"] != 10 {
		t.Errorf("expected k 10, got %v", info["k"])
	}
}

func TestTopKClear(t *testing.T) {
	tk := NewTopK(10)
	tk.Add("item1", 5)
	tk.Clear()

	count := tk.Query("item1")
	if count != 0 {
		t.Errorf("expected count 0 after clear, got %d", count)
	}
}

func TestNewCuckooFilter(t *testing.T) {
	cf := NewCuckooFilter(1000, 4)
	if cf == nil {
		t.Fatal("expected CuckooFilter")
	}
	if cf.size != 1000 {
		t.Errorf("expected size 1000, got %d", cf.size)
	}
}

func TestCuckooFilterAdd(t *testing.T) {
	cf := NewCuckooFilter(1000, 4)
	result := cf.Add([]byte("test"))
	if !result {
		t.Error("expected Add to return true")
	}
}

func TestCuckooFilterExists(t *testing.T) {
	cf := NewCuckooFilter(1000, 4)
	cf.Add([]byte("test"))

	if !cf.Exists([]byte("test")) {
		t.Error("expected Exists to return true for added item")
	}

	if cf.Exists([]byte("nonexistent")) {
		t.Error("expected Exists to return false for non-existent item")
	}
}

func TestCuckooFilterDelete(t *testing.T) {
	cf := NewCuckooFilter(1000, 4)
	cf.Add([]byte("test"))

	result := cf.Delete([]byte("test"))
	if !result {
		t.Error("expected Delete to return true")
	}

	if cf.Exists([]byte("test")) {
		t.Error("expected Exists to return false after delete")
	}
}

func TestCuckooFilterDeleteNonExistent(t *testing.T) {
	cf := NewCuckooFilter(1000, 4)
	result := cf.Delete([]byte("nonexistent"))
	if result {
		t.Error("expected Delete to return false for non-existent item")
	}
}

func TestCuckooFilterCount(t *testing.T) {
	cf := NewCuckooFilter(1000, 4)
	cf.Add([]byte("test1"))
	cf.Add([]byte("test2"))

	if cf.Count() != 2 {
		t.Errorf("expected count 2, got %d", cf.Count())
	}
}

func TestCuckooFilterInfo(t *testing.T) {
	cf := NewCuckooFilter(1000, 4)
	cf.Add([]byte("test"))

	info := cf.Info()
	if info == nil {
		t.Fatal("expected info")
	}
	if info["size"] != uint(1000) {
		t.Errorf("expected size 1000, got %v", info["size"])
	}
}

func TestEvictionPolicyConstants(t *testing.T) {
	if EvictionNoEviction != 0 {
		t.Errorf("expected EvictionNoEviction = 0, got %d", EvictionNoEviction)
	}
	if EvictionAllKeysLRU != 1 {
		t.Errorf("expected EvictionAllKeysLRU = 1, got %d", EvictionAllKeysLRU)
	}
}
