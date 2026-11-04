package store

import (
	"strconv"
	"testing"
	"time"
)

// Keys due at staggered times are each removed soon after they are due.
func TestActiveExpiryRemovesKeysAsTheyFallDue(t *testing.T) {
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
