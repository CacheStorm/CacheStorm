package store

import "testing"

func TestRecordCommandInSlot(t *testing.T) {
	m := NewMetrics()
	var slot CommandSlot
	count := func(cmd string) int64 {
		st := m.GetCommandStats(cmd)
		if st == nil {
			return 0
		}
		return st["count"].(int64)
	}
	m.RecordCommandIn(&slot, "SET", 10)
	m.RecordCommandIn(&slot, "SET", 10)
	m.RecordCommandIn(&slot, "GET", 10)
	m.RecordCommandIn(&slot, "SET", 10)
	if count("SET") != 3 || count("GET") != 1 {
		t.Fatalf("SET=%d GET=%d, want 3 and 1", count("SET"), count("GET"))
	}
	m.Reset()
	m.RecordCommandIn(&slot, "SET", 10)
	if count("SET") != 1 {
		t.Fatalf("SET after reset = %d, want 1", count("SET"))
	}
	if m.TotalCommands.Load() != 1 {
		t.Fatalf("total = %d, want 1", m.TotalCommands.Load())
	}
}
