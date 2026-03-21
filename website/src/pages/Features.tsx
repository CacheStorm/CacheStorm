import { Link } from "react-router-dom";
import {
  Zap,
  Shield,
  Database,
  Globe,
  Terminal,
  BarChart3,
  Network,
  Clock,
  Cpu,
  Lock,
  Layers,
  Radio,
  ArrowRight,
  Check,
  X,
  Minus,
} from "lucide-react";
import { cn } from "@/lib/utils";

/* ------------------------------------------------------------------ */
/*  Feature section with code example                                 */
/* ------------------------------------------------------------------ */

interface FeatureSection {
  icon: React.ReactNode;
  title: string;
  description: string;
  code: string;
  codeTitle: string;
  highlights: string[];
  reverse?: boolean;
}

function FeatureBlock({ feature }: { feature: FeatureSection }) {
  return (
    <div
      className={cn(
        "flex flex-col lg:flex-row gap-8 lg:gap-12 items-start",
        feature.reverse && "lg:flex-row-reverse"
      )}
    >
      {/* Text side */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-10 h-10 rounded-xl bg-[var(--color-surface)] border border-[var(--color-border)] flex items-center justify-center">
            {feature.icon}
          </div>
          <h3 className="text-2xl font-bold text-[var(--color-text)]">{feature.title}</h3>
        </div>
        <p className="text-[var(--color-text-secondary)] leading-relaxed mb-5">{feature.description}</p>
        <ul className="space-y-2">
          {feature.highlights.map((h) => (
            <li key={h} className="flex items-start gap-2 text-sm text-[var(--color-text-secondary)]">
              <Check className="w-4 h-4 text-green-600 dark:text-green-400 mt-0.5 shrink-0" />
              {h}
            </li>
          ))}
        </ul>
      </div>

      {/* Code side */}
      <div className="flex-1 min-w-0 w-full">
        <div className="rounded-xl overflow-hidden border border-[var(--color-border)] bg-[var(--color-bg-secondary)]">
          <div className="flex items-center gap-2 px-4 py-2.5 bg-[var(--color-surface)] border-b border-[var(--color-border)]">
            <div className="flex gap-1.5">
              <div className="w-3 h-3 rounded-full bg-red-500/60" />
              <div className="w-3 h-3 rounded-full bg-yellow-500/60" />
              <div className="w-3 h-3 rounded-full bg-green-500/60" />
            </div>
            <span className="text-xs text-[var(--color-text-tertiary)] ml-2">{feature.codeTitle}</span>
          </div>
          <pre className="p-4 overflow-x-auto text-sm leading-relaxed">
            <code className="text-[var(--color-text-secondary)]">{feature.code}</code>
          </pre>
        </div>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Performance benchmarks                                            */
/* ------------------------------------------------------------------ */

interface BenchmarkItem {
  label: string;
  cachestorm: number;
  redis: number;
  unit: string;
}

const benchmarks: BenchmarkItem[] = [
  { label: "SET ops/sec", cachestorm: 285000, redis: 250000, unit: "ops/s" },
  { label: "GET ops/sec", cachestorm: 310000, redis: 280000, unit: "ops/s" },
  { label: "INCR ops/sec", cachestorm: 295000, redis: 265000, unit: "ops/s" },
  { label: "LPUSH ops/sec", cachestorm: 270000, redis: 245000, unit: "ops/s" },
  { label: "P99 Latency", cachestorm: 0.3, redis: 0.4, unit: "ms" },
  { label: "Memory Efficiency", cachestorm: 92, redis: 85, unit: "%" },
];

function BenchmarkBar({ item }: { item: BenchmarkItem }) {
  const max = Math.max(item.cachestorm, item.redis);
  const csWidth = (item.cachestorm / max) * 100;
  const rdWidth = (item.redis / max) * 100;

  // For latency, lower is better - flip the visual emphasis
  const isLowerBetter = item.label.includes("Latency");

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-sm">
        <span className="text-[var(--color-text-secondary)] font-medium">{item.label}</span>
        <span className="text-xs text-[var(--color-text-tertiary)]">{item.unit}</span>
      </div>
      <div className="space-y-1.5">
        <div className="flex items-center gap-3">
          <span className="text-xs text-[var(--color-primary)] w-24 shrink-0">CacheStorm</span>
          <div className="flex-1 h-5 bg-[var(--color-surface)] rounded-full overflow-hidden">
            <div
              className={cn(
                "h-full rounded-full transition-all duration-1000",
                isLowerBetter ? "bg-emerald-500" : "bg-blue-500"
              )}
              style={{ width: `${csWidth}%` }}
            />
          </div>
          <span className="text-xs text-[var(--color-text-secondary)] w-20 text-right font-mono">
            {item.cachestorm.toLocaleString()}
          </span>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs text-[var(--color-text-tertiary)] w-24 shrink-0">Redis</span>
          <div className="flex-1 h-5 bg-[var(--color-surface)] rounded-full overflow-hidden">
            <div
              className="h-full bg-[var(--color-text-tertiary)] rounded-full transition-all duration-1000"
              style={{ width: `${rdWidth}%` }}
            />
          </div>
          <span className="text-xs text-[var(--color-text-tertiary)] w-20 text-right font-mono">
            {item.redis.toLocaleString()}
          </span>
        </div>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Comparison table                                                  */
/* ------------------------------------------------------------------ */

type CompareVal = "yes" | "no" | "partial";

interface CompareRow {
  feature: string;
  cachestorm: CompareVal;
  redis: CompareVal;
  note?: string;
}

const comparisonData: CompareRow[] = [
  { feature: "RESP Protocol Compatible", cachestorm: "yes", redis: "yes" },
  { feature: "HTTP/REST API", cachestorm: "yes", redis: "no", note: "Built-in" },
  { feature: "Prometheus Metrics", cachestorm: "yes", redis: "partial", note: "Requires exporter" },
  { feature: "TLS Encryption", cachestorm: "yes", redis: "yes" },
  { feature: "ACL System", cachestorm: "yes", redis: "yes" },
  { feature: "Lua Scripting", cachestorm: "yes", redis: "yes" },
  { feature: "Cluster Mode", cachestorm: "yes", redis: "yes" },
  { feature: "Streams", cachestorm: "yes", redis: "yes" },
  { feature: "Pub/Sub", cachestorm: "yes", redis: "yes" },
  { feature: "Multi-threaded I/O", cachestorm: "yes", redis: "partial", note: "Redis 6+ partial" },
  { feature: "Written in Go", cachestorm: "yes", redis: "no", note: "C" },
  { feature: "Single Binary Deployment", cachestorm: "yes", redis: "no" },
  { feature: "Built-in Profiling (pprof)", cachestorm: "yes", redis: "no" },
  { feature: "YAML Configuration", cachestorm: "yes", redis: "no", note: "Redis uses custom format" },
  { feature: "Env Variable Config", cachestorm: "yes", redis: "no" },
  { feature: "Open Source", cachestorm: "yes", redis: "partial", note: "SSPL since Redis 7.4" },
];

function CompareIcon({ val }: { val: CompareVal }) {
  switch (val) {
    case "yes":
      return <Check className="w-4 h-4 text-green-600 dark:text-green-400" />;
    case "no":
      return <X className="w-4 h-4 text-red-400/60" />;
    case "partial":
      return <Minus className="w-4 h-4 text-amber-400" />;
  }
}

/* ------------------------------------------------------------------ */
/*  Feature data                                                      */
/* ------------------------------------------------------------------ */

const features: FeatureSection[] = [
  {
    icon: <Zap className="w-5 h-5 text-[var(--color-primary)]" />,
    title: "Blazing Fast Performance",
    description:
      "CacheStorm is built from the ground up in Go for maximum performance. Multi-threaded I/O, zero-copy networking, and optimized data structures deliver sub-millisecond latencies at scale.",
    code: `# Benchmark: 100 concurrent connections, 1M requests
$ cachestorm-benchmark -c 100 -n 1000000 -t SET,GET

SET: 285,000 ops/sec  (p99: 0.3ms)
GET: 310,000 ops/sec  (p99: 0.2ms)

Memory usage: 45MB for 1M keys
Startup time: <100ms`,
    codeTitle: "benchmark",
    highlights: [
      "285K+ SET operations per second",
      "Sub-millisecond P99 latency",
      "Multi-threaded I/O with goroutine-per-connection",
      "Zero-copy networking for minimal overhead",
    ],
  },
  {
    icon: <Terminal className="w-5 h-5 text-[var(--color-primary)]" />,
    title: "Rich Data Structures",
    description:
      "Strings, hashes, lists, sets, sorted sets, streams, JSON, time series, bitmaps, HyperLogLog, and geospatial indexes. 1,600+ built-in commands cover every use case without external modules.",
    code: `from cachestorm import CacheStorm

cs = CacheStorm(host='localhost', port=6380)

# Session storage
cs.set('session:abc', user_data, ex=3600)

# Real-time leaderboard
cs.zadd('leaderboard', {'alice': 1500, 'bob': 1200})
top_10 = cs.zrange('leaderboard', 0, 9, desc=True)

# Analytics pipeline
cs.xadd('events', {'type': 'pageview', 'url': '/home'})

# Geospatial queries
cs.geoadd('stores', 28.9784, 41.0082, 'istanbul-hq')
nearby = cs.georadius('stores', 29.0, 41.0, 10, unit='km')`,
    codeTitle: "app.py",
    highlights: [
      "1,600+ commands across 12+ data types",
      "JSON documents, time series, graphs built-in",
      "Pub/Sub, Lua scripting, transactions",
      "Works with any RESP-compatible client library",
    ],
    reverse: true,
  },
  {
    icon: <Globe className="w-5 h-5 text-[var(--color-primary)]" />,
    title: "Built-in HTTP API",
    description:
      "CacheStorm ships with a native REST API. Execute commands, manage keys, check health, and pull metrics — all over HTTP with JSON. No sidecars, no proxies.",
    code: `# Health & readiness probes
$ curl http://localhost:8080/api/health
{"status":"ok","uptime":"24h3m"}

# Execute any command over HTTP
$ curl -X POST http://localhost:8080/api/execute \\
  -H "Authorization: Bearer your_token" \\
  -d '{"command":"SET","args":["key","value"]}'
{"result":"OK"}

# Prometheus metrics, built-in
$ curl http://localhost:8080/api/metrics
cachestorm_keys_total 48291
cachestorm_memory_used_bytes 134217728
cachestorm_connected_clients 42`,
    codeTitle: "terminal",
    highlights: [
      "RESTful CRUD for key-value operations",
      "Pipeline endpoint for batching commands",
      "Prometheus metrics at /metrics",
      "Health and readiness probes for Kubernetes",
    ],
  },
  {
    icon: <Shield className="w-5 h-5 text-[var(--color-primary)]" />,
    title: "Enterprise-Grade Security",
    description:
      "Secure your data with TLS encryption, fine-grained ACLs, and password authentication. CacheStorm supports mutual TLS for zero-trust environments and per-user command restrictions.",
    code: `# cachestorm.yaml
security:
  tls:
    enabled: true
    cert_file: "/etc/tls/server.crt"
    key_file: "/etc/tls/server.key"
    ca_file: "/etc/tls/ca.crt"

  acl:
    enabled: true
    file: "/etc/cachestorm/acl.conf"

# acl.conf
user admin on >secret ~* +@all
user app on >app-pwd ~app:* +@read +@write
user reader on >read-pwd ~* +@read`,
    codeTitle: "security config",
    highlights: [
      "TLS 1.2/1.3 with mutual TLS support",
      "Per-user ACLs with command and key restrictions",
      "Runtime ACL management commands",
      "Environment variable based secrets",
    ],
    reverse: true,
  },
  {
    icon: <BarChart3 className="w-5 h-5 text-[var(--color-primary)]" />,
    title: "Native Observability",
    description:
      "CacheStorm includes built-in Prometheus metrics, Go pprof profiling, and comprehensive server statistics. No external exporters needed -- monitoring is a first-class feature.",
    code: `# Prometheus metrics (built-in)
$ curl http://localhost:9121/metrics
cachestorm_uptime_seconds 86400
cachestorm_connected_clients 42
cachestorm_commands_total{cmd="GET"} 1500000
cachestorm_commands_duration_seconds{...}
cachestorm_memory_used_bytes 134217728
cachestorm_keyspace_hits_total 1200000
cachestorm_keyspace_misses_total 50000

# pprof profiling
$ go tool pprof http://localhost:6060/debug/pprof/heap`,
    codeTitle: "metrics",
    highlights: [
      "Built-in Prometheus /metrics endpoint",
      "Command latency histograms with percentiles",
      "Memory, connections, and keyspace statistics",
      "Go pprof for CPU and memory profiling",
    ],
  },
  {
    icon: <Network className="w-5 h-5 text-[var(--color-primary)]" />,
    title: "High Availability & Clustering",
    description:
      "Scale horizontally with cluster mode or vertically with replication. Sentinel provides automatic failover for zero-downtime deployments. Deploy across availability zones for geographic redundancy.",
    code: `# 3-master cluster with replicas
$ cachestorm-cli --cluster create \\
    node1:6380 node2:6380 node3:6380 \\
    node4:6380 node5:6380 node6:6380 \\
    --cluster-replicas 1

>>> Cluster created!
Master 1: slots 0-5460     (node1 -> node4)
Master 2: slots 5461-10922 (node2 -> node5)
Master 3: slots 10923-16383 (node3 -> node6)

# Sentinel auto-failover
$ SENTINEL get-master-addr-by-name primary
["10.0.1.10", "6380"]`,
    codeTitle: "cluster setup",
    highlights: [
      "Master-replica replication with automatic sync",
      "Sentinel mode for automatic failover",
      "Cluster mode with hash-slot sharding",
      "Cross-AZ deployment support",
    ],
    reverse: true,
  },
];

/* ------------------------------------------------------------------ */
/*  Page component                                                    */
/* ------------------------------------------------------------------ */

export default function Features() {
  return (
    <div className="min-h-screen bg-[var(--color-bg)] text-[var(--color-text-secondary)]">
      {/* ── Hero ─────────────────────────────────────────────── */}
      <section>
        <div className="max-w-6xl mx-auto px-6 pt-24 pb-16 text-center">
          <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-[var(--color-surface)] border border-[var(--color-border)] text-[var(--color-primary)] text-sm font-medium mb-6">
            <Zap className="w-4 h-4" />
            Features
          </div>

          <h1 className="text-5xl lg:text-6xl font-extrabold text-[var(--color-text)] tracking-tight mb-6">
            Everything You Need,
            <br />
            Nothing You Don't
          </h1>

          <p className="text-xl text-[var(--color-text-secondary)] max-w-2xl mx-auto leading-relaxed mb-10">
            One binary. 1,600+ commands. Built-in HTTP API, Prometheus metrics, TLS, ACL, clustering,
            persistence, and profiling. No plugins, no sidecars, no extra dependencies.
          </p>

          {/* Quick feature grid */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 max-w-3xl mx-auto">
            {[
              { icon: <Cpu className="w-5 h-5" />, label: "Multi-threaded" },
              { icon: <Lock className="w-5 h-5" />, label: "TLS & ACL" },
              { icon: <Layers className="w-5 h-5" />, label: "All Data Types" },
              { icon: <Radio className="w-5 h-5" />, label: "Pub/Sub & Streams" },
              { icon: <Database className="w-5 h-5" />, label: "Persistence" },
              { icon: <Globe className="w-5 h-5" />, label: "HTTP API" },
              { icon: <Clock className="w-5 h-5" />, label: "Sub-ms Latency" },
              { icon: <Network className="w-5 h-5" />, label: "Clustering" },
            ].map((item) => (
              <div
                key={item.label}
                className="flex items-center gap-2 px-3 py-2.5 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-sm text-[var(--color-text-secondary)]"
              >
                <span className="text-[var(--color-primary)]">{item.icon}</span>
                {item.label}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Feature sections ─────────────────────────────────── */}
      <section className="max-w-6xl mx-auto px-6 py-16 space-y-24">
        {features.map((feature) => (
          <FeatureBlock key={feature.title} feature={feature} />
        ))}
      </section>

      {/* ── Benchmarks ───────────────────────────────────────── */}
      <section className="border-t border-[var(--color-border)]">
        <div className="max-w-4xl mx-auto px-6 py-20">
          <div className="text-center mb-12">
            <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-[var(--color-surface)] border border-[var(--color-border)] text-[var(--color-primary)] text-sm font-medium mb-4">
              <BarChart3 className="w-4 h-4" />
              Performance
            </div>
            <h2 className="text-3xl lg:text-4xl font-extrabold text-[var(--color-text)] tracking-tight mb-4">
              Benchmark Comparison
            </h2>
            <p className="text-[var(--color-text-secondary)] max-w-xl mx-auto">
              Benchmarked on Linux with 100 concurrent connections, 1M requests.
              Higher is better for throughput; lower is better for latency.
            </p>
          </div>

          <div className="space-y-6 p-6 rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)]">
            {benchmarks.map((b) => (
              <BenchmarkBar key={b.label} item={b} />
            ))}
          </div>

          <p className="text-xs text-[var(--color-text-tertiary)] text-center mt-4">
            * Benchmarks run on AWS c6i.xlarge (4 vCPU, 8 GB RAM), Ubuntu 22.04, default configurations.
            Results may vary by workload and hardware.
          </p>
        </div>
      </section>

      {/* ── Comparison table ─────────────────────────────────── */}
      <section className="border-t border-[var(--color-border)]">
        <div className="max-w-4xl mx-auto px-6 py-20">
          <div className="text-center mb-12">
            <h2 className="text-3xl lg:text-4xl font-extrabold text-[var(--color-text)] tracking-tight mb-4">
              Feature Comparison
            </h2>
            <p className="text-[var(--color-text-secondary)] max-w-xl mx-auto">
              See what CacheStorm brings to the table
            </p>
          </div>

          <div className="rounded-2xl border border-[var(--color-border)] overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-[var(--color-border)] text-left">
                    <th className="px-5 py-3 font-medium text-[var(--color-text-secondary)]">Feature</th>
                    <th className="px-5 py-3 font-medium text-[var(--color-primary)] text-center">CacheStorm</th>
                    <th className="px-5 py-3 font-medium text-[var(--color-text-tertiary)] text-center">Redis</th>
                    <th className="px-5 py-3 font-medium text-[var(--color-text-tertiary)]">Note</th>
                  </tr>
                </thead>
                <tbody>
                  {comparisonData.map((row, i) => (
                    <tr
                      key={row.feature}
                      className={cn(
                        i < comparisonData.length - 1 && "border-b border-[var(--color-border)]",
                        i % 2 === 0 && "bg-[var(--color-bg-secondary)]"
                      )}
                    >
                      <td className="px-5 py-3 text-[var(--color-text-secondary)]">{row.feature}</td>
                      <td className="px-5 py-3 text-center">
                        <span className="inline-flex items-center justify-center">
                          <CompareIcon val={row.cachestorm} />
                        </span>
                      </td>
                      <td className="px-5 py-3 text-center">
                        <span className="inline-flex items-center justify-center">
                          <CompareIcon val={row.redis} />
                        </span>
                      </td>
                      <td className="px-5 py-3 text-xs text-[var(--color-text-tertiary)]">{row.note || ""}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </section>

      {/* ── CTA ──────────────────────────────────────────────── */}
      <section className="border-t border-[var(--color-border)]">
        <div className="max-w-3xl mx-auto px-6 py-20 text-center">
          <h2 className="text-3xl font-extrabold text-[var(--color-text)] tracking-tight mb-4">
            Ready to Get Started?
          </h2>
          <p className="text-[var(--color-text-secondary)] mb-8 max-w-xl mx-auto">
            Install CacheStorm in under a minute and start building faster applications.
          </p>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
            <Link
              to="/docs/getting-started"
              className="inline-flex items-center gap-2 px-6 py-3 rounded-xl bg-blue-600 hover:bg-blue-500 text-[var(--color-text)] font-semibold transition-colors"
            >
              Read the Docs
              <ArrowRight className="w-4 h-4" />
            </Link>
            <a
              href="https://github.com/CacheStorm/CacheStorm"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 px-6 py-3 rounded-xl border border-[var(--color-border)] hover:border-[var(--color-border)] text-[var(--color-text-secondary)] font-medium transition-colors"
            >
              View on GitHub
            </a>
          </div>
        </div>
      </section>
    </div>
  );
}
