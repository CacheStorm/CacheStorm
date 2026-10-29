package server

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

// opsSampleInterval and opsSamples set the window instantaneous_ops_per_sec
// and the instantaneous kbps figures average over, as Redis' 16 samples taken
// every 100ms do.
const (
	opsSampleInterval = 100 * time.Millisecond
	opsSamples        = 16
)

// rateSampler keeps the per-second rates INFO reports as instantaneous_*.
type rateSampler struct {
	mu      sync.Mutex
	last    [3]int64
	lastAt  time.Time
	samples [opsSamples][3]float64
	next    int
	filled  int
}

func (r *rateSampler) sample(now time.Time) {
	m := store.GlobalMetrics
	cur := [3]int64{m.TotalCommands.Load(), m.TotalBytesIn.Load(), m.TotalBytesOut.Load()}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.lastAt.IsZero() {
		secs := now.Sub(r.lastAt).Seconds()
		if secs > 0 {
			for i := range cur {
				r.samples[r.next][i] = float64(cur[i]-r.last[i]) / secs
			}
			r.next = (r.next + 1) % opsSamples
			if r.filled < opsSamples {
				r.filled++
			}
		}
	}
	r.last, r.lastAt = cur, now
}

func (r *rateSampler) rates() (ops int64, inKbps, outKbps float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.filled == 0 {
		return 0, 0, 0
	}
	var sum [3]float64
	for i := 0; i < r.filled; i++ {
		for j := range sum {
			sum[j] += r.samples[i][j]
		}
	}
	n := float64(r.filled)
	return int64(sum[0] / n), sum[1] / n / 1024, sum[2] / n / 1024
}

func newRunID() string {
	var b [20]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0000000000000000000000000000000000000000"
	}
	return hex.EncodeToString(b[:])
}

func (s *Server) serverStatus() command.ServerStatus {
	ops, in, out := s.rates.rates()
	st := command.ServerStatus{
		RunID:            s.runID,
		Started:          s.startedAt,
		ConfigFile:       s.cfg.SourceFile,
		ConnectedClients: s.connCount.Load(),
		BlockedClients:   s.blockedClients(),
		MaxClients:       s.maxClients.Load(),
		OpsPerSec:        ops,
		InputKbps:        in,
		OutputKbps:       out,
		RejectedConns:    s.rejectedConns.Load(),
	}
	if s.tracker != nil {
		st.TrackingClients = s.tracker.tracking.Load()
		st.TrackingKeys, st.TrackingItems, st.TrackingPrefixes = s.tracker.stats()
	}
	if s.repl != nil {
		st.SyncFull, st.SyncPartialOK, st.SyncPartialErr = s.repl.SyncStats()
	}
	return st
}

func (s *Server) blockedClients() int64 {
	var n int64
	s.conns.Range(func(_, v any) bool {
		if c, ok := v.(*Connection); ok && c.blocked.Load() {
			n++
		}
		return true
	})
	return n
}

func (s *Server) startRateSampler() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(opsSampleInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case now := <-ticker.C:
				s.rates.sample(now)
			}
		}
	}()
}
