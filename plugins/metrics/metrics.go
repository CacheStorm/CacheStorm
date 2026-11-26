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
	hitCount         atomic.Int64
	missCount        atomic.Int64
	evictedCount     atomic.Int64
	expiredCount     atomic.Int64
	connectedClients atomic.Int64
	tagInvalidations atomic.Int64
	tagKeys          atomic.Int64
	tags             atomic.Int64
	tagMembers       atomic.Int64
	keysTotal        atomic.Int64
	memoryBytes      atomic.Int64
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
	m.metrics.hitCount.Add(1)
}

func (m *MetricsPlugin) RecordMiss() {
	m.metrics.missCount.Add(1)
}

func (m *MetricsPlugin) RecordEviction() {
	m.metrics.evictedCount.Add(1)
}

func (m *MetricsPlugin) RecordExpiration() {
	m.metrics.expiredCount.Add(1)
}

func (m *MetricsPlugin) RecordTagInvalidation() {
	m.metrics.tagInvalidations.Add(1)
}

// SetTagStats sets the tag gauges and the INVALIDATE counters, summed over
// the databases.
func (m *MetricsPlugin) SetTagStats(tags, members, invalidations, invalidatedKeys int64) {
	m.metrics.tags.Store(tags)
	m.metrics.tagMembers.Store(members)
	m.metrics.tagInvalidations.Store(invalidations)
	m.metrics.tagKeys.Store(invalidatedKeys)
}

func (m *MetricsPlugin) SetConnectedClients(n int64) {
	m.metrics.connectedClients.Store(n)
}

func (m *MetricsPlugin) SetKeysTotal(n int64) {
	m.metrics.keysTotal.Store(n)
}

func (m *MetricsPlugin) SetMemoryBytes(n int64) {
	m.metrics.memoryBytes.Store(n)
}

func (m *MetricsPlugin) SetHitCount(n int64) {
	m.metrics.hitCount.Store(n)
}

func (m *MetricsPlugin) SetMissCount(n int64) {
	m.metrics.missCount.Store(n)
}

func (m *MetricsPlugin) SetEvictedCount(n int64) {
	m.metrics.evictedCount.Store(n)
}

func (m *MetricsPlugin) SetExpiredCount(n int64) {
	m.metrics.expiredCount.Store(n)
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
	result += "cachestorm_hit_total " + strconv.FormatInt(m.metrics.hitCount.Load(), 10) + "\n"

	result += "\n# HELP cachestorm_miss_total Total cache misses\n"
	result += "# TYPE cachestorm_miss_total counter\n"
	result += "cachestorm_miss_total " + strconv.FormatInt(m.metrics.missCount.Load(), 10) + "\n"

	result += "\n# HELP cachestorm_evicted_total Total keys evicted\n"
	result += "# TYPE cachestorm_evicted_total counter\n"
	result += "cachestorm_evicted_total " + strconv.FormatInt(m.metrics.evictedCount.Load(), 10) + "\n"

	result += "\n# HELP cachestorm_expired_total Total keys expired\n"
	result += "# TYPE cachestorm_expired_total counter\n"
	result += "cachestorm_expired_total " + strconv.FormatInt(m.metrics.expiredCount.Load(), 10) + "\n"

	result += "\n# HELP cachestorm_connected_clients Number of connected clients\n"
	result += "# TYPE cachestorm_connected_clients gauge\n"
	result += "cachestorm_connected_clients " + strconv.FormatInt(m.metrics.connectedClients.Load(), 10) + "\n"

	result += "\n# HELP cachestorm_keys_total Total number of keys\n"
	result += "# TYPE cachestorm_keys_total gauge\n"
	result += "cachestorm_keys_total " + strconv.FormatInt(m.metrics.keysTotal.Load(), 10) + "\n"

	result += "\n# HELP cachestorm_memory_bytes Memory usage in bytes\n"
	result += "# TYPE cachestorm_memory_bytes gauge\n"
	result += "cachestorm_memory_bytes " + strconv.FormatInt(m.metrics.memoryBytes.Load(), 10) + "\n"

	result += "\n# HELP cachestorm_tag_invalidations_total INVALIDATE commands run\n"
	result += "# TYPE cachestorm_tag_invalidations_total counter\n"
	result += "cachestorm_tag_invalidations_total " + strconv.FormatInt(m.metrics.tagInvalidations.Load(), 10) + "\n"

	result += "\n# HELP cachestorm_tag_invalidated_keys_total Keys INVALIDATE deleted\n"
	result += "# TYPE cachestorm_tag_invalidated_keys_total counter\n"
	result += "cachestorm_tag_invalidated_keys_total " + strconv.FormatInt(m.metrics.tagKeys.Load(), 10) + "\n"

	result += "\n# HELP cachestorm_tags Tags that have at least one key\n"
	result += "# TYPE cachestorm_tags gauge\n"
	result += "cachestorm_tags " + strconv.FormatInt(m.metrics.tags.Load(), 10) + "\n"

	result += "\n# HELP cachestorm_tag_memberships Tag and key pairs in the tag index\n"
	result += "# TYPE cachestorm_tag_memberships gauge\n"
	result += "cachestorm_tag_memberships " + strconv.FormatInt(m.metrics.tagMembers.Load(), 10) + "\n"

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
		"hit_count":         m.metrics.hitCount.Load(),
		"miss_count":        m.metrics.missCount.Load(),
		"evicted_count":     m.metrics.evictedCount.Load(),
		"expired_count":     m.metrics.expiredCount.Load(),
		"connected_clients": m.metrics.connectedClients.Load(),
		"tag_invalidations": m.metrics.tagInvalidations.Load(),
		"keys_total":        m.metrics.keysTotal.Load(),
		"memory_bytes":      m.metrics.memoryBytes.Load(),
	}
}

var _ plugin.AfterCommandHook = (*MetricsPlugin)(nil)
