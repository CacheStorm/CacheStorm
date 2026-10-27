package server

import (
	"fmt"
	"io"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
	"github.com/cachestorm/cachestorm/internal/version"
)

// serverMetrics is one reading of the values the Prometheus exposition
// reports, taken across every database.
type serverMetrics struct {
	keys        int64
	memUsed     int64
	memMax      int64
	memPressure float64
	clients     int64
	uptime      time.Duration
}

func readServerMetrics(dbs *store.Databases, base *store.Store, clients int64, started time.Time) serverMetrics {
	m := serverMetrics{clients: clients, uptime: time.Since(started)}
	if dbs != nil {
		m.keys = dbs.KeyCount()
		m.memUsed = dbs.MemUsage()
	} else if base != nil {
		m.keys = base.KeyCount()
		m.memUsed = base.MemUsage()
	}
	if base != nil {
		if mt := base.MemoryTracker(); mt != nil {
			m.memMax = mt.Max()
			m.memPressure = mt.PressurePercent()
		}
	}
	return m
}

func writeGauge(w io.Writer, name, help, value string) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %s\n", name, help, name, name, value)
}

func writeCounter(w io.Writer, name, help string, value int64) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, value)
}

// writePrometheusMetrics writes the server's metrics in the Prometheus text
// exposition format. The admin API and the metrics port serve the same text,
// so dashboards and alerts work against either.
func writePrometheusMetrics(w io.Writer, m serverMetrics) {
	writeGauge(w, "cachestorm_up", "Whether the server is serving (1)", "1")
	fmt.Fprintf(w, "# HELP cachestorm_build_info Build version\n# TYPE cachestorm_build_info gauge\ncachestorm_build_info{version=%q} 1\n", version.Version)
	writeGauge(w, "cachestorm_uptime_seconds", "Server uptime in seconds", fmt.Sprintf("%.0f", m.uptime.Seconds()))
	writeGauge(w, "cachestorm_keys_total", "Total number of keys stored in all databases", fmt.Sprint(m.keys))
	writeGauge(w, "cachestorm_memory_used_bytes", "Tracked dataset memory in bytes", fmt.Sprint(m.memUsed))
	writeGauge(w, "cachestorm_memory_max_bytes", "Configured maximum memory in bytes (0 = unlimited)", fmt.Sprint(m.memMax))
	writeGauge(w, "cachestorm_memory_pressure_percent", "Tracked memory as a percentage of the maximum", fmt.Sprintf("%.2f", m.memPressure))
	writeGauge(w, "cachestorm_connected_clients", "Number of connected clients", fmt.Sprint(m.clients))
	gm := store.GlobalMetrics
	writeCounter(w, "cachestorm_commands_processed_total", "Commands processed", gm.TotalCommands.Load())
	writeCounter(w, "cachestorm_keyspace_hits_total", "Successful key lookups", gm.TotalHits.Load())
	writeCounter(w, "cachestorm_keyspace_misses_total", "Failed key lookups", gm.TotalMisses.Load())
	writeCounter(w, "cachestorm_evicted_keys_total", "Keys evicted by the memory limit", gm.TotalEvictions.Load())
	writeCounter(w, "cachestorm_expired_keys_total", "Keys removed by expiry", gm.TotalExpirations.Load())
	writeCounter(w, "cachestorm_connections_received_total", "Connections accepted", gm.TotalConnections.Load())
	writeCounter(w, "cachestorm_net_input_bytes_total", "Bytes read from clients", gm.TotalBytesIn.Load())
	writeCounter(w, "cachestorm_net_output_bytes_total", "Bytes written to clients", gm.TotalBytesOut.Load())
}
