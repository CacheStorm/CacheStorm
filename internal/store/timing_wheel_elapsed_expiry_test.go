package store

import (
	"math"
	"testing"
)

func TestTimingWheelSchedulesElapsedDeadlines(t *testing.T) {
	for _, state := range []struct {
		name    string
		current int
		subtick int
	}{{"first-sweep", 0, 0}, {"slot-boundary", 42, 9}, {"hour-wrap", 3599, 9}} {
		t.Run(state.name, func(t *testing.T) {
			s := NewStore()
			s.ConfigureMemory(1<<20, EvictionNoEviction, 70, 85, 5)
			if err := s.Set("control", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
				t.Fatal(err)
			}
			baseline := s.MemoryTracker().Usage()
			if err := s.Set("expired", &StringValue{Data: []byte("payload")}, SetOptions{Tags: []string{"expired-tag"}}); err != nil {
				t.Fatal(err)
			}
			s.expiry.levels[0].current = state.current
			s.expiry.subtick = state.subtick
			if !s.SetExpiresAt("expired", 1) {
				t.Fatal("expiry setup failed")
			}
			calls := 0
			s.SetHooks(StoreHooks{OnExpire: func(key string, value interface{}) { calls++ }})
			s.expiry.Add("expired", 1)
			s.expiry.Add("missing", 1)
			s.expiry.tick()
			if s.KeyCount() != 1 || !s.shards[s.shardIndex("control")].Exists("control") || calls != 1 {
				t.Fatalf("elapsed expiry failed: keys=%d calls=%d", s.KeyCount(), calls)
			}
			if s.MemoryTracker().Usage() != baseline || len(s.GetTagIndex().GetKeys("expired-tag")) != 0 {
				t.Fatal("expiry did not clean up memory and tag membership")
			}
			s.expiry.tick()
			if calls != 1 {
				t.Fatal("duplicate schedule duplicated the callback")
			}
		})
	}
	for _, mode := range []string{"set-entry", "persisted", "rescheduled", "replaced"} {
		t.Run(mode, func(t *testing.T) {
			s := NewStore()
			e := NewEntry(&StringValue{Data: []byte("old")})
			e.ExpiresAt = 1
			s.SetEntry("key", e)
			if mode != "set-entry" {
				e = NewEntry(&StringValue{Data: []byte("live")})
				if mode == "rescheduled" || mode == "persisted" {
					e.ExpiresAt = math.MaxInt64
				}
				s.SetEntry("key", e)
				if mode == "persisted" && !s.Persist("key") {
					t.Fatal("persist failed")
				}
			}
			s.expiry.tick()
			want := int64(1)
			if mode == "set-entry" {
				want = 0
			}
			if s.KeyCount() != want {
				t.Fatalf("keys=%d want=%d", s.KeyCount(), want)
			}
		})
	}
}
