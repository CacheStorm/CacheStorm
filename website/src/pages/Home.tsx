import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  Zap, Shield, Database, BarChart3, Network, Eye,
  ArrowRight, Terminal, Github, Clock, Layers, Lock,
} from "lucide-react";
import { Button } from "../components/ui/button";
import { Card, CardContent } from "../components/ui/card";

const terminalLines = [
  { prompt: "$ ", command: "cachestorm --config cachestorm.yaml", delay: 800 },
  { prompt: "", command: "CacheStorm v0.2.0 started on :6380", delay: 500, isResponse: true },
  { prompt: "", command: "256 shards initialized, ready for connections", delay: 400, isResponse: true },
  { prompt: "", command: "", delay: 300, isResponse: true },
  { prompt: "$ ", command: "cachestorm-cli", delay: 600 },
  { prompt: "cachestorm> ", command: 'SET user:1001 \'{"name":"Alice","role":"admin"}\'', delay: 900 },
  { prompt: "", command: "OK (0.04ms)", delay: 400, isResponse: true },
  { prompt: "cachestorm> ", command: "EXPIRE user:1001 3600", delay: 600 },
  { prompt: "", command: "(integer) 1", delay: 300, isResponse: true },
  { prompt: "cachestorm> ", command: "HSET analytics:daily visitors 48291 pageviews 182440", delay: 800 },
  { prompt: "", command: "(integer) 2", delay: 300, isResponse: true },
];

function AnimatedTerminal() {
  const [visibleLines, setVisibleLines] = useState<Array<{ prompt: string; text: string; isResponse?: boolean }>>([]);
  const [currentLine, setCurrentLine] = useState(0);
  const [currentChar, setCurrentChar] = useState(0);
  const [isTyping, setIsTyping] = useState(true);

  useEffect(() => {
    if (currentLine >= terminalLines.length) {
      const timer = setTimeout(() => {
        setVisibleLines([]);
        setCurrentLine(0);
        setCurrentChar(0);
        setIsTyping(true);
      }, 4000);
      return () => clearTimeout(timer);
    }

    const line = terminalLines[currentLine];

    if (line.isResponse) {
      const timer = setTimeout(() => {
        setVisibleLines((prev) => [...prev, { prompt: line.prompt, text: line.command, isResponse: true }]);
        setCurrentLine((prev) => prev + 1);
        setCurrentChar(0);
      }, line.delay);
      return () => clearTimeout(timer);
    }

    if (currentChar === 0 && isTyping) {
      setVisibleLines((prev) => [...prev, { prompt: line.prompt, text: "" }]);
    }

    if (currentChar < line.command.length) {
      const timer = setTimeout(() => {
        setVisibleLines((prev) => {
          const updated = [...prev];
          updated[updated.length - 1] = { ...updated[updated.length - 1], text: line.command.slice(0, currentChar + 1) };
          return updated;
        });
        setCurrentChar((prev) => prev + 1);
      }, 25 + Math.random() * 35);
      return () => clearTimeout(timer);
    } else {
      const timer = setTimeout(() => {
        setCurrentLine((prev) => prev + 1);
        setCurrentChar(0);
        setIsTyping(true);
      }, line.delay);
      return () => clearTimeout(timer);
    }
  }, [currentLine, currentChar, isTyping]);

  const showCursor = currentLine < terminalLines.length && !terminalLines[currentLine]?.isResponse;

  return (
    <div className="rounded-xl border overflow-hidden" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-terminal-bg)" }}>
      <div className="flex items-center gap-2 border-b px-4 py-3" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-surface)" }}>
        <div className="flex gap-1.5">
          <div className="h-3 w-3 rounded-full bg-red-400" />
          <div className="h-3 w-3 rounded-full bg-yellow-400" />
          <div className="h-3 w-3 rounded-full bg-green-400" />
        </div>
        <span className="ml-2 text-xs font-mono" style={{ color: "var(--color-text-tertiary)" }}>cachestorm</span>
      </div>
      <div className="p-4 font-mono text-sm leading-relaxed h-[320px] overflow-hidden" style={{ backgroundColor: "var(--color-terminal-bg)" }}>
        {visibleLines.map((line, i) => (
          <div key={i} className="flex">
            {line.prompt && <span className="text-green-400 select-none shrink-0">{line.prompt}</span>}
            <span className={line.isResponse ? "text-gray-400" : "text-gray-200"}>{line.text}</span>
            {i === visibleLines.length - 1 && showCursor && <span className="terminal-cursor" />}
          </div>
        ))}
      </div>
    </div>
  );
}

const stats = [
  { value: "20M+", label: "Operations/sec" },
  { value: "<1ms", label: "P99 Latency" },
  { value: "256", label: "Concurrent Shards" },
  { value: "1,600+", label: "Built-in Commands" },
];

