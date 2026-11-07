package store

import (
	"testing"
	"time"
)

// GetBytes finds what Get finds: live keys directly, and through Get's path
// an expired key, which is reaped and missed.
func TestGetBytesAgreesWithGet(t *testing.T) {
	s := NewStore()
	if err := s.Set("live", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("gone", &StringValue{Data: []byte("v")}, SetOptions{TTL: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if e, ok := s.GetBytes([]byte("live")); !ok || string(e.Value.(*StringValue).Data) != "v" {
		t.Fatalf("live: %v %v", e, ok)
	}
	if _, ok := s.GetBytes([]byte("missing")); ok {
		t.Fatal("missing key found")
	}
	if _, ok := s.GetBytes([]byte("gone")); ok {
		t.Fatal("expired key found")
	}
	if s.Exists("gone") {
		t.Fatal("expired key not reaped")
	}
	key := []byte("live")
	if allocs := testing.AllocsPerRun(100, func() { s.GetBytes(key) }); allocs != 0 {
		t.Fatalf("GetBytes of a live key allocated %v times", allocs)
	}
}
