import DocsLayout, {
  DocHeading,
  CodeBlock,
  InfoBox,
  type TocItem,
} from "@/components/DocsLayout";
import { Settings, Server, HardDrive, Database, Globe, Shield, BarChart3, ScrollText } from "lucide-react";

const toc: TocItem[] = [
  { id: "overview", text: "Overview", level: 2 },
  { id: "server", text: "Server", level: 2 },
  { id: "memory", text: "Memory", level: 2 },
  { id: "persistence", text: "Persistence", level: 2 },
  { id: "http", text: "HTTP API", level: 2 },
  { id: "security", text: "Security", level: 2 },
  { id: "logging", text: "Logging", level: 2 },
  { id: "cluster", text: "Cluster", level: 2 },
  { id: "metrics", text: "Metrics", level: 2 },
  { id: "full-example", text: "Full Example", level: 2 },
];

function ConfigTable({
  rows,
}: {
  rows: { key: string; type: string; def: string; desc: string }[];
}) {
  return (
    <div className="my-4 rounded-xl border border-slate-800 overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-800 text-left text-slate-400">
              <th className="px-4 py-2 font-medium">Key</th>
              <th className="px-4 py-2 font-medium">Type</th>
              <th className="px-4 py-2 font-medium">Default</th>
              <th className="px-4 py-2 font-medium">Description</th>
            </tr>
          </thead>
          <tbody className="text-slate-300">
            {rows.map((r, i) => (
              <tr
                key={r.key}
                className={i < rows.length - 1 ? "border-b border-slate-800/60" : ""}
              >
                <td className="px-4 py-2 font-mono text-xs text-blue-300 whitespace-nowrap">
                  {r.key}
                </td>
                <td className="px-4 py-2 text-xs text-slate-500 whitespace-nowrap">{r.type}</td>
                <td className="px-4 py-2 font-mono text-xs text-amber-300/80">{r.def}</td>
                <td className="px-4 py-2 text-slate-400">{r.desc}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export default function Configuration() {
  return (
    <DocsLayout toc={toc}>
      {/* Hero */}
      <div className="mb-10">
        <div className="flex items-center gap-2 text-blue-400 text-sm font-medium mb-2">
          <Settings className="w-4 h-4" />
          Reference
        </div>
        <h1 className="text-4xl font-extrabold text-white tracking-tight mb-4">
          Configuration Reference
        </h1>
        <p className="text-lg text-slate-400 leading-relaxed max-w-2xl">
          Complete reference for all CacheStorm configuration options. Configuration can be
          set via YAML file, environment variables, or CLI flags.
        </p>
      </div>

      {/* ── Overview ─────────────────────────────────────────── */}
      <DocHeading id="overview" level={2}>
        Overview
      </DocHeading>

      <p className="mb-3 text-slate-400">
        CacheStorm loads configuration in the following order of precedence
        (highest to lowest):
      </p>

      <ol className="list-decimal list-inside text-slate-400 space-y-1 mb-4 ml-2">
        <li>CLI flags (<code className="text-xs bg-slate-800 px-1 py-0.5 rounded">--port 6380</code>)</li>
        <li>Environment variables (<code className="text-xs bg-slate-800 px-1 py-0.5 rounded">CACHESTORM_SERVER_PORT</code>)</li>
        <li>Configuration file (<code className="text-xs bg-slate-800 px-1 py-0.5 rounded">cachestorm.yaml</code>)</li>
        <li>Default values</li>
      </ol>

      <InfoBox type="info">
        Environment variables use the <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">CACHESTORM_</code> prefix
        with underscores replacing dots. For example,{" "}
        <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">server.port</code> becomes{" "}
        <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">CACHESTORM_SERVER_PORT</code>.
      </InfoBox>

      {/* ── Server ───────────────────────────────────────────── */}
      <DocHeading id="server" level={2}>
        <Server className="w-5 h-5 text-blue-400" />
        Server
      </DocHeading>

      <ConfigTable
        rows={[
          { key: "server.port", type: "integer", def: "6380", desc: "TCP port for RESP protocol connections" },
          { key: "server.bind", type: "string", def: "0.0.0.0", desc: "Network interface to bind to" },
          { key: "server.timeout", type: "duration", def: "300s", desc: "Client idle timeout (0 = no timeout)" },
          { key: "server.max_clients", type: "integer", def: "10000", desc: "Maximum concurrent client connections" },
          { key: "server.tcp_backlog", type: "integer", def: "511", desc: "TCP listen backlog size" },
          { key: "server.tcp_keepalive", type: "duration", def: "300s", desc: "TCP keepalive interval" },
          { key: "server.threads", type: "integer", def: "0", desc: "Worker threads (0 = num CPUs)" },
        ]}
      />

      <CodeBlock
        language="yaml"
        title="server section"
        code={`server:
  port: 6380
  bind: "0.0.0.0"
  timeout: 300s
  max_clients: 10000
  tcp_backlog: 511
  tcp_keepalive: 300s
  threads: 0  # auto-detect CPU count`}
      />

      {/* ── Memory ───────────────────────────────────────────── */}
      <DocHeading id="memory" level={2}>
        <Database className="w-5 h-5 text-blue-400" />
        Memory
      </DocHeading>

      <ConfigTable
        rows={[
          { key: "memory.maxmemory", type: "size", def: "0", desc: "Maximum memory limit (0 = unlimited). Accepts: 100mb, 1gb, etc." },
          { key: "memory.eviction_policy", type: "string", def: "noeviction", desc: "Policy when maxmemory is reached" },
          { key: "memory.samples", type: "integer", def: "5", desc: "Number of samples for LRU/LFU approximation" },
        ]}
      />

      <p className="mb-3 text-slate-400">Available eviction policies:</p>

      <div className="my-4 rounded-xl border border-slate-800 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-800 text-left text-slate-400">
                <th className="px-4 py-2 font-medium">Policy</th>
                <th className="px-4 py-2 font-medium">Description</th>
              </tr>
            </thead>
            <tbody className="text-slate-300">
              {[
                ["noeviction", "Return errors when memory limit is reached"],
                ["allkeys-lru", "Evict least recently used keys from all keys"],
                ["allkeys-lfu", "Evict least frequently used keys from all keys"],
                ["allkeys-random", "Randomly evict keys from all keys"],
                ["volatile-lru", "Evict LRU keys with TTL set"],
                ["volatile-lfu", "Evict LFU keys with TTL set"],
                ["volatile-random", "Randomly evict keys with TTL set"],
                ["volatile-ttl", "Evict keys with the shortest TTL"],
              ].map(([policy, desc], i, arr) => (
                <tr key={policy} className={i < arr.length - 1 ? "border-b border-slate-800/60" : ""}>
                  <td className="px-4 py-2 font-mono text-xs text-amber-300/80">{policy}</td>
                  <td className="px-4 py-2 text-slate-400">{desc}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <CodeBlock
        language="yaml"
        title="memory section"
        code={`memory:
  maxmemory: "1gb"
  eviction_policy: "allkeys-lru"
  samples: 5`}
      />

      {/* ── Persistence ──────────────────────────────────────── */}
      <DocHeading id="persistence" level={2}>
        <HardDrive className="w-5 h-5 text-blue-400" />
        Persistence
      </DocHeading>

      <ConfigTable
        rows={[
          { key: "persistence.enabled", type: "bool", def: "false", desc: "Enable snapshot persistence" },
          { key: "persistence.directory", type: "string", def: "./data", desc: "Directory for snapshot files" },
          { key: "persistence.interval", type: "duration", def: "60s", desc: "Snapshot interval" },
          { key: "persistence.aof_enabled", type: "bool", def: "false", desc: "Enable append-only file" },
          { key: "persistence.aof_sync", type: "string", def: "everysec", desc: "AOF fsync policy: always, everysec, no" },
        ]}
      />

      <CodeBlock
        language="yaml"
        title="persistence section"
        code={`persistence:
  enabled: true
  directory: "/var/lib/cachestorm/data"
  interval: 60s

  # Append-only file (recommended for durability)
  aof_enabled: true
  aof_sync: "everysec"  # always | everysec | no`}
      />

      <InfoBox type="warning">
        Enabling both snapshots and AOF provides the best durability guarantee, but increases
        disk I/O. For pure caching workloads, consider disabling persistence entirely.
      </InfoBox>

      {/* ── HTTP ─────────────────────────────────────────────── */}
      <DocHeading id="http" level={2}>
        <Globe className="w-5 h-5 text-blue-400" />
        HTTP API
      </DocHeading>

      <ConfigTable
        rows={[
          { key: "http.enabled", type: "bool", def: "true", desc: "Enable the HTTP API server" },
          { key: "http.port", type: "integer", def: "7280", desc: "HTTP API port" },
          { key: "http.bind", type: "string", def: "0.0.0.0", desc: "HTTP bind address" },
          { key: "http.cors_origins", type: "[]string", def: '["*"]', desc: "Allowed CORS origins" },
          { key: "http.read_timeout", type: "duration", def: "30s", desc: "HTTP read timeout" },
          { key: "http.write_timeout", type: "duration", def: "30s", desc: "HTTP write timeout" },
        ]}
      />

      <CodeBlock
        language="yaml"
        title="http section"
        code={`http:
  enabled: true
  port: 7280
  bind: "0.0.0.0"
  cors_origins:
    - "https://admin.example.com"
  read_timeout: 30s
  write_timeout: 30s`}
      />

      {/* ── Security ─────────────────────────────────────────── */}
      <DocHeading id="security" level={2}>
        <Shield className="w-5 h-5 text-blue-400" />
        Security
      </DocHeading>

      <ConfigTable
        rows={[
          { key: "security.password", type: "string", def: '""', desc: "Server password (requirepass)" },
          { key: "security.tls.enabled", type: "bool", def: "false", desc: "Enable TLS encryption" },
          { key: "security.tls.cert_file", type: "string", def: '""', desc: "Path to TLS certificate" },
          { key: "security.tls.key_file", type: "string", def: '""', desc: "Path to TLS private key" },
          { key: "security.tls.ca_file", type: "string", def: '""', desc: "Path to CA certificate for mutual TLS" },
          { key: "security.acl.enabled", type: "bool", def: "false", desc: "Enable ACL system" },
          { key: "security.acl.file", type: "string", def: '""', desc: "Path to ACL configuration file" },
        ]}
      />

      <CodeBlock
        language="yaml"
        title="security section"
        code={`security:
  password: "\${CACHESTORM_PASSWORD}"

  tls:
    enabled: true
    cert_file: "/etc/cachestorm/tls/server.crt"
    key_file: "/etc/cachestorm/tls/server.key"
    ca_file: "/etc/cachestorm/tls/ca.crt"

  acl:
    enabled: true
    file: "/etc/cachestorm/acl.conf"`}
      />

      {/* ── Logging ──────────────────────────────────────────── */}
      <DocHeading id="logging" level={2}>
        <ScrollText className="w-5 h-5 text-blue-400" />
        Logging
      </DocHeading>

      <ConfigTable
        rows={[
          { key: "logging.level", type: "string", def: "info", desc: "Log level: debug, info, warn, error" },
          { key: "logging.format", type: "string", def: "text", desc: "Log format: text, json" },
          { key: "logging.output", type: "string", def: "stderr", desc: "Log output: stderr, stdout, or file path" },
          { key: "logging.slow_log_threshold", type: "duration", def: "10ms", desc: "Threshold for slow command logging" },
        ]}
      />

      <CodeBlock
        language="yaml"
        title="logging section"
        code={`logging:
  level: "info"
  format: "json"
  output: "/var/log/cachestorm/server.log"
  slow_log_threshold: 10ms`}
      />

      {/* ── Cluster ──────────────────────────────────────────── */}
      <DocHeading id="cluster" level={2}>
        <Server className="w-5 h-5 text-blue-400" />
        Cluster
      </DocHeading>

      <ConfigTable
        rows={[
          { key: "cluster.enabled", type: "bool", def: "false", desc: "Enable cluster mode" },
          { key: "cluster.node_id", type: "string", def: '""', desc: "Unique node identifier (auto-generated if empty)" },
          { key: "cluster.announce_addr", type: "string", def: '""', desc: "Address to announce to cluster peers" },
          { key: "cluster.peers", type: "[]string", def: "[]", desc: "List of seed peer addresses" },
          { key: "cluster.replication.role", type: "string", def: "master", desc: "Node role: master or replica" },
          { key: "cluster.replication.master_addr", type: "string", def: '""', desc: "Master address (for replicas)" },
        ]}
      />

      <CodeBlock
        language="yaml"
        title="cluster section"
        code={`cluster:
  enabled: true
  node_id: "node-1"
  announce_addr: "10.0.1.10:6380"
  peers:
    - "10.0.1.11:6380"
    - "10.0.1.12:6380"

  replication:
    role: "master"   # master | replica
    master_addr: ""  # set for replica nodes`}
      />

      {/* ── Metrics ──────────────────────────────────────────── */}
      <DocHeading id="metrics" level={2}>
        <BarChart3 className="w-5 h-5 text-blue-400" />
        Metrics
      </DocHeading>

      <ConfigTable
        rows={[
          { key: "metrics.prometheus.enabled", type: "bool", def: "true", desc: "Enable Prometheus metrics endpoint" },
          { key: "metrics.prometheus.port", type: "integer", def: "9121", desc: "Prometheus metrics port" },
          { key: "metrics.pprof.enabled", type: "bool", def: "false", desc: "Enable Go pprof profiling endpoints" },
          { key: "metrics.pprof.port", type: "integer", def: "6060", desc: "pprof server port" },
        ]}
      />

      <CodeBlock
        language="yaml"
        title="metrics section"
        code={`metrics:
  prometheus:
    enabled: true
    port: 9121

  pprof:
    enabled: false  # enable only for debugging
    port: 6060`}
      />

      {/* ── Full Example ─────────────────────────────────────── */}
      <DocHeading id="full-example" level={2}>
        Full Example
      </DocHeading>

      <p className="mb-3 text-slate-400">
        A production-ready configuration example:
      </p>

      <CodeBlock
        language="yaml"
        title="cachestorm.yaml (production)"
        code={`server:
  port: 6380
  bind: "0.0.0.0"
  timeout: 300s
  max_clients: 10000
  threads: 0

memory:
  maxmemory: "4gb"
  eviction_policy: "allkeys-lru"
  samples: 10

persistence:
  enabled: true
  directory: "/var/lib/cachestorm/data"
  interval: 300s
  aof_enabled: true
  aof_sync: "everysec"

http:
  enabled: true
  port: 7280
  cors_origins:
    - "https://admin.example.com"

security:
  password: "\${CACHESTORM_PASSWORD}"
  tls:
    enabled: true
    cert_file: "/etc/cachestorm/tls/server.crt"
    key_file: "/etc/cachestorm/tls/server.key"
  acl:
    enabled: true
    file: "/etc/cachestorm/acl.conf"

logging:
  level: "info"
  format: "json"
  output: "/var/log/cachestorm/server.log"
  slow_log_threshold: 10ms

metrics:
  prometheus:
    enabled: true
    port: 9121`}
      />
    </DocsLayout>
  );
}