const features = [
  {
    icon: Zap,
    title: "Designed for Speed",
    description: "Lock-free 256-shard architecture processes 20M+ operations per second. No GC pauses, no lock contention, just raw throughput.",
  },
  {
    icon: Layers,
    title: "Rich Data Structures",
    description: "Strings, hashes, lists, sets, sorted sets, streams, JSON, time series, graphs, HyperLogLog, bitmaps and more. 1,600+ commands out of the box.",
  },
  {
    icon: Lock,
    title: "Production Security",
    description: "TLS 1.2+ with hardened ciphers, ACL system with per-user permissions, rate limiting, and command-level access control.",
  },
  {
    icon: Database,
    title: "Durable When You Need It",
    description: "AOF with configurable sync policies and point-in-time snapshots. Choose between speed and durability per workload.",
  },
  {
    icon: Network,
    title: "Scale Horizontally",
    description: "Built-in clustering with hash-slot sharding, automatic replication, sentinel monitoring, and zero-downtime failover.",
  },
  {
    icon: Eye,
    title: "Full Observability",
    description: "Prometheus metrics, Grafana dashboards, pprof endpoints, structured JSON logging, and slow query analysis. Know what's happening.",
  },
];

const whyCacheStorm = [
  {
    icon: Clock,
    title: "5 Minutes to Production",
    description: "Single binary, Docker image, or one-line install. No JVM, no external dependencies. Configure with a YAML file and you're live.",
  },
  {
    icon: BarChart3,
    title: "Built-in HTTP API",
    description: "REST API alongside the wire protocol. Manage keys, monitor health, execute commands, and integrate with any stack over HTTP.",
  },
  {
    icon: Shield,
    title: "Hardened by Default",
    description: "Input validation, memory bounds, execution timeouts, panic recovery on every goroutine. Security isn't an afterthought.",
  },
];

const quickStart = `# Install with one command
curl -fsSL https://cachestorm.com/install.sh | bash

# Or build from source
git clone https://github.com/CacheStorm/CacheStorm.git
cd CacheStorm && make build

# Start and connect
./cachestorm --config cachestorm.yaml
cachestorm-cli
cachestorm> SET hello "world"
OK`;

const configExample = `# cachestorm.yaml - that's all you need
server:
  port: 6380
  requirepass: "your_secret"

memory:
  max_memory: "4gb"
  eviction_policy: "allkeys-lru"

persistence:
  enabled: true
  aof: true
  aof_sync: "everysec"

http:
  enabled: true
  port: 8080

logging:
  level: "info"
  format: "json"`;

