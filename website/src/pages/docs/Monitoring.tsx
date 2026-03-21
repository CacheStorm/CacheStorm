import DocsLayout, {
  DocHeading,
  CodeBlock,
  InfoBox,
  type TocItem,
} from "@/components/DocsLayout";
import { BarChart3, Activity, Gauge, LineChart, Bug, AlertTriangle } from "lucide-react";

const toc: TocItem[] = [
  { id: "overview", text: "Overview", level: 2 },
  { id: "prometheus", text: "Prometheus", level: 2 },
  { id: "prometheus-config", text: "Configuration", level: 3 },
  { id: "prometheus-metrics", text: "Available Metrics", level: 3 },
  { id: "grafana", text: "Grafana Dashboards", level: 2 },
  { id: "grafana-setup", text: "Setup", level: 3 },
  { id: "grafana-panels", text: "Dashboard Panels", level: 3 },
  { id: "pprof", text: "pprof Profiling", level: 2 },
  { id: "info-command", text: "INFO Command", level: 2 },
  { id: "slow-log", text: "Slow Log", level: 2 },
  { id: "alerting", text: "Alerting", level: 2 },
];

export default function Monitoring() {
  return (
    <DocsLayout toc={toc}>
      {/* Hero */}
      <div className="mb-10">
        <div className="flex items-center gap-2 text-blue-400 text-sm font-medium mb-2">
          <BarChart3 className="w-4 h-4" />
          Operations
        </div>
        <h1 className="text-4xl font-extrabold text-white tracking-tight mb-4">
          Monitoring &amp; Observability
        </h1>
        <p className="text-lg text-slate-400 leading-relaxed max-w-2xl">
          Monitor CacheStorm with Prometheus metrics, Grafana dashboards, and Go pprof profiling.
          Track performance, memory usage, and connection statistics in real time.
        </p>
      </div>

      {/* ── Overview ─────────────────────────────────────────── */}
      <DocHeading id="overview" level={2}>
        Overview
      </DocHeading>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mb-6">
        {[
          { icon: <Gauge className="w-5 h-5 text-blue-400" />, title: "Prometheus", desc: "Metrics endpoint for scraping" },
          { icon: <LineChart className="w-5 h-5 text-emerald-400" />, title: "Grafana", desc: "Pre-built visualization dashboards" },
          { icon: <Bug className="w-5 h-5 text-amber-400" />, title: "pprof", desc: "Go runtime profiling" },
        ].map((item) => (
          <div
            key={item.title}
            className="flex flex-col items-center gap-2 p-4 rounded-xl border border-slate-800 bg-slate-900/50 text-center"
          >
            {item.icon}
            <p className="text-sm font-semibold text-white">{item.title}</p>
            <p className="text-xs text-slate-500">{item.desc}</p>
          </div>
        ))}
      </div>

      {/* ── Prometheus ───────────────────────────────────────── */}
      <DocHeading id="prometheus" level={2}>
        <Gauge className="w-5 h-5 text-blue-400" />
        Prometheus
      </DocHeading>

      <p className="mb-4 text-slate-400">
        CacheStorm exposes a Prometheus-compatible metrics endpoint for scraping. Metrics cover
        server health, memory, connections, commands, and key statistics.
      </p>

      <DocHeading id="prometheus-config" level={3}>
        Configuration
      </DocHeading>

      <CodeBlock
        language="yaml"
        title="cachestorm.yaml"
        code={`metrics:
  prometheus:
    enabled: true
    port: 9121
    # Optional: basic auth for metrics endpoint
    # username: "prometheus"
    # password: "\${PROMETHEUS_PASSWORD}"`}
      />

      <CodeBlock
        language="yaml"
        title="prometheus.yml"
        code={`scrape_configs:
  - job_name: "cachestorm"
    scrape_interval: 15s
    static_configs:
      - targets:
          - "cachestorm-node-1:9121"
          - "cachestorm-node-2:9121"
          - "cachestorm-node-3:9121"
    # If basic auth is enabled:
    # basic_auth:
    #   username: "prometheus"
    #   password: "your-password"`}
      />

      <CodeBlock
        language="bash"
        title="Verify metrics endpoint"
        code={`# Check metrics endpoint
curl http://localhost:9121/metrics

# Example output:
# cachestorm_uptime_seconds 3600
# cachestorm_connected_clients 42
# cachestorm_memory_used_bytes 134217728
# cachestorm_commands_processed_total 1500000
# ...`}
      />

      <DocHeading id="prometheus-metrics" level={3}>
        Available Metrics
      </DocHeading>

      <div className="my-4 rounded-xl border border-slate-800 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-800 text-left text-slate-400">
                <th className="px-4 py-2 font-medium">Metric</th>
                <th className="px-4 py-2 font-medium">Type</th>
                <th className="px-4 py-2 font-medium">Description</th>
              </tr>
            </thead>
            <tbody className="text-slate-300">
              {[
                ["cachestorm_uptime_seconds", "Gauge", "Server uptime in seconds"],
                ["cachestorm_connected_clients", "Gauge", "Number of connected clients"],
                ["cachestorm_memory_used_bytes", "Gauge", "Total memory used in bytes"],
                ["cachestorm_memory_max_bytes", "Gauge", "Maximum memory configured"],
                ["cachestorm_memory_fragmentation_ratio", "Gauge", "Memory fragmentation ratio"],
                ["cachestorm_commands_total", "Counter", "Total commands processed (by command)"],
                ["cachestorm_commands_duration_seconds", "Histogram", "Command latency distribution"],
                ["cachestorm_keyspace_hits_total", "Counter", "Total key hits"],
                ["cachestorm_keyspace_misses_total", "Counter", "Total key misses"],
                ["cachestorm_keys_total", "Gauge", "Total number of keys"],
                ["cachestorm_expired_keys_total", "Counter", "Total expired keys"],
                ["cachestorm_evicted_keys_total", "Counter", "Total evicted keys"],
                ["cachestorm_connections_total", "Counter", "Total connections accepted"],
                ["cachestorm_rejected_connections_total", "Counter", "Connections rejected (max clients)"],
                ["cachestorm_network_input_bytes_total", "Counter", "Total bytes received"],
                ["cachestorm_network_output_bytes_total", "Counter", "Total bytes sent"],
                ["cachestorm_replication_lag_seconds", "Gauge", "Replication lag (replicas only)"],
                ["cachestorm_persistence_last_save_seconds", "Gauge", "Time since last successful save"],
                ["cachestorm_persistence_rdb_changes_since_save", "Gauge", "Changes since last RDB save"],
              ].map(([metric, type, desc], i, arr) => (
                <tr key={metric} className={i < arr.length - 1 ? "border-b border-slate-800/60" : ""}>
                  <td className="px-4 py-2 font-mono text-xs text-blue-300 whitespace-nowrap">{metric}</td>
                  <td className="px-4 py-2 text-xs text-slate-500 whitespace-nowrap">{type}</td>
                  <td className="px-4 py-2 text-slate-400">{desc}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* ── Grafana ──────────────────────────────────────────── */}
      <DocHeading id="grafana" level={2}>
        <LineChart className="w-5 h-5 text-blue-400" />
        Grafana Dashboards
      </DocHeading>

      <DocHeading id="grafana-setup" level={3}>
        Setup
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Import our pre-built Grafana dashboard for a comprehensive overview of your CacheStorm cluster.
      </p>

      <CodeBlock
        language="bash"
        title="Docker Compose with Grafana"
        code={`# docker-compose.monitoring.yml
services:
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana-data:/var/lib/grafana
      - ./grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./grafana/datasources:/etc/grafana/provisioning/datasources

volumes:
  prometheus-data:
  grafana-data:`}
      />

      <CodeBlock
        language="json"
        title="grafana/datasources/prometheus.json"
        code={`{
  "apiVersion": 1,
  "datasources": [
    {
      "name": "Prometheus",
      "type": "prometheus",
      "access": "proxy",
      "url": "http://prometheus:9090",
      "isDefault": true
    }
  ]
}`}
      />

      <DocHeading id="grafana-panels" level={3}>
        Dashboard Panels
      </DocHeading>

      <p className="mb-3 text-slate-400">
        Our Grafana dashboard includes the following panels:
      </p>

      <div className="space-y-2 mb-6">
        {[
          { title: "Overview Row", items: "Uptime, Connected Clients, Memory Usage, Keys Count, Commands/sec" },
          { title: "Performance Row", items: "Command Latency (p50/p95/p99), Commands by Type, Hit/Miss Ratio" },
          { title: "Memory Row", items: "Memory Used vs Max, Fragmentation Ratio, Eviction Rate" },
          { title: "Network Row", items: "Input/Output Bytes, Connection Rate, Rejected Connections" },
          { title: "Persistence Row", items: "Last Save Time, AOF Size, RDB Changes Since Save" },
          { title: "Cluster Row", items: "Node Status, Replication Lag, Cluster Health" },
        ].map((row) => (
          <div
            key={row.title}
            className="flex items-start gap-3 p-3 rounded-lg border border-slate-800 bg-slate-900/30"
          >
            <Activity className="w-4 h-4 text-blue-400 mt-0.5 shrink-0" />
            <div>
              <p className="text-sm font-medium text-white">{row.title}</p>
              <p className="text-xs text-slate-500 mt-0.5">{row.items}</p>
            </div>
          </div>
        ))}
      </div>

      <p className="text-slate-400 mb-4">
        Key PromQL queries for your dashboards:
      </p>

      <CodeBlock
        language="promql"
        title="Useful PromQL queries"
        code={`# Commands per second
rate(cachestorm_commands_total[5m])

# Hit rate percentage
cachestorm_keyspace_hits_total /
  (cachestorm_keyspace_hits_total + cachestorm_keyspace_misses_total) * 100

# Memory usage percentage
cachestorm_memory_used_bytes / cachestorm_memory_max_bytes * 100

# P99 command latency
histogram_quantile(0.99, rate(cachestorm_commands_duration_seconds_bucket[5m]))

# Eviction rate
rate(cachestorm_evicted_keys_total[5m])

# Connection rate
rate(cachestorm_connections_total[5m])`}
      />

      {/* ── pprof ────────────────────────────────────────────── */}
      <DocHeading id="pprof" level={2}>
        <Bug className="w-5 h-5 text-blue-400" />
        pprof Profiling
      </DocHeading>

      <p className="mb-4 text-slate-400">
        CacheStorm includes Go's built-in pprof profiler for diagnosing performance issues.
      </p>

      <InfoBox type="warning">
        Only enable pprof in development or for debugging. It adds overhead and should not
        be exposed publicly.
      </InfoBox>

      <CodeBlock
        language="yaml"
        title="cachestorm.yaml"
        code={`metrics:
  pprof:
    enabled: true
    port: 6060`}
      />

      <CodeBlock
        language="bash"
        title="Using pprof"
        code={`# CPU profile (30 seconds)
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Memory (heap) profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine dump
go tool pprof http://localhost:6060/debug/pprof/goroutine

# Block profile (contention)
go tool pprof http://localhost:6060/debug/pprof/block

# Mutex profile
go tool pprof http://localhost:6060/debug/pprof/mutex

# Interactive web UI
go tool pprof -http=:8080 http://localhost:6060/debug/pprof/heap

# Download profile and analyze later
curl -o cpu.prof http://localhost:6060/debug/pprof/profile?seconds=30
go tool pprof cpu.prof`}
      />

      <CodeBlock
        language="bash"
        title="Available pprof endpoints"
        code={`GET /debug/pprof/              # Index page
GET /debug/pprof/profile       # CPU profile
GET /debug/pprof/heap          # Heap memory profile
GET /debug/pprof/goroutine     # Goroutine stack dump
GET /debug/pprof/block         # Block (contention) profile
GET /debug/pprof/mutex         # Mutex contention profile
GET /debug/pprof/threadcreate  # Thread creation profile
GET /debug/pprof/trace         # Execution trace`}
      />

      {/* ── INFO Command ─────────────────────────────────────── */}
      <DocHeading id="info-command" level={2}>
        INFO Command
      </DocHeading>

      <p className="mb-4 text-slate-400">
        The <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">INFO</code> command provides
        a comprehensive snapshot of server state, useful for quick diagnostics.
      </p>

      <CodeBlock
        language="bash"
        title="INFO command"
        code={`# All sections
INFO

# Specific section
INFO server
INFO memory
INFO clients
INFO stats
INFO replication
INFO keyspace
INFO persistence

# Example output (memory section):
# # Memory
# used_memory: 134217728
# used_memory_human: 128.00M
# used_memory_peak: 268435456
# used_memory_peak_human: 256.00M
# maxmemory: 1073741824
# maxmemory_human: 1.00G
# maxmemory_policy: allkeys-lru
# mem_fragmentation_ratio: 1.05`}
      />

      {/* ── Slow Log ─────────────────────────────────────────── */}
      <DocHeading id="slow-log" level={2}>
        Slow Log
      </DocHeading>

      <p className="mb-4 text-slate-400">
        The slow log records commands that exceed a configurable execution time threshold.
      </p>

      <CodeBlock
        language="yaml"
        title="cachestorm.yaml"
        code={`logging:
  slow_log_threshold: 10ms  # Log commands taking longer than 10ms
  # slow_log_max_len: 128   # Keep last 128 slow log entries`}
      />

      <CodeBlock
        language="bash"
        title="Slow log commands"
        code={`# Get last 10 slow log entries
SLOWLOG GET 10

# Example output:
# 1) 1) (integer) 1          # Entry ID
#    2) (integer) 1705000000  # Timestamp
#    3) (integer) 15234       # Duration (microseconds)
#    4) 1) "KEYS"             # Command
#       2) "*"
#    5) "10.0.1.20:54321"    # Client address

# Get slow log length
SLOWLOG LEN

# Reset slow log
SLOWLOG RESET`}
      />

      {/* ── Alerting ─────────────────────────────────────────── */}
      <DocHeading id="alerting" level={2}>
        <AlertTriangle className="w-5 h-5 text-blue-400" />
        Alerting
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Set up Prometheus alerting rules to catch issues before they affect your application.
      </p>

      <CodeBlock
        language="yaml"
        title="cachestorm-alerts.yml"
        code={`groups:
  - name: cachestorm
    rules:
      - alert: CacheStormDown
        expr: up{job="cachestorm"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "CacheStorm instance is down"

      - alert: CacheStormHighMemory
        expr: >
          cachestorm_memory_used_bytes / cachestorm_memory_max_bytes > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Memory usage above 90%"

      - alert: CacheStormHighEvictionRate
        expr: rate(cachestorm_evicted_keys_total[5m]) > 100
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High eviction rate (>100 keys/sec)"

      - alert: CacheStormLowHitRate
        expr: >
          cachestorm_keyspace_hits_total /
          (cachestorm_keyspace_hits_total + cachestorm_keyspace_misses_total)
          < 0.8
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "Cache hit rate below 80%"

      - alert: CacheStormHighLatency
        expr: >
          histogram_quantile(0.99,
            rate(cachestorm_commands_duration_seconds_bucket[5m])
          ) > 0.01
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "P99 latency above 10ms"

      - alert: CacheStormReplicationLag
        expr: cachestorm_replication_lag_seconds > 5
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Replication lag exceeds 5 seconds"`}
      />
    </DocsLayout>
  );
}
