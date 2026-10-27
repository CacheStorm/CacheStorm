package store

import (
	"testing"
	"time"
)

// A write looks its key up the way Redis' lookupKeyWrite does: it reaps an
// expired key and refreshes the access time, but counts neither a keyspace
// hit nor a miss.
func TestWriteLookupDoesNotCountKeyspaceHits(t *testing.T) {
	s := NewStore()
	hits, misses := GlobalMetrics.TotalHits.Load(), GlobalMetrics.TotalMisses.Load()
	for i := 0; i < 3; i++ {
		if _, err := s.ApplyHashPairs("h", false, [][]byte{[]byte("f"), []byte("v")}); err != nil {
			t.Fatal(err)
		}
	}
	if h, m := GlobalMetrics.TotalHits.Load()-hits, GlobalMetrics.TotalMisses.Load()-misses; h != 0 || m != 0 {
		t.Fatalf("HSET counted %d hits and %d misses, want none", h, m)
	}

	entry, _ := s.Get("h")
	before := entry.LastAccess.Load()
	time.Sleep(2 * time.Millisecond)
	if _, err := s.ApplyHashPairs("h", false, [][]byte{[]byte("f"), []byte("w")}); err != nil {
		t.Fatal(err)
	}
	if after := entry.LastAccess.Load(); after <= before {
		t.Fatalf("a write left the access time at %d, want it refreshed", after)
	}

	expirations := GlobalMetrics.TotalExpirations.Load()
	s.SetTTL("h", time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	if _, err := s.ApplyHashPairs("h", false, [][]byte{[]byte("g"), []byte("v")}); err != nil {
		t.Fatal(err)
	}
	if GlobalMetrics.TotalExpirations.Load() != expirations+1 {
		t.Fatal("writing to an expired key did not count an expiration")
	}
	if v, ok := s.Get("h"); !ok || len(v.Value.(*HashValue).Fields) != 1 {
		t.Fatal("writing to an expired key did not start a fresh hash")
	}
}

// The pooled HSET plan must not carry state between calls, including after a
// call with more pairs than its inline buffers hold.
func TestApplyHashPairsPooledPlanIsClean(t *testing.T) {
	s := NewStore()
	var many [][]byte
	for i := 0; i < 20; i++ {
		many = append(many, []byte{'a' + byte(i)}, []byte("x"))
	}
	many = append(many, []byte("a"), []byte("last"))
	added, err := s.ApplyHashPairs("big", false, many)
	if err != nil || added != 20 {
		t.Fatalf("added %d, %v; want 20", added, err)
	}
	if v, _ := s.Get("big"); string(v.Value.(*HashValue).Fields["a"]) != "last" {
		t.Fatal("a repeated field did not take its last value")
	}
	added, err = s.ApplyHashPairs("small", false, [][]byte{[]byte("f"), []byte("1")})
	if err != nil || added != 1 {
		t.Fatalf("added %d, %v; want 1", added, err)
	}
	if v, _ := s.Get("small"); len(v.Value.(*HashValue).Fields) != 1 {
		t.Fatalf("small hash has %d fields, want 1", len(v.Value.(*HashValue).Fields))
	}
}