export function Home() {
  return (
    <div>
      {/* Hero */}
      <section className="pt-32 pb-20 lg:pt-40 lg:pb-28">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="grid items-center gap-12 lg:grid-cols-2 lg:gap-16">
            <div className="max-w-xl">
              <div className="animate-fade-in-up">
                <span
                  className="inline-flex items-center gap-1.5 px-3 py-1 text-xs font-medium rounded-full border"
                  style={{ borderColor: "var(--color-border)", color: "var(--color-text-secondary)" }}
                >
                  <span className="h-1.5 w-1.5 rounded-full bg-green-500" />
                  Open Source
                </span>
              </div>

              <h1
                className="animate-fade-in-up animation-delay-100 mt-6 text-4xl font-bold tracking-tight sm:text-5xl"
                style={{ color: "var(--color-text)" }}
              >
                The In-Memory Data Store Built for What's Next
              </h1>

              <p
                className="animate-fade-in-up animation-delay-200 mt-5 text-lg leading-relaxed"
                style={{ color: "var(--color-text-secondary)" }}
              >
                CacheStorm is a high-performance caching server written in Go.
                256-shard lock-free architecture, 1,600+ commands, enterprise
                security, and full observability — in a single binary.
              </p>

              <div className="animate-fade-in-up animation-delay-300 mt-8 flex flex-wrap gap-3">
                <Link to="/docs/getting-started">
                  <Button size="lg" className="gap-2">
                    Get Started <ArrowRight className="h-4 w-4" />
                  </Button>
                </Link>
                <a href="https://github.com/CacheStorm/CacheStorm" target="_blank" rel="noopener noreferrer">
                  <Button variant="outline" size="lg" className="gap-2">
                    <Github className="h-4 w-4" /> GitHub
                  </Button>
                </a>
              </div>

              <div className="mt-6 flex items-center gap-2 text-sm" style={{ color: "var(--color-text-tertiary)" }}>
                <Terminal className="h-4 w-4" />
                <code className="font-mono" style={{ color: "var(--color-text-secondary)" }}>
                  curl -fsSL https://cachestorm.com/install.sh | bash
                </code>
              </div>
            </div>

            <div className="animate-fade-in-up animation-delay-200 lg:ml-auto w-full max-w-lg">
              <AnimatedTerminal />
            </div>
          </div>
        </div>
      </section>

      {/* Stats */}
      <section className="border-y" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-bg-secondary)" }}>
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="grid grid-cols-2 gap-8 py-10 md:grid-cols-4">
            {stats.map((stat) => (
              <div key={stat.label} className="text-center">
                <div className="text-2xl font-bold sm:text-3xl" style={{ color: "var(--color-text)" }}>{stat.value}</div>
                <div className="mt-1 text-sm" style={{ color: "var(--color-text-secondary)" }}>{stat.label}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="py-24 lg:py-28">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-2xl text-center">
            <h2 className="text-3xl font-bold tracking-tight sm:text-4xl" style={{ color: "var(--color-text)" }}>
              Everything, out of the box
            </h2>
            <p className="mt-4 text-lg" style={{ color: "var(--color-text-secondary)" }}>
              No plugins, no modules, no extra downloads. CacheStorm ships with every feature you need for production.
            </p>
          </div>

          <div className="mt-14 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {features.map((f) => (
              <Card key={f.title} className="p-0">
                <CardContent className="p-6">
                  <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-lg" style={{ backgroundColor: "var(--color-surface)" }}>
                    <f.icon className="h-5 w-5" style={{ color: "var(--color-primary)" }} />
                  </div>
                  <h3 className="text-base font-semibold" style={{ color: "var(--color-text)" }}>{f.title}</h3>
                  <p className="mt-2 text-sm leading-relaxed" style={{ color: "var(--color-text-secondary)" }}>{f.description}</p>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      </section>

      {/* Why CacheStorm */}
      <section className="border-y py-24 lg:py-28" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-bg-secondary)" }}>
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-2xl text-center">
            <h2 className="text-3xl font-bold tracking-tight sm:text-4xl" style={{ color: "var(--color-text)" }}>
              Why teams switch to CacheStorm
            </h2>
            <p className="mt-4 text-lg" style={{ color: "var(--color-text-secondary)" }}>
              We didn't build another cache layer. We built the one we wanted to use.
            </p>
          </div>

          <div className="mt-14 grid gap-8 lg:grid-cols-3">
            {whyCacheStorm.map((item) => (
              <div key={item.title} className="flex gap-4">
                <div className="shrink-0 flex h-10 w-10 items-center justify-center rounded-lg" style={{ backgroundColor: "var(--color-surface)" }}>
                  <item.icon className="h-5 w-5" style={{ color: "var(--color-primary)" }} />
                </div>
                <div>
                  <h3 className="text-base font-semibold" style={{ color: "var(--color-text)" }}>{item.title}</h3>
                  <p className="mt-1.5 text-sm leading-relaxed" style={{ color: "var(--color-text-secondary)" }}>{item.description}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Quick Start + Config */}
      <section className="py-24 lg:py-28">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-2xl text-center">
            <h2 className="text-3xl font-bold tracking-tight sm:text-4xl" style={{ color: "var(--color-text)" }}>
              Up and running in seconds
            </h2>
            <p className="mt-4 text-lg" style={{ color: "var(--color-text-secondary)" }}>
              One command to install. One file to configure. That's it.
            </p>
          </div>

          <div className="mt-14 grid gap-5 lg:grid-cols-2">
            <div className="rounded-xl border overflow-hidden" style={{ borderColor: "var(--color-border)" }}>
              <div className="flex items-center border-b px-4 py-2.5" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-surface)" }}>
                <span className="text-xs font-mono" style={{ color: "var(--color-text-tertiary)" }}>Quick Start</span>
              </div>
              <pre className="p-4 text-sm font-mono leading-relaxed overflow-x-auto" style={{ backgroundColor: "var(--color-code-bg)", color: "var(--color-code-text)" }}>
                <code>{quickStart}</code>
              </pre>
            </div>
            <div className="rounded-xl border overflow-hidden" style={{ borderColor: "var(--color-border)" }}>
              <div className="flex items-center border-b px-4 py-2.5" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-surface)" }}>
                <span className="text-xs font-mono" style={{ color: "var(--color-text-tertiary)" }}>cachestorm.yaml</span>
              </div>
              <pre className="p-4 text-sm font-mono leading-relaxed overflow-x-auto" style={{ backgroundColor: "var(--color-code-bg)", color: "var(--color-code-text)" }}>
                <code>{configExample}</code>
              </pre>
            </div>
          </div>

          <p className="mt-6 text-center text-sm" style={{ color: "var(--color-text-tertiary)" }}>
            Works with any RESP-compatible client library. Existing tools and SDKs just work.
          </p>
        </div>
      </section>

      {/* CTA */}
      <section className="border-t py-24 lg:py-28" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-bg-secondary)" }}>
        <div className="mx-auto max-w-2xl px-4 text-center sm:px-6 lg:px-8">
          <h2 className="text-3xl font-bold tracking-tight sm:text-4xl" style={{ color: "var(--color-text)" }}>
            Your cache should be fast, secure, and simple.
          </h2>
          <p className="mx-auto mt-4 max-w-lg text-lg" style={{ color: "var(--color-text-secondary)" }}>
            Stop fighting your infrastructure. Start shipping.
          </p>
          <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
            <Link to="/docs/getting-started">
              <Button size="lg" className="gap-2">
                Get Started <ArrowRight className="h-4 w-4" />
              </Button>
            </Link>
            <Link to="/docs">
              <Button variant="outline" size="lg">Documentation</Button>
            </Link>
          </div>
          <p className="mt-6 text-sm" style={{ color: "var(--color-text-tertiary)" }}>
            Open source &middot; MIT License &middot; Free forever
          </p>
        </div>
      </section>
    </div>
  );
}
