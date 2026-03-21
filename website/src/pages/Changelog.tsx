import { Tag, Shield, Wrench, Zap, TestTube, Eye } from "lucide-react";

interface Release {
  version: string;
  date: string;
  tag: "latest" | "previous";
  sections: {
    title: string;
    icon: React.ReactNode;
    items: string[];
  }[];
}

const releases: Release[] = [
  {
    version: "0.2.0",
    date: "2026-03-21",
    tag: "latest",
    sections: [
      {
        title: "Security",
        icon: <Shield className="h-4 w-4" />,
        items: [
          "TLS 1.2+ with hardened cipher suites (AEAD-only: AES-GCM, ChaCha20-Poly1305)",
          "CLUSTER MEET IP/port validation to prevent SSRF attacks",
          "Gossip node data validation (IP format, port range)",
          "HTTP API command blacklist (SHUTDOWN, FLUSHALL, DEBUG, CONFIG blocked)",
          "Lua script execution timeout (5 seconds) to prevent DoS",
          "Null byte validation in key names",
          "CONFIG SET value validation (maxmemory, maxclients, eviction policy)",
          "Bitmap offset bounds validation (0 to 2^32-1)",
          "JSON path depth limit (128 levels) to prevent stack overflow",
          "Integer overflow protection in parseInt64",
        ],
      },
      {
        title: "Bug Fixes",
        icon: <Wrench className="h-4 w-4" />,
        items: [
          "Network I/O write errors now checked in gossip, sentinel, replication",
          "Tag broadcast messages actually sent to cluster peers (was silently dropped)",
          "Data race in failover completeFailover() with proper mutex",
          "Data race in warmStatus global with sync.Mutex",
          "HTTPServer.ready changed to atomic.Bool for thread safety",
          "Unsafe type assertions across 8+ command files (panic prevention)",
          "PubSub subscriber leak on connection disconnect",
          "AOF Rewrite file handle leak on Windows (close before rename)",
          "Slot migration completion logic rewritten with proper range merging",
          "Encoding integer overflow in msgpack/cbor for data >255 bytes",
          "SCAN cursor int64 parsing with negative check",
          "GEORADIUS negative COUNT validation",
        ],
      },
      {
        title: "New Features",
        icon: <Zap className="h-4 w-4" />,
        items: [
          "Panic recovery on 6+ unprotected goroutines",
          "Store layer logging (eviction pressure, OOM rejections, flush)",
          "AOF replay error detection and detailed logging",
          "pprof debug endpoints (auth-protected) at /debug/pprof/",
          "HTTP session and rate limiter periodic cleanup goroutines",
          "Resource limits: function registry (1k), event listeners (100/event), circuits (10k)",
          "PubSub per-subscriber channel limit (10k) and channel name validation",
          "KEYS command result cap (100k), XREAD BLOCK bounded (100 retries + deadline)",
          "SETRANGE/APPEND bounds checking against MaxValueSize (512MB)",
          "Memory accounting includes key name size in shard tracking",
          "Config validation for bind address, HTTP/cluster ports, TLS file existence",
          "Prometheus alert rules and Grafana dashboard",
          "SECURITY.md with vulnerability reporting policy",
          "Production config template",
        ],
      },
      {
        title: "Observability",
        icon: <Eye className="h-4 w-4" />,
        items: [
          "Built-in Prometheus metrics endpoint (/api/metrics)",
          "Grafana dashboard with 6 panels (keys, memory, pressure, clients, uptime, status)",
          "Prometheus alert rules (CacheStormDown, HighMemoryPressure, CriticalMemoryPressure)",
          "pprof CPU/memory profiling endpoints",
          "Structured JSON logging with zerolog",
        ],
      },
      {
        title: "Testing",
        icon: <TestTube className="h-4 w-4" />,
        items: [
          "Test coverage: 82% average to 96% average",
          "3 packages at 100%: acl, config, logger",
          "13 packages at 95%+",
          "~3,000+ new test functions added",
          "18/18 packages pass with 100% success rate",
        ],
      },
    ],
  },
  {
    version: "0.1.27",
    date: "2026-02-25",
    tag: "previous",
    sections: [
      {
        title: "Improvements",
        icon: <Zap className="h-4 w-4" />,
        items: [
          "89.1% average test coverage across 18 packages",
          "Default port changed from 6379 to 6380",
          "Improved security, error handling, and test reliability",
          "Resolved multiple critical and high priority bugs",
          "Native Go build matrix (replaced goreleaser)",
        ],
      },
    ],
  },
];

export default function Changelog() {
  return (
    <div className="min-h-screen" style={{ backgroundColor: "var(--color-bg)" }}>
      <div className="mx-auto max-w-3xl px-4 pt-28 pb-20 sm:px-6">
        <div className="mb-12">
          <h1 className="text-4xl font-bold tracking-tight" style={{ color: "var(--color-text)" }}>
            Changelog
          </h1>
          <p className="mt-3 text-lg" style={{ color: "var(--color-text-secondary)" }}>
            All notable changes to CacheStorm.
          </p>
        </div>

        <div className="space-y-16">
          {releases.map((release) => (
            <article key={release.version}>
              <div className="flex items-center gap-3 mb-6">
                <Tag className="h-5 w-5" style={{ color: "var(--color-primary)" }} />
                <h2 className="text-2xl font-bold" style={{ color: "var(--color-text)" }}>
                  v{release.version}
                </h2>
                <span
                  className="text-sm px-2.5 py-0.5 rounded-full border"
                  style={{
                    borderColor: release.tag === "latest" ? "var(--color-primary)" : "var(--color-border)",
                    color: release.tag === "latest" ? "var(--color-primary)" : "var(--color-text-tertiary)",
                  }}
                >
                  {release.tag === "latest" ? "Latest" : release.date}
                </span>
                <span className="text-sm" style={{ color: "var(--color-text-tertiary)" }}>
                  {release.date}
                </span>
              </div>

              <div className="space-y-8 pl-2 border-l-2" style={{ borderColor: "var(--color-border)" }}>
                {release.sections.map((section) => (
                  <div key={section.title} className="pl-6">
                    <h3 className="flex items-center gap-2 text-sm font-semibold uppercase tracking-wider mb-3" style={{ color: "var(--color-text-secondary)" }}>
                      <span style={{ color: "var(--color-primary)" }}>{section.icon}</span>
                      {section.title}
                    </h3>
                    <ul className="space-y-1.5">
                      {section.items.map((item, i) => (
                        <li key={i} className="text-sm leading-relaxed" style={{ color: "var(--color-text-secondary)" }}>
                          <span style={{ color: "var(--color-text-tertiary)" }}>&bull;</span>{" "}
                          {item}
                        </li>
                      ))}
                    </ul>
                  </div>
                ))}
              </div>
            </article>
          ))}
        </div>

        <div className="mt-16 pt-8 border-t text-center" style={{ borderColor: "var(--color-border)" }}>
          <p className="text-sm" style={{ color: "var(--color-text-tertiary)" }}>
            Full changelog available on{" "}
            <a
              href="https://github.com/CacheStorm/CacheStorm/blob/main/CHANGELOG.md"
              target="_blank"
              rel="noopener noreferrer"
              className="underline"
              style={{ color: "var(--color-primary)" }}
            >
              GitHub
            </a>
          </p>
        </div>
      </div>
    </div>
  );
}
