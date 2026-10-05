package store

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func checkAudit50R23X(t *testing.T, label string, want, got interface{}) {
	t.Helper()
	t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s mismatch", label)
	}
}

func TestAudit50R23X(t *testing.T) {
	defer func() {
		if t.Failed() {
			t.Log("PROBLEM CONFIRMED")
		} else {
			t.Log("FIX VERIFIED")
		}
	}()

	s := NewStore()
	rng := rand.New(rand.NewSource(1))
	first, second := rng.Intn(NumShards), rng.Intn(NumShards)
	key := ""
	for i := 0; i < NumShards*4; i++ {
		candidate := fmt.Sprintf("volatile-%d", i)
		idx := int(s.shardIndex(candidate))
		if idx != first && idx != second {
			key = candidate
			break
		}
	}
	if key == "" {
		t.Fatal("harness cannot locate distinct shard")
	}
	s.Set(key, &StringValue{Data: []byte("v")}, SetOptions{})
	entry, _ := s.GetShard(key).Get(key)
	entry.ExpiresAt = 1<<63 - 1
	mt := NewMemoryTracker(1000, 80, 90)
	control := NewEvictionController(EvictionAllKeysRandom, 1000, s, mt, 1)
	control.rnd = rand.New(rand.NewSource(1))
	checkAudit50R23X(t, "control sparse random selection", key, control.selectVictim())
	ec := NewEvictionController(EvictionVolatileLRU, 1000, s, mt, 1)
	ec.rnd = rand.New(rand.NewSource(1))
	checkAudit50R23X(t, "sparse volatile selection", key, ec.selectVictim())

	persistent := ""
	for i := 0; i < NumShards*4; i++ {
		candidate := fmt.Sprintf("persistent-%d", i)
		if s.shardIndex(candidate) == s.shardIndex(key) {
			persistent = candidate
			break
		}
	}
	if persistent == "" {
		t.Fatal("harness no matching shard")
	}
	s.Set(persistent, &StringValue{Data: []byte("keep")}, SetOptions{})
	for i := int64(0); i < 5; i++ {
		ec.rnd = rand.New(rand.NewSource(i))
		checkAudit50R23X(t, "persistent neighbor skipped", key, ec.selectVictim())
	}
	s.Delete(key)
	checkAudit50R23X(t, "persistent-only no victim", "", ec.selectVictim())
	s.Delete(persistent)
	checkAudit50R23X(t, "empty store", "", ec.selectVictim())
	no := NewEvictionController(EvictionNoEviction, 1000, s, mt, 1)
	checkAudit50R23X(t, "no eviction policy", "", no.selectVictim())

}
