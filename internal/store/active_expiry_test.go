package store

import (
	"strconv"
	"testing"
	"time"
)

func volatileKeys(s *Store) int {
	n := 0
	for _, sh := range s.shards {
		sh.mu.RLock()
		n += len(sh.vol)
		sh.mu.RUnlock()
	}
	return n
}

// A key given a new expiry is still held once: the timing wheel kept an
// entry per schedule, which grew without bound under rewrites.
func TestRescheduledKeyIsHeldOnce(t *testing.T) {
	s := NewStore()
	for i := 0; i < 1000; i++ {
		if err := s.Set("k", &StringValue{Data: []byte("v")}, SetOptions{TTL: time.Duration(1000+i) * time.Second}); err != nil {
			t.Fatal(err)
		}
		s.SetExpiresAt("k", time.Now().Add(time.Hour).UnixMilli()+int64(i))
	}
	if n := volatileKeys(s); n != 1 {
		t.Fatalf("%d keys with an expiry held, want 1", n)
	}
	s.Persist("k")
	if n := volatileKeys(s); n != 0 {
		t.Fatalf("%d held after PERSIST", n)
	}
	s.Set("k", &StringValue{Data: []byte("v")}, SetOptions{TTL: time.Hour})
	s.Delete("k")
	if n := volatileKeys(s); n != 0 {
		t.Fatalf("%d held after DEL", n)
	}
	s.Set("k", &StringValue{Data: []byte("v")}, SetOptions{TTL: time.Hour})
	s.Set("k", &StringValue{Data: []byte("v")}, SetOptions{})
	if n := volatileKeys(s); n != 0 {
		t.Fatalf("%d held after a SET without expiry", n)
	}
}

// One cycle keeps sampling a shard while much of what it finds is due, so a
// burst of keys falling due together goes within a few cycles, and keys not
// yet due stay.
func TestActiveExpiryCycleClearsABurst(t *testing.T) {
	s := NewStore()
	for i := 0; i < 20000; i++ {
		if err := s.Set("due"+strconv.Itoa(i), &StringValue{Data: []byte("v")}, SetOptions{TTL: time.Millisecond}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 500; i++ {
		if err := s.Set("live"+strconv.Itoa(i), &StringValue{Data: []byte("v")}, SetOptions{TTL: time.Hour}); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(5 * time.Millisecond)
	for i := 0; i < 20 && s.KeyCount() > 500; i++ {
		s.expiry.cycle()
	}
	if n := s.KeyCount(); n != 500 {
		t.Fatalf("%d keys after the cycles, want the 500 not yet due", n)
	}
	if n := volatileKeys(s); n != 500 {
		t.Fatalf("%d keys with an expiry held, want 500", n)
	}
}

func TestFlushForgetsKeysWithAnExpiry(t *testing.T) {
	s := NewStore()
	for i := 0; i < 100; i++ {
		s.Set("k"+strconv.Itoa(i), &StringValue{Data: []byte("v")}, SetOptions{TTL: time.Hour})
	}
	s.Flush()
	if n := volatileKeys(s); n != 0 {
		t.Fatalf("%d held after a flush", n)
	}
}
