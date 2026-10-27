package store

import (
	"strconv"
	"testing"
	"time"
)

// A key due shortly after the level-0 pointer's next advance must still be
// swept: it was placed in the slot being left and waited an hour.
func TestTimingWheelKeyDueAfterSlotBoundaryExpires(t *testing.T) {
	s := NewStore()
	s.StartExpiry()
	defer s.StopExpiry()
	for i := 0; i < 12; i++ {
		key := "k" + strconv.Itoa(i)
		if err := s.Set(key, &StringValue{Data: []byte("v")}, SetOptions{TTL: 250 * time.Millisecond}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(85 * time.Millisecond)
	}
	deadline := time.Now().Add(2 * time.Second)
	for s.KeyCount() > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d keys were not actively expired", s.KeyCount())
		}
		time.Sleep(20 * time.Millisecond)
	}
}
