package store

import (
	"sync"
	"testing"
	"time"
)

// expireRecorder stands in for a registered plugin consumer: the server bridges
// StoreHooks.OnExpire to plugin.Manager.RunExpireHooks (internal/server), so any
// plugin observing expirations depends on this hook firing.
type expireRecorder struct {
	mu    sync.Mutex
	fired []string
}

func (r *expireRecorder) onExpire(key string, _ interface{}) {
	r.mu.Lock()
	r.fired = append(r.fired, key)
	r.mu.Unlock()
}

func (r *expireRecorder) saw(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, k := range r.fired {
		if k == key {
			return true
		}
	}
	return false
}

func (r *expireRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.fired)
}

// newHookedStore mirrors how internal/server wires the store: SetHooks installs
// the callbacks that feed the plugin dispatchers.
func newHookedStore() (*Store, *expireRecorder) {
	s := NewStore()
	rec := &expireRecorder{}
	s.SetHooks(StoreHooks{OnExpire: rec.onExpire})
	return s, rec
}

// seedLapsing writes key with a TTL and waits for it to lapse. Active expiry is
// deliberately NOT started, so the expired entry is deterministically still in
// the shard — the production window between a TTL lapsing and the wheel reaching
// the key's slot.
func seedLapsing(t *testing.T, s *Store, key string) {
	t.Helper()
	if err := s.Set(key, &StringValue{Data: []byte("v")}, SetOptions{TTL: 20 * time.Millisecond}); err != nil {
		t.Fatalf("seed %q: %v", key, err)
	}
	time.Sleep(80 * time.Millisecond)
}

// Store.Exists removes an expired key (shard.Delete, trackMemory, DeleteVersion,
// tagIndex.RemoveKey) but was the only reaping path that did not announce the
// expiry. DeleteIfExpired and Get's inline lazy expiry both call fireExpire, so
// the same event was signalled on two paths and silently dropped on the third —
// a registered plugin never learned the key had expired.
func TestStoreExistsExpiredKeyFiresOnExpireHook(t *testing.T) {
	s, rec := newHookedStore()
	seedLapsing(t, s, "ekey")

	if s.Exists("ekey") {
		t.Fatal("setup: the key should have expired")
	}
	if _, present := s.GetShard("ekey").Get("ekey"); present {
		t.Fatal("setup: Exists should have reaped the entry from the shard")
	}

	if !rec.saw("ekey") {
		t.Error("EXISTS reaped an expired key without firing OnExpire; every other " +
			"reaping path announces it, so plugins never learn of this expiration")
	}
}

// Control: EXISTS on a LIVE key must report true and must NOT fire OnExpire.
// The fix must only announce a real expiry, not ordinary existence checks.
func TestStoreExistsLiveKeyDoesNotFireOnExpire(t *testing.T) {
	s, rec := newHookedStore()
	if err := s.Set("live", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if !s.Exists("live") {
		t.Fatal("EXISTS on a live key must report true")
	}
	if rec.count() != 0 {
		t.Errorf("EXISTS on a live key fired OnExpire %d time(s); only a real expiry may announce", rec.count())
	}

	// Control: an absent key must not announce anything either.
	if s.Exists("nope") {
		t.Error("EXISTS on an absent key must report false")
	}
	if rec.count() != 0 {
		t.Errorf("EXISTS on an absent key fired OnExpire %d time(s)", rec.count())
	}
}

// The documented precedents must keep behaving: Get's inline lazy expiry and
// DeleteIfExpired both announce, and each announces exactly once per reaped key.
func TestEveryReapingPathFiresOnExpireExactlyOnce(t *testing.T) {
	t.Run("Get lazy expiry", func(t *testing.T) {
		s, rec := newHookedStore()
		seedLapsing(t, s, "gk")
		if _, ok := s.Get("gk"); ok {
			t.Fatal("setup: key should have expired")
		}
		if !rec.saw("gk") {
			t.Error("Get lazy expiry must fire OnExpire")
		}
		if got := rec.count(); got != 1 {
			t.Errorf("Get lazy expiry fired OnExpire %d times, want 1", got)
		}
	})

	t.Run("DeleteIfExpired", func(t *testing.T) {
		s, rec := newHookedStore()
		seedLapsing(t, s, "dk")
		if !s.DeleteIfExpired("dk") {
			t.Fatal("setup: DeleteIfExpired should remove the lapsed key")
		}
		if !rec.saw("dk") {
			t.Error("DeleteIfExpired must fire OnExpire")
		}
		if got := rec.count(); got != 1 {
			t.Errorf("DeleteIfExpired fired OnExpire %d times, want 1", got)
		}
	})

	t.Run("Exists reaps once", func(t *testing.T) {
		s, rec := newHookedStore()
		seedLapsing(t, s, "xk")
		s.Exists("xk") // reaps and announces
		s.Exists("xk") // already gone: must not announce again
		if got := rec.count(); got != 1 {
			t.Errorf("Exists announced %d times, want exactly 1 (the second call finds no entry)", got)
		}
	})
}
