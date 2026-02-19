package metrics

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/plugin"
)

type Metrics struct {
	mu sync.RWMutex

	commandsTotal    map[string]*int64
	commandDuration  map[string]*latencyHistogram
	hitCount         int64
	missCount        int64
	evictedCount     int64
	expiredCount     int64
	connectedClients int64
	tagInvalidations int64
	keysTotal        int64
	memoryBytes      int64
}

type latencyHistogram struct {
	mu      sync.Mutex
	buckets []bucket
	count   int64
	sum     int64
}

type bucket struct {
	le    time.Duration
	count int64
}

func newLatencyHistogram() *latencyHistogram {
	buckets := []time.Duration{
		100 * time.Microsecond,
		500 * time.Microsecond,
		time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		500 * time.Millisecond,
		time.Second,
	}

	h := &latencyHistogram{
		buckets: make([]bucket, len(buckets)),
	}
	for i, le := range buckets {
		h.buckets[i] = bucket{le: le}
	}
	return h
}

func (h *latencyHistogram) observe(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.count++
	h.sum += int64(d)

	for i := range h.buckets {
		if d <= h.buckets[i].le {
			h.buckets[i].count++
		}
	}
}

type MetricsPlugin struct {
	metrics *Metrics
	server  *http.Server
	enabled bool
}

func New(enabled bool) *MetricsPlugin {
	return &MetricsPlugin{
		metrics: &Metrics{
			commandsTotal:   make(map[string]*int64),
			commandDuration: make(map[string]*latencyHistogram),
		},
		enabled: enabled,
	}
}

func (m *MetricsPlugin) Name() string    { return "metrics" }
func (m *MetricsPlugin) Version() string { return "1.0.0" }

func (m *MetricsPlugin) Init(config interface{}) error {
	return nil
}

func (m *MetricsPlugin) Close() error {
	if m.server != nil {
		return m.server.Close()
	}
	return nil
}

func (m *MetricsPlugin) BeforeCommand(ctx *command.Context) error {
	return nil
}

func (m *MetricsPlugin) AfterCommand(ctx *command.Context) {
	if !m.enabled {
		return
	}

	duration := time.Since(ctx.StartTime)

	m.metrics.mu.Lock()
	if m.metrics.commandsTotal[ctx.Command] == nil {
		m.metrics.commandsTotal[ctx.Command] = new(int64)
		m.metrics.commandDuration[ctx.Command] = newLatencyHistogram()
	}
	m.metrics.mu.Unlock()

	atomic.AddInt64(m.metrics.commandsTotal[ctx.Command], 1)
	m.metrics.commandDuration[ctx.Command].observe(duration)
}

func (m *MetricsPlugin) RecordHit() {
	atomic.AddInt64(&m.metrics.hitCount, 1)
}

func (m *MetricsPlugin) RecordMiss() {
	atomic.AddInt64(&m.metrics.missCount, 1)
}

func (m *MetricsPlugin) RecordEviction() {
	atomic.AddInt64(&m.metrics.evictedCount, 1)
}

func (m *MetricsPlugin) RecordExpiration() {
	atomic.AddInt64(&m.metrics.expiredCount, 1)
}

func (m *MetricsPlugin) RecordTagInvalidation() {
	atomic.AddInt64(&m.metrics.tagInvalidations, 1)
}

func (m *MetricsPlugin) SetConnectedClients(n int64) {
	atomic.StoreInt64(&m.metrics.connectedClients, n)
}

func (m *MetricsPlugin) SetKeysTotal(n int64) {
	atomic.StoreInt64(&m.metrics.keysTotal, n)
}

func (m *MetricsPlugin) SetMemoryBytes(n int64) {
	atomic.StoreInt64(&m.metrics.memoryBytes, n)
}

func (m *MetricsPlugin) ExportPrometheus() string {
	var result string

	result += "# HELP cachestorm_commands_total Total number of commands executed\n"
	result += "# TYPE cachestorm_commands_total counter\n"

	m.metrics.mu.RLock()
	for cmd, count := range m.metrics.commandsTotal {
		result += "cachestorm_commands_total{command=\"" + cmd + "\"} " +
			string(rune(atomic.LoadInt64(count))) + "\n"
	}
	m.metrics.mu.RUnlock()

	result += "\n# HELP cachestorm_hit_total Total cache hits\n"
	result += "# TYPE cachestorm_hit_total counter\n"
	result += "cachestorm_hit_total " + string(rune(atomic.LoadInt64(&m.metrics.hitCount))) + "\n"

	result += "\n# HELP cachestorm_miss_total Total cache misses\n"
	result += "# TYPE cachestorm_miss_total counter\n"
	result += "cachestorm_miss_total " + string(rune(atomic.LoadInt64(&m.metrics.missCount))) + "\n"

	result += "\n# HELP cachestorm_evicted_total Total keys evicted\n"
	result += "# TYPE cachestorm_evicted_total counter\n"
	result += "cachestorm_evicted_total " + string(rune(atomic.LoadInt64(&m.metrics.evictedCount))) + "\n"

	result += "\n# HELP cachestorm_expired_total Total keys expired\n"
	result += "# TYPE cachestorm_expired_total counter\n"
	result += "cachestorm_expired_total " + string(rune(atomic.LoadInt64(&m.metrics.expiredCount))) + "\n"

	result += "\n# HELP cachestorm_connected_clients Number of connected clients\n"
	result += "# TYPE cachestorm_connected_clients gauge\n"
	result += "cachestorm_connected_clients " + string(rune(atomic.LoadInt64(&m.metrics.connectedClients))) + "\n"

	result += "\n# HELP cachestorm_keys_total Total number of keys\n"
	result += "# TYPE cachestorm_keys_total gauge\n"
	result += "cachestorm_keys_total " + string(rune(atomic.LoadInt64(&m.metrics.keysTotal))) + "\n"

	result += "\n# HELP cachestorm_memory_bytes Memory usage in bytes\n"
	result += "# TYPE cachestorm_memory_bytes gauge\n"
	result += "cachestorm_memory_bytes " + string(rune(atomic.LoadInt64(&m.metrics.memoryBytes))) + "\n"

	result += "\n# HELP cachestorm_tag_invalidations_total Total tag invalidations\n"
	result += "# TYPE cachestorm_tag_invalidations_total counter\n"
	result += "cachestorm_tag_invalidations_total " + string(rune(atomic.LoadInt64(&m.metrics.tagInvalidations))) + "\n"

	return result
}

func (m *MetricsPlugin) GetMetrics() map[string]interface{} {
	m.metrics.mu.RLock()
	commands := make(map[string]int64)
	for k, v := range m.metrics.commandsTotal {
		commands[k] = atomic.LoadInt64(v)
	}
	m.metrics.mu.RUnlock()

	return map[string]interface{}{
		"commands_total":    commands,
		"hit_count":         atomic.LoadInt64(&m.metrics.hitCount),
		"miss_count":        atomic.LoadInt64(&m.metrics.missCount),
		"evicted_count":     atomic.LoadInt64(&m.metrics.evictedCount),
		"expired_count":     atomic.LoadInt64(&m.metrics.expiredCount),
		"connected_clients": atomic.LoadInt64(&m.metrics.connectedClients),
		"tag_invalidations": atomic.LoadInt64(&m.metrics.tagInvalidations),
		"keys_total":        atomic.LoadInt64(&m.metrics.keysTotal),
		"memory_bytes":      atomic.LoadInt64(&m.metrics.memoryBytes),
	}
}

var _ plugin.AfterCommandHook = (*MetricsPlugin)(nil)
