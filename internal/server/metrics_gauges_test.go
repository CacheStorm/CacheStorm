package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: the metrics refresh ticker only pushed keys/memory/clients into
// the plugin; the hit/miss/evicted/expired counters were never pushed, so
// /metrics advertised them as 0 forever. updateMetricsGauges must publish all
// of them from the live store counters.
func TestUpdateMetricsGaugesPublishesCounters(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Bind: "127.0.0.1", Port: 6399},
		HTTP:   config.HTTPConfig{Enabled: false},
	}
	cfg.Plugins.Metrics.Enabled = true
	cfg.Plugins.Metrics.Port = 9199
	cfg.Plugins.Metrics.Path = "/metrics"

	s, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	st := s.Store()
	hitsBefore := store.GlobalMetrics.TotalHits.Load()
	missesBefore := store.GlobalMetrics.TotalMisses.Load()
	expirationsBefore := store.GlobalMetrics.TotalExpirations.Load()

	// Real store events: miss, hit, lazy expiration, eviction.
	if _, ok := st.Get("gauges:missing"); ok {
		t.Fatal("expected miss for non-existent key")
	}
	if err := st.Set("gauges:hit", &store.StringValue{Data: []byte("v")}, store.SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, ok := st.Get("gauges:hit"); !ok {
		t.Fatal("expected hit for live key")
	}
	if err := st.Set("gauges:ttl", &store.StringValue{Data: []byte("v")}, store.SetOptions{TTL: 5 * time.Millisecond}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, ok := st.Get("gauges:ttl"); ok {
		t.Fatal("expected miss for expired key")
	}
	st.ConfigureMemory(1024, store.EvictionAllKeysLRU, 70, 85, 128)
	for i := 0; i < 64; i++ {
		if err := st.Set(fmt.Sprintf("gauges:ev%d", i), &store.StringValue{Data: []byte("v")}, store.SetOptions{}); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	// The store's live accounting now evicts during the setup loop above, so
	// snapshot after it: the asserted delta must isolate the forced evictions.
	evictionsBefore := store.GlobalMetrics.TotalEvictions.Load()
	if n := st.Evictor().ForceEvict(2); n != 2 {
		t.Fatalf("ForceEvict evicted %d keys, want 2", n)
	}

	s.updateMetricsGauges()

	if got := store.GlobalMetrics.TotalHits.Load() - hitsBefore; got != 1 {
		t.Fatalf("expected 1 hit recorded, got %d", got)
	}
	if got := store.GlobalMetrics.TotalMisses.Load() - missesBefore; got != 2 {
		t.Fatalf("expected 2 misses recorded, got %d", got)
	}
	if got := store.GlobalMetrics.TotalExpirations.Load() - expirationsBefore; got != 1 {
		t.Fatalf("expected 1 expiration recorded, got %d", got)
	}
	if got := store.GlobalMetrics.TotalEvictions.Load() - evictionsBefore; got != 2 {
		t.Fatalf("expected 2 evictions recorded, got %d", got)
	}

	out := s.metricsPlugin.ExportPrometheus()
	for _, want := range []string{
		fmt.Sprintf("cachestorm_hit_total %d\n", store.GlobalMetrics.TotalHits.Load()),
		fmt.Sprintf("cachestorm_miss_total %d\n", store.GlobalMetrics.TotalMisses.Load()),
		fmt.Sprintf("cachestorm_expired_total %d\n", store.GlobalMetrics.TotalExpirations.Load()),
		fmt.Sprintf("cachestorm_evicted_total %d\n", store.GlobalMetrics.TotalEvictions.Load()),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics output missing %q after updateMetricsGauges:\n%s", want, out)
		}
	}
}
