package store

import (
	"encoding/json"
	"testing"
)

func TestLRUStatisticsSerializeAtZeroCapacity(t *testing.T) {
	for _, capacity := range []int{0, -1, 1, 2} {
		cache := NewLRUCache(capacity)
		for i := 0; i < 3; i++ {
			stats := cache.Stats()
			if _, err := json.Marshal(stats); err != nil {
				t.Fatal(err)
			}
			want := float64(0)
			if capacity > 0 {
				want = float64(cache.Size) / float64(capacity) * 100
			}
			if stats["usage"].(float64) != want {
				t.Fatalf("usage=%v want=%v", stats["usage"], want)
			}
			cache.Set(string(rune('a'+i)), "v")
		}
		cache.Clear()
		if cache.Stats()["usage"].(float64) != 0 {
			t.Fatal("clear left nonzero usage")
		}
	}
	t.Log("FIX VERIFIED")
}
