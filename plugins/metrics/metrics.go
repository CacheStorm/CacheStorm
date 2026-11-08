package metrics

import (
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/plugin"
)

type Metrics struct {
	// commandsTotal maps a command name to its *atomic.Int64 count. A
	// sync.Map keeps the per-command path lock-free: the set of names is
	// fixed once each command has run, which is the case sync.Map is for.
	commandsTotal    sync.Map
	hitCount         int64
	missCount        int64
	evictedCount     int64
	expiredCount     int64
	connectedClients int64
	tagInvalidations int64
	keysTotal        int64
	memoryBytes      int64
}

type MetricsPlugin struct {
	metrics *Metrics
	server  *http.Server
	enabled bool
}

func New(enabled bool) *MetricsPlugin {
	return &MetricsPlugin{
		metrics: &Metrics{},
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

	n, ok := m.metrics.commandsTotal.Load(ctx.Command)
	if !ok {
		n, _ = m.metrics.commandsTotal.LoadOrStore(ctx.Command, new(atomic.Int64))
	}
	n.(*atomic.Int64).Add(1)
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

func (m *MetricsPlugin) SetHitCount(n int64) {
	atomic.StoreInt64(&m.metrics.hitCount, n)
}

func (m *MetricsPlugin) SetMissCount(n int64) {
	atomic.StoreInt64(&m.metrics.missCount, n)
}

func (m *MetricsPlugin) SetEvictedCount(n int64) {
	atomic.StoreInt64(&m.metrics.evictedCount, n)
}

func (m *MetricsPlugin) SetExpiredCount(n int64) {
	atomic.StoreInt64(&m.metrics.expiredCount, n)
}

func (m *MetricsPlugin) ExportPrometheus() string {
	var result string

	result += "# HELP cachestorm_commands_total Total number of commands executed\n"
	result += "# TYPE cachestorm_commands_total counter\n"

	m.metrics.commandsTotal.Range(func(cmd, count any) bool {
		result += "cachestorm_commands_total{command=\"" + cmd.(string) + "\"} " +
			strconv.FormatInt(count.(*atomic.Int64).Load(), 10) + "\n"
		return true
	})

	result += "\n# HELP cachestorm_hit_total Total cache hits\n"
	result += "# TYPE cachestorm_hit_total counter\n"
	result += "cachestorm_hit_total " + strconv.FormatInt(atomic.LoadInt64(&m.metrics.hitCount), 10) + "\n"

	result += "\n# HELP cachestorm_miss_total Total cache misses\n"
	result += "# TYPE cachestorm_miss_total counter\n"
	result += "cachestorm_miss_total " + strconv.FormatInt(atomic.LoadInt64(&m.metrics.missCount), 10) + "\n"

	result += "\n# HELP cachestorm_evicted_total Total keys evicted\n"
	result += "# TYPE cachestorm_evicted_total counter\n"
	result += "cachestorm_evicted_total " + strconv.FormatInt(atomic.LoadInt64(&m.metrics.evictedCount), 10) + "\n"

	result += "\n# HELP cachestorm_expired_total Total keys expired\n"
	result += "# TYPE cachestorm_expired_total counter\n"
	result += "cachestorm_expired_total " + strconv.FormatInt(atomic.LoadInt64(&m.metrics.expiredCount), 10) + "\n"

	result += "\n# HELP cachestorm_connected_clients Number of connected clients\n"
	result += "# TYPE cachestorm_connected_clients gauge\n"
	result += "cachestorm_connected_clients " + strconv.FormatInt(atomic.LoadInt64(&m.metrics.connectedClients), 10) + "\n"

	result += "\n# HELP cachestorm_keys_total Total number of keys\n"
	result += "# TYPE cachestorm_keys_total gauge\n"
	result += "cachestorm_keys_total " + strconv.FormatInt(atomic.LoadInt64(&m.metrics.keysTotal), 10) + "\n"

	result += "\n# HELP cachestorm_memory_bytes Memory usage in bytes\n"
	result += "# TYPE cachestorm_memory_bytes gauge\n"
	result += "cachestorm_memory_bytes " + strconv.FormatInt(atomic.LoadInt64(&m.metrics.memoryBytes), 10) + "\n"

	result += "\n# HELP cachestorm_tag_invalidations_total Total tag invalidations\n"
	result += "# TYPE cachestorm_tag_invalidations_total counter\n"
	result += "cachestorm_tag_invalidations_total " + strconv.FormatInt(atomic.LoadInt64(&m.metrics.tagInvalidations), 10) + "\n"

	return result
}

func (m *MetricsPlugin) GetMetrics() map[string]interface{} {
	commands := make(map[string]int64)
	m.metrics.commandsTotal.Range(func(k, v any) bool {
		commands[k.(string)] = v.(*atomic.Int64).Load()
		return true
	})

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
