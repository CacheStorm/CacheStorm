package store

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestTimingWheelFarFutureCallbacksReleaseSchedulingLock(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("expired-%d", count), func(t *testing.T) {
			s := NewStore()
			for _, key := range []string{"callback", "persisted", "rescheduled"} {
				e := NewEntry(&StringValue{Data: []byte("v")})
				if key == "rescheduled" {
					e.ExpiresAt = math.MaxInt64
				}
				s.SetEntry(key, e)
			}
			for _, key := range []string{"persisted", "rescheduled"} {
				s.expiry.farFuture.keys[key] = 1
			}
			for i := 0; i < count; i++ {
				key := fmt.Sprintf("expired:%d", i)
				e := NewEntry(&StringValue{Data: []byte("v")})
				e.ExpiresAt = 1
				s.SetEntry(key, e)
				s.expiry.farFuture.keys[key] = 1
			}
			entered := make(chan bool)
			release := make(chan struct{})
			results := make(chan bool, count)
			done := make(chan struct{})
			s.SetHooks(StoreHooks{OnExpire: func(key string, value interface{}) {
				available := s.expiry.mu.TryLock()
				if available {
					s.expiry.mu.Unlock()
				}
				entered <- available
				<-release
				rescheduled := false
				if available {
					rescheduled = s.SetTTL("callback", time.Hour)
					s.expiry.Remove(key)
				}
				results <- rescheduled
			}})
			go func() {
				s.expiry.cleanupFarFuture()
				close(done)
			}()
			for i := 0; i < count; i++ {
				available := <-entered
				if available {
					s.expiry.Add("queued-during-callback", math.MaxInt64)
					s.expiry.Remove("queued-during-callback")
				} else {
					t.Error("scheduling mutex held during callback")
				}
				release <- struct{}{}
			}
			<-done
			for i := 0; i < count; i++ {
				if !<-results {
					t.Error("callback did not reschedule TTL")
				}
			}
			if s.KeyCount() != 3 {
				t.Fatalf("keys=%d want=3", s.KeyCount())
			}
			if !s.expiry.mu.TryLock() {
				t.Fatal("cleanup leaked the scheduling lock")
			}
			s.expiry.mu.Unlock()
			s.expiry.cleanupFarFuture()
		})
	}
}
