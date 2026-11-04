package store

import (
	"math"
	"testing"
	"time"
)

func TestExpiryMillisecondsFarFuture(t *testing.T) {
	s := NewStore()
	for _, key := range []string{"a", "b", "c"} {
		if err := s.Set(key, &StringValue{Data: []byte("v")}, SetOptions{ExpireAtMS: math.MaxInt64}); err != nil {
			t.Fatal(err)
		}
	}
	s.expiry.cycle()
	for _, key := range []string{"a", "b", "c"} {
		if _, ok := s.shards[s.shardIndex(key)].vol[key]; !ok {
			t.Errorf("%s is not among the keys with an expiry", key)
		}
	}
	before := time.Now().UnixMilli()
	n, average := s.ExpiryStats()
	after := time.Now().UnixMilli()
	if n != 3 || average < math.MaxInt64-after || average > math.MaxInt64-before {
		t.Fatalf("ExpiryStats = (%d, %d), want (3, [%d, %d])", n, average, math.MaxInt64-after, math.MaxInt64-before)
	}
	if got := s.TTL("a"); got != time.Duration(math.MaxInt64) {
		t.Fatalf("duration TTL = %v, want saturated duration", got)
	}
	if !s.Persist("a") || !s.SetExpiresAt("b", 1) {
		t.Fatal("could not update expiry")
	}
	s.expiry.cycle()
	if !s.Exists("a") || s.Exists("b") {
		t.Fatal("active expiry must preserve persisted keys and reap expired keys")
	}
}
