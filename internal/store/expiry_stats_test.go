package store

import (
	"math/rand"
	"strconv"
	"testing"
	"time"
)

// The per-shard expiry counters agree with a recount after sets with and
// without TTLs, EXPIRE, PERSIST, deletes, renames and flushes.
func TestExpiryStatsMatchRecount(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	s := NewStore()
	recount := func() (n int64) {
		for _, sh := range &s.shards {
			sh.mu.RLock()
			for _, e := range sh.data {
				if e.ExpiresAt.Load() > 0 {
					n++
				}
			}
			sh.mu.RUnlock()
		}
		return n
	}
	now := time.Now()
	for step := 0; step < 5000; step++ {
		k := "k" + strconv.Itoa(rng.Intn(300))
		switch rng.Intn(7) {
		case 0:
			_ = s.Set(k, &StringValue{Data: []byte("v")}, SetOptions{})
		case 1:
			_ = s.Set(k, &StringValue{Data: []byte("v")}, SetOptions{TTL: time.Hour})
		case 2:
			_ = s.Set(k, &StringValue{Data: []byte("w")}, SetOptions{KeepTTL: true})
		case 3:
			s.SetExpiresAt(k, now.Add(time.Duration(rng.Intn(1000))*time.Second).UnixMilli())
		case 4:
			s.Persist(k)
		case 5:
			s.Delete(k)
		default:
			_, _ = s.Transfer(k, s, "k"+strconv.Itoa(rng.Intn(300)), TransferOptions{Replace: true})
		}
		if step%250 == 0 {
			if got, _ := s.ExpiryStats(); got != recount() {
				t.Fatalf("step %d: expires %d, recount %d", step, got, recount())
			}
		}
	}
	if _, avg := s.ExpiryStats(); avg <= 0 || avg > int64(time.Hour/time.Millisecond) {
		t.Fatalf("avg_ttl %d ms", avg)
	}
	s.Flush()
	if got, avg := s.ExpiryStats(); got != 0 || avg != 0 {
		t.Fatalf("after FLUSH: %d %d", got, avg)
	}
}
